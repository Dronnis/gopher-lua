package lua

import (
	"testing"
)

func TestStringPack(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: упаковка чисел
	if err := L.DoString(`result = string.pack("b", 42)`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 1 || result[0] != 42 {
		t.Errorf("string.pack(\"b\", 42) = %q, expected single byte 42", result)
	}

	// Тест: упаковка нескольких чисел
	if err := L.DoString(`result = string.pack("bb", 10, 20)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 2 || result[0] != 10 || result[1] != 20 {
		t.Errorf("string.pack(\"bb\", 10, 20) = %q", result)
	}

	// Тест: unsigned char
	if err := L.DoString(`result = string.pack("B", 255)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 1 || result[0] != 255 {
		t.Errorf("string.pack(\"B\", 255) = %q", result)
	}

	// Тест: short (little-endian)
	if err := L.DoString(`result = string.pack("<h", 256)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 2 || result[0] != 0 || result[1] != 1 {
		t.Errorf("string.pack(\"<h\", 256) = %q, expected [0, 1]", result)
	}

	// Тест: long (little-endian)
	if err := L.DoString(`result = string.pack("<l", 16909060)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 4 || result[0] != 4 || result[1] != 3 || result[2] != 2 || result[3] != 1 {
		t.Errorf("string.pack(\"<l\", 16909060) = %q, expected [4, 3, 2, 1]", result)
	}

	// Тест: big-endian
	if err := L.DoString(`result = string.pack(">h", 256)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 2 || result[0] != 1 || result[1] != 0 {
		t.Errorf("string.pack(\">h\", 256) = %q, expected [1, 0]", result)
	}

	// Тест: float
	if err := L.DoString(`result = string.pack("<f", 1.0)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 4 {
		t.Errorf("string.pack(\"<f\", 1.0) length = %d, expected 4", len(result))
	}

	// Тест: double
	if err := L.DoString(`result = string.pack("<d", 1.0)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 8 {
		t.Errorf("string.pack(\"<d\", 1.0) length = %d, expected 8", len(result))
	}

	// Тест: строка фиксированной длины
	if err := L.DoString(`result = string.pack("c4", "AB")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 4 || result[0] != 'A' || result[1] != 'B' || result[2] != 0 || result[3] != 0 {
		t.Errorf("string.pack(\"c4\", \"AB\") = %q", result)
	}

	// Тест: строка с нулевым терминатором
	if err := L.DoString(`result = string.pack("z", "ABC")`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 4 || result[0] != 'A' || result[1] != 'B' || result[2] != 'C' || result[3] != 0 {
		t.Errorf("string.pack(\"z\", \"ABC\") = %q", result)
	}

	// Тест: повторения
	if err := L.DoString(`result = string.pack("3b", 1, 2, 3)`); err != nil {
		t.Fatal(err)
	}
	result = L.GetField(L.Get(EnvironIndex), "result").String()
	if len(result) != 3 || result[0] != 1 || result[1] != 2 || result[2] != 3 {
		t.Errorf("string.pack(\"3b\", 1, 2, 3) = %q", result)
	}
}

func TestStringUnpack(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: распаковка signed char
	if err := L.DoString(`val, pos = string.unpack("b", string.char(42))`); err != nil {
		t.Fatal(err)
	}
	val := int(L.GetField(L.Get(EnvironIndex), "val").(LNumber).Int64())
	if val != 42 {
		t.Errorf("string.unpack(\"b\", ...) = %d, expected 42", val)
	}

	// Тест: распаковка unsigned char
	if err := L.DoString(`val, pos = string.unpack("B", string.char(255))`); err != nil {
		t.Fatal(err)
	}
	val = int(L.GetField(L.Get(EnvironIndex), "val").(LNumber).Int64())
	if val != 255 {
		t.Errorf("string.unpack(\"B\", ...) = %d, expected 255", val)
	}

	// Тест: распаковка short (little-endian)
	if err := L.DoString(`val, pos = string.unpack("<h", string.char(0, 1))`); err != nil {
		t.Fatal(err)
	}
	val = int(L.GetField(L.Get(EnvironIndex), "val").(LNumber).Int64())
	if val != 256 {
		t.Errorf("string.unpack(\"<h\", ...) = %d, expected 256", val)
	}

	// Тест: распаковка long (little-endian)
	if err := L.DoString(`val, pos = string.unpack("<l", string.char(4, 3, 2, 1))`); err != nil {
		t.Fatal(err)
	}
	val = int(L.GetField(L.Get(EnvironIndex), "val").(LNumber).Int64())
	if val != 16909060 {
		t.Errorf("string.unpack(\"<l\", ...) = %d, expected 16909060", val)
	}

	// Тест: распаковка big-endian
	if err := L.DoString(`val, pos = string.unpack(">h", string.char(1, 0))`); err != nil {
		t.Fatal(err)
	}
	val = int(L.GetField(L.Get(EnvironIndex), "val").(LNumber).Int64())
	if val != 256 {
		t.Errorf("string.unpack(\">h\", ...) = %d, expected 256", val)
	}

	// Тест: распаковка нескольких значений
	if err := L.DoString(`a, b, c, pos = string.unpack("bbb", string.char(10, 20, 30))`); err != nil {
		t.Fatal(err)
	}
	a := int(L.GetField(L.Get(EnvironIndex), "a").(LNumber).Int64())
	b := int(L.GetField(L.Get(EnvironIndex), "b").(LNumber).Int64())
	c := int(L.GetField(L.Get(EnvironIndex), "c").(LNumber).Int64())
	if a != 10 || b != 20 || c != 30 {
		t.Errorf("string.unpack(\"bbb\", ...) = %d,%d,%d, expected 10,20,30", a, b, c)
	}

	// Тест: распаковка строки фиксированной длины
	if err := L.DoString(`val, pos = string.unpack("c4", "AB" .. string.char(0, 0))`); err != nil {
		t.Fatal(err)
	}
	valStr := L.GetField(L.Get(EnvironIndex), "val").String()
	if valStr != "AB" {
		t.Errorf("string.unpack(\"c4\", ...) = %q, expected \"AB\"", valStr)
	}

	// Тест: распаковка строки с нулевым терминатором
	if err := L.DoString(`val, pos = string.unpack("z", "ABC" .. string.char(0))`); err != nil {
		t.Fatal(err)
	}
	valStr = L.GetField(L.Get(EnvironIndex), "val").String()
	if valStr != "ABC" {
		t.Errorf("string.unpack(\"z\", ...) = %q, expected \"ABC\"", valStr)
	}

	// Тест: позиция
	if err := L.DoString(`val, pos = string.unpack("b", string.char(42))`); err != nil {
		t.Fatal(err)
	}
	pos := int(L.GetField(L.Get(EnvironIndex), "pos").(LNumber).Int64())
	if pos != 2 {
		t.Errorf("string.unpack next position = %d, expected 2", pos)
	}
}

func TestStringPackSize(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: размер signed char
	if err := L.DoString(`size = string.packsize("b")`); err != nil {
		t.Fatal(err)
	}
	size := int(L.GetField(L.Get(EnvironIndex), "size").(LNumber).Int64())
	if size != 1 {
		t.Errorf("string.packsize(\"b\") = %d, expected 1", size)
	}

	// Тест: размер short
	if err := L.DoString(`size = string.packsize("h")`); err != nil {
		t.Fatal(err)
	}
	size = int(L.GetField(L.Get(EnvironIndex), "size").(LNumber).Int64())
	if size != 2 {
		t.Errorf("string.packsize(\"h\") = %d, expected 2", size)
	}

	// Тест: размер long
	if err := L.DoString(`size = string.packsize("l")`); err != nil {
		t.Fatal(err)
	}
	size = int(L.GetField(L.Get(EnvironIndex), "size").(LNumber).Int64())
	if size != 4 {
		t.Errorf("string.packsize(\"l\") = %d, expected 4", size)
	}

	// Тест: размер float
	if err := L.DoString(`size = string.packsize("f")`); err != nil {
		t.Fatal(err)
	}
	size = int(L.GetField(L.Get(EnvironIndex), "size").(LNumber).Int64())
	if size != 4 {
		t.Errorf("string.packsize(\"f\") = %d, expected 4", size)
	}

	// Тест: размер double
	if err := L.DoString(`size = string.packsize("d")`); err != nil {
		t.Fatal(err)
	}
	size = int(L.GetField(L.Get(EnvironIndex), "size").(LNumber).Int64())
	if size != 8 {
		t.Errorf("string.packsize(\"d\") = %d, expected 8", size)
	}

	// Тест: составной формат
	if err := L.DoString(`size = string.packsize("bhl")`); err != nil {
		t.Fatal(err)
	}
	size = int(L.GetField(L.Get(EnvironIndex), "size").(LNumber).Int64())
	if size != 7 {
		t.Errorf("string.packsize(\"bhl\") = %d, expected 7", size)
	}

	// Тест: формат с повторениями
	if err := L.DoString(`size = string.packsize("3b")`); err != nil {
		t.Fatal(err)
	}
	size = int(L.GetField(L.Get(EnvironIndex), "size").(LNumber).Int64())
	if size != 3 {
		t.Errorf("string.packsize(\"3b\") = %d, expected 3", size)
	}

	// Тест: формат с переменным размером (z, s) возвращает nil
	if err := L.DoString(`size = string.packsize("z")`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "size")
	if result != LNil {
		t.Errorf("string.packsize(\"z\") = %v, expected nil", result)
	}
}

func TestStringPackUnpackRoundTrip(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: round-trip для чисел
	if err := L.DoString(`
		local packed = string.pack("bhlfd", 42, 1000, 50000, 1.5, 2.5)
		local a, b, c, d, e = string.unpack("bhlfd", packed)
		assert(a == 42)
		assert(b == 1000)
		assert(c == 50000)
		assert(math.abs(d - 1.5) < 0.001)
		assert(math.abs(e - 2.5) < 0.001)
	`); err != nil {
		t.Fatal(err)
	}

	// Тест: round-trip для строки
	if err := L.DoString(`
		local packed = string.pack("c10", "Hello")
		local s = string.unpack("c10", packed)
		assert(s == "Hello")
	`); err != nil {
		t.Fatal(err)
	}
}

func TestStringPackBigEndian(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: big-endian упаковка и распаковка
	if err := L.DoString(`
		local packed = string.pack(">L", 0x01020304)
		local val = string.unpack(">L", packed)
		assert(val == 0x01020304)
		assert(#packed == 4)
		assert(string.byte(packed, 1) == 1)
		assert(string.byte(packed, 2) == 2)
		assert(string.byte(packed, 3) == 3)
		assert(string.byte(packed, 4) == 4)
	`); err != nil {
		t.Fatal(err)
	}
}
