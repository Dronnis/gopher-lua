package lua

import (
	"math"
	"math/rand"
)

func OpenMath(L *LState) int {
	mod := L.RegisterModule(MathLibName, mathFuncs).(*LTable)
	mod.RawSetString("pi", LNumberFloat(math.Pi))
	mod.RawSetString("huge", LNumberFloat(math.Inf(1)))
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
	"tointeger": mathToInteger,
	"type":      mathType,
	"ult":       mathUlt,
	// Bitwise operations (Lua 5.3)
	"band":    mathBand,
	"bor":     mathBor,
	"bxor":    mathBxor,
	"bnot":    mathBnot,
	"lshift":  mathLshift,
	"rshift":  mathRshift,
	"extract": mathExtract,
	"replace": mathReplace,
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
	// Lua 5.3: math.atan can accept optional second argument
	// math.atan(y) returns atan(y)
	// math.atan(y, x) returns atan2(y, x)
	if L.GetTop() >= 2 {
		v2 := L.CheckNumber(2)
		L.Push(LNumberFloat(math.Atan2(v.Float64(), v2.Float64())))
	} else {
		L.Push(LNumberFloat(math.Atan(v.Float64())))
	}
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
	// Lua 5.3: if both arguments are integers, return integer
	if v1.IsInteger() && v2.IsInteger() {
		i1 := v1.Int64()
		i2 := v2.Int64()
		if i2 == 0 {
			L.RaiseError("zero")
		}
		// Integer fmod: same as C fmod for integers
		result := i1 % i2
		L.Push(LNumberInt(result))
	} else {
		r := math.Mod(v1.Float64(), v2.Float64())
		L.Push(LNumberFloat(r))
	}
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
	base := L.OptNumber(2, LNumberFloat(math.E)) // Default to natural log
	// Lua 5.3: math.log(x, base) = log(x) / log(base)
	result := math.Log(v.Float64()) / math.Log(base.Float64())
	L.Push(LNumberFloat(result))
	return 1
}

func mathLog10(L *LState) int {
	v := L.CheckNumber(1)
	L.Push(LNumberFloat(math.Log10(v.Float64())))
	return 1
}

func mathMax(L *LState) int {
	if L.GetTop() == 0 {
		L.RaiseError("value expected")
	}
	max := L.CheckNumber(1)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		v := L.CheckNumber(i)
		// Lua 5.3: preserve integer type when possible
		if max.IsInteger() && v.IsInteger() {
			if v.Int64() > max.Int64() {
				max = v
			}
		} else {
			if v.Float64() > max.Float64() {
				max = v
			}
		}
	}
	L.Push(max)
	return 1
}

