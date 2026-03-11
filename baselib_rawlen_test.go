package lua

import (
	"testing"
)

func TestRawLen(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: rawlen для строки
	if err := L.DoString(`result = rawlen("Hello")`); err != nil {
		t.Fatal(err)
	}
	result := int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 5 {
		t.Errorf("rawlen(\"Hello\") = %d, expected 5", result)
	}

	// Тест: rawlen для таблицы
	if err := L.DoString(`result = rawlen({1, 2, 3, 4, 5})`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 5 {
		t.Errorf("rawlen({1,2,3,4,5}) = %d, expected 5", result)
	}

	// Тест: rawlen для пустой таблицы
	if err := L.DoString(`result = rawlen({})`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 0 {
		t.Errorf("rawlen({}) = %d, expected 0", result)
	}

	// Тест: rawlen для таблицы с nil элементами
	if err := L.DoString(`t = {1, 2, nil, 4, 5}; result = rawlen(t)`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 5 {
		t.Errorf("rawlen({1, 2, nil, 4, 5}) = %d, expected 5", result)
	}

	// Тест: rawlen с метаметодом __len (должен игнорировать его)
	if err := L.DoString(`
		t = {1, 2, 3}
		mt = {__len = function() return 100 end}
		setmetatable(t, mt)
		result = rawlen(t)
	`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 3 {
		t.Errorf("rawlen with __len metamethod = %d, expected 3 (should ignore __len)", result)
	}

	// Тест: rawlen для пустой строки
	if err := L.DoString(`result = rawlen("")`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 0 {
		t.Errorf("rawlen(\"\") = %d, expected 0", result)
	}

	// Тест: rawlen для Unicode строки (длина в байтах)
	if err := L.DoString(`result = rawlen("Привет")`); err != nil {
		t.Fatal(err)
	}
	result = int(L.GetField(L.Get(EnvironIndex), "result").(LNumber).Int64())
	if result != 12 {
		t.Errorf("rawlen(\"Привет\") = %d, expected 12 (bytes)", result)
	}

	// Тест: rawlen для числа (должна быть ошибка)
	err := L.DoString(`result = rawlen(42)`)
	if err == nil {
		t.Error("rawlen(42) should return error")
	}

	// Тест: rawlen для nil (должна быть ошибка)
	err = L.DoString(`result = rawlen(nil)`)
	if err == nil {
		t.Error("rawlen(nil) should return error")
	}

	// Тест: rawlen для функции (должна быть ошибка)
	err = L.DoString(`result = rawlen(function() end)`)
	if err == nil {
		t.Error("rawlen(function) should return error")
	}
}

func TestRawLenVsLen(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: сравнение # и rawlen с метаметодом __len
	if err := L.DoString(`
		t = {1, 2, 3, 4, 5}
		mt = {__len = function() return 10 end}
		setmetatable(t, mt)
		raw = rawlen(t)
		meta = #t
	`); err != nil {
		t.Fatal(err)
	}

	raw := int(L.GetField(L.Get(EnvironIndex), "raw").(LNumber).Int64())
	meta := int(L.GetField(L.Get(EnvironIndex), "meta").(LNumber).Int64())

	if raw != 5 {
		t.Errorf("rawlen(t) = %d, expected 5", raw)
	}
	if meta != 10 {
		t.Errorf("#t with __len = %d, expected 10", meta)
	}
}
