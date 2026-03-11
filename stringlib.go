package lua

import (
	"fmt"
	"math"
	"strings"

	"github.com/yuin/gopher-lua/pm"
)

const emptyLString LString = LString("")

func OpenString(L *LState) int {
	var mod *LTable
	//_, ok := L.G.builtinMts[int(LTString)]
	//if !ok {
	mod = L.RegisterModule(StringLibName, strFuncs).(*LTable)
	gmatch := L.NewClosure(strGmatch, L.NewFunction(strGmatchIter))
	mod.RawSetString("gmatch", gmatch)
	mod.RawSetString("gfind", gmatch)
	mod.RawSetString("__index", mod)
	L.G.builtinMts[int(LTString)] = mod
	//}
	L.Push(mod)
	return 1
}

var strFuncs = map[string]LGFunction{
	"byte":    strByte,
	"char":    strChar,
	"dump":    strDump,
	"find":    strFind,
	"format":  strFormat,
	"gsub":    strGsub,
	"len":     strLen,
	"lower":   strLower,
	"match":   strMatch,
	"pack":    strPack,
	"packsize": strPackSize,
	"rep":     strRep,
	"reverse": strReverse,
	"sub":     strSub,
	"unpack":  strUnpack,
	"upper":   strUpper,
}

func strByte(L *LState) int {
	str := L.CheckString(1)
	start := L.OptInt(2, 1) - 1
	end := L.OptInt(3, -1)
	l := len(str)
	if start < 0 {
		start = l + start + 1
	}
	if end < 0 {
		end = l + end + 1
	}

	if L.GetTop() == 2 {
		if start < 0 || start >= l {
			return 0
		}
		L.Push(LNumberInt(int64(str[start])))
		return 1
	}

	start = intMax(start, 0)
	end = intMin(end, l)
	if end < 0 || end <= start || start >= l {
		return 0
	}

	for i := start; i < end; i++ {
		L.Push(LNumberInt(int64(str[i])))
	}
	return end - start
}

func strChar(L *LState) int {
	top := L.GetTop()
	bytes := make([]byte, L.GetTop())
	for i := 1; i <= top; i++ {
		bytes[i-1] = uint8(L.CheckInt(i))
	}
	L.Push(LString(string(bytes)))
	return 1
}

func strDump(L *LState) int {
	L.RaiseError("GopherLua does not support the string.dump")
	return 0
}

func strFind(L *LState) int {
	str := L.CheckString(1)
	pattern := L.CheckString(2)
	if len(pattern) == 0 {
		L.Push(LNumberInt(1))
		L.Push(LNumberInt(0))
		return 2
	}
	init := luaIndex2StringIndex(str, L.OptInt(3, 1), true)
	plain := false
	if L.GetTop() == 4 {
		plain = LVAsBool(L.Get(4))
	}

	if plain {
		pos := strings.Index(str[init:], pattern)
		if pos < 0 {
			L.Push(LNil)
			return 1
		}
		L.Push(LNumberInt(int64(init+pos) + 1))
		L.Push(LNumberInt(int64(init + pos + len(pattern))))
		return 2
	}

	mds, err := pm.Find(pattern, unsafeFastStringToReadOnlyBytes(str), init, 1)
	if err != nil {
		L.RaiseError(err.Error())
	}
	if len(mds) == 0 {
		L.Push(LNil)
		return 1
	}
	md := mds[0]
	L.Push(LNumberInt(int64(md.Capture(0) + 1)))
	L.Push(LNumberInt(int64(md.Capture(1))))
	for i := 2; i < md.CaptureLength(); i += 2 {
		if md.IsPosCapture(i) {
			L.Push(LNumberInt(int64(md.Capture(i))))
		} else {
			L.Push(LString(str[md.Capture(i):md.Capture(i+1)]))
		}
	}
	return md.CaptureLength()/2 + 1
}

func strFormat(L *LState) int {
	str := L.CheckString(1)
	args := make([]interface{}, L.GetTop()-1)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		args[i-2] = L.Get(i)
	}
	npat := strings.Count(str, "%") - strings.Count(str, "%%")
	L.Push(LString(fmt.Sprintf(str, args[:intMin(npat, len(args))]...)))
	return 1
}

