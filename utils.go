package lua

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

func intMin(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func intMax(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func defaultFormat(v interface{}, f fmt.State, c rune) {
	buf := make([]string, 0, 10)
	buf = append(buf, "%")
	for i := 0; i < 128; i++ {
		if f.Flag(i) {
			buf = append(buf, string(rune(i)))
		}
	}

	if w, ok := f.Width(); ok {
		buf = append(buf, strconv.Itoa(w))
	}
	if p, ok := f.Precision(); ok {
		buf = append(buf, "."+strconv.Itoa(p))
	}
	buf = append(buf, string(c))
	format := strings.Join(buf, "")
	fmt.Fprintf(f, format, v)
}

type flagScanner struct {
	flag       byte
	start      string
	end        string
	buf        []byte
	str        string
	Length     int
	Pos        int
	HasFlag    bool
	ChangeFlag bool
}

func newFlagScanner(flag byte, start, end, str string) *flagScanner {
	return &flagScanner{flag, start, end, make([]byte, 0, len(str)), str, len(str), 0, false, false}
}

func (fs *flagScanner) AppendString(str string) { fs.buf = append(fs.buf, str...) }

func (fs *flagScanner) AppendChar(ch byte) { fs.buf = append(fs.buf, ch) }

func (fs *flagScanner) String() string { return string(fs.buf) }

func (fs *flagScanner) Next() (byte, bool) {
	c := byte('\000')
	fs.ChangeFlag = false
	if fs.Pos == fs.Length {
		if fs.HasFlag {
			fs.AppendString(fs.end)
		}
		return c, true
	} else {
		c = fs.str[fs.Pos]
		if c == fs.flag {
			if fs.Pos < (fs.Length-1) && fs.str[fs.Pos+1] == fs.flag {
				fs.HasFlag = false
				fs.AppendChar(fs.flag)
				fs.Pos += 2
				return fs.Next()
			} else if fs.Pos != fs.Length-1 {
				if fs.HasFlag {
					fs.AppendString(fs.end)
				}
				fs.AppendString(fs.start)
				fs.ChangeFlag = true
				fs.HasFlag = true
			}
		}
	}
	fs.Pos++
	return c, false
}

var cDateFlagToGo = map[byte]string{
	'a': "mon", 'A': "Monday", 'b': "Jan", 'B': "January", 'c': "02 Jan 06 15:04 MST", 'd': "02",
	'F': "2006-01-02", 'H': "15", 'I': "03", 'm': "01", 'M': "04", 'p': "PM", 'P': "pm", 'S': "05",
	'x': "15/04/05", 'X': "15:04:05", 'y': "06", 'Y': "2006", 'z': "-0700", 'Z': "MST"}

func strftime(t time.Time, cfmt string) string {
	sc := newFlagScanner('%', "", "", cfmt)
	for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
		if !sc.ChangeFlag {
			if sc.HasFlag {
				if v, ok := cDateFlagToGo[c]; ok {
					sc.AppendString(t.Format(v))
				} else {
					switch c {
					case 'w':
						sc.AppendString(fmt.Sprint(int(t.Weekday())))
					default:
						sc.AppendChar('%')
						sc.AppendChar(c)
					}
				}
				sc.HasFlag = false
			} else {
				sc.AppendChar(c)
			}
		}
	}

	return sc.String()
}

func isInteger(v LNumber) bool {
	return v.IsInteger()
}

func isArrayKey(v LNumber) bool {
	if !v.IsInteger() {
		return false
	}
	iv := v.Int64()
	return iv > 0 && iv < int64(MaxArrayIndex)
}

func parseNumber(number string) (LNumber, error) {
	number = strings.Trim(number, " \t\n")

	// Handle positive sign (Lua 5.3 allows "+" prefix)
	if strings.HasPrefix(number, "+") {
		number = number[1:]
	}

	// Handle negative numbers
	isNegative := false
	if strings.HasPrefix(number, "-") {
		isNegative = true
		number = number[1:]
	}

	// Check for hexadecimal format (0x...)
	if strings.HasPrefix(strings.ToLower(number), "0x") {
		// Check if it contains 'p' exponent (hexadecimal float with binary exponent)
		// Format: 0x...p±exp where exp is decimal
		pIndex := -1
		for i := 2; i < len(number); i++ {
			if number[i] == 'p' || number[i] == 'P' {
				pIndex = i
				break
			}
		}
		
		if pIndex >= 0 {
			// Has p exponent, parse as hex float with binary exponent
			if v, err := parseHexFloatWithExp(number); err == nil {
				if isNegative {
					v = -v
				}
				return LNumberFloat(v), nil
			}
		} else if strings.IndexByte(number, '.') >= 0 {
			// Parse hexadecimal float without p exponent
			if v, err := parseHexFloat(number); err == nil {
				if isNegative {
					v = -v
				}
				return LNumberFloat(v), nil
			}
		} else {
			// Try to parse as uint64 first, then convert to int64 using two's complement
			if v, err := strconv.ParseUint(number, 0, 64); err == nil {
				result := int64(v)
				if isNegative {
					result = -result
				}
				return LNumberInt(result), nil
			} else if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
				// For overflow, use two's complement wraparound
				// Parse the hex digits and keep only the lower 64 bits
				hexStr := strings.ToLower(strings.TrimPrefix(number, "0x"))
				if len(hexStr) > 16 {
					// Keep only the last 16 hex digits (64 bits)
					hexStr = hexStr[len(hexStr)-16:]
				}
				if v, err := strconv.ParseUint("0x"+hexStr, 16, 64); err == nil {
					result := int64(v)
					if isNegative {
						result = -result
					}
					return LNumberInt(result), nil
				}
				// If still fails, return 0
				return LNumberInt(0), nil
			}
		}
	}

	// Restore negative sign for other parsing
	if isNegative {
		number = "-" + number
	}

	// Lua 5.3 does not accept 'inf', 'nan' and similar strings in tonumber
	// Check for these special strings and reject them
	lowerNumber := strings.ToLower(number)
	if strings.HasPrefix(lowerNumber, "inf") || strings.HasPrefix(lowerNumber, "+inf") || strings.HasPrefix(lowerNumber, "-inf") ||
		strings.HasPrefix(lowerNumber, "nan") || strings.HasPrefix(lowerNumber, "+nan") || strings.HasPrefix(lowerNumber, "-nan") ||
		lowerNumber == "infinity" || lowerNumber == "+infinity" || lowerNumber == "-infinity" {
		return LNumberInt(0), fmt.Errorf("invalid number format: %s", number)
	}

	// Check if the number contains a decimal point or exponent
	// If so, parse as float to preserve type information
	hasDecimal := strings.IndexByte(number, '.') >= 0
	hasExponent := strings.IndexAny(number, "eE") >= 0

	if hasDecimal || hasExponent {
		// Parse as float to preserve type (1.0 stays float)
		if v, err := strconv.ParseFloat(number, 64); err == nil {
			return LNumberFloat(v), nil
		}
	}

	// Try to parse as integer first (for pure integer literals like 123)
	// Use base 10 to avoid interpreting leading zeros as octal (Lua 5.3 behavior)
	if v, err := strconv.ParseInt(number, 10, 64); err == nil {
		return LNumberInt(v), nil
	}

	// Fall back to float
	if v, err := strconv.ParseFloat(number, 64); err == nil {
		return LNumberFloat(v), nil
	}

	return LNumberInt(0), fmt.Errorf("invalid number format: %s", number)
}

