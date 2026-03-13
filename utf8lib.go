package lua

import (
	"fmt"
	"unicode/utf8"
)

// Шаблоны для UTF-8 (Lua 5.3 совместимость)
// charpattern - шаблон для одного символа UTF-8
const utf8CharPattern = "[\x00-\x7F\xC2-\xF4][\x80-\xBF]*"

// codespattern - эквивалентен charpattern
const utf8CodesPattern = "[\x00-\x7F\xC2-\xF4][\x80-\xBF]*"

// Максимальная кодовая точка Unicode
const utf8MaxCodePoint = 0x10FFFF

// decodeCodePoint декодирует кодовую точку из байтов UTF-8
func decodeCodePoint(s string, i int) (rune, int, bool) {
	if i >= len(s) {
		return 0, 0, false
	}

	b := s[i]

	// ASCII (1 байт)
	if b < 0x80 {
		return rune(b), 1, true
	}

	// Определяем длину последовательности по первому байту
	var expectedLen int
	if b < 0xC0 {
		return 0, 0, false
	} else if b < 0xE0 {
		expectedLen = 2
	} else if b < 0xF0 {
		expectedLen = 3
	} else if b < 0xF8 {
		expectedLen = 4
	} else {
		return 0, 0, false
	}

	// Проверяем, достаточно ли байтов
	if i+expectedLen > len(s) {
		return 0, 0, false
	}

	// Проверяем байты продолжения
	for j := 1; j < expectedLen; j++ {
		if s[i+j]&0xC0 != 0x80 {
			return 0, 0, false
		}
	}

	// Декодируем руну
	r, size := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && size == 1 {
		return 0, 0, false
	}

	return r, size, true
}

// encodeCodePoint кодирует кодовую точку в UTF-8
func encodeCodePoint(codePoint rune) (string, error) {
	if codePoint < 0 || codePoint > utf8MaxCodePoint {
		return "", fmt.Errorf("code point out of range: %d", codePoint)
	}

	// Проверяем на суррогатные пары
	if codePoint >= 0xD800 && codePoint <= 0xDFFF {
		return "", fmt.Errorf("invalid code point (surrogate): %d", codePoint)
	}

	buf := make([]byte, utf8.UTFMax)
	n := utf8.EncodeRune(buf, codePoint)
	return string(buf[:n]), nil
}

// isValidCodePoint проверяет допустимость кодовой точки
func isValidCodePoint(codePoint rune) bool {
	return codePoint >= 0 && codePoint <= utf8MaxCodePoint &&
		!(codePoint >= 0xD800 && codePoint <= 0xDFFF)
}

// utf8Char - utf8.char(codepoint, ...)
func utf8Char(L *LState) int {
	top := L.GetTop()

	result := ""
	for i := 1; i <= top; i++ {
		cp := L.CheckInt(i)
		codePoint := rune(cp)

		if !isValidCodePoint(codePoint) {
			L.ArgError(i, "value out of range")
		}

		s, err := encodeCodePoint(codePoint)
		if err != nil {
			L.ArgError(i, "value out of range")
		}
		result += s
	}

	L.Push(LString(result))
	return 1
}

// utf8CodePoint - utf8.codepoint(s [, i [, j]])
func utf8CodePoint(L *LState) int {
	s := L.CheckString(1)
	i := L.OptInt(2, 1)

	// Если j не указан, он равен i (возвращаем только один символ)
	j := i
	if L.GetTop() >= 3 {
		j = L.CheckInt(3)
	}

	// Преобразуем отрицательные индексы
	l := len(s)
	if i < 0 {
		i = l + i + 1
	}
	if j < 0 {
		j = l + j + 1
	}

	// Преобразуем в 0-based индексы
	i = i - 1
	j = j - 1

	// Если i > j, возвращаем пустой результат (без ошибок)
	if i > j {
		return 0
	}

	// Проверяем диапазон
	if i < 0 || i >= l || j < 0 || j >= l {
		L.RaiseError("out of range")
		return 0
	}

	nargs := 0
	pos := i
	for pos <= j && pos < len(s) {
		r, size, ok := decodeCodePoint(s, pos)
		if !ok {
			L.RaiseError("invalid UTF-8 code")
			return 0
		}
		L.Push(LNumberInt(int64(r)))
		nargs++
		pos += size
	}

	return nargs
}

type utf8CodesData struct {
	str string
	pos int
}

// utf8CodesIter - итератор для utf8.codes
func utf8CodesIter(L *LState) int {
	ud := L.CheckUserData(1)
	data := ud.Value.(*utf8CodesData)
	str := data.str
	pos := data.pos

	if pos >= len(str) {
		L.Push(LNil)
		return 1
	}

	pos0 := pos
	r, size, ok := decodeCodePoint(str, pos0)
	if !ok {
		L.RaiseError("invalid UTF-8 code")
		return 0
	}

	data.pos += size
	L.Push(LNumberInt(int64(pos + 1))) // Текущая позиция (1-based)
	L.Push(LNumberInt(int64(r)))       // Кодовая точка

	return 2
}