func strGsub(L *LState) int {
	str := L.CheckString(1)
	pat := L.CheckString(2)
	L.CheckTypes(3, LTString, LTTable, LTFunction)
	repl := L.CheckAny(3)
	limit := L.OptInt(4, -1)

	mds, err := pm.Find(pat, unsafeFastStringToReadOnlyBytes(str), 0, limit)
	if err != nil {
		L.RaiseError(err.Error())
	}
	if len(mds) == 0 {
		L.SetTop(1)
		L.Push(LNumberInt(0))
		return 2
	}
	switch lv := repl.(type) {
	case LString:
		L.Push(LString(strGsubStr(L, str, string(lv), mds)))
	case *LTable:
		L.Push(LString(strGsubTable(L, str, lv, mds)))
	case *LFunction:
		L.Push(LString(strGsubFunc(L, str, lv, mds)))
	}
	L.Push(LNumberInt(int64(len(mds))))
	return 2
}

type replaceInfo struct {
	Indicies []int
	String   string
}

func checkCaptureIndex(L *LState, m *pm.MatchData, idx int) {
	if idx <= 2 {
		return
	}
	if idx >= m.CaptureLength() {
		L.RaiseError("invalid capture index")
	}
}

func capturedString(L *LState, m *pm.MatchData, str string, idx int) string {
	checkCaptureIndex(L, m, idx)
	if idx >= m.CaptureLength() && idx == 2 {
		idx = 0
	}
	if m.IsPosCapture(idx) {
		return fmt.Sprint(m.Capture(idx))
	} else {
		return str[m.Capture(idx):m.Capture(idx+1)]
	}

}

func strGsubDoReplace(str string, info []replaceInfo) string {
	offset := 0
	buf := []byte(str)
	for _, replace := range info {
		oldlen := len(buf)
		b1 := append([]byte(""), buf[0:offset+replace.Indicies[0]]...)
		b2 := []byte("")
		index2 := offset + replace.Indicies[1]
		if index2 <= len(buf) {
			b2 = append(b2, buf[index2:len(buf)]...)
		}
		buf = append(b1, replace.String...)
		buf = append(buf, b2...)
		offset += len(buf) - oldlen
	}
	return string(buf)
}

func strGsubStr(L *LState, str string, repl string, matches []*pm.MatchData) string {
	infoList := make([]replaceInfo, 0, len(matches))
	for _, match := range matches {
		start, end := match.Capture(0), match.Capture(1)
		sc := newFlagScanner('%', "", "", repl)
		for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
			if !sc.ChangeFlag {
				if sc.HasFlag {
					if c >= '0' && c <= '9' {
						sc.AppendString(capturedString(L, match, str, 2*(int(c)-48)))
					} else {
						sc.AppendChar('%')
						sc.AppendChar(c)
					}
					sc.HasFlag = false
				} else {
					sc.AppendChar(c)
				}
			}
		}
		infoList = append(infoList, replaceInfo{[]int{start, end}, sc.String()})
	}

	return strGsubDoReplace(str, infoList)
}

func strGsubTable(L *LState, str string, repl *LTable, matches []*pm.MatchData) string {
	infoList := make([]replaceInfo, 0, len(matches))
	for _, match := range matches {
		idx := 0
		if match.CaptureLength() > 2 { // has captures
			idx = 2
		}
		var value LValue
		if match.IsPosCapture(idx) {
			value = L.GetTable(repl, LNumberInt(int64(match.Capture(idx))))
		} else {
			value = L.GetField(repl, str[match.Capture(idx):match.Capture(idx+1)])
		}
		if !LVIsFalse(value) {
			infoList = append(infoList, replaceInfo{[]int{match.Capture(0), match.Capture(1)}, LVAsString(value)})
		}
	}
	return strGsubDoReplace(str, infoList)
}

func strGsubFunc(L *LState, str string, repl *LFunction, matches []*pm.MatchData) string {
	infoList := make([]replaceInfo, 0, len(matches))
	for _, match := range matches {
		start, end := match.Capture(0), match.Capture(1)
		L.Push(repl)
		nargs := 0
		if match.CaptureLength() > 2 { // has captures
			for i := 2; i < match.CaptureLength(); i += 2 {
				if match.IsPosCapture(i) {
					L.Push(LNumberInt(int64(match.Capture(i))))
				} else {
					L.Push(LString(capturedString(L, match, str, i)))
				}
				nargs++
			}
		} else {
			L.Push(LString(capturedString(L, match, str, 0)))
			nargs++
		}
		L.Call(nargs, 1)
		ret := L.reg.Pop()
		if !LVIsFalse(ret) {
			infoList = append(infoList, replaceInfo{[]int{start, end}, LVAsString(ret)})
		}
	}
	return strGsubDoReplace(str, infoList)
}

