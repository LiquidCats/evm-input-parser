package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// Multicall3ValueDecoder handles Multicall3.aggregate3Value, which is the
// only Multicall3 entrypoint that carries an explicit per-call ETH value:
//
//	aggregate3Value(
//	    (address target, bool allowFailure, uint256 value, bytes callData)[] calls
//	)   0x174dea71
//
// For each `calls[i]` whose value > 0 we emit a deterministic transfer from
// the Multicall3 contract to `target`. The inner callData is also re-parsed
// through the SubParser so nested ETH-moving patterns (e.g. an inner
// unwrapWETH9) surface as additional transfers.
type Multicall3ValueDecoder struct {
	sub SubParser
}

func NewMulticall3ValueDecoder(sub SubParser) *Multicall3ValueDecoder {
	return &Multicall3ValueDecoder{sub: sub}
}

func (d *Multicall3ValueDecoder) SetSubParser(sub SubParser) { d.sub = sub }

var selMC3Aggregate3Value = types.Selector{0x17, 0x4d, 0xea, 0x71}

func (d *Multicall3ValueDecoder) CanDecode(s types.Selector) bool {
	return s == selMC3Aggregate3Value
}

func (d *Multicall3ValueDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if sel != selMC3Aggregate3Value {
		return nil, fmt.Errorf("multicall3: unsupported selector %s", sel)
	}

	arrOff, err := ReadOffset(params, 0)
	if err != nil {
		return nil, fmt.Errorf("multicall3: array offset: %w", err)
	}
	count, err := ReadArrayLength(params, arrOff)
	if err != nil {
		return nil, fmt.Errorf("multicall3: array length: %w", err)
	}

	out := &types.ParsedInputData{Selector: sel}
	headBase := arrOff + wordSize
	for i := range count {
		relOff, err := ReadOffsetAt(params, headBase+i*wordSize)
		if err != nil {
			return nil, fmt.Errorf("multicall3: tuple[%d] offset: %w", i, err)
		}
		structBase := headBase + relOff

		target, err := ReadAddressAt(params, structBase+0*wordSize)
		if err != nil {
			return nil, fmt.Errorf("multicall3: tuple[%d] target: %w", i, err)
		}
		// word 1 is bool allowFailure — skip
		value, err := ReadUint256At(params, structBase+2*wordSize)
		if err != nil {
			return nil, fmt.Errorf("multicall3: tuple[%d] value: %w", i, err)
		}
		bytesRelOff, err := ReadOffsetAt(params, structBase+3*wordSize)
		if err != nil {
			return nil, fmt.Errorf("multicall3: tuple[%d] bytes offset: %w", i, err)
		}
		callData, err := ReadDynamicBytesAt(params, structBase+bytesRelOff)
		if err != nil {
			return nil, fmt.Errorf("multicall3: tuple[%d] callData: %w", i, err)
		}

		if value.Sign() > 0 {
			out.Transfers = append(out.Transfers, types.Transfer{
				To:         target,
				Value:      value,
				Confidence: types.Deterministic,
			})
		}

		if d.sub != nil && len(callData) >= 4 {
			inner, err := d.sub.ParseBytes(callData)
			if err == nil && inner != nil {
				out.Transfers = append(out.Transfers, inner.Transfers...)
			}
		}
	}

	return out, nil
}
