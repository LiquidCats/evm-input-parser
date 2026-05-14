package codec

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/LiquidCats/evm-input-parser/types"
)

const wordSize = 32

// ReadWord reads a single 32-byte word at the given word index.
func ReadWord(params []byte, wordIdx int) ([]byte, error) {
	return ReadWordAt(params, wordIdx*wordSize)
}

func ReadWordAt(params []byte, offset int) ([]byte, error) {
	if offset < 0 || offset+wordSize > len(params) {
		return nil, fmt.Errorf("ReadWordAt: out of bounds at offset %d (need %d, have %d)", offset, offset+wordSize, len(params))
	}
	return params[offset : offset+wordSize], nil
}

// ReadAddress reads a 20-byte address from a 32-byte word at wordIdx.
func ReadAddress(params []byte, wordIdx int) (types.Address, error) {
	return ReadAddressAt(params, wordIdx*wordSize)
}

func ReadAddressAt(params []byte, offset int) (types.Address, error) {
	word, err := ReadWordAt(params, offset)
	if err != nil {
		return types.Address{}, err
	}
	var addr types.Address
	copy(addr[:], word[12:32])
	return addr, nil
}

// ReadUint256 reads a *big.Int from a 32-byte word at wordIdx.
func ReadUint256(params []byte, wordIdx int) (*big.Int, error) {
	return ReadUint256At(params, wordIdx*wordSize)
}

func ReadUint256At(params []byte, offset int) (*big.Int, error) {
	word, err := ReadWordAt(params, offset)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(word), nil
}

// ReadUint64 reads a uint64 from the low 8 bytes of the word at wordIdx.
func ReadUint64(params []byte, wordIdx int) (uint64, error) {
	word, err := ReadWord(params, wordIdx)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(word[24:32]), nil
}

// ReadBool reads a boolean from the word at wordIdx.
func ReadBool(params []byte, wordIdx int) (bool, error) {
	word, err := ReadWord(params, wordIdx)
	if err != nil {
		return false, err
	}
	return word[31] != 0, nil
}

// ReadOffset reads a dynamic offset (in bytes) from wordIdx.
func ReadOffset(params []byte, wordIdx int) (int, error) {
	return ReadOffsetAt(params, wordIdx*wordSize)
}

func ReadOffsetAt(params []byte, offset int) (int, error) {
	v, err := ReadUint256At(params, offset)
	if err != nil {
		return 0, err
	}
	if !v.IsInt64() || v.Int64() > math.MaxInt32 {
		return 0, fmt.Errorf("offset is too large: %s", v)
	}
	return int(v.Int64()), nil
}

// ReadDynamicBytes reads a bytes/bytes[] from an offset pointer at wordIdx.
// The offset points to a length-prefixed byte sequence.
func ReadDynamicBytes(params []byte, wordIdx int) ([]byte, error) {
	offset, err := ReadOffset(params, wordIdx)
	if err != nil {
		return nil, fmt.Errorf("ReadDynamicBytes: read offset: %w", err)
	}
	return ReadDynamicBytesAt(params, offset)
}

// ReadDynamicBytesAt reads a length-prefixed byte sequence at the given
// absolute byte offset within params.
func ReadDynamicBytesAt(params []byte, offset int) ([]byte, error) {
	if offset < 0 || offset+wordSize > len(params) {
		return nil, fmt.Errorf("ReadDynamicBytesAt: length out of bounds at offset %d", offset)
	}
	length := new(big.Int).SetBytes(params[offset : offset+wordSize])
	if !length.IsInt64() || length.Int64() > math.MaxInt32 {
		return nil, fmt.Errorf("ReadDynamicBytesAt: unreasonable length %s", length)
	}
	dataStart := offset + wordSize
	dataEnd := dataStart + int(length.Int64())
	if dataEnd < dataStart || dataEnd > len(params) {
		return nil, fmt.Errorf("ReadDynamicBytesAt: data out of bounds: need %d, have %d", dataEnd, len(params))
	}
	return params[dataStart:dataEnd], nil
}

// ReadArrayLength reads the length of a dynamic array at the given absolute
// byte offset.
func ReadArrayLength(params []byte, offset int) (int, error) {
	if offset < 0 || offset+wordSize > len(params) {
		return 0, fmt.Errorf("ReadArrayLength: out of bounds at offset %d", offset)
	}
	n := new(big.Int).SetBytes(params[offset : offset+wordSize])
	if !n.IsInt64() || n.Int64() > 10_000 {
		return 0, fmt.Errorf("ReadArrayLength: unreasonable length %s", n)
	}
	return int(n.Int64()), nil
}

// ReadBytesArrayElements reads an array of dynamic bytes elements.
// arrayOffset is the absolute byte offset of the array length word.
func ReadBytesArrayElements(params []byte, arrayOffset int) ([][]byte, error) {
	count, err := ReadArrayLength(params, arrayOffset)
	if err != nil {
		return nil, fmt.Errorf("ReadBytesArrayElements: %w", err)
	}

	elemBase := arrayOffset + wordSize // first element offset pointer
	elements := make([][]byte, 0, count)

	for i := range count {
		ptrOffset := elemBase + i*wordSize
		relOffset, err := ReadOffsetAt(params, ptrOffset)
		if err != nil {
			return nil, fmt.Errorf("ReadBytesArrayElements: pointer %d: %w", i, err)
		}
		data, err := ReadDynamicBytesAt(params, elemBase+relOffset)
		if err != nil {
			return nil, fmt.Errorf("ReadBytesArrayElements[%d]: %w", i, err)
		}
		elements = append(elements, data)
	}

	return elements, nil
}