// parseHexFloat parses hexadecimal floating point numbers like 0xAA.5
// Supports very long numbers by using ldexp for scaling
func parseHexFloat(s string) (float64, error) {
	s = strings.ToLower(s)
	if !strings.HasPrefix(s, "0x") {
		return 0, fmt.Errorf("not a hex number")
	}

	s = s[2:] // Remove "0x" prefix

	// Find decimal point
	dotIndex := strings.IndexByte(s, '.')
	if dotIndex == -1 {
		return 0, fmt.Errorf("no decimal point found")
	}

	integerPart := s[:dotIndex]
	fractionalPart := s[dotIndex+1:]

	// Parse integer part using scaling for large numbers
	// float64 has 53 bits of precision, which is about 15-16 hex digits
	// For larger numbers, we use math.Ldexp to scale
	var intValue float64
	integerLen := len(integerPart)
	
	if integerLen > 0 {
		// For very long numbers, parse in chunks
		if integerLen <= 15 {
			// Small enough to parse directly
			for _, c := range integerPart {
				var digit int
				if c >= '0' && c <= '9' {
					digit = int(c - '0')
				} else if c >= 'a' && c <= 'f' {
					digit = int(c - 'a' + 10)
				} else {
					return 0, fmt.Errorf("invalid hex digit: %c", c)
				}
				intValue = intValue*16 + float64(digit)
			}
		} else {
			// For long numbers, parse first 15 digits and scale
			// Take first 15 digits for mantissa
			mantissaDigits := 15
			if integerLen < mantissaDigits {
				mantissaDigits = integerLen
			}
			
			// Parse the first mantissaDigits
			for i := 0; i < mantissaDigits; i++ {
				c := integerPart[i]
				var digit int
				if c >= '0' && c <= '9' {
					digit = int(c - '0')
				} else if c >= 'a' && c <= 'f' {
					digit = int(c - 'a' + 10)
				} else {
					return 0, fmt.Errorf("invalid hex digit: %c", c)
				}
				intValue = intValue*16 + float64(digit)
			}
			
			// Scale by 4 bits (one hex digit) for each remaining digit
			remainingDigits := integerLen - mantissaDigits
			if remainingDigits > 0 {
				intValue = math.Ldexp(intValue, remainingDigits*4)
			}
		}
	}

	// Parse fractional part
	var fracValue float64
	if len(fractionalPart) > 0 {
		power := 1.0 / 16.0 // Start with 1/16 for first fractional digit
		for _, c := range fractionalPart {
			var digit int
			if c >= '0' && c <= '9' {
				digit = int(c - '0')
			} else if c >= 'a' && c <= 'f' {
				digit = int(c - 'a' + 10)
			} else {
				return 0, fmt.Errorf("invalid hex digit: %c", c)
			}
			fracValue += float64(digit) * power
			power /= 16.0 // Next digit has 1/16 the weight
		}
	}

	return intValue + fracValue, nil
}

