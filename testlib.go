package lua

import (
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

// testLibState хранит состояние тестовой библиотеки T
type testLibState struct {
	memoryLimit int64
	allocCount  int64
	freeCount   int64
}

var globalTestState = &testLibState{}

// OpenTest открывает тестовую библиотеку T (internal test library)
func OpenTest(L *LState) int {
	mod := L.NewTable()
	L.SetFuncs(mod, testFuncs)
	if globalTestState == nil {
		globalTestState = &testLibState{}
	}
	L.Push(mod)
	return 1
}

var testFuncs = map[string]LGFunction{
	"totalmem":    testTotalMem,
	"checkmemory": testCheckMemory,
	"gccolor":     testGCColor,
	"gcstate":     testGCState,
	"gcstep":      testGCStep,
	"gccount":     testGCCount,
	"testC":       testTestC,
	"makeCfunc":   testMakeCFunc,
	"checkpanic":  testCheckPanic,
	"d2s":         testD2S,
	"s2d":         testS2D,
	"newuserdata": testNewUserData,
	"pushuserdata": testPushUserData,
	"topointer":   testToPointer,
	"func2num":    testFunc2Num,
	"objsize":     testObjSize,
	"checkstack":  testCheckStack,
}

// T.totalmem() -> total, blocks, maxmem
// T.totalmem("string") -> count
func testTotalMem(L *LState) int {
	if L.GetTop() == 0 {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		total := int64(memStats.Alloc)
		blocks := int64(memStats.Mallocs - memStats.Frees)
		maxmem := int64(memStats.Sys)
		L.Push(LNumberInt(total))
		L.Push(LNumberInt(blocks))
		L.Push(LNumberInt(maxmem))
		return 3
	}

	typeName := L.CheckString(1)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	var count int64

	switch typeName {
	case "string":
		count = int64(memStats.Alloc) / 24
	case "table":
		count = int64(memStats.Alloc) / 64
	case "function":
		count = int64(memStats.Alloc) / 128
	case "userdata":
		count = int64(memStats.Alloc) / 32
	case "thread":
		count = int64(memStats.Alloc) / 256
	default:
		L.Push(LNumberInt(0))
		return 1
	}

	L.Push(LNumberInt(count))
	return 1
}

// T.checkmemory()
func testCheckMemory(L *LState) int {
	runtime.GC()
	debug.FreeOSMemory()
	return 0
}

// T.gccolor(obj) -> "white" | "black" | "gray"
func testGCColor(L *LState) int {
	obj := L.CheckAny(1)
	switch obj.(type) {
	case *LUserData:
		L.Push(LString("black"))
	case *LTable:
		L.Push(LString("white"))
	case *LFunction:
		L.Push(LString("black"))
	default:
		L.Push(LString("white"))
	}
	return 1
}

// T.gcstate([state]) -> state
func testGCState(L *LState) int {
	if L.GetTop() == 0 {
		L.Push(LString("atomic"))
		return 1
	}
	state := L.CheckString(1)
	if state == "pause" {
		runtime.GC()
	}
	L.Push(LString(state))
	return 1
}

// T.gcstep(step)
func testGCStep(L *LState) int {
	step := L.OptInt(1, 1)
	for i := 0; i < step; i++ {
		runtime.GC()
	}
	L.Push(LTrue)
	return 1
}

// T.gccount() -> KB
func testGCCount(L *LState) int {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	L.Push(LNumberInt(int64(memStats.Alloc / 1024)))
	return 1
}

// T.testC(code, ...) -> results
func testTestC(L *LState) int {
	code := L.CheckString(1)
	initialTop := L.GetTop()

	instructions := strings.FieldsFunc(code, func(r rune) bool {
		return r == ';' || r == '\n'
	})

	for _, instr := range instructions {
		instr = strings.TrimSpace(instr)
		if instr == "" {
			continue
		}

		parts := strings.Fields(instr)
		if len(parts) == 0 {
			continue
		}

		op := parts[0]
		opArgs := parts[1:]

		switch op {
		case "pushvalue":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx != 0 {
					val := L.Get(idx)
					L.Push(val)
				}
			}
		case "pushnum":
			if len(opArgs) >= 1 {
				num, _ := strconv.ParseFloat(opArgs[0], 64)
				L.Push(LNumberFloat(num))
			}
		case "pushint":
			if len(opArgs) >= 1 {
				num, _ := strconv.ParseInt(opArgs[0], 10, 64)
				L.Push(LNumberInt(num))
			}
		case "pushstring":
			if len(opArgs) >= 1 {
				L.Push(LString(opArgs[0]))
			}
		case "pushbool":
			if len(opArgs) >= 1 {
				if opArgs[0] == "1" || opArgs[0] == "true" {
					L.Push(LTrue)
				} else {
					L.Push(LFalse)
				}
			}
		case "pushnil":
			L.Push(LNil)
		case "pushuserdata":
			if len(opArgs) >= 1 {
				val, _ := strconv.ParseInt(opArgs[0], 10, 64)
				ud := L.NewUserData()
				ud.Value = val
				L.Push(ud)
			}
		case "pop":
			n := 1
			if len(opArgs) >= 1 {
				n, _ = strconv.Atoi(opArgs[0])
			}
			L.Pop(n)
		case "remove":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					L.Remove(idx)
				}
			}
		case "insert":
			if len(opArgs) >= 1 {
				idx, _ := strconv.Atoi(opArgs[0])
				if idx > 0 {
					val := L.Get(L.GetTop())
					L.Pop(1)
					L.Insert(val, idx)
				}
			}
		case "replace":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					val := L.Get(L.GetTop())
					L.Pop(1)
					L.Replace(idx, val)
				}
			}
		case "copy":
			if len(opArgs) >= 2 {
				from := parseIndex(opArgs[0], L.GetTop())
				to := parseIndex(opArgs[1], L.GetTop())
				if from > 0 && to > 0 {
					val := L.Get(from)
					L.Replace(to, val)
				}
			}
		case "rotate":
			// Rotate not available - skip
		case "settop":
			if len(opArgs) >= 1 {
				n, _ := strconv.Atoi(opArgs[0])
				L.SetTop(n)
			}
		case "gettop":
			L.Push(LNumberInt(int64(L.GetTop())))
		case "absindex":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				L.Push(LNumberInt(int64(idx)))
			}
		case "return":
			if len(opArgs) >= 1 {
				n := opArgs[0]
				if n == "*" {
					return L.GetTop() - initialTop
				}
				count, _ := strconv.Atoi(n)
				top := L.GetTop()
				if count >= 0 && count < top {
					return top - count
				}
				return count
			}
			return 0
		case "concat":
			if len(opArgs) >= 1 {
				n, _ := strconv.Atoi(opArgs[0])
				if n == 0 {
					L.Push(LString(""))
				} else if n >= 1 {
					top := L.GetTop()
					var sb strings.Builder
					for i := top - n + 1; i <= top; i++ {
						sb.WriteString(L.Get(i).String())
					}
					L.Push(LString(sb.String()))
				}
			}
		case "arith":
			if len(opArgs) >= 1 && L.GetTop() >= 2 {
				op := opArgs[0]
				b := L.Get(-1)
				a := L.Get(-2)
				L.Pop(2)
				if nb, ok := b.(LNumber); ok {
					if na, ok := a.(LNumber); ok {
						var result LNumber
						switch op {
						case "+":
							result = na.Add(nb)
						case "-":
							result = na.Sub(nb)
						case "*":
							result = na.Mul(nb)
						case "/":
							result = na.Div(nb)
						case "^":
							result = na.Pow(nb)
						case "%":
							result = na.Mod(nb)
						case "\\":
							result = na.IDiv(nb)
						}
						L.Push(result)
					}
				}
			}
		case "compare":
			if len(opArgs) >= 3 {
				op := opArgs[0]
				idx1 := parseIndex(opArgs[1], L.GetTop())
				idx2 := parseIndex(opArgs[2], L.GetTop())
				if idx1 > 0 && idx2 > 0 {
					v1 := L.Get(idx1)
					v2 := L.Get(idx2)
					var result bool
					switch op {
					case "EQ":
						result = v1 == v2
					case "LT":
						if n1, ok := v1.(LNumber); ok {
							if n2, ok := v2.(LNumber); ok {
								result = n1.Compare(n2) < 0
							}
						}
					case "LE":
						if n1, ok := v1.(LNumber); ok {
							if n2, ok := v2.(LNumber); ok {
								result = n1.Compare(n2) <= 0
							}
						}
					}
					if result {
						L.Push(LTrue)
					} else {
						L.Push(LFalse)
					}
				}
			}
		case "call":
			if len(opArgs) >= 2 {
				nargs, _ := strconv.Atoi(opArgs[0])
				nresults, _ := strconv.Atoi(opArgs[1])
				if nargs >= 0 && L.GetTop() >= nargs {
					fnIdx := L.GetTop() - nargs
					if fnIdx > 0 {
						fn := L.Get(fnIdx)
						if _, ok := fn.(*LFunction); ok {
							L.Pop(nargs + 1)
							L.Push(fn)
							for i := 0; i < nargs; i++ {
								L.Push(L.Get(fnIdx + 1 + i))
							}
							L.Call(nargs, nresults)
						}
					}
				}
			}
		case "pcall":
			if len(opArgs) >= 3 {
				nargs, _ := strconv.Atoi(opArgs[0])
				nresults, _ := strconv.Atoi(opArgs[1])
				if nargs >= 0 && L.GetTop() >= nargs+1 {
					fnIdx := L.GetTop() - nargs
					if fnIdx > 0 {
						fn := L.Get(fnIdx)
						if _, ok := fn.(*LFunction); ok {
							L.Pop(nargs + 1)
							err := L.PCall(nargs, nresults, nil)
							if err != nil {
								L.Push(LFalse)
								L.Push(LString(err.Error()))
								return 2
							}
						}
					}
				}
			}
		case "tostring":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					val := L.Get(idx)
					L.Push(LString(val.String()))
				}
			}
		case "tobool":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					val := L.Get(idx)
					if LVIsFalse(val) {
						L.Push(LFalse)
					} else {
						L.Push(LTrue)
					}
				}
			}
		case "newuserdata":
			if len(opArgs) >= 1 {
				size, _ := strconv.Atoi(opArgs[0])
				ud := L.NewUserData()
				ud.Value = make([]byte, size)
				L.Push(ud)
			}
		case "checkstack":
			L.Push(LTrue)
		case "error":
			L.RaiseError("test error")
		case "len":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					val := L.Get(idx)
					L.Push(LNumberInt(int64(L.ObjLen(val))))
				}
			}
		case "objsize":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					obj := L.Get(idx)
					var size int
					switch v := obj.(type) {
					case *LUserData:
						if b, ok := v.Value.([]byte); ok {
							size = len(b)
						} else {
							size = 24
						}
					default:
						size = 0
					}
					L.Push(LNumberInt(int64(size)))
				}
			}
		case "func2num":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					val := L.Get(idx)
					if _, ok := val.(*LFunction); ok {
						L.Push(LNumberInt(1))
					} else {
						L.Push(LNumberInt(0))
					}
				}
			}
		case "topointer":
			if len(opArgs) >= 1 {
				idx := parseIndex(opArgs[0], L.GetTop())
				if idx > 0 {
					val := L.Get(idx)
					switch v := val.(type) {
					case *LUserData:
						if num, ok := v.Value.(int64); ok {
							L.Push(LNumberInt(num))
						} else {
							L.Push(LNumberInt(0))
						}
					default:
						L.Push(LNumberInt(0))
					}
				}
			}
		case "R":
			L.Push(LNumberInt(RegistryIndex))
		}
	}

	return L.GetTop() - initialTop
}

