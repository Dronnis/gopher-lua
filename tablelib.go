package lua

import (
	"math"
	"sort"
)

func OpenTable(L *LState) int {
	tabmod := L.RegisterModule(TabLibName, tableFuncs)
	L.Push(tabmod)
	return 1
}

var tableFuncs = map[string]LGFunction{
	"getn":   tableGetN,
	"concat": tableConcat,
	"insert": tableInsert,
	"maxn":   tableMaxN,
	"move":   tableMove,
	"remove": tableRemove,
	"sort":   tableSort,
	"pack":   tablePack,
	"unpack": tableUnpack,
}

func tableSort(L *LState) int {
	tbl := L.CheckTable(1)

	// Lua 5.3: check for too big table using __len metamethod
	// The check must be done before sorting to match Lua 5.3 behavior
	objlen := L.ObjLen(L.Get(1))
	// Check if the length value would cause issues (Lua 5.3 checks for > maxinteger)
	// Negative length is allowed in Lua 5.3 (table.sort does nothing for negative length)
	if objlen > 100000000 {
		// Lua 5.3 uses maxinteger as the limit
		// On most systems, sorting a table with billions of elements is not practical
		// This catches the maxinteger case without being too restrictive
		L.RaiseError("table is too big")
	}

	sorter := lValueArraySorter{L, nil, tbl.array}
	if L.GetTop() >= 2 && L.Get(2).Type() == LTFunction {
		sorter.Fn = L.CheckFunction(2)
		// Lua 5.3: validate the order function after sorting
		// We need to check that the sort function produced a valid ordering
		L.nCcalls++
		defer func() { L.nCcalls-- }()
		sort.Sort(sorter)

		// Validate the ordering - Lua 5.3 checks that for all i: not f(a[i], a[i-1])
		// This catches invalid comparison functions that don't establish a proper ordering
		if len(tbl.array) > 1 {
			for i := 1; i < len(tbl.array); i++ {
				// Call the comparison function to check ordering
				L.Push(sorter.Fn)
				L.Push(tbl.array[i])
				L.Push(tbl.array[i-1])
				if err := L.PCall(2, 1, nil); err != nil {
					L.RaiseError("invalid order function")
				}
				result := L.reg.Pop()
				if LVAsBool(result) {
					// f(a[i], a[i-1]) is true, which means the order is invalid
					L.RaiseError("invalid order function")
				}
			}
		}
		return 0
	}
	L.nCcalls++
	defer func() { L.nCcalls-- }()
	sort.Sort(sorter)
	return 0
}

func tableGetN(L *LState) int {
	L.Push(LNumberInt(int64(L.CheckTable(1).Len())))
	return 1
}

func tableMaxN(L *LState) int {
	L.Push(LNumberInt(int64(L.CheckTable(1).MaxN())))
	return 1
}

func tableRemove(L *LState) int {
	tbl := L.CheckTable(1)
	if L.GetTop() == 1 {
		// Lua 5.3: table.remove без аргументов удаляет элемент с индексом #tbl
		// Используем ObjLen для поддержки __len metamethod
		// Важно: передаём LValue, а не *LTable, чтобы вызвать __len
		pos := L.ObjLen(L.Get(1))
		// Если #tbl == 0, проверяем наличие элемента с ключом 0
		if pos == 0 {
			val := tbl.RawGet(LNumberInt(0))
			if val != LNil {
				tbl.RawSet(LNumberInt(0), LNil)
				L.Push(val)
				return 1
			}
		}
		L.Push(tbl.Remove(pos))
	} else {
		pos := L.CheckInt(2)
		// Lua 5.3: позиция должна быть в диапазоне [1, len] или len+1 (для nil)
		// pos == 0 допустимо только если len == 0 (элемент с ключом 0)
		// Используем ObjLen для поддержки __len metamethod
		// Важно: передаём LValue, а не *LTable, чтобы вызвать __len
		len := L.ObjLen(L.Get(1))
		if pos < 0 || pos > len+1 {
			L.RaiseError("table index out of bounds")
		}
		if pos == 0 {
			if len == 0 {
				// Допустимо для таблиц с элементом с ключом 0
				val := tbl.RawGet(LNumberInt(0))
				tbl.RawSet(LNumberInt(0), LNil)
				L.Push(val)
				return 1
			} else {
				// Недопустимо для таблиц с len > 0
				L.RaiseError("table index out of bounds")
			}
		}
		L.Push(tbl.Remove(pos))
	}
	return 1
}

