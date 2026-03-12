package lua

import (
	"fmt"
	"strings"
)

// Debug hook constants (Lua 5.3 compatible)
const (
	HookMaskCall    = 1 << iota // 'c'
	HookMaskReturn              // 'r'
	HookMaskLine                // 'l'
	HookMaskCount               // 'n'
)

// HookEvent represents the type of hook event
type HookEvent int

const (
	HookEventCall      HookEvent = iota // "call"
	HookEventReturn                      // "return"
	HookEventLine                        // "line"
	HookEventCount                       // "count"
	HookEventTailReturn                  // "tail return"
)

// DebugHook represents a debug hook
type DebugHook struct {
	Function *LFunction
	Mask     int
	Count    int
}

// callHook calls the debug hook with the given event
func callHook(L *LState, event HookEvent, line int) {
	if L.G.Hook == nil {
		return
	}

	// Prevent recursive hook calls
	if L.G.InHook {
		return
	}
	L.G.InHook = true
	defer func() { L.G.InHook = false }()

	// Push hook function
	L.Push(L.G.Hook)

	// Push event string
	var eventStr string
	switch event {
	case HookEventCall:
		eventStr = "call"
	case HookEventReturn:
		eventStr = "return"
	case HookEventLine:
		eventStr = "line"
	case HookEventCount:
		eventStr = "count"
	case HookEventTailReturn:
		eventStr = "tail return"
	}
	L.Push(LString(eventStr))

	// Push line number (or nil)
	if line > 0 {
		L.Push(LNumberInt(int64(line)))
	} else {
		L.Push(LNil)
	}

	// Call hook (no return values expected)
	if err := L.PCall(2, 0, nil); err != nil {
		// Ignore hook errors
	}
}

func OpenDebug(L *LState) int {
	dbgmod := L.RegisterModule(DebugLibName, debugFuncs)
	L.Push(dbgmod)
	return 1
}

var debugFuncs = map[string]LGFunction{
	"gethook":       debugGetHook,
	"getinfo":       debugGetInfo,
	"getlocal":      debugGetLocal,
	"getmetatable":  debugGetMetatable,
	"getregistry":   debugGetRegistry,
	"getupvalue":    debugGetUpvalue,
	"getuservalue":  debugGetUserValue,
	"sethook":       debugSetHook,
	"setlocal":      debugSetLocal,
	"setmetatable":  debugSetMetatable,
	"setupvalue":    debugSetUpvalue,
	"setuservalue":  debugSetUserValue,
	"traceback":     debugTraceback,
	"upvalueid":     debugUpvalueID,
	"upvaluejoin":   debugUpvalueJoin,
}

func debugGetInfo(L *LState) int {
	L.CheckTypes(1, LTFunction, LTNumber)
	arg1 := L.Get(1)
	what := L.OptString(2, "Slunf")
	var dbg *Debug
	var fn LValue
	var err error
	var ok bool
	switch lv := arg1.(type) {
	case *LFunction:
		dbg = &Debug{}
		fn, err = L.GetInfo(">"+what, dbg, lv)
	case LNumber:
		dbg, ok = L.GetStack(int(lv.Int64()))
		if !ok {
			L.Push(LNil)
			return 1
		}
		fn, err = L.GetInfo(what, dbg, LNil)
	}

	if err != nil {
		L.Push(LNil)
		return 1
	}
	tbl := L.NewTable()
	if len(dbg.Name) > 0 {
		tbl.RawSetString("name", LString(dbg.Name))
	} else {
		tbl.RawSetString("name", LNil)
	}
	tbl.RawSetString("what", LString(dbg.What))
	tbl.RawSetString("source", LString(dbg.Source))
	tbl.RawSetString("currentline", LNumberInt(int64(dbg.CurrentLine)))
	tbl.RawSetString("nups", LNumberInt(int64(dbg.NUpvalues)))
	tbl.RawSetString("linedefined", LNumberInt(int64(dbg.LineDefined)))
	tbl.RawSetString("lastlinedefined", LNumberInt(int64(dbg.LastLineDefined)))
	tbl.RawSetString("func", fn)
	L.Push(tbl)
	return 1
}

func debugGetLocal(L *LState) int {
	level := L.CheckInt(1)
	idx := L.CheckInt(2)
	dbg, ok := L.GetStack(level)
	if !ok {
		L.ArgError(1, "level out of range")
	}
	name, value := L.GetLocal(dbg, idx)
	if len(name) > 0 {
		L.Push(LString(name))
		L.Push(value)
		return 2
	}
	L.Push(LNil)
	return 1
}

func debugGetMetatable(L *LState) int {
	L.Push(L.GetMetatable(L.CheckAny(1)))
	return 1
}

func debugGetUpvalue(L *LState) int {
	fn := L.CheckFunction(1)
	idx := L.CheckInt(2)
	name, value := L.GetUpvalue(fn, idx)
	if len(name) > 0 {
		L.Push(LString(name))
		L.Push(value)
		return 2
	}
	L.Push(LNil)
	return 1
}

func debugSetLocal(L *LState) int {
	level := L.CheckInt(1)
	idx := L.CheckInt(2)
	value := L.CheckAny(3)
	dbg, ok := L.GetStack(level)
	if !ok {
		L.ArgError(1, "level out of range")
	}
	name := L.SetLocal(dbg, idx, value)
	if len(name) > 0 {
		L.Push(LString(name))
	} else {
		L.Push(LNil)
	}
	return 1
}

