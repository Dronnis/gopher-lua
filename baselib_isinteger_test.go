package lua

import (
	"testing"
)

func TestIsInteger(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: isinteger для целого числа
	if err := L.DoString(`result = isinteger(42)`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result")
	if result != LTrue {
		t.Errorf("isinteger(42) = %v, expected true", result)
	}

	// Тест: isinteger для числа с плавающей точкой
	if err := L.DoString(`result = isinteger(3.14)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LFalse {
		t.Errorf("isinteger(3.14) = %v, expected false", result)
	}

	// Тест: isinteger для отрицательного целого числа
	if err := L.DoString(`result = isinteger(-10)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LTrue {
		t.Errorf("isinteger(-10) = %v, expected true", result)
	}

	// Тест: isinteger для нуля
	if err := L.DoString(`result = isinteger(0)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LTrue {
		t.Errorf("isinteger(0) = %v, expected true", result)
	}

	// Тест: isinteger для строки (должно вернуть false)
	if err := L.DoString(`result = isinteger("42")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LFalse {
		t.Errorf("isinteger(\"42\") = %v, expected false", result)
	}

	// Тест: isinteger для nil (должно вернуть false)
	if err := L.DoString(`result = isinteger(nil)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LFalse {
		t.Errorf("isinteger(nil) = %v, expected false", result)
	}

	// Тест: isinteger для таблицы (должно вернуть false)
	if err := L.DoString(`result = isinteger({})`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LFalse {
		t.Errorf("isinteger({}) = %v, expected false", result)
	}

	// Тест: isinteger для boolean (должно вернуть false)
	if err := L.DoString(`result = isinteger(true)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LFalse {
		t.Errorf("isinteger(true) = %v, expected false", result)
	}

	// Тест: isinteger для function (должно вернуть false)
	if err := L.DoString(`result = isinteger(function() end)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LFalse {
		t.Errorf("isinteger(function) = %v, expected false", result)
	}

	// Тест: isinteger для math.huge (должно вернуть false)
	if err := L.DoString(`result = isinteger(math.huge)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result")
	if result != LFalse {
		t.Errorf("isinteger(math.huge) = %v, expected false", result)
	}
}

func TestIsIntegerVsMathType(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Сравнение isinteger и math.type для целого числа
	if err := L.DoString(`
		local val = 42
		local int_result = isinteger(val)
		local type_result = math.type(val)
		return int_result, type_result
	`); err != nil {
		t.Fatal(err)
	}

	intResult := L.Get(1)
	typeResult := L.Get(2).String()
	L.Pop(2)

	if intResult != LTrue {
		t.Errorf("isinteger(42) = %v, expected true", intResult)
	}
	if typeResult != "integer" {
		t.Errorf("math.type(42) = %q, expected \"integer\"", typeResult)
	}

	// Тест для float
	if err := L.DoString(`
		local val = 3.14
		local int_result = isinteger(val)
		local type_result = math.type(val)
		return int_result, type_result
	`); err != nil {
		t.Fatal(err)
	}

	intResult = L.Get(1)
	typeResult = L.Get(2).String()
	L.Pop(2)

	if intResult != LFalse {
		t.Errorf("isinteger(3.14) = %v, expected false", intResult)
	}
	if typeResult != "float" {
		t.Errorf("math.type(3.14) = %q, expected \"float\"", typeResult)
	}
}
