package types

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// Confidence describes how strongly the calldata pattern implies the transfer.
type Confidence uint8

const (
	// Deterministic means recipient and amount are encoded directly in calldata.
	// If the surrounding call succeeds, the transfer occurs exactly as reported.
	Deterministic Confidence = iota
	// Likely means the pattern implies an ETH movement, but the amount depends on
	// runtime state. Reserved for parsers you may add later.
	Likely
	// Possible means the function may move ETH, but it is not reliably
	// predictable from calldata alone.
	Possible
)

func (c Confidence) String() string {
	switch c {
	case Deterministic:
		return "deterministic"
	case Likely:
		return "likely"
	case Possible:
		return "possible"
	}
	return "unknown"
}

type Address [20]byte

// ZeroAddress is the zero-value address (0x0000...0000).
var ZeroAddress Address

func AddressFromHex(s string) (Address, error) {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if len(s) != 40 {
		return Address{}, fmt.Errorf("invalid address length: %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Address{}, fmt.Errorf("invalid hex: %w", err)
	}
	var addr Address
	copy(addr[:], b)
	return addr, nil
}

func (a Address) Hex() string {
	return "0x" + hex.EncodeToString(a[:])
}

func (a Address) String() string {
	return a.Hex()
}

func (a Address) IsZero() bool {
	return a == ZeroAddress
}

// Selector represents a 4-byte method selector.
type Selector [4]byte

func (s Selector) Hex() string {
	return "0x" + hex.EncodeToString(s[:])
}

func (s Selector) String() string {
	return s.Hex()
}

func SelectorFromBytes(b []byte) Selector {
	var sel Selector
	copy(sel[:], b)
	return sel
}

type RawInputData string

type InputParams []byte

type ParsedInputData struct {
	Selector  Selector
	Transfers []Transfer
}

type Transfer struct {
	From       Address
	To         Address
	Value      *big.Int
	Confidence Confidence
}