type strMatchData struct {
	str     string
	pos     int
	matches []*pm.MatchData
}

func strGmatchIter(L *LState) int {
	md := L.CheckUserData(1).Value.(*strMatchData)
	str := md.str
	matches := md.matches
	idx := md.pos
	md.pos += 1
	if idx == len(matches) {
		return 0
	}
	L.Push(L.Get(1))
	match := matches[idx]
	if match.CaptureLength() == 2 {
		L.Push(LString(str[match.Capture(0):match.Capture(1)]))
		return 1
	}

	for i := 2; i < match.CaptureLength(); i += 2 {
		if match.IsPosCapture(i) {
			L.Push(LNumberInt(int64(match.Capture(i))))
		} else {
			L.Push(LString(str[match.Capture(i):match.Capture(i+1)]))
		}
	}
	return match.CaptureLength()/2 - 1
}

func strGmatch(L *LState) int {
	str := L.CheckString(1)
	pattern := L.CheckString(2)
	mds, err := pm.Find(pattern, []byte(str), 0, -1)
	if err != nil {
		L.RaiseError(err.Error())
	}
	L.Push(L.Get(UpvalueIndex(1)))
	ud := L.NewUserData()
	ud.Value = &strMatchData{str, 0, mds}
	L.Push(ud)
	return 2
}

func strLen(L *LState) int {
	str := L.CheckString(1)
	L.Push(LNumberInt(int64(len(str))))
	return 1
}

func strLower(L *LState) int {
	str := L.CheckString(1)
	L.Push(LString(strings.ToLower(str)))
	return 1
}

func strMatch(L *LState) int {
	str := L.CheckString(1)
	pattern := L.CheckString(2)
	offset := L.OptInt(3, 1)
	l := len(str)
	if offset < 0 {
		offset = l + offset + 1
	}
	offset--
	if offset < 0 {
		offset = 0
	}

	mds, err := pm.Find(pattern, unsafeFastStringToReadOnlyBytes(str), offset, 1)
	if err != nil {
		L.RaiseError(err.Error())
	}
	if len(mds) == 0 {
		L.Push(LNil)
		return 0
	}
	md := mds[0]
	nsubs := md.CaptureLength() / 2
	switch nsubs {
	case 1:
		L.Push(LString(str[md.Capture(0):md.Capture(1)]))
		return 1
	default:
		for i := 2; i < md.CaptureLength(); i += 2 {
			if md.IsPosCapture(i) {
				L.Push(LNumberInt(int64(md.Capture(i))))
			} else {
				L.Push(LString(str[md.Capture(i):md.Capture(i+1)]))
			}
		}
		return nsubs - 1
	}
}

func strRep(L *LState) int {
	str := L.CheckString(1)
	n := L.CheckInt(2)
	sep := L.OptString(3, "")
	
	if n <= 0 {
		L.Push(emptyLString)
	} else if sep == "" {
		// Без разделителя - просто повторяем строку
		L.Push(LString(strings.Repeat(str, n)))
	} else {
		// С разделителем - вставляем между повторениями
		// Для n повторений нужно n-1 разделителей
		result := make([]byte, 0, len(str)*n+len(sep)*(n-1))
		for i := 0; i < n; i++ {
			if i > 0 {
				result = append(result, sep...)
			}
			result = append(result, str...)
		}
		L.Push(LString(string(result)))
	}
	return 1
}

func strReverse(L *LState) int {
	str := L.CheckString(1)
	bts := []byte(str)
	out := make([]byte, len(bts))
	for i, j := 0, len(bts)-1; j >= 0; i, j = i+1, j-1 {
		out[i] = bts[j]
	}
	L.Push(LString(string(out)))
	return 1
}

func strSub(L *LState) int {
	str := L.CheckString(1)
	start := luaIndex2StringIndex(str, L.CheckInt(2), true)
	end := luaIndex2StringIndex(str, L.OptInt(3, -1), false)
	l := len(str)
	if start >= l || end < start {
		L.Push(emptyLString)
	} else {
		L.Push(LString(str[start:end]))
	}
	return 1
}

func strUpper(L *LState) int {
	str := L.CheckString(1)
	L.Push(LString(strings.ToUpper(str)))
	return 1
}