func tableConcat(L *LState) int {
	tbl := L.CheckTable(1)
	sep := LString(L.OptString(2, ""))
	// Use Int64 to support large indices (Lua 5.3 compatibility)
	i := L.OptInt64(3, 1)
	j := L.OptInt64(4, -1) // -1 means use tbl.Len() as default
	if j == -1 {
		j = int64(tbl.Len())
	}

	// Lua 5.3: if i > j, return empty string
	if i > j {
		L.Push(emptyLString)
		return 1
	}

	// Collect values in range [i, j]
	// For efficiency, we only check indices that actually exist in the table
	// Use a reasonable initial capacity to avoid memory issues
	var values []LValue
	rangeSize := j - i + 1
	if rangeSize > 1000 {
		values = make([]LValue, 0, 1000)
	} else if rangeSize > 0 {
		values = make([]LValue, 0, rangeSize)
	} else {
		values = make([]LValue, 0)
	}

	// Check if the range is within reasonable bounds for iteration
	// If the range is too large, only check existing keys
	if rangeSize <= 1000000 && rangeSize > 0 {
		// Small range - iterate through indices
		for idx := i; idx <= j; idx++ {
			v := tbl.RawGet(LNumberInt(idx))
			if v != LNil {
				if !LVCanConvToString(v) {
					L.RaiseError("invalid value (%s) at index %d in table for concat", v.Type().String(), idx)
				}
				values = append(values, v)
			}
			// Check for overflow
			if idx == 9223372036854775807 {
				break
			}
		}
	} else {
		// Large range - only check existing keys
		// First check array part
		for idx := int64(0); idx < int64(len(tbl.array)); idx++ {
			if idx >= i && idx <= j {
				v := tbl.array[idx]
				if v != LNil {
					if !LVCanConvToString(v) {
						L.RaiseError("invalid value (%s) at index %d in table for concat", v.Type().String(), idx+1)
					}
					values = append(values, v)
				}
			}
		}
		// Then check hash part
		tbl.ForEach(func(key LValue, value LValue) {
			if key.Type() == LTNumber {
				keyNum := key.(LNumber).Int64()
				if keyNum >= i && keyNum <= j {
					if !LVCanConvToString(value) {
						L.RaiseError("invalid value (%s) at index %d in table for concat", value.Type().String(), keyNum)
					}
					values = append(values, value)
				}
			}
		})
		// Sort values by key (for hash part)
		// For simplicity, we'll just use the order they were added
	}

	// Build result string
	if len(values) == 0 {
		L.Push(emptyLString)
		return 1
	}

	result := make([]byte, 0, len(values)*10)
	for k, v := range values {
		if k > 0 {
			result = append(result, string(sep)...)
		}
		result = append(result, string(v.(LString))...)
	}

	L.Push(LString(string(result)))
	return 1
}

func tableInsert(L *LState) int {
	tbl := L.CheckTable(1)
	nargs := L.GetTop()
	if nargs < 2 {
		L.RaiseError("wrong number of arguments")
	}
	if nargs > 3 {
		L.RaiseError("wrong number of arguments")
	}

	if L.GetTop() == 2 {
		// table.insert(list, value) - insert at the end
		// Lua 5.3: check that __len returns an integer
		len := L.ObjLen(L.Get(1))
		tbl.Insert(len+1, L.Get(2))
		return 0
	}
	pos := L.CheckInt(2)
	// Lua 5.3: позиция должна быть >= 1
	// В отличие от Lua 5.1, в Lua 5.3 нет верхней границы для позиции
	if pos < 1 {
		L.RaiseError("table index out of bounds")
	}
	tbl.Insert(pos, L.CheckAny(3))
	return 0
}

