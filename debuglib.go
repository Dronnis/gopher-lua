package lua

import (
	"fmt"
	"os"
	"strings"
)

// Debug hook constants (Lua 5.3 compatible)
const (
	HookMaskCall   = 1 << iota // 'c'
	HookMaskReturn             // 'r'
	HookMaskLine               // 'l'
	HookMaskCount              // 'n'
)

// HookEvent represents the type of hook event
type HookEvent int

const (
	HookEventCall       HookEvent = iota // "call"
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
	"debug":        debugDebug,
	"gethook":      debugGetHook,
	"getinfo":      debugGetInfo,
	"getlocal":     debugGetLocal,
	"getmetatable": debugGetMetatable,
	"getregistry":  debugGetRegistry,
	"getupvalue":   debugGetUpvalue,
	"getuservalue": debugGetUserValue,
	"sethook":      debugSetHook,
	"setlocal":     debugSetLocal,
	"setmetatable": debugSetMetatable,
	"setupvalue":   debugSetUpvalue,
	"setuservalue": debugSetUserValue,
	"traceback":    debugTraceback,
	"upvalueid":    debugUpvalueID,
	"upvaluejoin":  debugUpvalueJoin,
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
// In Lua 5.3+, this returns any value (not just tables).
func debugGetUserValue(L *LState) int {
	ud := L.CheckUserData(1)

	// In GopherLua, LUserData has an Env field that serves as uservalue
	if ud.Env != nil {
		// Check if this is a wrapper table for a non-table value
		// Wrapper tables have exactly one element at index 1 and no metatable
		if ud.Env.Len() == 1 && ud.Env.Metatable == LNil {
			val := ud.Env.RawGetInt(1)
			// Return the wrapped value (could be any type including nil)
			L.Push(val)
			return 1
		}
		L.Push(ud.Env)
	} else {
		L.Push(LNil)
	}
	return 1
}

// debug.setuservalue(udata, value)
// Sets the user value associated with the userdata.
// In Lua 5.3+, the value can be any Lua value (not just tables).
// Returns the userdata.
func debugSetUserValue(L *LState) int {
	ud := L.CheckUserData(1)
	value := L.CheckAny(2)

	// In Lua 5.3+, uservalue can be any value (not just table or nil)
	// Store the value directly in the Env field
	// For non-table values, we wrap them in a table with a special key
	if tb, ok := value.(*LTable); ok {
		ud.Env = tb
	} else {
		// For non-table values, create a wrapper table
		// This maintains compatibility with existing code that expects Env to be a table
		wrapper := L.NewTable()
		wrapper.RawSetInt(1, value) // Store value at index 1
		ud.Env = wrapper
	}

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

// debug.debug([prompt[, env]])
// Enters an interactive debug console/REPL.
// In GopherLua, this is a simplified implementation that executes code from stdin.
func debugDebug(L *LState) int {
	prompt := L.OptString(1, "debug> ")

	// Get environment table (optional second argument)
	var env *LTable
	if L.GetTop() >= 2 {
		if tb, ok := L.Get(2).(*LTable); ok {
			env = tb
		} else {
			L.ArgError(2, "table expected")
			return 0
		}
	}

	// Save original environment
	originalEnv := L.Env
	if env != nil {
		L.Env = env
	}

	// Print welcome message
	fmt.Fprintf(os.Stdout, "Lua 5.3 Debug Console (GopherLua)\n")
	fmt.Fprintf(os.Stdout, "Type 'exit' or 'quit' to leave, 'help' for help.\n\n")

	// Interactive REPL loop
	for {
		// Print prompt
		fmt.Fprintf(os.Stdout, "%s", prompt)

		// Read line from stdin
		var line string
		_, err := fmt.Scanln(&line)
		if err != nil {
			fmt.Fprintf(os.Stdout, "\n")
			break
		}

		// Check for exit commands
		line = strings.TrimSpace(line)
		if line == "exit" || line == "quit" || line == "cont" {
			break
		}

		// Check for help command
		if line == "help" {
			fmt.Fprintf(os.Stdout, "Debug Console Commands:\n")
			fmt.Fprintf(os.Stdout, "  exit, quit, cont - Exit debug console\n")
			fmt.Fprintf(os.Stdout, "  help             - Show this help\n")
			fmt.Fprintf(os.Stdout, "  <lua code>       - Execute Lua code\n")
			fmt.Fprintf(os.Stdout, "  = <expr>         - Evaluate and print expression\n")
			fmt.Fprintf(os.Stdout, "\n")
			continue
		}

		// Check for print shortcut (= expr)
		if strings.HasPrefix(line, "=") {
			line = "return " + strings.TrimPrefix(line, "=")
		}

		// Skip empty lines
		if line == "" {
			continue
		}

		// Execute the code
		if err := L.DoString(line); err != nil {
			fmt.Fprintf(os.Stdout, "Error: %v\n", err)
		} else {
			// Print any return values
			top := L.GetTop()
			if top > 0 {
				for i := 1; i <= top; i++ {
					val := L.Get(i)
					if i > 1 {
						fmt.Fprintf(os.Stdout, "\t")
					}
					fmt.Fprintf(os.Stdout, "%v", val)
				}
				fmt.Fprintf(os.Stdout, "\n")
				L.SetTop(0)
			}
		}
	}

	// Restore original environment
	L.Env = originalEnv

	return 0
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
// Returns a unique identifier for the upvalue (like a light userdata in Lua 5.3)
// The identifier is based on the upvalue's address and can be compared for equality
// Works for both Lua functions and C (Go) functions
// Raises an error if the index is out of range (Lua 5.3 behavior)
func debugUpvalueID(L *LState) int {
	fn := L.CheckFunction(1)
	n := L.CheckInt(2)

	if n < 1 {
		L.ArgError(2, "invalid upvalue index")
		return 0
	}

	// For Lua functions
	if !fn.IsG {
		if n > len(fn.Upvalues) {
			L.ArgError(2, "invalid upvalue index")
			return 0
		}
		uv := fn.Upvalues[n-1]
		if uv == nil {
			L.Push(LNil)
			return 1
		}
		// In Lua 5.3, this returns a light userdata with the upvalue's address
		// We simulate this by returning a unique string based on the upvalue pointer
		L.Push(LString(fmt.Sprintf("upvalue_%p", uv)))
		return 1
	}

	// For C (Go) functions - they can also have upvalues
	if n > len(fn.Upvalues) {
		L.ArgError(2, "invalid upvalue index")
		return 0
	}
	uv := fn.Upvalues[n-1]
	if uv == nil {
		L.Push(LNil)
		return 1
	}
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
