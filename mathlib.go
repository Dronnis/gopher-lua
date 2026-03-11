package lua

import (
	"math"
	"math/rand"
)

func OpenMath(L *LState) int {
	mod := L.RegisterModule(MathLibName, mathFuncs).(*LTable)
	mod.RawSetString("pi", LNumberFloat(math.Pi))
	mod.RawSetString("huge", LNumberFloat(math.MaxFloat64))
	// Lua 5.3 integer constants
	mod.RawSetString("maxinteger", LNumberInt(math.MaxInt64))
	mod.RawSetString("mininteger", LNumberInt(math.MinInt64))
	L.Push(mod)
	return 1
}

var mathFuncs = map[string]LGFunction{
	"abs":        mathAbs,
	"acos":       mathAcos,
	"asin":       mathAsin,
	"atan":       mathAtan,
	"atan2":      mathAtan2,
	"ceil":       mathCeil,
	"cos":        mathCos,
	"cosh":       mathCosh,
	"deg":        mathDeg,
	"exp":        mathExp,
	"floor":      mathFloor,
	"fmod":       mathFmod,
	"frexp":      mathFrexp,
	"ldexp":      mathLdexp,
	"log":        mathLog,
	"log10":      mathLog10,
	"max":        mathMax,
	"min":        mathMin,
	"mod":        mathMod,
	"modf":       mathModf,
	"pow":        mathPow,
	"rad":        mathRad,
	"random":     mathRandom,
	"randomseed": mathRandomseed,
	"sin":        mathSin,
	"sinh":       mathSinh,
	"sqrt":       mathSqrt,
	"tan":        mathTan,
	"tanh":       mathTanh,
	// Lua 5.3 functions
	"tointeger":  mathToInteger,
	"type":       mathType,
	"ult":        mathUlt,
	// Bitwise operations (Lua 5.3)
	"band":       mathBand,
	"bor":        mathBor,
	"bxor":       mathBxor,
	"bnot":       mathBnot,
	"lshift":     mathLshift,
	"rshift":     mathRshift,
	"extract":    mathExtract,
	"replace":    mathReplace,
}

func mathAbs(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(v.Abs())
	return 1
}

func mathAcos(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Acos(v.Float64())))
	return 1
}

func mathAsin(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Asin(v.Float64())))
	return 1
}

func mathAtan(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Atan(v.Float64())))
	return 1
}

func mathAtan2(L *LState) int {
	v1 := L.CheckNumber(1)
	v2 := L.CheckNumber(2)
	L.Push(LNumberFloat(math.Atan2(v1.Float64(), v2.Float64())))
	return 1
}

func mathCeil(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(v.Ceil())
	return 1
}

func mathCos(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Cos(v.Float64())))
	return 1
}

func mathCosh(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Cosh(v.Float64())))
	return 1
}

func mathDeg(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(v.Float64() * 180 / math.Pi))
	return 1
}

func mathExp(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Exp(v.Float64())))
	return 1
}

func mathFloor(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(v.Floor())
	return 1
}

func mathFmod(L *LState) int {
	v1 := L.CheckNumber(1)
	v2 := L.CheckNumber(2)
	// fmod is different from modulo - it's the remainder of division
	r := math.Mod(v1.Float64(), v2.Float64())
	L.Push(LNumberFloat(r))
	return 1
}

func mathFrexp(L *LState) int {
	v := L.CheckNumber(1)
	f1, f2 := math.Frexp(v.Float64())
	L.Push(LNumberFloat(f1))
	L.Push(LNumberInt(int64(f2)))
	return 2
}

func mathLdexp(L *LState) int {
	v := L.CheckNumber(1)
	exp := L.CheckInt(2)
	L.Push(LNumberFloat(math.Ldexp(v.Float64(), exp)))
	return 1
}

func mathLog(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Log(v.Float64())))
	return 1
}

func mathLog10(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Log10(v.Float64())))
	return 1
}

func mathMax(L *LState) int {
	if L.GetTop() == 0 {
		L.RaiseError("wrong number of arguments")
	}
	max := L.CheckNumber(1)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		v := L.CheckNumber(i)
		if v.Float64() > max.Float64() {
			max = v
		}
	}
	L.Push(max)
	return 1
}

func mathMin(L *LState) int {
	if L.GetTop() == 0 {
		L.RaiseError("wrong number of arguments")
	}
	min := L.CheckNumber(1)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		v := L.CheckNumber(i)
		if v.Float64() < min.Float64() {
			min = v
		}
	}
	L.Push(min)
	return 1
}

func mathMod(L *LState) int {
	lhs := L.CheckNumber(1)
	rhs := L.CheckNumber(2)
	L.Push(lhs.Mod(rhs))
	return 1
}

func mathModf(L *LState) int {
	v := L.CheckNumber(1)
	f := v.Float64()
	intPart, fracPart := math.Modf(f)
	
	// Return integer part as integer if possible
	var intLNum LNumber
	if intPart == float64(int64(intPart)) {
		intLNum = LNumberInt(int64(intPart))
	} else {
		intLNum = LNumberFloat(intPart)
	}
	
	L.Push(intLNum)
	L.Push(LNumberFloat(fracPart))
	return 2
}