// {{{ string.pack / string.unpack / string.packsize

// Форматы для string.pack/unpack (Lua 5.3)
const (
	packLittleEndian = '<'
	packBigEndian    = '>'
	packNative       = '='
	packMaxAlign     = 16
)

// packFormat описывает опции форматирования
type packFormat struct {
	endian    byte
	align     int
	options   []packOption
}

// packOption - отдельная опция упаковки
type packOption struct {
	code  byte
	size  int
	count int // для повторений
}

// parsePackFormat разбирает строку формата
func parsePackFormat(format string) (*packFormat, error) {
	pf := &packFormat{
		endian:  packLittleEndian, // Lua 5.3 использует little-endian по умолчанию
		align:   1,
		options: make([]packOption, 0),
	}

	i := 0
	for i < len(format) {
		c := format[i]

		// Модификаторы порядка байтов
		if c == packLittleEndian || c == packBigEndian || c == packNative {
			pf.endian = c
			i++
			continue
		}

		// Выравнивание
		if c == '!' {
			i++
			if i >= len(format) {
				return nil, fmt.Errorf("invalid format: missing alignment value")
			}
			// Читаем число после !
			n := 0
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				n = n*10 + int(format[i]-'0')
				i++
			}
			if n == 0 {
				n = 1
			}
			if n > packMaxAlign {
				return nil, fmt.Errorf("invalid format: alignment too large")
			}
			pf.align = n
			continue
		}

		// Повторения (число перед кодом)
		count := 1
		if c >= '0' && c <= '9' {
			count = 0
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				count = count*10 + int(format[i]-'0')
				i++
			}
			if i >= len(format) {
				return nil, fmt.Errorf("invalid format: missing format code after count")
			}
			c = format[i]
			i++
		} else {
			i++
		}

		// Код формата
		size := 0
		switch c {
		case 'b', 'B': // signed/unsigned char
			size = 1
		case 'h', 'H': // signed/unsigned short
			size = 2
		case 'l', 'L': // signed/unsigned long
			size = 4
		case 'j', 'J': // lua_Integer / lua_Unsigned
			size = 8 // int64 в Go
		case 'T': // size_t
			size = 8
		case 'f': // float
			size = 4
		case 'd', 'n': // double / lua_Number
			size = 8
		case 'x': // padding byte
			size = 1
		case 'c': // fixed-length string
			// Читаем длину после c
			if i >= len(format) {
				return nil, fmt.Errorf("invalid format: missing length for 'c'")
			}
			n := 0
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				n = n*10 + int(format[i]-'0')
				i++
			}
			size = n
		case 'z', 's': // zero-terminated string / string with size
			size = -1 // variable size
		default:
			return nil, fmt.Errorf("invalid format code: %c", c)
		}

		pf.options = append(pf.options, packOption{code: c, size: size, count: count})
	}

	return pf, nil
}

// alignTo выравнивает позицию до следующей границы выравнивания
func alignTo(pos, align int) int {
	if align <= 1 {
		return pos
	}
	return (pos + align - 1) & ^(align - 1)
}

// writeEndian записывает число с учётом порядка байтов
func writeEndian(buf []byte, pos int, value uint64, size int, endian byte) {
	if endian == packBigEndian {
		for i := size - 1; i >= 0; i-- {
			buf[pos+i] = byte(value & 0xFF)
			value >>= 8
		}
	} else { // little-endian или native (little-endian для x86/x64)
		for i := 0; i < size; i++ {
			buf[pos+i] = byte(value & 0xFF)
			value >>= 8
		}
	}
}

// readEndian читает число с учётом порядка байтов
func readEndian(buf []byte, pos int, size int, endian byte) uint64 {
	var value uint64 = 0
	if endian == packBigEndian {
		for i := 0; i < size; i++ {
			value = (value << 8) | uint64(buf[pos+i])
		}
	} else { // little-endian
		for i := size - 1; i >= 0; i-- {
			value = (value << 8) | uint64(buf[pos+i])
		}
	}
	return value
}