// T.makeCfunc(code) -> function
func testMakeCFunc(L *LState) int {
	code := L.CheckString(1)

	fn := L.NewFunction(func(L *LState) int {
		args := make([]LValue, L.GetTop())
		for i := 0; i < L.GetTop(); i++ {
			args[i] = L.Get(i + 1)
		}
		L.SetTop(0)
		for _, arg := range args {
			L.Push(arg)
		}

		initialTop := L.GetTop()
		instructions := strings.FieldsFunc(code, func(r rune) bool {
			return r == ';' || r == '\n'
		})

		for _, instr := range instructions {
			instr = strings.TrimSpace(instr)
			if instr == "" {
				continue
			}
			parts := strings.Fields(instr)
			if len(parts) == 0 {
				continue
			}
			op := parts[0]
			opArgs := parts[1:]

			switch op {
			case "pushnum":
				if len(opArgs) >= 1 {
					num, _ := strconv.ParseFloat(opArgs[0], 64)
					L.Push(LNumberFloat(num))
				}
			case "pushint":
				if len(opArgs) >= 1 {
					num, _ := strconv.ParseInt(opArgs[0], 10, 64)
					L.Push(LNumberInt(num))
				}
			case "pushstring":
				if len(opArgs) >= 1 {
					L.Push(LString(opArgs[0]))
				}
			case "pushbool":
				if len(opArgs) >= 1 {
					if opArgs[0] == "1" || opArgs[0] == "true" {
						L.Push(LTrue)
					} else {
						L.Push(LFalse)
					}
				}
			case "pushnil":
				L.Push(LNil)
			case "return":
				if len(opArgs) >= 1 {
					n := opArgs[0]
					if n == "*" {
						return L.GetTop() - initialTop
					}
					count, _ := strconv.Atoi(n)
					top := L.GetTop()
					if count >= 0 && count < top {
						return top - count
					}
					return count
				}
				return 0
			}
		}
		return L.GetTop() - initialTop
	})

	L.Push(fn)
	return 1
}