func mathPow(L *LState) int {
	v1 := L.CheckNumber(1)
	v2 := L.CheckNumber(2)
	L.Push(v1.Pow(v2))
	return 1
}

func mathRad(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(v.Float64() * math.Pi / 180))
	return 1
}

func mathRandom(L *LState) int {
	switch L.GetTop() {
	case 0:
		L.Push(LNumberFloat(rand.Float64()))
	case 1:
		n := L.CheckInt(1)
		L.Push(LNumberInt(int64(rand.Intn(n) + 1)))
	default:
		min := L.CheckInt(1)
		max := L.CheckInt(2) + 1
		L.Push(LNumberInt(int64(rand.Intn(max-min) + min)))
	}
	return 1
}

func mathRandomseed(L *LState) int {
	rand.Seed(L.CheckInt64(1))
	return 0
}

func mathSin(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Sin(v.Float64())))
	return 1
}

func mathSinh(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Sinh(v.Float64())))
	return 1
}

func mathSqrt(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Sqrt(v.Float64())))
	return 1
}

func mathTan(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Tan(v.Float64())))
	return 1
}

func mathTanh(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Tanh(v.Float64())))
	return 1
}

// Lua 5.3 functions

// math.tointeger converts a number to integer if possible
func mathToInteger(L *LState) int {
	v := L.CheckNumber(1)
	if v.IsInteger() {
		L.Push(v)
	} else {
		f := v.Float64()
		if f == float64(int64(f)) && !math.IsInf(f, 0) && !math.IsNaN(f) {
			L.Push(LNumberInt(int64(f)))
		} else {
			L.Push(LNil)
		}
	}
	return 1
}

// math.type returns "integer" or "float"
func mathType(L *LState) int {
	v := L.CheckAny(1)
	if num, ok := v.(LNumber); ok {
		if num.IsInteger() {
			L.Push(LString("integer"))
		} else {
			L.Push(LString("float"))
		}
	} else {
		L.RaiseError("number expected, got %s", v.Type().String())
	}
	return 1
}

// math.ult returns true if a < b when compared as unsigned integers
func mathUlt(L *LState) int {
	a := L.CheckNumber(1)
	b := L.CheckNumber(2)

	// Convert to uint64 for unsigned comparison
	// This handles negative numbers correctly as large positive values
	ua := uint64(a.Int64())
	ub := uint64(b.Int64())

	if ua < ub {
		L.Push(LTrue)
	} else {
		L.Push(LFalse)
	}
	return 1
}

// Bitwise operations (Lua 5.3)

// math.band performs bitwise AND
func mathBand(L *LState) int {
	if L.GetTop() < 2 {
		L.RaiseError("wrong number of arguments")
	}
	result := L.CheckNumber(1)
	for i := 2; i <= L.GetTop(); i++ {
		result = result.Band(L.CheckNumber(i))
	}
	L.Push(result)
	return 1
}

// math.bor performs bitwise OR
func mathBor(L *LState) int {
	if L.GetTop() < 2 {
		L.RaiseError("wrong number of arguments")
	}
	result := L.CheckNumber(1)
	for i := 2; i <= L.GetTop(); i++ {
		result = result.Bor(L.CheckNumber(i))
	}
	L.Push(result)
	return 1
}

// math.bxor performs bitwise XOR
func mathBxor(L *LState) int {
	if L.GetTop() < 2 {
		L.RaiseError("wrong number of arguments")
	}
	result := L.CheckNumber(1)
	for i := 2; i <= L.GetTop(); i++ {
		result = result.Bxor(L.CheckNumber(i))
	}
	L.Push(result)
	return 1
}

// math.bnot performs bitwise NOT
func mathBnot(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(v.Bnot())
	return 1
}

// math.lshift performs bitwise left shift
func mathLshift(L *LState) int {
	v := L.CheckNumber(1)
	shift := L.CheckNumber(2)
	L.Push(v.Shl(shift))
	return 1
}

// math.rshift performs bitwise right shift
func mathRshift(L *LState) int {
	v := L.CheckNumber(1)
	shift := L.CheckNumber(2)
	L.Push(v.Shr(shift))
	return 1
}

// math.extract extracts bits from a number
func mathExtract(L *LState) int {
	v := L.CheckNumber(1)
	field := L.CheckInt(2)
	width := L.OptInt(3, 1)
	
	if field < 0 || width < 1 || field+width > 64 {
		L.RaiseError("invalid field width")
	}
	
	mask := (int64(1) << uint(width)) - 1
	result := (v.Int64() >> uint(field)) & mask
	L.Push(LNumberInt(result))
	return 1
}

// math.replace replaces bits in a number
func mathReplace(L *LState) int {
	v := L.CheckNumber(1)
	value := L.CheckNumber(2)
	field := L.CheckInt(3)
	width := L.OptInt(4, 1)
	
	if field < 0 || width < 1 || field+width > 64 {
		L.RaiseError("invalid field width")
	}
	
	mask := (int64(1) << uint(width)) - 1
	maskedValue := value.Int64() & mask
	maskedV := v.Int64() &^ (mask << uint(field))
	result := maskedV | (maskedValue << uint(field))
	L.Push(LNumberInt(result))
	return 1
}