// strPack - string.pack(fmt, v1, v2, ...)
func strPack(L *LState) int {
	fmtStr := L.CheckString(1)
	pf, err := parsePackFormat(fmtStr)
	if err != nil {
		L.ArgError(1, err.Error())
	}

	// Сначала вычисляем размер
	totalSize := 0
	argIdx := 2

	for _, opt := range pf.options {
		for c := 0; c < opt.count; c++ {
			if opt.code == '!' {
				totalSize = alignTo(totalSize, pf.align)
				continue
			}

			if opt.code == 'x' {
				totalSize += opt.size
				continue
			}

			if opt.code == 'c' {
				totalSize += opt.size
				continue
			}

			if opt.code == 'z' || opt.code == 's' {
				if argIdx > L.GetTop() {
					L.ArgError(argIdx, "missing argument")
				}
				str := L.CheckString(argIdx)
				argIdx++
				if opt.code == 'z' {
					totalSize += len(str) + 1 // +1 для нулевого терминатора
				} else {
					// 's' - строка с размером (size_t + данные)
					totalSize += 8 + len(str)
				}
				continue
			}

			// Числовые типы
			if argIdx > L.GetTop() {
				L.ArgError(argIdx, "missing argument")
			}
			argIdx++

			if opt.size < 0 {
				continue
			}
			totalSize += opt.size
		}
	}

	// Выделяем буфер
	buf := make([]byte, totalSize)
	pos := 0
	argIdx = 2

	for _, opt := range pf.options {
		for c := 0; c < opt.count; c++ {
			// Применяем выравнивание
			if opt.code == '!' {
				pos = alignTo(pos, pf.align)
				continue
			}

			if opt.code == 'x' {
				// Padding byte (нулевой)
				for i := 0; i < opt.size; i++ {
					buf[pos] = 0
					pos++
				}
				continue
			}

			if opt.code == 'c' {
				// Фиксированная строка
				if argIdx > L.GetTop() {
					L.ArgError(argIdx, "missing argument")
				}
				str := L.CheckString(argIdx)
				argIdx++
				copyLen := len(str)
				if copyLen > opt.size {
					copyLen = opt.size
				}
				copy(buf[pos:], str[:copyLen])
				// Заполняем остаток нулями
				for i := copyLen; i < opt.size; i++ {
					buf[pos+i] = 0
				}
				pos += opt.size
				continue
			}

			if opt.code == 'z' {
				// Строка с нулевым терминатором
				if argIdx > L.GetTop() {
					L.ArgError(argIdx, "missing argument")
				}
				str := L.CheckString(argIdx)
				argIdx++
				copy(buf[pos:], str)
				buf[pos+len(str)] = 0
				pos += len(str) + 1
				continue
			}

			if opt.code == 's' {
				// Строка с размером (size_t + данные)
				if argIdx > L.GetTop() {
					L.ArgError(argIdx, "missing argument")
				}
				str := L.CheckString(argIdx)
				argIdx++
				// Записываем размер как size_t (8 байт)
				writeEndian(buf, pos, uint64(len(str)), 8, pf.endian)
				pos += 8
				copy(buf[pos:], str)
				pos += len(str)
				continue
			}

			// Числовые типы
			if argIdx > L.GetTop() {
				L.ArgError(argIdx, "missing argument")
			}
			var value int64
			switch v := L.Get(argIdx).(type) {
			case LNumber:
				value = v.Int64()
			case LString:
				// Пытаемся распарсить строку как число
				if n, err := parseNumber(string(v)); err == nil {
					value = n.Int64()
				}
			}
			argIdx++

			// Для знаковых типов нужно правильно обработать отрицательные числа
			var uvalue uint64
			switch opt.code {
			case 'b': // signed char
				uvalue = uint64(int8(value))
			case 'B': // unsigned char
				uvalue = uint64(uint8(value))
			case 'h': // signed short
				uvalue = uint64(int16(value))
			case 'H': // unsigned short
				uvalue = uint64(uint16(value))
			case 'l': // signed long
				uvalue = uint64(int32(value))
			case 'L': // unsigned long
				uvalue = uint64(uint32(value))
			case 'j', 'J', 'T': // int64 / uint64
				uvalue = uint64(value)
			case 'f': // float
				uvalue = uint64(math.Float32bits(float32(L.Get(argIdx-1).(LNumber).Float64())))
			case 'd', 'n': // double
				uvalue = math.Float64bits(L.Get(argIdx-1).(LNumber).Float64())
			}

			writeEndian(buf, pos, uvalue, opt.size, pf.endian)
			pos += opt.size
		}
	}

	L.Push(LString(string(buf)))
	return 1
}

