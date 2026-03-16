package lua

import (
	"context"
	"fmt"
	"os"
)

type LValueType int

const (
	LTNil LValueType = iota
	LTBool
	LTNumber
	LTString
	LTFunction
	LTUserData
	LTThread
	LTTable
	LTChannel
)

var lValueNames = [9]string{"nil", "boolean", "number", "string", "function", "userdata", "thread", "table", "channel"}

func (vt LValueType) String() string {
	return lValueNames[int(vt)]
}

type LValue interface {
	String() string
	Type() LValueType
}

// LVIsFalse returns true if a given LValue is a nil or false otherwise false.
func LVIsFalse(v LValue) bool { return v == LNil || v == LFalse }

// LVIsFalse returns false if a given LValue is a nil or false otherwise true.
func LVAsBool(v LValue) bool { return v != LNil && v != LFalse }

// LVAsString returns string representation of a given LValue
// if the LValue is a string or number, otherwise an empty string.
func LVAsString(v LValue) string {
	switch sn := v.(type) {
	case LString, LNumber:
		return sn.String()
	default:
		return ""
	}
}

// LVCanConvToString returns true if a given LValue is a string or number
// otherwise false.
func LVCanConvToString(v LValue) bool {
	switch v.(type) {
	case LString, LNumber:
		return true
	default:
		return false
	}
}

// LVAsNumber tries to convert a given LValue to a number.
func LVAsNumber(v LValue) LNumber {
	switch lv := v.(type) {
	case LNumber:
		return lv
	case LString:
		if num, err := parseNumber(string(lv)); err == nil {
			return num
		}
	}
	return LNumberInt(0)
}

// LVAsNumberStrict tries to convert a given LValue to a number.
// Returns the number and a boolean indicating success.
// Used for bitwise operations where conversion failure should cause an error.
func LVAsNumberStrict(v LValue) (LNumber, bool) {
	switch lv := v.(type) {
	case LNumber:
		return lv, true
	case LString:
		if num, err := parseNumber(string(lv)); err == nil {
			return num, true
		}
		return LNumberInt(0), false
	}
	return LNumberInt(0), false
}

type LNilType struct{}

func (nl *LNilType) String() string   { return "nil" }
func (nl *LNilType) Type() LValueType { return LTNil }

var LNil = LValue(&LNilType{})

type LBool bool

func (bl LBool) String() string {
	if bool(bl) {
		return "true"
	}
	return "false"
}
func (bl LBool) Type() LValueType { return LTBool }

var LTrue = LBool(true)
var LFalse = LBool(false)

type LString string

func (st LString) String() string   { return string(st) }
func (st LString) Type() LValueType { return LTString }

// fmt.Formatter interface
func (st LString) Format(f fmt.State, c rune) {
	switch c {
	case 'd', 'i':
		if nm, err := parseNumber(string(st)); err != nil {
			defaultFormat(nm, f, 'd')
		} else {
			defaultFormat(string(st), f, 's')
		}
	default:
		defaultFormat(string(st), f, c)
	}
}

type LTable struct {
	Metatable LValue

	array   []LValue
	dict    map[LValue]LValue
	strdict map[string]LValue
	keys    []LValue
	k2i     map[LValue]int
}

func (tb *LTable) String() string   { return fmt.Sprintf("table: %p", tb) }
func (tb *LTable) Type() LValueType { return LTTable }

type LFunction struct {
	IsG       bool
	Env       *LTable
	Proto     *FunctionProto
	GFunction LGFunction
	Upvalues  []*Upvalue
}
type LGFunction func(*LState) int

func (fn *LFunction) String() string   { return fmt.Sprintf("function: %p", fn) }
func (fn *LFunction) Type() LValueType { return LTFunction }

