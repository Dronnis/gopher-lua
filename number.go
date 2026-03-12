package lua

import (
	"fmt"
	"math"
	"strconv"
)

// LNumberInt creates an integer LNumber
func LNumberInt(v int64) LNumber {
	return LNumber{value: luaIntegerType(v)}
}

// LNumberFloat creates a float LNumber
func LNumberFloat(v float64) LNumber {
	// Normalize -0.0 to 0.0 for consistent table key behavior
	// In Lua, -0.0 == 0.0 and they should be the same table key
	if v == 0 && math.Signbit(v) {
		return LNumber{value: luaFloatType(0)}
	}
	return LNumber{value: luaFloatType(v)}
}

// LNumberFromInterface creates an LNumber from an interface{} value
func LNumberFromInterface(v interface{}) LNumber {
	switch val := v.(type) {
	case int:
		return LNumberInt(int64(val))
	case int8:
		return LNumberInt(int64(val))
	case int16:
		return LNumberInt(int64(val))
	case int32:
		return LNumberInt(int64(val))
	case int64:
		return LNumberInt(val)
	case uint:
		return LNumberInt(int64(val))
	case uint8:
		return LNumberInt(int64(val))
	case uint16:
		return LNumberInt(int64(val))
	case uint32:
		return LNumberInt(int64(val))
	case uint64:
		if val <= math.MaxInt64 {
			return LNumberInt(int64(val))
		}
		return LNumberFloat(float64(val))
	case float32:
		return LNumberFloat(float64(val))
	case float64:
		// In Lua 5.3, float values stay as float even if they represent whole numbers
		// This preserves the type information from the source
		return LNumberFloat(val)
	case luaIntegerType:
		return LNumber{value: val}
	case luaFloatType:
		return LNumber{value: val}
	case LNumber:
		return val
	default:
		return LNumberInt(0)
	}
}

// Kind returns the kind of number (integer or float)
func (nm LNumber) Kind() NumberKind {
	switch nm.value.(type) {
	case luaIntegerType:
		return NumberKindInt
	case luaFloatType:
		return NumberKindFloat
	default:
		return NumberKindFloat
	}
}

// IsInteger returns true if the number is an integer
func (nm LNumber) IsInteger() bool {
	_, ok := nm.value.(luaIntegerType)
	return ok
}

// Int64 returns the int64 value of the number
func (nm LNumber) Int64() int64 {
	switch v := nm.value.(type) {
	case luaIntegerType:
		return int64(v)
	case luaFloatType:
		return int64(v)
	default:
		return 0
	}
}

// Float64 returns the float64 value of the number
func (nm LNumber) Float64() float64 {
	if nm.value == nil {
		return 0.0
	}
	switch v := nm.value.(type) {
	case luaIntegerType:
		return float64(v)
	case luaFloatType:
		return float64(v)
	default:
		return 0.0
	}
}

// String returns the string representation of the number
func (nm LNumber) String() string {
	if nm.value == nil {
		return "0"
	}
	switch v := nm.value.(type) {
	case luaIntegerType:
		return strconv.FormatInt(int64(v), 10)
	case luaFloatType:
		// Use %g format for floats (like Lua 5.3)
		s := strconv.FormatFloat(float64(v), 'g', -1, 64)
		// Ensure there's a decimal point or exponent for floats
		if !math.IsInf(float64(v), 0) && !math.IsNaN(float64(v)) {
			hasDecimal := false
			hasExponent := false
			for _, c := range s {
				if c == '.' {
					hasDecimal = true
				}
				if c == 'e' || c == 'E' {
					hasExponent = true
				}
			}
			// If it's a whole number without decimal point or exponent, add .0
			if !hasDecimal && !hasExponent {
				s = s + ".0"
			}
		}
		return s
	default:
		return "0"
	}
}

// Type returns the LValueType
func (nm LNumber) Type() LValueType { return LTNumber }

// fmt.Formatter interface
func (nm LNumber) Format(f fmt.State, c rune) {
	switch c {
	case 'q', 's':
		defaultFormat(nm.String(), f, c)
	case 'b', 'c', 'd', 'o', 'x', 'X', 'U':
		defaultFormat(nm.Int64(), f, c)
	case 'e', 'E', 'f', 'F', 'g', 'G':
		defaultFormat(nm.Float64(), f, c)
	case 'i':
		defaultFormat(nm.Int64(), f, 'd')
	default:
		if nm.IsInteger() {
			defaultFormat(nm.Int64(), f, c)
		} else {
			defaultFormat(nm.Float64(), f, c)
		}
	}
}

// Equal checks if two LNumbers are equal
func (nm LNumber) Equal(other LNumber) bool {
	if nm.IsInteger() && other.IsInteger() {
		return nm.Int64() == other.Int64()
	}
	f1 := nm.Float64()
	f2 := other.Float64()
	// Handle infinity and NaN properly
	if math.IsInf(f1, 0) || math.IsInf(f2, 0) {
		// Both must be the same infinity
		return math.IsInf(f1, 0) && math.IsInf(f2, 0) && math.Signbit(f1) == math.Signbit(f2)
	}
	if math.IsNaN(f1) || math.IsNaN(f2) {
		// NaN is never equal to anything, including itself
		return false
	}
	return f1 == f2
}

