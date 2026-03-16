package lua

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// File descriptor indices (from iolib.go)
const (
	fileDefOutIndex = 1
	fileDefInIndex  = 2
)

/* basic functions {{{ */

func OpenBase(L *LState) int {
	global := L.Get(GlobalsIndex).(*LTable)
	L.SetGlobal("_G", global)
	L.SetGlobal("_VERSION", LString(LuaVersion))
	L.SetGlobal("_GOPHER_LUA_VERSION", LString(PackageName+" "+PackageVersion))
	// Lua 5.3: _ENV is the environment table for global access
	// Set _ENV as a global variable that points to the current environment
	L.SetGlobal("_ENV", global)
	// Register base functions in the global table (_ENV)
	// Use SetFuncs to add functions directly to the global table
	L.SetFuncs(global, baseFuncs)
	global.RawSetString("ipairs", L.NewClosure(baseIpairs, L.NewFunction(ipairsaux)))
	global.RawSetString("pairs", L.NewClosure(basePairs, L.NewFunction(pairsaux)))
	L.Push(global)
	return 1
}

var baseFuncs = map[string]LGFunction{
	"assert":         baseAssert,
	"collectgarbage": baseCollectGarbage,
	"dofile":         baseDoFile,
	"error":          baseError,
	"getmetatable":   baseGetMetatable,
	"isinteger":      baseIsInteger,
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

// GCState stores the state of the garbage collector
type GCState struct {
	Stop       bool  // true if GC is stopped
	Pause      int   // pause multiplier (default 200)
	StepMul    int   // step multiplier (default 200)
	TotalBytes int64 // total allocated bytes
	MaxBytes   int64 // maximum bytes before next GC
}

// Global GC state
var globalGCState = &GCState{
	Stop:       false,
	Pause:      200,
	StepMul:    200,
	TotalBytes: 0,
	MaxBytes:   0,
}

func baseCollectGarbage(L *LState) int {
	option := L.OptString(1, "collect")

	switch option {
	case "collect":
		// Close tracked files that are no longer referenced
		// This handles the case where io.lines(file) is called but the iterator is not consumed
		// Don't close files that are the current input or output files
		var input_file, output_file *lFile
		uv := L.Get(UpvalueIndex(1))
		if uv != nil {
			if tb, ok := uv.(*LTable); ok {
				if ud := tb.RawGetInt(fileDefInIndex); ud != nil {
					if u, ok := ud.(*LUserData); ok {
						input_file = u.Value.(*lFile)
					}
				}
				if ud := tb.RawGetInt(fileDefOutIndex); ud != nil {
					if u, ok := ud.(*LUserData); ok {
						output_file = u.Value.(*lFile)
					}
				}
			}
		}
		for lfile := range L.G.openFiles {
			if !lfile.closed && !lfile.std && lfile != input_file && lfile != output_file {
				lfile.fp.Close()
				lfile.closed = true
			}
		}
		L.G.openFiles = make(map[*lFile]bool)

		// Perform a full garbage collection cycle
		if !globalGCState.Stop {
			runtime.GC()
		}
		// Return 0 (no return value in Lua 5.3 for "collect")
		return 0

	case "stop":
		// Stop the garbage collector
		globalGCState.Stop = true
		return 0

	case "restart":
		// Restart the garbage collector
		globalGCState.Stop = false
		return 0

	case "count":
		// Return total memory in KB
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		// Return two values: total KB and bytes remainder
		totalKB := memStats.Alloc / 1024
		bytesRem := memStats.Alloc % 1024
		L.Push(LNumberFloat(float64(totalKB) + float64(bytesRem)/1024.0))
		return 1

	case "step":
		// Perform a garbage collection step
		stepSize := L.OptInt(2, 0)
		if !globalGCState.Stop {
			if stepSize > 0 {
				// Perform multiple small GC steps
				for i := 0; i < stepSize; i++ {
					runtime.GC()
				}
			} else {
				runtime.GC()
			}
		}
		// Return true if the step finished a collection cycle
		// (we always return true for simplicity)
		L.Push(LTrue)
		return 1

	case "setpause":
		// Set the pause multiplier
		newPause := L.CheckInt(2)
		oldPause := globalGCState.Pause
		globalGCState.Pause = newPause
		L.Push(LNumberInt(int64(oldPause)))
		return 1

	case "setstepmul":
		// Set the step multiplier
		newStepMul := L.CheckInt(2)
		oldStepMul := globalGCState.StepMul
		globalGCState.StepMul = newStepMul
		L.Push(LNumberInt(int64(oldStepMul)))
		return 1

	case "isrunning":
		// Return whether the collector is running
		L.Push(LBool(!globalGCState.Stop))
		return 1

	case "setmajorinc":
		// Lua 5.3 option - set major increment
		// We don't implement incremental GC, so just return 0
		L.Push(LNumberInt(0))
		return 1

	case "getmajorinc":
		// Lua 5.3 option - get major increment
		// Return default value
		L.Push(LNumberInt(0))
		return 1

	default:
		L.ArgError(1, "collectgarbage: invalid option '"+option+"'")
		return 0
	}
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
	obj := L.Get(1)
	if obj == nil {
		obj = LNil
	}
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

func loadaux(L *LState, reader io.Reader, chunkname string, env LValue) int {
	if fn, err := L.Load(reader, chunkname); err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		// Set the environment for the loaded function
		if env != nil {
			// In Lua 5.3, _ENV is the first upvalue of the function
			// We need to set it to the provided environment value
			if len(fn.Upvalues) > 0 {
				// Find the _ENV upvalue (should be the first one)
				if fn.Proto != nil && len(fn.Proto.DbgUpvalues) > 0 && fn.Proto.DbgUpvalues[0] == "_ENV" {
					// Create a new upvalue with the environment value
					fn.Upvalues[0] = &Upvalue{
						value:  env,
						closed: true,
					}
				} else if len(fn.Proto.DbgUpvalues) == 0 {
					// Binary chunk without DbgUpvalues - assume first upvalue is _ENV
					fn.Upvalues[0] = &Upvalue{
						value:  env,
						closed: true,
					}
				}
			} else {
				// If the function has no upvalues, create one for _ENV
				// This is needed for Lua 5.3 compatibility when loading binary chunks with env
				fn.Upvalues = []*Upvalue{
					{
						value:  env,
						closed: true,
					},
				}
			}
			// Also set Env for backward compatibility with table environments
			if tb, ok := env.(*LTable); ok {
				fn.Env = tb
			}
		}
		L.Push(fn)
		return 1
	}
}

func baseLoad(L *LState) int {
	// Lua 5.3 compatibility: load(chunk [, chunkname [, mode [, env]]])
	// chunk can be a string or a function
	chunk := L.Get(1)

	// Lua 5.3: default chunkname for string chunks is [string "first 20 chars..."]
	// For function chunks, it's "<function>"
	chunkname := L.OptString(2, "")
	if chunkname == "" {
		if s, ok := chunk.(LString); ok {
			// Use first 20 characters of the first line (Lua 5.3 behavior)
			str := string(s)
			// Take only the first line (Lua 5.3 strips newlines)
			if idx := strings.IndexAny(str, "\n\r"); idx >= 0 {
				str = str[:idx]
			}
			if len(str) > 20 {
				str = str[:20] + "..."
			}
			chunkname = "[string \"" + str + "\"]"
		} else {
			chunkname = "<function>"
		}
	}
	mode := L.OptString(3, "bt") // Default mode is "bt" (both text and binary)

	// Get environment (4th argument) - can be any value in Lua 5.3
	var env LValue
	if L.GetTop() >= 4 {
		env = L.Get(4)
	}

	var reader io.Reader
	var chunkData string

	switch c := chunk.(type) {
	case LString:
		// If chunk is a string, use it directly
		chunkData = string(c)
		reader = strings.NewReader(chunkData)
	case *LFunction:
		// If chunk is a function, read from it
		top := L.GetTop()
		buf := []string{}
		for {
			L.SetTop(top)
			L.Push(c)
			err := L.PCall(0, 1, nil)
			if err != nil {
				L.Push(LNil)
				L.Push(LString("error loading module: " + err.Error()))
				return 2
			}
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
		reader = strings.NewReader(chunkData)
	default:
		L.Push(LNil)
		L.Push(LString("bad argument #1 to load (function or string expected, got " + chunk.Type().String() + ")"))
		return 2
	}

	// Check mode compatibility (Lua 5.3)
	// Binary chunks start with the Lua signature: 0x1B 0x4C 0x75 0x61 (ESC Lua)
	isBinary := false
	if len(chunkData) >= 4 {
		// Check for Lua binary signature: ESC + "Lua"
		if chunkData[0] == 0x1B && chunkData[1] == 'L' && chunkData[2] == 'u' && chunkData[3] == 'a' {
			isBinary = true
		}
	}

	// Mode validation
	// "b" = only binary chunks
	// "t" = only text chunks
	// "bt" or "tb" = both (default)
	// Empty chunks are considered text chunks
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
	var env LValue
	if L.GetTop() >= 4 {
		env = L.Get(4)
	}
	return loadaux(L, reader, chunkname, env)
}

func baseLoadString(L *LState) int {
	// Lua 5.3: loadstring(string [, chunkname [, env]])
	chunkname := L.OptString(2, "<string>")
	// Get environment (3rd argument)
	var env LValue
	if L.GetTop() >= 3 {
		env = L.Get(3)
	}
	return loadaux(L, strings.NewReader(L.CheckString(1)), chunkname, env)
}

func baseNext(L *LState) int {
	tb := L.CheckTable(1)
	index := LNil
	if L.GetTop() >= 2 {
		index = L.Get(2)
	}
	// Lua 5.3: next() должен вызывать ошибку, если ключ не существует
	if index != LNil {
		// Проверяем, существует ли ключ в таблице
		if tb.RawGet(index) == LNil {
			// Ключ не найден - проверяем, действительно ли его нет
			// (может быть nil значением, но ключ существует)
			found := false
			tb.ForEach(func(k, v LValue) {
				if k == index {
					found = true
				}
			})
			if !found {
				L.RaiseError("invalid key to 'next'")
			}
		}
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

	// Increment pcallLevel to allow yield inside pcall (Lua 5.3 behavior)
	L.pcallLevel++
	defer func() { L.pcallLevel-- }()

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

// baseIsInteger - isinteger (x)
// Проверяет, является ли значение целым числом.
// Возвращает true, если значение является целым числом, false в противном случае.
// Для нечисловых типов возвращает false.
func baseIsInteger(L *LState) int {
	v := L.CheckAny(1)
	if num, ok := v.(LNumber); ok {
		if num.IsInteger() {
			L.Push(LTrue)
		} else {
			L.Push(LFalse)
		}
	} else {
		L.Push(LFalse)
	}
	return 1
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

		// If no base is specified, use parseNumber for full support including hex floats
		if noBase {
			if num, err := parseNumber(str); err == nil {
				L.Push(num)
			} else {
				L.Push(LNil)
			}
		} else {
			// Base is specified
			// For base != 16, reject hexadecimal prefix 0x
			if base != 16 && strings.HasPrefix(strings.ToLower(str), "0x") {
				L.Push(LNil)
				return 1
			}

			if strings.Index(str, ".") > -1 {
				if v, err := strconv.ParseFloat(str, base); err != nil {
					L.Push(LNil)
				} else {
					L.Push(LNumberFloat(v))
				}
			} else {
				// For base 16 (hex), handle wraparound for large numbers
				if base == 16 {
					// Try to parse as unsigned first for wraparound behavior
					if v, err := strconv.ParseUint(str, base, 64); err == nil {
						L.Push(LNumberInt(int64(v)))
					} else if v, err := strconv.ParseInt(str, base, 64); err == nil {
						L.Push(LNumberInt(v))
					} else {
						L.Push(LNil)
					}
				} else {
					if v, err := strconv.ParseInt(str, base, 64); err != nil {
						L.Push(LNil)
					} else {
						L.Push(LNumberInt(v))
					}
				}
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
		// Use ToStringMeta to respect __tostring metamethod
		result := L.ToStringMeta(v1)
		L.Push(result)
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

	// Save original top before we start modifying the stack
	origTop := L.GetTop()
	nargs := origTop - 2 // Number of additional arguments

	// Push the function to call
	L.Push(fn)
	// Push additional arguments (from index 3 onwards)
	for i := 3; i <= origTop; i++ {
		L.Push(L.Get(i))
	}

	// Increment pcallLevel to allow yield inside xpcall (Lua 5.3 behavior)
	L.pcallLevel++
	defer func() { L.pcallLevel-- }()

	if err := L.PCall(nargs, MultRet, errfunc); err != nil {
		L.Push(LFalse)
		if aerr, ok := err.(*ApiError); ok {
			L.Push(aerr.Object)
		} else {
			L.Push(LString(err.Error()))
		}
		return 2
	} else {
		// Results are now on the stack starting from position origTop+1
		// We need to insert true at the beginning of results
		nresults := L.GetTop() - origTop
		if nresults > 0 {
			L.Insert(LTrue, origTop+1)
		} else {
			L.Push(LTrue)
		}
		return L.GetTop() - origTop
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
	// Lua 5.3: also check package.searchers field (can be changed by user)
	// But only use it if it's a valid table
	pkg_searchers := L.GetField(L.GetField(L.Get(EnvironIndex), "package"), "searchers")
	if pkg_searchers != LNil {
		// package.searchers exists, check if it's a table
		if pkg_table, ok := pkg_searchers.(*LTable); ok {
			// Use package.searchers if it's a table (user modified it)
			searchers = pkg_table
		} else {
			// package.searchers exists but is not a table - error
			L.RaiseError("package.searchers must be a table")
		}
	}
	// If package.searchers is nil (not set), continue using _SEARCHERS from registry
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
		// After Call, the return values are at the top of the stack
		// If loader returned N values, they are at positions: -N, -N+1, ..., -1
		// The first return value (function or error string) is at position -N
		top := L.GetTop()
		if top == 0 {
			// No return values, continue to next searcher
			continue
		}
		// First return value is at position -top (the "lowest" return value on the stack)
		ret := L.Get(-top)
		// Second return value (if any) is extra data like file path (Lua 5.3)
		var extra LValue
		if top >= 2 {
			extra = L.Get(-top + 1) // Second return value
		}
		switch retv := ret.(type) {
		case *LFunction:
			modasfunc = retv
			modname = extra // Use the extra value (file path) as the module name
			goto loopbreak
		case LString:
			messages = append(messages, string(retv))
		}
		// Clear the return values from the stack
		L.Pop(top)
	}
loopbreak:
	L.SetField(loaded, name, loopdetection)
	L.Push(modasfunc)
	// Pass module name and extra data (file path) to the module function
	// Lua 5.3: loader returns function, extra_data; then function is called with (name, extra_data)
	if modname != nil {
		L.Push(LString(name)) // First argument: module name
		L.Push(modname)       // Second argument: extra data (file path)
		L.Call(2, MultRet)
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
