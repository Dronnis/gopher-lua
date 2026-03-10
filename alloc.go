package lua

import (
	"reflect"
	"unsafe"
)

// iface is an internal representation of the go-interface.
type iface struct {
	itab unsafe.Pointer
	word unsafe.Pointer
}

const preloadLimit = 128

var preloadsInt [int(preloadLimit)]LValue
var preloadsFloat [int(preloadLimit)]LValue

func init() {
	for i := 0; i < int(preloadLimit); i++ {
		preloadsInt[i] = LNumber{value: luaIntegerType(i)}
		preloadsFloat[i] = LNumber{value: luaFloatType(float64(i))}
	}
}

// allocator is a fast bulk memory allocator for the LValue.
type allocator struct {
	size    int
	fptrs   []float64
	iptrs   []int64
	fheader *reflect.SliceHeader
	iheader *reflect.SliceHeader
}

func newAllocator(size int) *allocator {
	al := &allocator{
		size:    size,
		fptrs:   make([]float64, 0, size),
		iptrs:   make([]int64, 0, size),
		fheader: nil,
		iheader: nil,
	}
	al.fheader = (*reflect.SliceHeader)(unsafe.Pointer(&al.fptrs))
	al.iheader = (*reflect.SliceHeader)(unsafe.Pointer(&al.iptrs))
	return al
}

// LNumber2I takes a number value and returns an interface LValue representing the same number.
func (al *allocator) LNumber2I(v LNumber) LValue {
	// first check for shared preloaded numbers
	if v.IsInteger() {
		iv := v.Int64()
		if iv >= 0 && iv < int64(preloadLimit) {
			return preloadsInt[int(iv)]
		}
	} else {
		fv := v.Float64()
		if fv >= 0 && fv < float64(preloadLimit) && fv == float64(int64(fv)) {
			return preloadsFloat[int(fv)]
		}
	}

	// check if we need a new alloc page
	if v.IsInteger() {
		if cap(al.iptrs) == len(al.iptrs) {
			al.iptrs = make([]int64, 0, al.size)
			al.iheader = (*reflect.SliceHeader)(unsafe.Pointer(&al.iptrs))
		}

		// alloc a new int, and store our value into it
		al.iptrs = append(al.iptrs, v.Int64())
		iptr := &al.iptrs[len(al.iptrs)-1]
		return LNumber{value: luaIntegerType(*iptr)}
	} else {
		if cap(al.fptrs) == len(al.fptrs) {
			al.fptrs = make([]float64, 0, al.size)
			al.fheader = (*reflect.SliceHeader)(unsafe.Pointer(&al.fptrs))
		}

		// alloc a new float, and store our value into it
		al.fptrs = append(al.fptrs, v.Float64())
		fptr := &al.fptrs[len(al.fptrs)-1]
		return LNumber{value: luaFloatType(*fptr)}
	}
}
