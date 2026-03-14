package lua

import (
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
	sorter := lValueArraySorter{L, nil, tbl.array}
	if L.GetTop() != 1 {
		sorter.Fn = L.CheckFunction(2)
	}
	L.nCcalls++
	sort.Sort(sorter)
	L.nCcalls--
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
	i := L.OptInt(3, 1)
	j := L.OptInt(4, tbl.Len())
	if L.GetTop() == 3 {
		if i > tbl.Len() || i < 1 {
			L.Push(emptyLString)
			return 1
		}
	}
	i = intMax(intMin(i, tbl.Len()), 1)
	j = intMin(intMin(j, tbl.Len()), tbl.Len())
	if i > j {
		L.Push(emptyLString)
		return 1
	}
	//TODO should flushing?
	retbottom := L.GetTop()
	for ; i <= j; i++ {
		v := tbl.RawGetInt(i)
		if !LVCanConvToString(v) {
			L.RaiseError("invalid value (%s) at index %d in table for concat", v.Type().String(), i)
		}
		L.Push(v)
		if i != j {
			L.Push(sep)
		}
	}
	L.Push(stringConcat(L, L.GetTop()-retbottom, L.reg.Top()-1))
	return 1
}

func tableInsert(L *LState) int {
	tbl := L.CheckTable(1)
	nargs := L.GetTop()
	if nargs == 1 {
		L.RaiseError("wrong number of arguments")
	}

	if L.GetTop() == 2 {
		tbl.Append(L.Get(2))
		return 0
	}
	pos := L.CheckInt(2)
	// Lua 5.3: позиция должна быть в диапазоне [1, #tbl+1]
	// Используем ObjLen для поддержки __len metamethod
	// Важно: передаём LValue, а не *LTable, чтобы вызвать __len
	len := L.ObjLen(L.Get(1))
	if pos < 1 || pos > len+1 {
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
	f := L.CheckInt(2)
	e := L.CheckInt(3)
	t := L.CheckInt(4)

	var a2 *LTable
	if L.GetTop() >= 5 {
		a2 = L.CheckTable(5)
	} else {
		a2 = a1
	}

	// Convert to 0-based indexing for internal operations
	f0 := f - 1
	e0 := e - 1
	t0 := t - 1

	if f0 > e0 {
		L.Push(a2)
		return 1
	}

	// Determine direction of copy to handle overlapping ranges
	if a1 == a2 && t0 > f0 {
		// Copy backwards to avoid overwriting elements that haven't been copied yet
		for i := e0; i >= f0; i-- {
			val := a1.RawGetInt(i + 1) // RawGetInt uses 1-based indexing
			a2.RawSetInt(t0+(i-f0)+1, val)
		}
	} else {
		// Copy forwards
		for i := f0; i <= e0; i++ {
			val := a1.RawGetInt(i + 1) // RawGetInt uses 1-based indexing
			a2.RawSetInt(t0+(i-f0)+1, val)
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
	if i > j {
		return 0
	}
	for idx := i; idx <= j; idx++ {
		L.Push(tbl.RawGetInt(idx))
	}
	return j - i + 1
}

//
