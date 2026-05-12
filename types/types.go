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

// ChildCall is calldata discovered inside a decoded call frame. The parser, not
// individual codecs, owns recursion over these child calls.
type ChildCall struct {
	Context CallContext
	Data    []byte
}

// DecodeResult is the output of one codec for one call frame.
type DecodeResult struct {
	Transfers  []Transfer
	ChildCalls []ChildCall
}

// CallContext mirrors a single EVM call frame. For parsers that only receive
// calldata, From/To can be left as zero addresses; decoded transfers will still
// include deterministic recipients and values when calldata explicitly contains
// them.
type CallContext struct {
	From  Address
	To    Address
	Value *big.Int // wei attached to this call frame
	Depth int      // 0 = root tx; recursive parsers bump this
}

// Call returns the context for a normal CALL from the current frame to target.
func (c CallContext) Call(target Address, value *big.Int) CallContext {
	return CallContext{
		From:  c.To,
		To:    target,
		Value: CopyBigInt(value),
		Depth: c.Depth + 1,
	}
}

// DelegateCall returns the context for a DELEGATECALL. The executing code
// changes, but address(this), msg.sender, and msg.value stay in the caller frame.
func (c CallContext) DelegateCall() CallContext {
	return CallContext{
		From:  c.From,
		To:    c.To,
		Value: CopyBigInt(c.Value),
		Depth: c.Depth + 1,
	}
}

// Child is kept for custom codecs that need to model non-standard execution.
func (c CallContext) Child(from Address, to Address, value *big.Int) CallContext {
	return CallContext{
		From:  from,
		To:    to,
		Value: CopyBigInt(value),
		Depth: c.Depth + 1,
	}
}

func CopyBigInt(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(v)
}
