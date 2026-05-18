package codec

import (
	"math/big"

	"github.com/LiquidCats/evm-input-parser/types"
)

// HeuristicDecoder is the fallback of last resort. It accepts every selector
// (CanDecode always returns true) and scans the calldata word-by-word looking
// for adjacent (address, uint256) pairs that look plausible enough to be an
// ETH transfer.
//
// The output is intentionally noisy — every emitted Transfer carries
// Possible confidence and callers are expected to filter further (for
// instance by cross-referencing with transaction receipts / logs).
//
// Register this decoder LAST in the parser chain so it only fires for
// selectors that no specific decoder recognised.
type HeuristicDecoder struct {
	// maxTransfers caps the number of guesses returned per call. Real
	// internal-transfer-bearing calldata rarely contains more than a few
	// dozen plausible pairs; bounding the output keeps the result usable.
	maxTransfers int
	// minValue rejects values smaller than this (in wei). Defaults to 0
	// — callers can tighten if they want to drop micro-amounts.
	minValue *big.Int
	// maxValue rejects values larger than this. Defaults to 2^240, which
	// keeps the heuristic from latching onto words that are themselves
	// addresses or hashes.
	maxValue *big.Int
}

func NewHeuristicDecoder() *HeuristicDecoder {
	return &HeuristicDecoder{
		maxTransfers: 64,
		minValue:     new(big.Int),
		maxValue:     new(big.Int).Lsh(big.NewInt(1), 240),
	}
}

// CanDecode always returns true — this decoder is the universal fallback.
func (d *HeuristicDecoder) CanDecode(_ types.Selector) bool { return true }

func (d *HeuristicDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	out := &types.ParsedInputData{Selector: sel}
	if len(params) < 2*wordSize {
		return out, nil
	}

	type key struct {
		a types.Address
		v string
	}
	seen := make(map[key]struct{})

	for i := 0; i+2*wordSize <= len(params); i += wordSize {
		addr, ok := plausibleAddressWord(params[i : i+wordSize])
		if !ok {
			continue
		}
		val := new(big.Int).SetBytes(params[i+wordSize : i+2*wordSize])
		if !d.plausibleValue(val) {
			continue
		}
		k := key{a: addr, v: val.String()}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}

		out.Transfers = append(out.Transfers, types.Transfer{
			To:         addr,
			Value:      val,
			Confidence: types.Possible,
		})
		if len(out.Transfers) >= d.maxTransfers {
			break
		}
	}

	return out, nil
}

// plausibleAddressWord checks whether a 32-byte word looks like an
// ABI-encoded address: top 12 bytes must be zero, and the address itself
// (bottom 20 bytes) must have at least one non-zero byte in its top 4 bytes.
// The second check rejects small integers (lengths, offsets, deadlines)
// masquerading as addresses.
func plausibleAddressWord(word []byte) (types.Address, bool) {
	if len(word) != wordSize {
		return types.Address{}, false
	}
	for _, b := range word[:12] {
		if b != 0 {
			return types.Address{}, false
		}
	}
	// Require a non-zero byte in the top 4 bytes of the 20-byte address.
	if word[12] == 0 && word[13] == 0 && word[14] == 0 && word[15] == 0 {
		return types.Address{}, false
	}
	var a types.Address
	copy(a[:], word[12:])
	return a, true
}

func (d *HeuristicDecoder) plausibleValue(v *big.Int) bool {
	if v.Sign() <= 0 {
		return false
	}
	if v.Cmp(d.minValue) < 0 {
		return false
	}
	if v.Cmp(d.maxValue) > 0 {
		return false
	}
	return true
}
