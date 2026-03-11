package lua

import (
	"testing"
)

func TestUtf8Char(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: простой ASCII символ
	if err := L.DoString(`result = utf8.char(65)`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "A" {
		t.Errorf("utf8.char(65) = %q, expected \"A\"", result)
	}

	// Тест: символ Unicode (€ - Euro sign)
	if err := L.DoString(`result = utf8.char(8364)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "€" {
		t.Errorf("utf8.char(8364) = %q, expected \"€\"", result)
	}

	// Тест: несколько кодовых точек
	if err := L.DoString(`result = utf8.char(72, 101, 108, 108, 111)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "Hello" {
		t.Errorf("utf8.char(72,101,108,108,111) = %q, expected \"Hello\"", result)
	}

	// Тест: кириллица
	if err := L.DoString(`result = utf8.char(1055, 1088, 1080, 1074, 1077, 1090)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "Привет" {
		t.Errorf("utf8.char cyrillic = %q, expected \"Привет\"", result)
	}
}

func TestUtf8Codepoint(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: простая строка
	if err := L.DoString(`result = utf8.codepoint("Hello")`); err != nil {
		t.Fatal(err)
	}
	result := int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 72 {
		t.Errorf("utf8.codepoint(\"Hello\") = %d, expected 72", result)
	}

	// Тест: несколько кодовых точек
	if err := L.DoString(`a, b, c, d, e = utf8.codepoint("Hello", 1, 5)`); err != nil {
		t.Fatal(err)
	}
	a := int(L.GetField(L.Get(EnvironIndex), "a").(LNumber).Int64())
	b := int(L.GetField(L.Get(EnvironIndex), "b").(LNumber).Int64())
	c := int(L.GetField(L.Get(EnvironIndex), "c").(LNumber).Int64())
	d := int(L.GetField(L.Get(EnvironIndex), "d").(LNumber).Int64())
	e := int(L.GetField(L.Get(EnvironIndex), "e").(LNumber).Int64())
	if a != 72 || b != 101 || c != 108 || d != 108 || e != 111 {
		t.Errorf("utf8.codepoint(\"Hello\", 1, 5) = %d,%d,%d,%d,%d", a, b, c, d, e)
	}

	// Тест: Unicode символ
	if err := L.DoString(`result = utf8.codepoint("€")`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 8364 {
		t.Errorf("utf8.codepoint(\"€\") = %d, expected 8364", result)
	}
}

func TestUtf8Codes(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: итератор для ASCII
	if err := L.DoString(`
		result = {}
		for pos, code in utf8.codes("ABC") do
			table.insert(result, {pos, code})
		end
	`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result").(*LTable)
	if result.Len() != 3 {
		t.Errorf("utf8.codes(\"ABC\") returned %d items, expected 3", result.Len())
	}

	// Тест: итератор для Unicode
	if err := L.DoString(`
		result = {}
		for pos, code in utf8.codes("€") do
			table.insert(result, {pos, code})
		end
		codepoint = result[1][2]
	`); err != nil {
		t.Fatal(err)
	}
	resultCode := int(L.GetField(L.Get(EnvironIndex), "codepoint").(LNumber).Int64())
	if resultCode != 8364 {
		t.Errorf("utf8.codes(\"€\") code = %d, expected 8364", resultCode)
	}
}

func TestUtf8Len(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: ASCII строка
	if err := L.DoString(`result = utf8.len("Hello")`); err != nil {
		t.Fatal(err)
	}
	result := int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 5 {
		t.Errorf("utf8.len(\"Hello\") = %d, expected 5", result)
	}

	// Тест: Unicode строка
	if err := L.DoString(`result = utf8.len("Привет")`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 6 {
		t.Errorf("utf8.len(\"Привет\") = %d, expected 6", result)
	}

	// Тест: смешанная строка
	if err := L.DoString(`result = utf8.len("Hello €")`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 7 {
		t.Errorf("utf8.len(\"Hello €\") = %d, expected 7", result)
	}

	// Тест: диапазон
	if err := L.DoString(`result = utf8.len("Hello", 1, 3)`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 3 {
		t.Errorf("utf8.len(\"Hello\", 1, 3) = %d, expected 3", result)
	}
}

func TestUtf8Offset(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: смещение вперёд
	if err := L.DoString(`result = utf8.offset("Hello", 2)`); err != nil {
		t.Fatal(err)
	}
	result := int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 2 {
		t.Errorf("utf8.offset(\"Hello\", 2) = %d, expected 2", result)
	}

	// Тест: отрицательное смещение
	if err := L.DoString(`result = utf8.offset("Hello", -1)`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 5 {
		t.Errorf("utf8.offset(\"Hello\", -1) = %d, expected 5", result)
	}

	// Тест: Unicode строка
	if err := L.DoString(`result = utf8.offset("Привет", 2)`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 3 {
		t.Errorf("utf8.offset(\"Привет\", 2) = %d, expected 3", result)
	}
}

func TestUtf8Patterns(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: charpattern
	if err := L.DoString(`result = utf8.charpattern`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result").String()
	if result == "" {
		t.Error("utf8.charpattern should not be empty")
	}

	// Тест: codespattern
	if err := L.DoString(`result = utf8.codespattern`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result == "" {
		t.Error("utf8.codespattern should not be empty")
	}
}

func TestUtf8LenWithLax(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: невалидный UTF-8 без lax
	// Используем string.char для создания бинарных байтов
	if err := L.DoString(`result, pos = utf8.len(string.char(128, 129, 130))`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result")
	if result != LNil {
		t.Error("utf8.len with invalid UTF-8 should return nil")
	}

	// Тест: невалидный UTF-8 с lax = true
	if err := L.DoString(`result = utf8.len(string.char(128, 129, 130), 1, -1, true)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result.Type() != LTNumber {
		t.Error("utf8.len with lax=true should return a number")
	}
}
