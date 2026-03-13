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
	// Lua 5.3: integers and floats can be equal if they represent the same mathematical value
	// But we need to be careful about precision
	
	// If both are integers, compare as integers
	if nm.IsInteger() && other.IsInteger() {
		return nm.Int64() == other.Int64()
	}
	
	// If one is integer and one is float, we need special handling
	if nm.IsInteger() && !other.IsInteger() {
		// nm is integer, other is float
		intVal := nm.Int64()
		floatVal := other.Float64()
		
		// Check if float can represent the integer exactly
		if math.IsInf(floatVal, 0) || math.IsNaN(floatVal) {
			return false
		}
		
		// Check if converting float back to int gives the same value
		if floatVal != math.Trunc(floatVal) {
			return false // float has fractional part
		}
		
		// Check if the float can exactly represent this integer
		// For large integers, float64 may lose precision
		convertedBack := int64(floatVal)
		return intVal == convertedBack && float64(convertedBack) == floatVal
	}
	
	if !nm.IsInteger() && other.IsInteger() {
		// nm is float, other is integer
		floatVal := nm.Float64()
		intVal := other.Int64()
		
		// Check if float can represent the integer exactly
		if math.IsInf(floatVal, 0) || math.IsNaN(floatVal) {
			return false
		}
		
		// Check if converting float back to int gives the same value
		if floatVal != math.Trunc(floatVal) {
			return false // float has fractional part
		}
		
		// Check if the float can exactly represent this integer
		// For large integers, float64 may lose precision
		convertedBack := int64(floatVal)
		return intVal == convertedBack && float64(convertedBack) == floatVal
	}
	
	// Both are floats
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
	
	// For mixed types, we need to be careful about precision
	if nm.IsInteger() && !other.IsInteger() {
		// nm is integer, other is float
		intVal := nm.Int64()
		floatVal := other.Float64()
		
		// Handle special float values
		if math.IsInf(floatVal, 1) {
			return -1 // integer < +inf
		}
		if math.IsInf(floatVal, -1) {
			return 1 // integer > -inf
		}
		if math.IsNaN(floatVal) {
			return 0 // NaN comparisons are false
		}
		
		// Check if float has fractional part
		if floatVal != math.Trunc(floatVal) {
			// Float has fractional part, use float comparison
			intAsFloat := float64(intVal)
			if intAsFloat < floatVal {
				return -1
			} else if intAsFloat > floatVal {
				return 1
			}
			return 0
		}
		
		// Float is a whole number, check if it can be represented as int64
		if floatVal >= float64(math.MinInt64) && floatVal <= float64(math.MaxInt64) {
			// Try to convert float to integer
			floatAsInt := int64(floatVal)
			// Check if conversion is exact
			if float64(floatAsInt) == floatVal {
				// Exact conversion, compare as integers
				if intVal < floatAsInt {
					return -1
				} else if intVal > floatAsInt {
					return 1
				}
				return 0
			}
		}
		
		// Float cannot be exactly represented as integer, or is out of range
		// Use float comparison, but be aware of precision issues
		intAsFloat := float64(intVal)
		if intAsFloat < floatVal {
			return -1
		} else if intAsFloat > floatVal {
			return 1
		}
		
		// They are equal as floats, but we need to check if this is due to precision loss
		// If the float is very large and integer is at the boundary, assume integer < float
		if math.Abs(floatVal) >= (1<<53) && intVal == math.MaxInt64 {
			// Special case: maxint compared to a large positive float
			// The float likely represents a value > maxint but was rounded down
			return -1
		}
		if math.Abs(floatVal) >= (1<<53) && intVal == math.MinInt64 {
			// Special case: minint compared to a large negative float
			// The float likely represents a value < minint but was rounded up
			return 1
		}
		
		return 0
	}
	
	if !nm.IsInteger() && other.IsInteger() {
		// nm is float, other is integer - reverse the logic
		result := LNumberInt(other.Int64()).Compare(LNumberFloat(nm.Float64()))
		return -result // reverse the result
	}
	
	// Both are floats
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

// Floor returns the floor of the number as integer if possible, otherwise as float
func (nm LNumber) Floor() LNumber {
	if nm.IsInteger() {
		return nm
	}
	floored := math.Floor(nm.Float64())
	// Check if the result fits in int64
	// Use strict comparison to avoid overflow during conversion
	// math.MaxInt64 as float64 is 9.223372036854776e+18, which is slightly larger than actual MaxInt64
	// So we need to check if the floored value can be safely converted
	if !math.IsInf(floored, 0) && !math.IsNaN(floored) {
		// Try to convert and check if it's safe
		if floored >= -9.223372036854775808e+18 && floored < 9.223372036854775808e+18 {
			intVal := int64(floored)
			// Verify the conversion didn't overflow
			if float64(intVal) == floored {
				return LNumberInt(intVal)
			}
		}
	}
	return LNumberFloat(floored)
}

// Ceil returns the ceiling of the number as integer if possible, otherwise as float
func (nm LNumber) Ceil() LNumber {
	if nm.IsInteger() {
		return nm
	}
	ceiled := math.Ceil(nm.Float64())
	// Check if the result fits in int64
	// Use strict comparison to avoid overflow during conversion
	if !math.IsInf(ceiled, 0) && !math.IsNaN(ceiled) {
		// Try to convert and check if it's safe
		if ceiled >= -9.223372036854775808e+18 && ceiled < 9.223372036854775808e+18 {
			intVal := int64(ceiled)
			// Verify the conversion didn't overflow
			if float64(intVal) == ceiled {
				return LNumberInt(intVal)
			}
		}
	}
	return LNumberFloat(ceiled)
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
// Lua 5.3: floor division preserves the type of operands - if either operand is float, result is float
func (nm LNumber) IDiv(other LNumber) LNumber {
	// Special case: if both are integers, do integer arithmetic to avoid precision loss
	if nm.IsInteger() && other.IsInteger() {
		a := nm.Int64()
		b := other.Int64()
		
		// Division by zero
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
		
		// Integer floor division
		q := a / b
		// Adjust for floor division (towards negative infinity)
		if a%b != 0 && (a < 0) != (b < 0) {
			q--
		}
		return LNumberInt(q)
	}
	
	// At least one operand is float, use float arithmetic
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
// Lua 5.3: logical right shift (zero-fill, not sign-extending)
func (nm LNumber) Shr(other LNumber) LNumber {
	shift := other.Int64()
	if shift >= 64 {
		return LNumberInt(0) // All bits become 0 for large shifts
	}
	if shift <= -64 {
		return LNumberInt(0)
	}
	if shift < 0 {
		// Negative shift = left shift
		return LNumberInt(nm.Int64() << uint(-shift))
	}
	// Logical right shift: treat as unsigned for the shift operation
	uval := uint64(nm.Int64())
	result := uval >> uint(shift)
	return LNumberInt(int64(result))
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
