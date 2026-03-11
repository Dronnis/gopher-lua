package lua

import (
	"testing"
)

func TestTableMove(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: перемещение элементов в другую таблицу
	if err := L.DoString(`
		a1 = {1, 2, 3, 4, 5}
		a2 = {}
		table.move(a1, 1, 3, 1, a2)
	`); err != nil {
		t.Fatal(err)
	}
	a2 := L.GetField(L.Get(EnvironIndex), "a2").(*LTable)
	if a2.RawGetInt(1).(LNumber).Int64() != 1 ||
		a2.RawGetInt(2).(LNumber).Int64() != 2 ||
		a2.RawGetInt(3).(LNumber).Int64() != 3 {
		t.Errorf("table.move to another table failed")
	}

	// Тест: перемещение элементов в ту же таблицу (вперёд)
	if err := L.DoString(`
		a = {1, 2, 3, 4, 5}
		table.move(a, 1, 3, 4)
	`); err != nil {
		t.Fatal(err)
	}
	a := L.GetField(L.Get(EnvironIndex), "a").(*LTable)
	if a.RawGetInt(4).(LNumber).Int64() != 1 ||
		a.RawGetInt(5).(LNumber).Int64() != 2 ||
		a.RawGetInt(6).(LNumber).Int64() != 3 {
		t.Errorf("table.move forward in same table failed")
	}

	// Тест: перемещение элементов в ту же таблицу (назад)
	if err := L.DoString(`
		a = {1, 2, 3, 4, 5, 6, 7}
		table.move(a, 4, 6, 2)
	`); err != nil {
		t.Fatal(err)
	}
	a = L.GetField(L.Get(EnvironIndex), "a").(*LTable)
	if a.RawGetInt(2).(LNumber).Int64() != 4 ||
		a.RawGetInt(3).(LNumber).Int64() != 5 ||
		a.RawGetInt(4).(LNumber).Int64() != 6 {
		t.Errorf("table.move backward in same table failed")
	}

	// Тест: перемещение с отрицательными индексами не поддерживается (Lua 5.3 тоже)
	// Тест: пустой диапазон
	if err := L.DoString(`
		a1 = {1, 2, 3}
		a2 = {}
		table.move(a1, 3, 1, 1, a2)
	`); err != nil {
		t.Fatal(err)
	}
	a2 = L.GetField(L.Get(EnvironIndex), "a2").(*LTable)
	if a2.Len() != 0 {
		t.Errorf("table.move with f > e should return empty table")
	}

	// Тест: перемещение с смещением
	if err := L.DoString(`
		a1 = {1, 2, 3, 4, 5}
		a2 = {}
		table.move(a1, 2, 4, 3, a2)
	`); err != nil {
		t.Fatal(err)
	}
	a2 = L.GetField(L.Get(EnvironIndex), "a2").(*LTable)
	if a2.RawGetInt(1).(LValue) != LNil ||
		a2.RawGetInt(2).(LValue) != LNil ||
		a2.RawGetInt(3).(LNumber).Int64() != 2 ||
		a2.RawGetInt(4).(LNumber).Int64() != 3 ||
		a2.RawGetInt(5).(LNumber).Int64() != 4 {
		t.Errorf("table.move with offset failed")
	}

	// Тест: перемещение возвращает a2
	if err := L.DoString(`
		a1 = {1, 2, 3}
		a2 = {}
		result = table.move(a1, 1, 3, 1, a2)
	`); err != nil {
		t.Fatal(err)
	}
	result := L.GetField(L.Get(EnvironIndex), "result")
	a2 = L.GetField(L.Get(EnvironIndex), "a2").(*LTable)
	if result != a2 {
		t.Errorf("table.move should return a2")
	}

	// Тест: перемещение без указания a2 (по умолчанию a1)
	if err := L.DoString(`
		a = {1, 2, 3, 4, 5}
		table.move(a, 1, 2, 4)
	`); err != nil {
		t.Fatal(err)
	}
	a = L.GetField(L.Get(EnvironIndex), "a").(*LTable)
	if a.RawGetInt(4).(LNumber).Int64() != 1 ||
		a.RawGetInt(5).(LNumber).Int64() != 2 {
		t.Errorf("table.move without a2 failed")
	}
}

func TestTableMoveOverlapping(t *testing.T) {
	L := NewState()
	defer L.Close()

	// Тест: перекрывающиеся диапазоны (вперёд)
	if err := L.DoString(`
		a = {1, 2, 3, 4, 5}
		table.move(a, 1, 3, 2)
	`); err != nil {
		t.Fatal(err)
	}
	a := L.GetField(L.Get(EnvironIndex), "a").(*LTable)
	// Ожидаем: {1, 1, 2, 3, 5}
	if a.RawGetInt(1).(LNumber).Int64() != 1 ||
		a.RawGetInt(2).(LNumber).Int64() != 1 ||
		a.RawGetInt(3).(LNumber).Int64() != 2 ||
		a.RawGetInt(4).(LNumber).Int64() != 3 {
		t.Errorf("table.move overlapping forward failed: got [%v, %v, %v, %v]",
			a.RawGetInt(1), a.RawGetInt(2), a.RawGetInt(3), a.RawGetInt(4))
	}

	// Тест: перекрывающиеся диапазоны (назад)
	if err := L.DoString(`
		a = {1, 2, 3, 4, 5}
		table.move(a, 3, 5, 1)
	`); err != nil {
		t.Fatal(err)
	}
	a = L.GetField(L.Get(EnvironIndex), "a").(*LTable)
	// Ожидаем: {3, 4, 5, 4, 5}
	if a.RawGetInt(1).(LNumber).Int64() != 3 ||
		a.RawGetInt(2).(LNumber).Int64() != 4 ||
		a.RawGetInt(3).(LNumber).Int64() != 5 {
		t.Errorf("table.move overlapping backward failed")
	}
}