func mathMin(L *LState) int {
	if L.GetTop() == 0 {
		L.RaiseError("value expected")
	}
	min := L.CheckNumber(1)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		v := L.CheckNumber(i)
		// Lua 5.3: preserve integer type when possible
		if min.IsInteger() && v.IsInteger() {
			if v.Int64() < min.Int64() {
				min = v
			}
		} else {
			if v.Float64() < min.Float64() {
				min = v
			}
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

	// Handle special cases: NaN and Inf
	if math.IsNaN(f) {
		L.Push(LNumberFloat(math.NaN()))
		L.Push(LNumberFloat(math.NaN()))
		return 2
	}
	if math.IsInf(f, 0) {
		L.Push(LNumberFloat(f))
		L.Push(LNumberFloat(0.0))
		return 2
	}

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
	// Lua 5.3: math.random() accepts 0, 1, or 2 arguments
	if L.GetTop() > 2 {
		L.ArgError(3, "wrong number of arguments")
		return 0
	}
	switch L.GetTop() {
	case 0:
		L.Push(LNumberFloat(rand.Float64()))
	case 1:
		n := L.CheckInt64(1)
		if n <= 0 {
			L.ArgError(1, "interval is empty")
			return 0
		}
		// Use Int63n for positive numbers, handle large values
		if n > math.MaxInt64 {
			// For very large numbers, use float64 approximation
			L.Push(LNumberInt(int64(rand.Float64()*float64(n)) + 1))
		} else {
			L.Push(LNumberInt(rand.Int63n(n) + 1))
		}
	default:
		min := L.CheckInt64(1)
		max := L.CheckInt64(2)
		if max < min {
			L.ArgError(2, "interval is empty")
			return 0
		}
		// Lua 5.3: reject intervals that are too large for uniform distribution
		// The range (max - min) must fit in a positive int64
		if min < 0 && max >= 0 {
			// Range crosses zero (or is [minint, 0]). The range is max - min + 1.
			// Special case: [minint, 0] is too large
			if min == math.MinInt64 && max == 0 {
				L.ArgError(1, "interval too large")
				return 0
			}
			// For other cases where max > 0, check if the range overflows
			if max > 0 {
				// This overflows if max > math.MaxInt64 + min (i.e., max + (-min) > math.MaxInt64)
				// Since min < 0, -min > 0. We need to check if max + (-min) > math.MaxInt64.
				// But -min can overflow if min == math.MinInt64 (handled above).
				if min == math.MinInt64 {
					// Already handled [minint, 0] above, so this is [minint, -1] or similar
					// [minint, -1] is handled specially below
					L.ArgError(1, "interval too large")
					return 0
				}
				// Now -min is safe (doesn't overflow)
				if max > math.MaxInt64+min {
					// max - min > math.MaxInt64
					L.ArgError(1, "interval too large")
					return 0
				}
			}
		}
		// Handle the full int64 range specially
		if min == math.MinInt64 && max == math.MaxInt64 {
			// For full range, use random bytes
			var buf [8]byte
			rand.Read(buf[:])
			L.Push(LNumberInt(int64(buf[0]) | int64(buf[1])<<8 | int64(buf[2])<<16 |
				int64(buf[3])<<24 | int64(buf[4])<<32 | int64(buf[5])<<40 |
				int64(buf[6])<<48 | int64(buf[7])<<56))
		} else if min == 0 && max == math.MaxInt64 {
			// Special case for [0, maxint]: max+1 overflows, so use Int63() directly
			L.Push(LNumberInt(rand.Int63()))
		} else if min == math.MinInt64 && max == -1 {
			// Special case for [minint, -1]: use random negative number
			// -minint overflows, so use Int63() and negate
			L.Push(LNumberInt(-rand.Int63() - 1))
		} else {
			// Calculate range carefully to avoid overflow
			range_ := max - min
			if range_ < 0 {
				// Overflow in subtraction, use two-step approach
				// Generate random sign and magnitude
				if rand.Float64() < 0.5 {
					L.Push(LNumberInt(rand.Int63n(-min+1) + min))
				} else {
					L.Push(LNumberInt(rand.Int63n(max + 1)))
				}
			} else if range_ > math.MaxInt64/2 {
				// Very large range, use combination of Int63n calls
				// Split range into two halves
				half := range_ / 2
				if rand.Float64() < 0.5 {
					L.Push(LNumberInt(rand.Int63n(half+1) + min))
				} else {
					L.Push(LNumberInt(rand.Int63n(range_-half) + min + half + 1))
				}
			} else {
				// Normal case: range_ + 1 to include max
				L.Push(LNumberInt(rand.Int63n(range_+1) + min))
			}
		}
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
	// Lua 5.3: accepts numbers and strings, returns nil for invalid values
	arg := L.Get(1)

	// Try to convert to number first (handles both numbers and numeric strings)
	var v LNumber
	switch val := arg.(type) {
	case LNumber:
		v = val
	case LString:
		// Try to parse string as number
		num, err := parseNumber(string(val))
		if err != nil {
			L.Push(LNil)
			return 1
		}
		v = num
	default:
		// Not a number or string, return nil
		L.Push(LNil)
		return 1
	}

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

// math.type returns "integer", "float", or nil
// Lua 5.3: returns nil for non-numeric values (doesn't raise error)
func mathType(L *LState) int {
	v := L.CheckAny(1)
	if num, ok := v.(LNumber); ok {
		if num.IsInteger() {
			L.Push(LString("integer"))
		} else {
			L.Push(LString("float"))
		}
	} else {
		// Return nil for non-numeric values (Lua 5.3 behavior)
		L.Push(LNil)
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
