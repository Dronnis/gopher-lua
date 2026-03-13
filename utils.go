package lua

import (
	"bufio"
	"fmt"
	"io"
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

	// Handle negative numbers
	isNegative := false
	if strings.HasPrefix(number, "-") {
		isNegative = true
		number = number[1:]
	}

	// Check for hexadecimal float format (0x...)
	if strings.HasPrefix(strings.ToLower(number), "0x") {
		// Check if it contains a decimal point (hexadecimal float)
		if strings.IndexByte(number, '.') >= 0 {
			// Parse hexadecimal float manually
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
	if v, err := strconv.ParseInt(number, 0, 64); err == nil {
		return LNumberInt(v), nil
	}

	// Fall back to float
	if v, err := strconv.ParseFloat(number, 64); err == nil {
		return LNumberFloat(v), nil
	}

	return LNumberInt(0), fmt.Errorf("invalid number format: %s", number)
}

// parseHexFloat parses hexadecimal floating point numbers like 0xAA.5
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
	
	// Check for overflow in integer part - if it's too long, it might overflow
	if len(integerPart) > 15 { // More than 15 hex digits would overflow float64 precision
		return 0, fmt.Errorf("hex integer part too large")
	}
	
	// Parse integer part
	var intValue float64
	if len(integerPart) > 0 {
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