// utf8Codes - utf8.codes(s)
func utf8Codes(L *LState) int {
	s := L.CheckString(1)

	// Создаём UserData для хранения состояния
	ud := L.NewUserData()
	ud.Value = &utf8CodesData{str: s, pos: 0}

	// Возвращаем итератор и состояние
	L.Push(L.Get(UpvalueIndex(1)))
	L.Push(ud)

	return 2
}

// utf8Len - utf8.len(s [, i [, j [, lax]]])
func utf8Len(L *LState) int {
	s := L.CheckString(1)
	i := L.OptInt(2, 1)
	j := L.OptInt(3, -1)
	lax := L.OptBool(4, false)

	i = luaIndex2StringIndex(s, i, true)
	if j == -1 {
		j = len(s)
	} else {
		j = luaIndex2StringIndex(s, j, false)
	}

	if i > j || i >= len(s) {
		L.Push(LNumberInt(0))
		return 1
	}

	count := 0
	pos := i
	for pos < j && pos < len(s) {
		r, size, ok := decodeCodePoint(s, pos)
		if !ok {
			if lax {
				pos++
				count++
				continue
			}
			L.Push(LNil)
			L.Push(LNumberInt(int64(pos + 1)))
			return 2
		}

		if !lax && (r < 0 || (r >= 0xD800 && r <= 0xDFFF) || r > utf8MaxCodePoint) {
			L.Push(LNil)
			L.Push(LNumberInt(int64(pos + 1)))
			return 2
		}

		count++
		pos += size
	}

	L.Push(LNumberInt(int64(count)))
	return 1
}

// utf8Offset - utf8.offset(s, n [, i])
func utf8Offset(L *LState) int {
	s := L.CheckString(1)
	n := L.CheckInt(2)

	var i int
	if L.GetTop() >= 3 {
		i = L.CheckInt(3)
	} else {
		if n >= 0 {
			i = 1
		} else {
			i = len(s) + 1
		}
	}

	// Преобразуем отрицательные индексы
	if i < 0 {
		i = len(s) + i + 1
	}

	// Проверяем диапазон (1-based: от 1 до len(s)+1)
	if i < 1 || i > len(s)+1 {
		L.RaiseError("position out of range")
		return 0
	}

	i = i - 1 // конвертируем в 0-based индекс

	if n == 0 {
		// Для пустой строки возвращаем 1
		if len(s) == 0 {
			L.Push(LNumberInt(1))
			return 1
		}
		if i >= len(s) {
			L.Push(LNil)
			return 1
		}
		// Если i указывает на байт продолжения, ищем начало символа
		for i > 0 && s[i]&0xC0 == 0x80 {
			i--
		}
		L.Push(LNumberInt(int64(i + 1)))
		return 1
	}

	// Для n != 0, проверяем, что i не указывает на байт продолжения
	if i < len(s) && s[i]&0xC0 == 0x80 {
		L.RaiseError("continuation byte")
		return 0
	}

	if n > 0 {
		pos := i
		n-- // не перемещаемся для 1-го символа
		for n > 0 && pos < len(s) {
			// Перемещаемся к началу следующего символа
			pos++
			for pos < len(s) && s[pos]&0xC0 == 0x80 {
				pos++
			}
			n--
		}
		if n == 0 {
			L.Push(LNumberInt(int64(pos + 1)))
			return 1
		}
		L.Push(LNil)
		return 1
	} else {
		pos := i - 1
		if pos >= len(s) {
			pos = len(s) - 1
		}

		for n < 0 && pos >= 0 {
			start := pos
			for start > 0 && s[start]&0xC0 == 0x80 {
				start--
			}

			_, _, ok := decodeCodePoint(s, start)
			if !ok {
				L.Push(LNil)
				return 1
			}

			n++
			if n == 0 {
				L.Push(LNumberInt(int64(start + 1)))
				return 1
			}
			pos = start - 1
		}
	}

	L.Push(LNil)
	return 1
}

// OpenUtf8 открывает библиотеку utf8
func OpenUtf8(L *LState) int {
	mod := L.RegisterModule(Utf8LibName, utf8Funcs).(*LTable)

	mod.RawSetString("charpattern", LString(utf8CharPattern))
	mod.RawSetString("codespattern", LString(utf8CodesPattern))

	// Создаём utf8.codes как замыкание
	codesFn := L.NewClosure(utf8Codes, L.NewFunction(utf8CodesIter))
	mod.RawSetString("codes", codesFn)

	L.Push(mod)
	return 1
}

var utf8Funcs = map[string]LGFunction{
	"char":      utf8Char,
	"codepoint": utf8CodePoint,
	"codes":     utf8Codes,
	"len":       utf8Len,
	"offset":    utf8Offset,
}