// parseHexFloatWithExp parses hexadecimal floating point numbers with binary exponent like 0xAA.5p+10
// Format: 0x<hex_digits>[.<hex_digits>][p±<decimal_exponent>]
func parseHexFloatWithExp(s string) (float64, error) {
	s = strings.ToLower(s)
	if !strings.HasPrefix(s, "0x") {
		return 0, fmt.Errorf("not a hex number")
	}

	s = s[2:] // Remove "0x" prefix

	// Find 'p' exponent marker
	pIndex := strings.IndexByte(s, 'p')
	if pIndex == -1 {
		return 0, fmt.Errorf("no p exponent found")
	}

	hexPart := s[:pIndex]
	expPart := s[pIndex+1:]

	// Parse exponent
	expSign := 1
	if len(expPart) > 0 && expPart[0] == '+' {
		expPart = expPart[1:]
	} else if len(expPart) > 0 && expPart[0] == '-' {
		expSign = -1
		expPart = expPart[1:]
	}
	
	// Check for double sign (e.g., "+-" or "-+")
	if len(expPart) > 0 && (expPart[0] == '+' || expPart[0] == '-') {
		return 0, fmt.Errorf("invalid exponent: double sign")
	}
	
	// Trim spaces from exponent part
	expPart = strings.TrimSpace(expPart)
	
	// Exponent part must not be empty and must contain only digits
	if len(expPart) == 0 {
		return 0, fmt.Errorf("missing exponent after 'p'")
	}
	
	binaryExponent := 0
	for _, c := range expPart {
		if c >= '0' && c <= '9' {
			binaryExponent = binaryExponent*10 + int(c-'0')
		} else {
			return 0, fmt.Errorf("invalid exponent digit: %c", c)
		}
	}
	binaryExponent *= expSign

	// Find decimal point in hex part
	dotIndex := strings.IndexByte(hexPart, '.')
	
	var mantissa float64
	var hexExponent int // Additional exponent from hex digit positions

	if dotIndex == -1 {
		// No decimal point: 0x<hex_digits>p<exp>
		// All hex digits are integer part
		integerPart := hexPart
		
		// Parse up to 15 significant hex digits for mantissa
		integerLen := len(integerPart)
		if integerLen > 0 {
			// Parse first 15 digits (or less) for mantissa
			mantissaDigits := 15
			if integerLen < mantissaDigits {
				mantissaDigits = integerLen
			}
			
			for i := 0; i < mantissaDigits; i++ {
				c := integerPart[i]
				var digit int
				if c >= '0' && c <= '9' {
					digit = int(c - '0')
				} else if c >= 'a' && c <= 'f' {
					digit = int(c - 'a' + 10)
				} else {
					return 0, fmt.Errorf("invalid hex digit: %c", c)
				}
				mantissa = mantissa*16 + float64(digit)
			}
			
			// The remaining digits contribute to the exponent
			// Each hex digit = 4 bits
			hexExponent = (integerLen - mantissaDigits) * 4
		}
	} else {
		// Has decimal point: 0x<hex>.<hex>p<exp>
		integerPart := hexPart[:dotIndex]
		fractionalPart := hexPart[dotIndex+1:]

		// Parse integer part
		integerLen := len(integerPart)
		if integerLen > 0 {
			mantissaDigits := 15
			if integerLen < mantissaDigits {
				mantissaDigits = integerLen
			}
			
			for i := 0; i < mantissaDigits; i++ {
				c := integerPart[i]
				var digit int
				if c >= '0' && c <= '9' {
					digit = int(c - '0')
				} else if c >= 'a' && c <= 'f' {
					digit = int(c - 'a' + 10)
				} else {
					return 0, fmt.Errorf("invalid hex digit: %c", c)
				}
				mantissa = mantissa*16 + float64(digit)
			}
			
			hexExponent = (integerLen - mantissaDigits) * 4
		}

		// Parse fractional part
		if len(fractionalPart) > 0 {
			// If we haven't filled 15 digits yet, parse fractional digits
			currentDigits := integerLen
			if currentDigits > 15 {
				currentDigits = 15
			}
			remainingMantissaDigits := 15 - currentDigits
			
			fracLen := len(fractionalPart)
			
			// Find first non-zero digit
			firstNonZero := -1
			for i := 0; i < fracLen; i++ {
				if fractionalPart[i] != '0' {
					firstNonZero = i
					break
				}
			}
			
			if firstNonZero >= 0 {
				// We have non-zero digits, adjust exponent for leading zeros
				hexExponent -= firstNonZero * 4
				
				// Parse up to remainingMantissaDigits starting from firstNonZero
				parseDigits := remainingMantissaDigits
				availableDigits := fracLen - firstNonZero
				if availableDigits < parseDigits {
					parseDigits = availableDigits
				}
				
				// Scale factor for fractional part (start at 1.0 since we already adjusted exponent)
				scale := 1.0
				for i := 0; i < parseDigits; i++ {
					scale /= 16.0
					c := fractionalPart[firstNonZero+i]
					var digit int
					if c >= '0' && c <= '9' {
						digit = int(c - '0')
					} else if c >= 'a' && c <= 'f' {
						digit = int(c - 'a' + 10)
					} else {
						return 0, fmt.Errorf("invalid hex digit: %c", c)
					}
					mantissa += float64(digit) * scale
				}
				
				// Remaining digits after mantissa contribute negative exponent
				remainingFracDigits := fracLen - firstNonZero - parseDigits
				if remainingFracDigits > 0 {
					hexExponent -= remainingFracDigits * 4
				}
			} else {
				// All zeros in fractional part
				hexExponent -= fracLen * 4
			}
		}
	}

	// Combine mantissa with total exponent (hexExponent + binaryExponent)
	totalExponent := hexExponent + binaryExponent
	value := math.Ldexp(mantissa, totalExponent)
	
	return value, nil
}

