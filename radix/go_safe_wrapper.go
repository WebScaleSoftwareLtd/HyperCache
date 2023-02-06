package radix

import (
	"C"
	"reflect"
	"runtime"
	"unsafe"
)

type byteSlice struct {
	value  uintptr
	length uintptr
}

// This function is VERY unsafe. For the love of god, make sure to "defer runtime.KeepAlive(b)" and
// "defer runtime.KeepAlive(keepThisAlive)" in the calling function. Otherwise, the GC will
// collect the byte slice and the radix tree will be left with a dangling pointer.
func shortTermByteSlice(b []byte) (keepThisAlive *byteSlice, cValue ByteSlice) {
	var ptr uintptr
	bLen := uintptr(len(b))
	if bLen != 0 {
		ptr = uintptr(unsafe.Pointer(&b[0]))
	}
	val := &byteSlice{
		value:  ptr,
		length: bLen,
	}
	cptr := SwigcptrByteSlice(unsafe.Pointer(&val.value))
	return val, cptr
}

type RadixTree struct {
	cObj RadixTreeRoot
}

func NewRadixTree() RadixTree {
	return RadixTree{cObj: NewRadixTreeRoot__SWIG_0()}
}

func (r RadixTree) Get(key []byte) (value []byte, deallocator func()) {
	defer runtime.KeepAlive(key)
	keepAlive, keyC := shortTermByteSlice(key)
	defer runtime.KeepAlive(keepAlive)

	possibleValue := r.cObj.Get(keyC)
	if possibleValue == nil || possibleValue.Swigcptr() == 0 {
		return nil, func() {}
	}

	byteSlice := *(*byteSlice)(unsafe.Pointer(possibleValue.Swigcptr()))
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
			Data: byteSlice.value,
			Len:  int(byteSlice.length),
			Cap:  int(byteSlice.length),
		})), func() {
			Swig_free(possibleValue.Swigcptr())
			Swig_free(byteSlice.value)
		}
}

type RadixTreeWalkValueGo struct {
	key   byteSlice
	value byteSlice
}

type WalkerDestructor interface {
	MarkToFree(value uintptr)
}

type ImmediateFreer struct{}

func (ImmediateFreer) MarkToFree(value uintptr) {
	Swig_free(value)
}

type freerStack struct {
	prev *freerStack
	val  uintptr
}

type PendingFreer struct {
	x *freerStack
}

func (p *PendingFreer) MarkToFree(value uintptr) {
	p.x = &freerStack{prev: p.x, val: value}
}

func (p *PendingFreer) FreeAll() {
	for p.x != nil {
		Swig_free(p.x.val)
		p.x = p.x.prev
	}
}

func (r RadixTree) WalkPrefix(prefix []byte, hn func(key, value []byte) bool, destructor WalkerDestructor) {
	keepAlive, cVal := shortTermByteSlice(prefix)
	walker := r.cObj.Walk_prefix(cVal)

	// In this instance, the prefix only needs to be kept alive to here.
	runtime.KeepAlive(keepAlive)
	runtime.KeepAlive(prefix)

	// Defer the destruction of the walker.
	defer Swig_free(walker.Swigcptr())

	for {
		// Get the next value.
		val := walker.Next()

		// Check if it is null and return if so.
		if val == nil || val.Swigcptr() == 0 {
			return
		}

		// Turn this into a Go type.
		goVal := *(*RadixTreeWalkValueGo)(unsafe.Pointer(val.Swigcptr()))

		// Free the inner walker value.
		destructor.MarkToFree(val.Swigcptr())

		// Create unsafe headers for both key and value.
		unsafeKey := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
			Data: goVal.key.value,
			Len:  int(goVal.key.length),
			Cap:  int(goVal.key.length),
		}))
		unsafeValue := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
			Data: goVal.value.value,
			Len:  int(goVal.value.length),
			Cap:  int(goVal.value.length),
		}))

		// Call the specified handler.
		shouldNotBreak := hn(unsafeKey, unsafeValue)

		// Free both the key and value.
		destructor.MarkToFree(goVal.key.value)
		destructor.MarkToFree(goVal.value.value)

		// If we should break, do so now.
		if !shouldNotBreak {
			break
		}
	}
}

func (r RadixTree) Set(key, value []byte) bool {
	defer runtime.KeepAlive(key)
	keepAlive, keyC := shortTermByteSlice(key)
	defer runtime.KeepAlive(keepAlive)

	defer runtime.KeepAlive(value)
	var ptr *byte
	if len(value) != 0 {
		ptr = &value[0]
	}
	return Set_with_stack_value(
		r.cObj, keyC,
		SwigcptrUint8_t(unsafe.Pointer(ptr)),
		int64(len(value)))
}

func (r RadixTree) DeleteKey(key []byte) bool {
	defer runtime.KeepAlive(key)
	keepAlive, cVal := shortTermByteSlice(key)
	defer runtime.KeepAlive(keepAlive)

	return r.cObj.Delete_key(cVal)
}

func (r RadixTree) DeletePrefix(prefix []byte) uint64 {
	defer runtime.KeepAlive(prefix)
	keepAlive, cVal := shortTermByteSlice(prefix)
	defer runtime.KeepAlive(keepAlive)

	return uint64(r.cObj.Delete_prefix(cVal))
}

func (r RadixTree) FreeTree() {
	r.cObj.Free_tree()
}
