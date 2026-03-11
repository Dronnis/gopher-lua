package lua

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

/* basic functions {{{ */

func OpenBase(L *LState) int {
	global := L.Get(GlobalsIndex).(*LTable)
	L.SetGlobal("_G", global)
	L.SetGlobal("_VERSION", LString(LuaVersion))
	L.SetGlobal("_GOPHER_LUA_VERSION", LString(PackageName+" "+PackageVersion))
	basemod := L.RegisterModule("_G", baseFuncs)
	global.RawSetString("ipairs", L.NewClosure(baseIpairs, L.NewFunction(ipairsaux)))
	global.RawSetString("pairs", L.NewClosure(basePairs, L.NewFunction(pairsaux)))
	L.Push(basemod)
	return 1
}

var baseFuncs = map[string]LGFunction{
	"assert":         baseAssert,
	"collectgarbage": baseCollectGarbage,
	"dofile":         baseDoFile,
	"error":          baseError,
	"getmetatable":   baseGetMetatable,
	"load":           baseLoad,
	"loadfile":       baseLoadFile,
	"loadstring":     baseLoadString,
	"next":           baseNext,
	"pcall":          basePCall,
	"print":          basePrint,
	"rawequal":       baseRawEqual,
	"rawget":         baseRawGet,
	"rawlen":         baseRawLen,
	"rawset":         baseRawSet,
	"select":         baseSelect,
	"_printregs":     base_PrintRegs,
	"setmetatable":   baseSetMetatable,
	"tonumber":       baseToNumber,
	"tostring":       baseToString,
	"type":           baseType,
	"unpack":         baseUnpack,
	"xpcall":         baseXPCall,
	// loadlib
	"require": loRequire,
	// hidden features
	"newproxy": baseNewProxy,
}

func baseAssert(L *LState) int {
	if !L.ToBool(1) {
		L.RaiseError(L.OptString(2, "assertion failed!"))
		return 0
	}
	return L.GetTop()
}

func baseCollectGarbage(L *LState) int {
	runtime.GC()
	return 0
}

func baseDoFile(L *LState) int {
	src := L.ToString(1)
	top := L.GetTop()
	fn, err := L.LoadFile(src)
	if err != nil {
		L.Push(LString(err.Error()))
		L.Panic(L)
	}
	L.Push(fn)
	L.Call(0, MultRet)
	return L.GetTop() - top
}

func baseError(L *LState) int {
	obj := L.CheckAny(1)
	level := L.OptInt(2, 1)
	L.Error(obj, level)
	return 0
}

func baseGetMetatable(L *LState) int {
	L.Push(L.GetMetatable(L.CheckAny(1)))
	return 1
}

func ipairsaux(L *LState) int {
	tb := L.CheckTable(1)
	i := L.CheckInt(2)
	i++
	v := tb.RawGetInt(i)
	if v == LNil {
		return 0
	} else {
		L.Pop(1)
		L.Push(LNumberInt(int64(i)))
		L.Push(LNumberInt(int64(i)))
		L.Push(v)
		return 2
	}
}

func baseIpairs(L *LState) int {
	tb := L.CheckTable(1)
	L.Push(L.Get(UpvalueIndex(1)))
	L.Push(tb)
	L.Push(LNumberInt(0))
	return 3
}

func loadaux(L *LState, reader io.Reader, chunkname string, env *LTable) int {
	if fn, err := L.Load(reader, chunkname); err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		// Set the environment for the loaded function
		if env != nil {
			fn.Env = env
		}
		L.Push(fn)
		return 1
	}
}