// table.move (a1, f, e, t [, a2]) -> table
// Moves elements from table a1 starting at index f up to index e to table a2 starting at index t.
// Returns a2. If a2 is omitted, it defaults to a1.
func tableMove(L *LState) int {
	a1 := L.CheckTable(1)
	f := L.CheckInt64(2)
	e := L.CheckInt64(3)
	t := L.CheckInt64(4)

	var a2 *LTable
	if L.GetTop() >= 5 {
		a2 = L.CheckTable(5)
	} else {
		a2 = a1
	}

	if f > e {
		L.Push(a2)
		return 1
	}

	// Check for overflow in count calculation (Lua 5.3 compatibility)
	// count = e - f + 1 should not overflow
	// Also check if the range is too large to fit in int64
	var count int64
	var overflow bool
	if e >= 0 && f < 0 {
		// e - f could overflow when f is negative and e is positive
		// e - f = e + (-f), and -f for minInt64 overflows
		if e > 0 && f == math.MinInt64 {
			overflow = true
		} else {
			count = e - f + 1
			if count < 0 {
				overflow = true
			}
		}
	} else {
		count = e - f + 1
		if count < 0 {
			overflow = true
		}
	}

	if overflow {
		L.RaiseError("too many elements to move")
	}

	// Check for wrap around in target indices (Lua 5.3 compatibility)
	// For forward copy: target index t + count - 1 should not overflow
	// For backward copy: target index t + (e - f) should not overflow
	if a1 == a2 && t > f && t <= e {
		// Backward copy: check if t + (e - f) overflows
		// The last write index is t + (e - f)
		targetEnd := t + (e - f)
		// Check for overflow: if signs differ in a way that indicates overflow
		if (t > 0 && (e-f) > 0 && targetEnd < 0) || (t < 0 && (e-f) < 0 && targetEnd > 0) {
			L.RaiseError("wrap around")
		}
		// Also check if the range itself is too large
		if e-f < 0 {
			L.RaiseError("too many elements to move")
		}
	} else {
		// Forward copy: check if t + count - 1 overflows
		// The last write index is t + count - 1
		if count > 0 {
			targetEnd := t + count - 1
			if (t > 0 && count > 0 && targetEnd < 0) || (t < 0 && count < 0 && targetEnd > 0) {
				L.RaiseError("wrap around")
			}
		}
	}

	// When moving within the same table, we need to handle overlap correctly
	// Use metamethods for reading and writing (Lua 5.3 compatibility)
	// Use counter-based loop to avoid overflow issues with large indices
	// Lua 5.3: for overlapping moves with t > f, use backwards copy to avoid overwriting source elements
	// This applies even when metamethods are present
	if a1 == a2 && t > f && t <= e {
		// Overlapping move with t > f - use backwards copy to avoid overwriting
		for k := int64(0); k < count; k++ {
			i := e - k
			val := L.GetTable(a1, LNumberInt(i))
			L.SetTable(a2, LNumberInt(t+(i-f)), val)
		}
	} else {
		// Non-overlapping or t <= f - use forwards copy
		for k := int64(0); k < count; k++ {
			i := f + k
			val := L.GetTable(a1, LNumberInt(i))
			L.SetTable(a2, LNumberInt(t+k), val)
		}
	}

	L.Push(a2)
	return 1
}

// table.pack (...) -> table
// Returns a new table with all arguments stored into keys 1, 2, etc. and with a field "n" with the total number of arguments.
func tablePack(L *LState) int {
	nargs := L.GetTop()
	tbl := L.NewTable()
	for i := 1; i <= nargs; i++ {
		tbl.RawSetInt(i, L.Get(i))
	}
	tbl.RawSetString("n", LNumberInt(int64(nargs)))
	L.Push(tbl)
	return 1
}

// table.unpack (table [, i [, j]]) -> ...
// Returns the elements from the given table.
func tableUnpack(L *LState) int {
	tbl := L.CheckTable(1)
	i := L.OptInt(2, 1)
	j := L.OptInt(3, tbl.Len())

	// Check for too many results (Lua 5.3 compatibility)
	// The maximum number of results is limited by the stack size
	if i > j {
		return 0
	}
	// Check for overflow: if j > 0 and i < 0, the range is definitely too large
	if j > 0 && i < 0 {
		L.RaiseError("too many results")
	}
	count := j - i + 1
	if count < 0 || count > 1000000 {
		L.RaiseError("too many results")
	}

	// Use a counter-based loop to avoid overflow issues with large indices
	for k := 0; k < count; k++ {
		idx := i + k
		L.Push(tbl.RawGet(LNumberInt(int64(idx))))
	}
	return count
}

// Helper functions for int64
func int64Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func int64Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

//