// strUnpack - string.unpack(fmt, s [, pos])
func strUnpack(L *LState) int {
	fmtStr := L.CheckString(1)
	s := L.CheckString(2)
	pos := L.OptInt(3, 1) - 1 // 0-based

	pf, err := parsePackFormat(fmtStr)
	if err != nil {
		L.ArgError(1, err.Error())
	}

	if pos < 0 || pos > len(s) {
		L.ArgError(2, "initial position out of range")
	}

	results := make([]LValue, 0)

	for _, opt := range pf.options {
		for c := 0; c < opt.count; c++ {
			// Применяем выравнивание
			if opt.code == '!' {
				pos = alignTo(pos, pf.align)
				continue
			}

			if opt.code == 'x' {
				// Padding byte - просто пропускаем
				pos += opt.size
				continue
			}

			if opt.code == 'c' {
				// Фиксированная строка
				if pos+opt.size > len(s) {
					L.ArgError(2, "data string too short")
				}
				str := string(s[pos : pos+opt.size])
				// Удаляем нулевые байты с конца
				for len(str) > 0 && str[len(str)-1] == 0 {
					str = str[:len(str)-1]
				}
				results = append(results, LString(str))
				pos += opt.size
				continue
			}

			if opt.code == 'z' {
				// Строка с нулевым терминатором
				start := pos
				for pos < len(s) && s[pos] != 0 {
					pos++
				}
				if pos >= len(s) {
					L.ArgError(2, "data string too short")
				}
				results = append(results, LString(s[start:pos]))
				pos++ // пропускаем нулевой байт
				continue
			}

			if opt.code == 's' {
				// Строка с размером
				if pos+8 > len(s) {
					L.ArgError(2, "data string too short")
				}
				strLen := int(readEndian([]byte(s), pos, 8, pf.endian))
				pos += 8
				if pos+strLen > len(s) {
					L.ArgError(2, "data string too short")
				}
				results = append(results, LString(s[pos:pos+strLen]))
				pos += strLen
				continue
			}

			// Числовые типы
			if pos+opt.size > len(s) {
				L.ArgError(2, "data string too short")
			}

			uvalue := readEndian([]byte(s), pos, opt.size, pf.endian)
			pos += opt.size

			var value LValue
			switch opt.code {
			case 'b': // signed char
				value = LNumberInt(int64(int8(uvalue)))
			case 'B': // unsigned char
				value = LNumberInt(int64(uvalue))
			case 'h': // signed short
				value = LNumberInt(int64(int16(uvalue)))
			case 'H': // unsigned short
				value = LNumberInt(int64(uvalue))
			case 'l': // signed long
				value = LNumberInt(int64(int32(uvalue)))
			case 'L': // unsigned long
				value = LNumberInt(int64(uvalue))
			case 'j', 'J', 'T': // int64 / uint64
				value = LNumberInt(int64(uvalue))
			case 'f': // float
				value = LNumberFloat(float64(math.Float32frombits(uint32(uvalue))))
			case 'd', 'n': // double
				value = LNumberFloat(math.Float64frombits(uvalue))
			}
			results = append(results, value)
		}
	}

	// Возвращаем распакованные значения и следующую позицию
	for _, v := range results {
		L.Push(v)
	}
	L.Push(LNumberInt(int64(pos + 1))) // 1-based позиция

	return len(results) + 1
}

// strPackSize - string.packsize(fmt)
func strPackSize(L *LState) int {
	fmtStr := L.CheckString(1)
	pf, err := parsePackFormat(fmtStr)
	if err != nil {
		L.ArgError(1, err.Error())
	}

	totalSize := 0
	for _, opt := range pf.options {
		for c := 0; c < opt.count; c++ {
			if opt.code == '!' {
				totalSize = alignTo(totalSize, pf.align)
				continue
			}

			if opt.code == 'x' {
				totalSize += opt.size
				continue
			}

			if opt.code == 'c' {
				totalSize += opt.size
				continue
			}

			if opt.code == 'z' || opt.code == 's' {
				// Переменный размер - не можем вычислить
				L.Push(LNil)
				return 1
			}

			if opt.size < 0 {
				L.Push(LNil)
				return 1
			}
			totalSize += opt.size
		}
	}

	L.Push(LNumberInt(int64(totalSize)))
	return 1
}

// }}}

func luaIndex2StringIndex(str string, i int, start bool) int {
	if start && i != 0 {
		i -= 1
	}
	l := len(str)
	if i < 0 {
		i = l + i + 1
	}
	i = intMax(0, i)
	if !start && i > l {
		i = l
	}
	return i
}

//