func baseLoad(L *LState) int {
	// Lua 5.3 compatibility: load(chunk [, chunkname [, mode [, env]]])
	// chunk can be a string or a function
	chunk := L.Get(1)
	var reader io.Reader
	var chunkData string

	switch c := chunk.(type) {
	case LString:
		// If chunk is a string, use it directly
		chunkData = string(c)
		reader = strings.NewReader(chunkData)
	case *LFunction:
		// If chunk is a function, read from it
		chunkname := L.OptString(2, "?")
		top := L.GetTop()
		buf := []string{}
		for {
			L.SetTop(top)
			L.Push(c)
			L.Call(0, 1)
			ret := L.reg.Pop()
			if ret == LNil {
				break
			} else if LVCanConvToString(ret) {
				str := ret.String()
				if len(str) > 0 {
					buf = append(buf, string(str))
				} else {
					break
				}
			} else {
				L.Push(LNil)
				L.Push(LString("reader function must return a string"))
				return 2
			}
		}
		chunkData = strings.Join(buf, "")
		// Get environment (4th argument)
		var env *LTable
		if L.GetTop() >= 4 {
			if lv, ok := L.Get(4).(*LTable); ok {
				env = lv
			}
		}
		return loadaux(L, strings.NewReader(chunkData), chunkname, env)
	default:
		L.Push(LNil)
		L.Push(LString("bad argument #1 to load (function or string expected, got " + chunk.Type().String() + ")"))
		return 2
	}

	chunkname := L.OptString(2, "<load>")
	mode := L.OptString(3, "bt")  // Default mode is "bt" (both text and binary)

	// Get environment (4th argument)
	var env *LTable
	if L.GetTop() >= 4 {
		if lv, ok := L.Get(4).(*LTable); ok {
			env = lv
		}
	}

	// Check mode compatibility (Lua 5.3)
	// Binary chunks start with the Lua signature: 0x1B 0x4C 0x75 0x61 (ESC Lua)
	// We need to check the raw bytes, not the string representation
	isBinary := false
	if len(chunkData) >= 4 {
		// Check for Lua binary signature: ESC + "Lua"
		if chunkData[0] == 0x1B && chunkData[1] == 'L' && chunkData[2] == 'u' && chunkData[3] == 'a' {
			isBinary = true
		}
	}

	if mode == "b" && !isBinary {
		L.Push(LNil)
		L.Push(LString("attempt to load a text chunk"))
		return 2
	}
	if mode == "t" && isBinary {
		L.Push(LNil)
		L.Push(LString("attempt to load a binary chunk"))
		return 2
	}

	return loadaux(L, reader, chunkname, env)
}

func baseLoadFile(L *LState) int {
	var reader io.Reader
	var chunkname string
	var err error
	// Lua 5.3: loadfile([filename [, mode [, env]]])
	if L.GetTop() < 1 {
		reader = os.Stdin
		chunkname = "<stdin>"
	} else {
		chunkname = L.CheckString(1)
		reader, err = os.Open(chunkname)
		if err != nil {
			L.Push(LNil)
			L.Push(LString(fmt.Sprintf("can not open file: %v", chunkname)))
			return 2
		}
		defer reader.(*os.File).Close()
	}
	// Get environment (4th argument)
	var env *LTable
	if L.GetTop() >= 4 {
		if lv, ok := L.Get(4).(*LTable); ok {
			env = lv
		}
	}
	return loadaux(L, reader, chunkname, env)
}

func baseLoadString(L *LState) int {
	// Lua 5.3: loadstring(string [, chunkname [, env]])
	chunkname := L.OptString(2, "<string>")
	// Get environment (3rd argument)
	var env *LTable
	if L.GetTop() >= 3 {
		if lv, ok := L.Get(3).(*LTable); ok {
			env = lv
		}
	}
	return loadaux(L, strings.NewReader(L.CheckString(1)), chunkname, env)
}

func baseNext(L *LState) int {
	tb := L.CheckTable(1)
	index := LNil
	if L.GetTop() >= 2 {
		index = L.Get(2)
	}
	key, value := tb.Next(index)
	if key == LNil {
		L.Push(LNil)
		return 1
	}
	L.Push(key)
	L.Push(value)
	return 2
}

func pairsaux(L *LState) int {
	tb := L.CheckTable(1)
	key, value := tb.Next(L.Get(2))
	if key == LNil {
		return 0
	} else {
		L.Pop(1)
		L.Push(key)
		L.Push(key)
		L.Push(value)
		return 2
	}
}

func basePairs(L *LState) int {
	tb := L.CheckTable(1)
	L.Push(L.Get(UpvalueIndex(1)))
	L.Push(tb)
	L.Push(LNil)
	return 3
}

func basePCall(L *LState) int {
	L.CheckAny(1)
	v := L.Get(1)
	if v == LNil || (v.Type() != LTFunction && L.GetMetaField(v, "__call").Type() != LTFunction) {
		L.Push(LFalse)
		L.Push(LString("attempt to call a " + v.Type().String() + " value"))
		return 2
	}
	nargs := L.GetTop() - 1
	if err := L.PCall(nargs, MultRet, nil); err != nil {
		L.Push(LFalse)
		if aerr, ok := err.(*ApiError); ok {
			L.Push(aerr.Object)
		} else {
			L.Push(LString(err.Error()))
		}
		return 2
	} else {
		L.Insert(LTrue, 1)
		return L.GetTop()
	}
}