func debugSetMetatable(L *LState) int {
	L.CheckTypes(2, LTNil, LTTable)
	obj := L.Get(1)
	mt := L.Get(2)
	L.SetMetatable(obj, mt)
	L.SetTop(1)
	return 1
}

func debugSetUpvalue(L *LState) int {
	fn := L.CheckFunction(1)
	idx := L.CheckInt(2)
	value := L.CheckAny(3)
	name := L.SetUpvalue(fn, idx, value)
	if len(name) > 0 {
		L.Push(LString(name))
	} else {
		L.Push(LNil)
	}
	return 1
}

// debug.getuservalue(udata)
// Returns the user value associated with the userdata.
// In Lua 5.3+, this returns the environment table or nil.
func debugGetUserValue(L *LState) int {
	ud := L.CheckUserData(1)
	
	// In GopherLua, LUserData has an Env field that serves as uservalue
	if ud.Env != nil {
		L.Push(ud.Env)
	} else {
		L.Push(LNil)
	}
	return 1
}

// debug.setuservalue(value, udata)
// Sets the user value associated with the userdata.
// Returns the userdata.
func debugSetUserValue(L *LState) int {
	value := L.CheckAny(1)
	ud := L.CheckUserData(2)
	
	// The value must be a table or nil (Lua 5.3 compatibility)
	if value != LNil {
		if _, ok := value.(*LTable); !ok {
			L.ArgError(1, "table expected or nil")
			return 0
		}
	}
	
	// Set the Env field of the userdata
	ud.Env, _ = value.(*LTable)
	
	// Return the userdata
	L.Push(ud)
	return 1
}

func debugTraceback(L *LState) int {
	msg := ""
	level := L.OptInt(2, 1)
	ls := L
	if L.GetTop() > 0 {
		if s, ok := L.Get(1).(LString); ok {
			msg = string(s)
		}
		if l, ok := L.Get(1).(*LState); ok {
			ls = l
			msg = ""
		}
	}

	traceback := strings.TrimSpace(ls.stackTrace(level))
	if len(msg) > 0 {
		traceback = fmt.Sprintf("%s\n%s", msg, traceback)
	}
	L.Push(LString(traceback))
	return 1
}

// debug.sethook(hook, mask[, count])
func debugSetHook(L *LState) int {
	hook := L.Get(1)
	
	// If hook is nil, disable hooks
	if hook == LNil {
		L.G.Hook = nil
		L.G.HookMask = 0
		L.G.HookCount = 0
		return 0
	}
	
	maskStr := L.CheckString(2)
	count := L.OptInt(3, 0)

	mask := 0
	for _, c := range maskStr {
		switch c {
		case 'c':
			mask |= HookMaskCall
		case 'r':
			mask |= HookMaskReturn
		case 'l':
			mask |= HookMaskLine
		case 'n':
			mask |= HookMaskCount
		}
	}

	if fn, ok := hook.(*LFunction); ok {
		L.G.Hook = fn
		L.G.HookMask = mask
		L.G.HookCount = count
	} else {
		L.ArgError(1, "hook must be a function or nil")
	}
	return 0
}

// debug.gethook([thread])
func debugGetHook(L *LState) int {
	if L.G.Hook == nil {
		L.Push(LNil)
	} else {
		L.Push(L.G.Hook)
	}

	// Build mask string
	mask := ""
	if L.G.HookMask&HookMaskCall != 0 {
		mask += "c"
	}
	if L.G.HookMask&HookMaskReturn != 0 {
		mask += "r"
	}
	if L.G.HookMask&HookMaskLine != 0 {
		mask += "l"
	}
	if L.G.HookMask&HookMaskCount != 0 {
		mask += "n"
	}
	L.Push(LString(mask))

	L.Push(LNumberInt(int64(L.G.HookCount)))
	return 3
}

// debug.getregistry()
func debugGetRegistry(L *LState) int {
	L.Push(L.Get(RegistryIndex))
	return 1
}

// debug.upvalueid(f, n)
func debugUpvalueID(L *LState) int {
	fn := L.CheckFunction(1)
	n := L.CheckInt(2)

	if fn.IsG {
		L.Push(LNil)
		return 1
	}

	if n < 1 || n > len(fn.Upvalues) {
		L.Push(LNil)
		return 1
	}

	uv := fn.Upvalues[n-1]
	// Return a unique identifier for the upvalue (its address)
	L.Push(LString(fmt.Sprintf("upvalue_%p", uv)))
	return 1
}

// debug.upvaluejoin(f1, n1, f2, n2)
func debugUpvalueJoin(L *LState) int {
	f1 := L.CheckFunction(1)
	n1 := L.CheckInt(2)
	f2 := L.CheckFunction(3)
	n2 := L.CheckInt(4)

	if f1.IsG || f2.IsG {
		L.ArgError(1, "Lua function expected")
		return 0
	}

	if n1 < 1 || n1 > len(f1.Upvalues) {
		L.ArgError(2, "invalid upvalue index")
		return 0
	}

	if n2 < 1 || n2 > len(f2.Upvalues) {
		L.ArgError(4, "invalid upvalue index")
		return 0
	}

	// Share the upvalue
	f1.Upvalues[n1-1] = f2.Upvalues[n2-1]
	return 0
}
