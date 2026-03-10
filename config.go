package lua

import (
	"os"
)

var CompatVarArg = true
var FieldsPerFlush = 50
var RegistrySize = 256 * 20
var RegistryGrowStep = 32
var CallStackSize = 256
var MaxTableGetLoop = 100
var MaxArrayIndex = 67108864

// LNumber represents a Lua number which can be either int64 or float64.
// This follows Lua 5.3+ number system with separate integer and float types.
type LNumber struct {
	value interface{}
}

// NumberKind indicates the kind of number: integer or float
type NumberKind int

const (
	NumberKindFloat NumberKind = iota
	NumberKindInt
)

const LNumberBit = 64
const LNumberScanFormat = "%f"
const LuaVersion = "Lua 5.3"

// luaIntegerType is the internal type for integer numbers
type luaIntegerType int64

// luaFloatType is the internal type for float numbers  
type luaFloatType float64

// Number constants for Lua 5.3
const (
	LuaIntegerFormat = "d"
	LuaNumberFormat  = "g"
)

var LuaPath = "LUA_PATH"
var LuaLDir string
var LuaPathDefault string
var LuaOS string
var LuaDirSep string
var LuaPathSep = ";"
var LuaPathMark = "?"
var LuaExecDir = "!"
var LuaIgMark = "-"

func init() {
	if os.PathSeparator == '/' { // unix-like
		LuaOS = "unix"
		LuaLDir = "/usr/local/share/lua/5.1"
		LuaDirSep = "/"
		LuaPathDefault = "./?.lua;" + LuaLDir + "/?.lua;" + LuaLDir + "/?/init.lua"
	} else { // windows
		LuaOS = "windows"
		LuaLDir = "!\\lua"
		LuaDirSep = "\\"
		LuaPathDefault = ".\\?.lua;" + LuaLDir + "\\?.lua;" + LuaLDir + "\\?\\init.lua"
	}
}
