package main

import "C"

import (
	"errors"
	"sync"
	"unsafe"

	decompressor "github.com/consensys/zkevm-monorepo/prover/lib/compressor/blob"
	"github.com/consensys/zkevm-monorepo/prover/lib/compressor/blob/dictionary"
)

//go:generate go build -tags nocorset -ldflags "-s -w" -buildmode=c-shared -o libdecompressor.so libdecompressor.go
func main() {}

var (
	dictStore dictionary.Store
	lastError error
	lock      sync.Mutex // probably unnecessary if coordinator guarantees single-threaded access
)

// Init initializes the decompressor.
//
//export Init
func Init() {
	dictStore = dictionary.NewStore()
}

// LoadDictionary loads nbDicts dictionaries into the decompressor
// Returns true if the operation is successful, false otherwise.
// If false is returned, the Error() method will return a string describing the error.
//
//export LoadDictionary
func LoadDictionary(dictPaths **C.char, nbDicts C.int) bool {
	lock.Lock()
	defer lock.Unlock()
	fpaths := make([]string, nbDicts)
	for i := 0; i < int(nbDicts); i++ {
		cStr := *(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(dictPaths)) + uintptr(i)*unsafe.Sizeof(*dictPaths)))
		fpaths[i] = C.GoString(cStr)
	}

	if err := dictStore.Load(fpaths...); err != nil {
		lastError = err
		return false
	}
	return true
}

// Decompress processes a blob b and writes the resulting blocks in out, serialized in the format of
// prover/backend/ethereum.
// Returns the number of bytes in out, or -1 in case of failure
// If -1 is returned, the Error() method will return a string describing the error.
//
//export Decompress
func Decompress(blob *C.char, blobLength C.int, out *C.char, outMaxLength C.int) C.int {

	lock.Lock()
	defer lock.Unlock()

	bGo := C.GoBytes(unsafe.Pointer(blob), blobLength)

	blocks, err := decompressor.DecompressBlob(bGo, dictStore)
	if err != nil {
		lastError = err
		return -1
	}

	if len(blocks) > int(outMaxLength) {
		lastError = errors.New("decoded blob does not fit in output buffer")
		return -1
	}

	outSlice := unsafe.Slice((*byte)(unsafe.Pointer(out)), len(blocks))
	copy(outSlice, blocks)

	return C.int(len(blocks))
}
