package utility

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
)

func RandomSeed() (seed int64, err error) {
	err = binary.Read(rand.Reader, binary.LittleEndian, &seed)
	return
}

// RandId creates a random identifier of the specified length
func RandId(idLen int) string {
	b := make([]byte, idLen)
	var randVal uint32
	for i := 0; i < idLen; i++ {
		byteIdx := i % 4
		if byteIdx == 0 {
			randVal = mrand.Uint32()
		}
		b[i] = byte((randVal >> (8 * uint(byteIdx))) & 0xFF)
	}
	return fmt.Sprintf("%x", b)
}

// SecureRandId like @RandId, but uses a crypto/rand for secure random identifiers
func SecureRandId(idLen int) (id string, err error) {
	b := make([]byte, idLen)
	var n int
	if n, err = rand.Read(b); n != idLen {
		err = fmt.Errorf("only generated %d random bytes, %d requested", n, idLen)
	}
	if err != nil {
		return
	}
	id = fmt.Sprintf("%x", b)
	return
}

func SecureRandIdOrPanic(idLen int) string {
	id, err := SecureRandId(idLen)
	if err != nil {
		panic(err)
	}
	return id
}