type Global struct {
	MainThread    *LState
	CurrentThread *LState
	Registry      *LTable
	Global        *LTable

	builtinMts map[int]LValue
	tempFiles  []*os.File
	gccount    int32

	// Debug hooks (Lua 5.3 compatible)
	Hook      *LFunction
	HookMask  int
	HookCount int
	InHook    bool

	// Track open files for proper GC behavior
	openFiles map[*lFile]bool
}

type LState struct {
	G       *Global
	Parent  *LState
	Env     *LTable
	Panic   func(*LState)
	Dead    bool
	Options Options

	stop         int32
	reg          *registry
	stack        callFrameStack
	alloc        *allocator
	currentFrame *callFrame
	wrapped      bool
	uvcache      *Upvalue
	hasErrorFunc bool
	mainLoop     func(*LState, *callFrame)
	ctx          context.Context
	ctxCancelFn  context.CancelFunc
	// nCcalls tracks nested C calls for yield protection
	// When > 0, yield is not allowed (yield across C boundary)
	nCcalls int
	// pcallLevel tracks nested pcall/xpcall calls for yield support
	// When > 0, yield is allowed inside pcall/xpcall (Lua 5.3 behavior)
	pcallLevel int
	// lastValueSource tracks the source of the last loaded value for better error messages
	// Format: "global 'name'", "field 'name'", "method 'name'", "upvalue 'name'", "local 'name'", ""
	lastValueSource string
	// lastObjectSource tracks the source of the object for method/field calls
	// Used to generate error messages like "field 'bbb' (global 'aaa')"
	lastObjectSource string
	// regValueSources tracks the source of values in registers for error messages
	// This is used to track global variable access through local _ENV
	regValueSources []string
}

// getLocalVarName returns the name of a local variable at a given register and PC
func (ls *LState) getLocalVarName(regIdx int) string {
	cf := ls.currentFrame
	if cf == nil || cf.Fn == nil || cf.Fn.Proto == nil {
		return ""
	}
	proto := cf.Fn.Proto
	lbase := cf.LocalBase
	// Convert absolute register index to relative index within the function
	relRegIdx := regIdx - lbase
	if relRegIdx < 0 {
		return ""
	}
	// Search through debug local info for a matching register
	for _, local := range proto.DbgLocals {
		if local.StartPc <= cf.Pc && cf.Pc < local.EndPc {
			if local.Register == relRegIdx {
				return local.Name
			}
		}
	}
	return ""
}

// setRegValueSource sets the source of a value in a register for error messages
func (ls *LState) setRegValueSource(regIdx int, source string) {
	if regIdx >= 0 && regIdx < len(ls.regValueSources) {
		ls.regValueSources[regIdx] = source
	}
}

// getRegValueSource returns the source of a value in a register
func (ls *LState) getRegValueSource(regIdx int) string {
	if regIdx >= 0 && regIdx < len(ls.regValueSources) {
		return ls.regValueSources[regIdx]
	}
	return ""
}

// clearRegValueSource clears the source of a value in a register
func (ls *LState) clearRegValueSource(regIdx int) {
	if regIdx >= 0 && regIdx < len(ls.regValueSources) {
		ls.regValueSources[regIdx] = ""
	}
}

// trackLocalVar tracks a local variable at the given register for error messages
func (ls *LState) trackLocalVar(regIdx int) {
	name := ls.getLocalVarName(regIdx)
	if name != "" {
		ls.lastValueSource = fmt.Sprintf("local '%s'", name)
	}
}

func (ls *LState) String() string   { return fmt.Sprintf("thread: %p", ls) }
func (ls *LState) Type() LValueType { return LTThread }

type LUserData struct {
	Value     interface{}
	Env       *LTable
	Metatable LValue
}

func (ud *LUserData) String() string   { return fmt.Sprintf("userdata: %p", ud) }
func (ud *LUserData) Type() LValueType { return LTUserData }

type LChannel chan LValue

func (ch LChannel) String() string   { return fmt.Sprintf("channel: %p", ch) }
func (ch LChannel) Type() LValueType { return LTChannel }