func basePrint(L *LState) int {
	top := L.GetTop()
	for i := 1; i <= top; i++ {
		lv := L.Get(i)
		var s string
		// Use tostring from the environment to convert value to string
		// This matches Lua 5.3 behavior where print calls tostring from _ENV for all values
		tostring := L.GetField(L.Env, "tostring")
		// Call tostring directly (not via PCall) to allow errors to propagate
		// This way pcall(print, ...) will catch errors from tostring
		L.Push(tostring)
		L.Push(lv)
		L.Call(1, 1)
		result := L.Get(-1)
		if resultStr, ok := result.(LString); ok {
			s = string(resultStr)
		} else {
			L.RaiseError("'tostring' must return a string")
		}
		fmt.Print(s)
		if i != top {
			fmt.Print("\t")
		}
	}
	fmt.Println("")
	return 0
}

func base_PrintRegs(L *LState) int {
	L.printReg()
	return 0
}

func baseRawEqual(L *LState) int {
	if L.CheckAny(1) == L.CheckAny(2) {
		L.Push(LTrue)
	} else {
		L.Push(LFalse)
	}
	return 1
}

func baseRawGet(L *LState) int {
	L.Push(L.RawGet(L.CheckTable(1), L.CheckAny(2)))
	return 1
}

func baseRawSet(L *LState) int {
	L.RawSet(L.CheckTable(1), L.CheckAny(2), L.CheckAny(3))
	return 0
}

// baseRawLen - rawlen (v)
// Возвращает длину объекта v (таблица или строка) без использования метаметода __len.
// Возвращает ошибку, если v не таблица и не строка.
func baseRawLen(L *LState) int {
	switch v := L.CheckAny(1).(type) {
	case *LTable:
		L.Push(LNumberInt(int64(v.Len())))
		return 1
	case LString:
		L.Push(LNumberInt(int64(len(v))))
		return 1
	default:
		L.ArgError(1, "table or string expected")
		return 0
	}
}

func baseSelect(L *LState) int {
	L.CheckTypes(1, LTNumber, LTString)
	switch lv := L.Get(1).(type) {
	case LNumber:
		idx := int(lv.Int64())
		num := L.GetTop()
		if idx < 0 {
			idx = num + idx
		} else if idx > num {
			idx = num
		}
		if 1 > idx {
			L.ArgError(1, "index out of range")
		}
		return num - idx
	case LString:
		if string(lv) != "#" {
			L.ArgError(1, "invalid string '"+string(lv)+"'")
		}
		L.Push(LNumberInt(int64(L.GetTop() - 1)))
		return 1
	}
	return 0
}

func baseSetMetatable(L *LState) int {
	L.CheckTypes(2, LTNil, LTTable)
	obj := L.Get(1)
	if obj == LNil {
		L.RaiseError("cannot set metatable to a nil object.")
	}
	mt := L.Get(2)
	if m := L.metatable(obj, true); m != LNil {
		if tb, ok := m.(*LTable); ok && tb.RawGetString("__metatable") != LNil {
			L.RaiseError("cannot change a protected metatable")
		}
	}
	L.SetMetatable(obj, mt)
	L.SetTop(1)
	return 1
}

func baseToNumber(L *LState) int {
	base := L.OptInt(2, 10)
	noBase := L.Get(2) == LNil

	switch lv := L.CheckAny(1).(type) {
	case LNumber:
		L.Push(lv)
	case LString:
		str := strings.Trim(string(lv), " \n\t")
		if strings.Index(str, ".") > -1 {
			if v, err := strconv.ParseFloat(str, 64); err != nil {
				L.Push(LNil)
			} else {
				L.Push(LNumberFloat(v))
			}
		} else {
			// Check for hex prefix 0x or 0X
			if noBase && strings.HasPrefix(strings.ToLower(str), "0x") {
				base, str = 16, str[2:]
			}
			// Also check for standalone hex numbers like "0xF"
			if noBase && len(str) > 1 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X') {
				base, str = 16, str[2:]
			}
			if v, err := strconv.ParseInt(str, base, 64); err != nil {
				L.Push(LNil)
			} else {
				L.Push(LNumberInt(v))
			}
		}
	default:
		L.Push(LNil)
	}
	return 1
}

