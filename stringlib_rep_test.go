package lua

import (
	"testing"
)

func TestStringRep(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: повторение без разделителя
	if err := L.DoString(`result = string.rep("ab", 3)`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "ababab" {
		t.Errorf("string.rep(\"ab\", 3) = %q, expected \"ababab\"", result)
	}

	// Тест: повторение с разделителем
	if err := L.DoString(`result = string.rep("ab", 3, "-")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "ab-ab-ab" {
		t.Errorf("string.rep(\"ab\", 3, \"-\") = %q, expected \"ab-ab-ab\"", result)
	}

	// Тест: повторение с пустым разделителем
	if err := L.DoString(`result = string.rep("ab", 3, "")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "ababab" {
		t.Errorf("string.rep(\"ab\", 3, \"\") = %q, expected \"ababab\"", result)
	}

	// Тест: повторение 0 раз
	if err := L.DoString(`result = string.rep("ab", 0)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "" {
		t.Errorf("string.rep(\"ab\", 0) = %q, expected \"\"", result)
	}

	// Тест: повторение отрицательное число раз
	if err := L.DoString(`result = string.rep("ab", -1)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "" {
		t.Errorf("string.rep(\"ab\", -1) = %q, expected \"\"", result)
	}

	// Тест: повторение 1 раз
	if err := L.DoString(`result = string.rep("ab", 1)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "ab" {
		t.Errorf("string.rep(\"ab\", 1) = %q, expected \"ab\"", result)
	}

	// Тест: повторение с длинным разделителем
	if err := L.DoString(`result = string.rep("x", 4, "---")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "x---x---x---x" {
		t.Errorf("string.rep(\"x\", 4, \"---\") = %q, expected \"x---x---x---x\"", result)
	}

	// Тест: повторение пустой строки
	if err := L.DoString(`result = string.rep("", 5)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "" {
		t.Errorf("string.rep(\"\", 5) = %q, expected \"\"", result)
	}

	// Тест: повторение с разделителем для одного элемента
	if err := L.DoString(`result = string.rep("ab", 1, "-")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "ab" {
		t.Errorf("string.rep(\"ab\", 1, \"-\") = %q, expected \"ab\"", result)
	}

	// Тест: повторение с разделителем для двух элементов
	if err := L.DoString(`result = string.rep("ab", 2, "-")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "ab-ab" {
		t.Errorf("string.rep(\"ab\", 2, \"-\") = %q, expected \"ab-ab\"", result)
	}

	// Тест: Unicode строка с разделителем
	if err := L.DoString(`result = string.rep("аб", 3, "|")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if result != "аб|аб|аб" {
		t.Errorf("string.rep(\"аб\", 3, \"|\") = %q, expected \"аб|аб|аб\"", result)
	}
}