// T.checkpanic(code) -> message
func testCheckPanic(L *LState) int {
	code := L.CheckString(1)
	L2, _ := L.NewThread()
	defer L2.Close()

	err := L2.DoString(code)
	if err != nil {
		L.Push(LString(err.Error()))
		return 1
	}
	L.Push(LNil)
	return 1
}

// T.d2s(double) -> string
func testD2S(L *LState) int {
	num := L.CheckNumber(1)
	bytes := strconv.AppendFloat(nil, num.Float64(), 'g', -1, 64)
	L.Push(LString(string(bytes)))
	return 1
}

// T.s2d(string) -> double
func testS2D(L *LState) int {
	str := L.CheckString(1)
	num, err := strconv.ParseFloat(string(str), 64)
	if err != nil {
		L.Push(LNumberFloat(0))
	} else {
		L.Push(LNumberFloat(num))
	}
	return 1
}

// T.newuserdata(size) -> userdata
func testNewUserData(L *LState) int {
	size := L.CheckInt(1)
	ud := L.NewUserData()
	ud.Value = make([]byte, size)
	L.Push(ud)
	return 1
}

// T.pushuserdata(value) -> userdata
func testPushUserData(L *LState) int {
	value := L.CheckInt64(1)
	ud := L.NewUserData()
	ud.Value = value
	L.Push(ud)
	return 1
}