// Compare compares two LNumbers. Returns -1, 0, or 1
// For NaN comparisons, returns 0 (special case for Lua semantics)
func (nm LNumber) Compare(other LNumber) int {
	// For integers, use integer comparison to avoid float precision loss
	if nm.IsInteger() && other.IsInteger() {
		a, b := nm.Int64(), other.Int64()
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	}
	
	// For floats or mixed, use float comparison
	f1 := nm.Float64()
	f2 := other.Float64()

	// Handle infinity
	if math.IsInf(f1, 1) {  // f1 is +inf
		if math.IsInf(f2, 1) {
			return 0  // +inf == +inf
		}
		return 1  // +inf > anything
	}
	if math.IsInf(f1, -1) {  // f1 is -inf
		if math.IsInf(f2, -1) {
			return 0  // -inf == -inf
		}
		return -1  // -inf < anything
	}
	if math.IsInf(f2, 1) {  // f2 is +inf
		return -1  // anything < +inf
	}
	if math.IsInf(f2, -1) {  // f2 is -inf
		return 1  // anything > -inf
	}

	// NaN comparisons are special - they always return false
	if math.IsNaN(f1) || math.IsNaN(f2) {
		return 0 // This will make all comparisons return false
	}

	if f1 < f2 {
		return -1
	} else if f1 > f2 {
		return 1
	}
	return 0
}

// ToInteger converts the number to integer if possible
func (nm LNumber) ToInteger() (int64, bool) {
	if nm.IsInteger() {
		return nm.Int64(), true
	}
	f := nm.Float64()
	if f == float64(int64(f)) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return int64(f), true
	}
	return 0, false
}

// ToFloat converts the number to float
func (nm LNumber) ToFloat() float64 {
	return nm.Float64()
}

// Floor returns the floor of the number as integer
func (nm LNumber) Floor() LNumber {
	if nm.IsInteger() {
		return nm
	}
	return LNumberInt(int64(math.Floor(nm.Float64())))
}

// Ceil returns the ceiling of the number as integer
func (nm LNumber) Ceil() LNumber {
	if nm.IsInteger() {
		return nm
	}
	return LNumberInt(int64(math.Ceil(nm.Float64())))
}

// Abs returns the absolute value
func (nm LNumber) Abs() LNumber {
	if nm.IsInteger() {
		v := nm.Int64()
		if v < 0 {
			return LNumberInt(-v)
		}
		return nm
	}
	return LNumberFloat(math.Abs(nm.Float64()))
}

// UnaryMinus returns the negation of the number
// Lua 5.3: integer negation wraps around (two's complement)
func (nm LNumber) UnaryMinus() LNumber {
	if nm.IsInteger() {
		a := nm.Int64()
		// Lua 5.3: wrap around on overflow (two's complement arithmetic)
		// Use uint64 for proper wrap-around: -minint = minint
		result := uint64(0) - uint64(a)
		return LNumberInt(int64(result))
	}
	return LNumberFloat(-nm.Float64())
}

// Add adds two numbers following Lua 5.3 semantics
// Lua 5.3: integer overflow wraps around (two's complement)
func (nm LNumber) Add(other LNumber) LNumber {
	if nm.IsInteger() && other.IsInteger() {
		a, b := nm.Int64(), other.Int64()
		// Lua 5.3: wrap around on overflow (two's complement arithmetic)
		// Use uint64 for proper wrap-around behavior
		result := uint64(a) + uint64(b)
		return LNumberInt(int64(result))
	}
	return LNumberFloat(nm.Float64() + other.Float64())
}

// Sub subtracts two numbers following Lua 5.3 semantics
// Lua 5.3: integer overflow wraps around (two's complement)
func (nm LNumber) Sub(other LNumber) LNumber {
	if nm.IsInteger() && other.IsInteger() {
		a, b := nm.Int64(), other.Int64()
		// Lua 5.3: wrap around on overflow (two's complement arithmetic)
		// Use uint64 for proper wrap-around behavior
		result := uint64(a) - uint64(b)
		return LNumberInt(int64(result))
	}
	return LNumberFloat(nm.Float64() - other.Float64())
}

// Mul multiplies two numbers following Lua 5.3 semantics
// Lua 5.3: integer multiplication uses wrap-around (modular arithmetic mod 2^64)
func (nm LNumber) Mul(other LNumber) LNumber {
	if nm.IsInteger() && other.IsInteger() {
		a, b := nm.Int64(), other.Int64()
		// Lua 5.3: pure wrap-around multiplication (mod 2^64)
		// Use uint64 for proper two's complement wrap-around
		result := uint64(a) * uint64(b)
		return LNumberInt(int64(result))
	}
	return LNumberFloat(nm.Float64() * other.Float64())
}