func baseToString(L *LState) int {
	v1 := L.CheckAny(1)
	if v1 == LNil {
		L.Push(LString("nil"))
	} else {
		s := LVAsString(v1)
		if s == "" && v1.Type() != LTString {
			s = v1.String()
		}
		L.Push(LString(s))
	}
	return 1
}

func baseType(L *LState) int {
	lv := L.CheckAny(1)
	if lv == LNil {
		L.Push(LString("nil"))
	} else {
		t := lv.Type()
		L.Push(LString(t.String()))
	}
	return 1
}

func baseUnpack(L *LState) int {
	tb := L.CheckTable(1)
	start := L.OptInt(2, 1)
	end := L.OptInt(3, tb.Len())
	for i := start; i <= end; i++ {
		L.Push(tb.RawGetInt(i))
	}
	ret := end - start + 1
	if ret < 0 {
		return 0
	}
	return ret
}

func baseXPCall(L *LState) int {
	fn := L.CheckFunction(1)
	errfunc := L.CheckFunction(2)

	top := L.GetTop()
	L.Push(fn)
	if err := L.PCall(0, MultRet, errfunc); err != nil {
		L.Push(LFalse)
		if aerr, ok := err.(*ApiError); ok {
			L.Push(aerr.Object)
		} else {
			L.Push(LString(err.Error()))
		}
		return 2
	} else {
		L.Insert(LTrue, top+1)
		return L.GetTop() - top
	}
}

/* }}} */

/* load lib {{{ */

var loopdetection = &LUserData{}

func loRequire(L *LState) int {
	name := L.CheckString(1)
	loaded := L.GetField(L.Get(RegistryIndex), "_LOADED")
	lv := L.GetField(loaded, name)
	if LVAsBool(lv) {
		if lv == loopdetection {
			L.RaiseError("loop or previous error loading module: %s", name)
		}
		L.Push(lv)
		return 1
	}
	searchers, ok := L.GetField(L.Get(RegistryIndex), "_SEARCHERS").(*LTable)
	if !ok {
		L.RaiseError("package.searchers must be a table")
	}
	messages := []string{}
	var modasfunc LValue
	var modname LValue
	for i := 1; ; i++ {
		loader := L.RawGetInt(searchers, i)
		if loader == LNil {
			L.RaiseError("module %s not found:\n\t%s, ", name, strings.Join(messages, "\n\t"))
		}
		L.Push(loader)
		L.Push(LString(name))
		L.Call(1, MultRet)
		// Get the return values from the loader
		// First return value is the function (or error string)
		ret := L.Get(-1)
		// Second return value (if any) is extra data like file path (Lua 5.3)
		var extra LValue
		if L.GetTop() >= 2 {
			extra = L.Get(-2)
		}
		switch retv := ret.(type) {
		case *LFunction:
			modasfunc = retv
			modname = extra  // Use the extra value (file path) as the module name
			goto loopbreak
		case LString:
			messages = append(messages, string(retv))
		}
		L.Pop(1)
	}
loopbreak:
	L.SetField(loaded, name, loopdetection)
	L.Push(modasfunc)
	// Pass module name and extra data (file path) to the module function
	if modname != nil {
		L.Push(modname)
		L.Call(1, MultRet)
	} else {
		L.Push(LString(name))
		L.Call(1, MultRet)
	}
	ret := L.reg.Pop()
	modv := L.GetField(loaded, name)
	if ret != LNil && modv == loopdetection {
		L.SetField(loaded, name, ret)
		L.Push(ret)
	} else if modv == loopdetection {
		L.SetField(loaded, name, LTrue)
		L.Push(LTrue)
	} else {
		L.Push(modv)
	}
	return 1
}

/* }}} */

/* hidden features {{{ */

func baseNewProxy(L *LState) int {
	ud := L.NewUserData()
	L.SetTop(1)
	if L.Get(1) == LTrue {
		L.SetMetatable(ud, L.NewTable())
	} else if d, ok := L.Get(1).(*LUserData); ok {
		L.SetMetatable(ud, L.GetMetatable(d))
	}
	L.Push(ud)
	return 1
}

/* }}} */

//