func popenArgs(arg string) (string, []string) {
	cmd := "/bin/sh"
	args := []string{"-c"}
	if LuaOS == "windows" {
		cmd = "C:\\Windows\\system32\\cmd.exe"
		args = []string{"/c"}
	}
	args = append(args, arg)
	return cmd, args
}

func isGoroutineSafe(lv LValue) bool {
	switch v := lv.(type) {
	case *LFunction, *LUserData, *LState:
		return false
	case *LTable:
		return v.Metatable == LNil
	default:
		return true
	}
}

func readBufioSize(reader *bufio.Reader, size int64) ([]byte, error, bool) {
	result := []byte{}
	read := int64(0)
	var err error
	var n int
	for read != size {
		buf := make([]byte, size-read)
		n, err = reader.Read(buf)
		if err != nil {
			break
		}
		read += int64(n)
		result = append(result, buf[:n]...)
	}
	e := err
	if e != nil && e == io.EOF {
		e = nil
	}

	return result, e, len(result) == 0 && err == io.EOF
}

func readBufioLine(reader *bufio.Reader) ([]byte, error, bool) {
	result := []byte{}
	var buf []byte
	var err error
	var isprefix bool = true
	for isprefix {
		buf, isprefix, err = reader.ReadLine()
		if err != nil {
			break
		}
		result = append(result, buf...)
	}
	e := err
	if e != nil && e == io.EOF {
		e = nil
	}

	return result, e, len(result) == 0 && err == io.EOF
}

func int2Fb(val int) int {
	e := 0
	x := val
	for x >= 16 {
		x = (x + 1) >> 1
		e++
	}
	if x < 8 {
		return x
	}
	return ((e + 1) << 3) | (x - 8)
}

func strCmp(s1, s2 string) int {
	len1 := len(s1)
	len2 := len(s2)
	for i := 0; ; i++ {
		c1 := -1
		if i < len1 {
			c1 = int(s1[i])
		}
		c2 := -1
		if i != len2 {
			c2 = int(s2[i])
		}
		switch {
		case c1 < c2:
			return -1
		case c1 > c2:
			return +1
		case c1 < 0:
			return 0
		}
	}
}

func unsafeFastStringToReadOnlyBytes(s string) (bs []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&bs))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len
	return
}