// Div divides two numbers following Lua 5.3 semantics (always returns float)
func (nm LNumber) Div(other LNumber) LNumber {
	a := nm.Float64()
	b := other.Float64()
	// Lua 5.3: division by zero returns inf (not error)
	if b == 0 {
		if a == 0 {
			return LNumberFloat(math.NaN())  // 0/0 = NaN
		}
		// Return +inf or -inf based on signs
		if (a < 0) != (b < 0) {
			return LNumberFloat(math.Inf(-1))
		}
		return LNumberFloat(math.Inf(1))
	}
	result := a / b
	return LNumberFloat(result)
}

// IDiv performs integer division following Lua 5.3 semantics (floor division)
func (nm LNumber) IDiv(other LNumber) LNumber {
	a := nm.Float64()
	b := other.Float64()
	// Lua 5.3: division by zero returns inf
	if b == 0 {
		if a == 0 {
			return LNumberFloat(math.NaN())  // 0/0 = NaN
		}
		// Return +inf or -inf based on signs
		if (a < 0) != (b < 0) {
			return LNumberFloat(math.Inf(-1))
		}
		return LNumberFloat(math.Inf(1))
	}
	result := a / b
	floored := math.Floor(result)
	// Return integer if the result is a whole number and fits in int64
	if floored == float64(int64(floored)) && !math.IsInf(floored, 0) {
		return LNumberInt(int64(floored))
	}
	return LNumberFloat(floored)
}

// Mod performs modulo following Lua 5.3 semantics
func (nm LNumber) Mod(other LNumber) LNumber {
	if nm.IsInteger() && other.IsInteger() {
		a, b := nm.Int64(), other.Int64()
		if b == 0 {
			return LNumberFloat(math.NaN())
		}
		r := a % b
		if r != 0 && (b > 0) != (r > 0) {
			r += b
		}
		return LNumberInt(r)
	}
	flhs := nm.Float64()
	frhs := other.Float64()
	v := math.Mod(flhs, frhs)
	if frhs > 0 && v < 0 || frhs < 0 && v > 0 {
		v += frhs
	}
	return LNumberFloat(v)
}

// Bitwise operations follow Lua 5.3 semantics
// All bitwise operations convert operands to integers

// Band performs bitwise AND
func (nm LNumber) Band(other LNumber) LNumber {
	return LNumberInt(nm.Int64() & other.Int64())
}

// Bor performs bitwise OR
func (nm LNumber) Bor(other LNumber) LNumber {
	return LNumberInt(nm.Int64() | other.Int64())
}

// Bxor performs bitwise XOR
func (nm LNumber) Bxor(other LNumber) LNumber {
	return LNumberInt(nm.Int64() ^ other.Int64())
}

// Shl performs bitwise left shift
// Lua 5.3: shift amounts >= 64 result in 0
func (nm LNumber) Shl(other LNumber) LNumber {
	shift := other.Int64()
	if shift >= 64 || shift <= -64 {
		return LNumberInt(0)
	}
	if shift < 0 {
		// Negative shift = right shift
		return LNumberInt(nm.Int64() >> uint(-shift))
	}
	return LNumberInt(nm.Int64() << uint(shift))
}

// Shr performs bitwise right shift
// Lua 5.3: shift amounts >= 64 result in 0 (for unsigned) or sign extension (for signed)
func (nm LNumber) Shr(other LNumber) LNumber {
	shift := other.Int64()
	if shift >= 64 || shift <= -64 {
		return LNumberInt(0)
	}
	if shift < 0 {
		// Negative shift = left shift
		return LNumberInt(nm.Int64() << uint(-shift))
	}
	return LNumberInt(nm.Int64() >> uint(shift))
}

// Bnot performs bitwise NOT
func (nm LNumber) Bnot() LNumber {
	return LNumberInt(^nm.Int64())
}

// Pow raises a number to a power following Lua 5.3 semantics
func (nm LNumber) Pow(other LNumber) LNumber {
	result := math.Pow(nm.Float64(), other.Float64())
	// If both are integers and result is integer, return integer
	if nm.IsInteger() && other.IsInteger() && other.Int64() >= 0 {
		if result == float64(int64(result)) && !math.IsInf(result, 0) && !math.IsNaN(result) {
			return LNumberInt(int64(result))
		}
	}
	return LNumberFloat(result)
}

// Unm is alias for UnaryMinus
func (nm LNumber) Unm() LNumber {
	return nm.UnaryMinus()
}

// IDivInt performs integer division and returns both quotient and remainder
func (nm LNumber) IDivInt(other LNumber) (LNumber, LNumber) {
	if nm.IsInteger() && other.IsInteger() {
		q := nm.Int64() / other.Int64()
		r := nm.Int64() % other.Int64()
		return LNumberInt(q), LNumberInt(r)
	}
	q := int64(math.Floor(nm.Float64() / other.Float64()))
	r := nm.Float64() - float64(q)*other.Float64()
	return LNumberInt(q), LNumberFloat(r)
}