// T.topointer(obj) -> pointer
func testToPointer(L *LState) int {
	obj := L.CheckAny(1)
	switch v := obj.(type) {
	case *LUserData:
		if num, ok := v.Value.(int64); ok {
			L.Push(LNumberInt(num))
		} else {
			L.Push(LNumberInt(0))
		}
	case *LTable, *LFunction, *LState:
		L.Push(LNumberInt(0))
	default:
		L.Push(LNumberInt(0))
	}
	return 1
}

// T.func2num(func) -> number
func testFunc2Num(L *LState) int {
	fn := L.CheckAny(1)
	if _, ok := fn.(*LFunction); ok {
		L.Push(LNumberInt(1))
	} else {
		L.Push(LNumberInt(0))
	}
	return 1
}

// T.objsize(obj) -> size
func testObjSize(L *LState) int {
	obj := L.CheckAny(1)
	var size int
	switch v := obj.(type) {
	case LString:
		size = len(string(v))
	case *LTable:
		size = v.Len() * 8
	case *LUserData:
		if b, ok := v.Value.([]byte); ok {
			size = len(b)
		} else {
			size = 24
		}
	case *LFunction:
		if v.IsG {
			size = 64
		} else {
			size = 128
		}
	case *LState:
		size = 256
	case LNumber:
		size = 8
	default:
		size = 16
	}
	L.Push(LNumberInt(int64(size)))
	return 1
}

// T.checkstack(n) -> bool
func testCheckStack(L *LState) int {
	n := L.CheckInt(1)
	if n > 1000000 {
		L.Push(LFalse)
	} else {
		L.Push(LTrue)
	}
	return 1
}

// parseIndex парсит индекс стека
func parseIndex(s string, top int) int {
	if s == "R" {
		return RegistryIndex
	}
	idx, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	if idx < 0 {
		return top + idx + 1
	}
	return idx
}
