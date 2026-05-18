package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// MulticallDecoder handles the "wrapper" calls that carry an array of inner
// calldata payloads. The decoder owns no transfer logic itself — it walks
// the payloads and delegates each one back to the supplied SubParser, then
// aggregates whatever ETH transfers the inner decoders surface.
//
// Supported selectors (Uniswap V3 router / SwapRouter02 / Multicall3 legacy):
//
//	multicall(bytes[] data)                                  0xac9650d8
//	multicall(uint256 deadline, bytes[] data)                0x5ae401dc
//	multicall(bytes32 previousBlockHash, bytes[] data)       0x1f0464d1
//	aggregate3((address,bool,bytes)[] calls)                 0x82ad56cb
//	tryAggregate(bool, (address,bytes)[] calls)              0xbce38bd7
//	aggregate((address,bytes)[] calls)                       0x252dba42
//
// For the tuple-array selectors only the `bytes` field is recursed; the
// `target` address is not, because the inner ETH-moving call (if any) is
// what carries the transfer pattern, not the target.
type MulticallDecoder struct {
	sub SubParser
}

func NewMulticallDecoder(sub SubParser) *MulticallDecoder {
	return &MulticallDecoder{sub: sub}
}

// SetSubParser allows late binding when the parser is created after the
// decoder it depends on.
func (d *MulticallDecoder) SetSubParser(sub SubParser) { d.sub = sub }

var (
	selMulticallBytes        = types.Selector{0xac, 0x96, 0x50, 0xd8}
	selMulticallDeadline     = types.Selector{0x5a, 0xe4, 0x01, 0xdc}
	selMulticallPrevHash     = types.Selector{0x1f, 0x04, 0x64, 0xd1}
	selMC3Aggregate3         = types.Selector{0x82, 0xad, 0x56, 0xcb}
	selMC3TryAggregate       = types.Selector{0xbc, 0xe3, 0x8b, 0xd7}
	selMC3Aggregate          = types.Selector{0x25, 0x2d, 0xba, 0x42}
)

func (d *MulticallDecoder) CanDecode(s types.Selector) bool {
	switch s {
	case selMulticallBytes, selMulticallDeadline, selMulticallPrevHash,
		selMC3Aggregate3, selMC3TryAggregate, selMC3Aggregate:
		return true
	}
	return false
}

func (d *MulticallDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if d.sub == nil {
		return nil, fmt.Errorf("multicall: no sub-parser configured")
	}

	out := &types.ParsedInputData{Selector: sel}

	switch sel {
	case selMulticallBytes:
		// multicall(bytes[]) — array offset at word 0.
		arrOff, err := ReadOffset(params, 0)
		if err != nil {
			return nil, fmt.Errorf("multicall: array offset: %w", err)
		}
		return d.recurseBytesArray(out, params, arrOff)

	case selMulticallDeadline, selMulticallPrevHash:
		// multicall(<word>, bytes[]) — array offset at word 1.
		arrOff, err := ReadOffset(params, 1)
		if err != nil {
			return nil, fmt.Errorf("multicall: array offset: %w", err)
		}
		return d.recurseBytesArray(out, params, arrOff)

	case selMC3Aggregate3:
		return d.recurseTupleArray(out, params, 0, []int{2})
	case selMC3TryAggregate:
		// tryAggregate(bool, (address,bytes)[]) — array offset at word 1,
		// tuple shape (address, bytes) -> bytes at struct word 1.
		return d.recurseTupleArray(out, params, 1, []int{1})
	case selMC3Aggregate:
		return d.recurseTupleArray(out, params, 0, []int{1})
	}

	return nil, fmt.Errorf("multicall: unsupported selector %s", sel)
}

// recurseBytesArray walks a bytes[] starting at arrOff and re-parses every
// element through the sub-parser.
func (d *MulticallDecoder) recurseBytesArray(out *types.ParsedInputData, params []byte, arrOff int) (*types.ParsedInputData, error) {
	elems, err := ReadBytesArrayElements(params, arrOff)
	if err != nil {
		return nil, fmt.Errorf("multicall: read elements: %w", err)
	}
	for _, raw := range elems {
		d.appendSub(out, raw)
	}
	return out, nil
}

// recurseTupleArray walks a (struct[]) where each struct contains a `bytes`
// field at one of `bytesWordIdxInStruct`. structHeadWordCount is the index
// of the array offset within the top-level head section.
func (d *MulticallDecoder) recurseTupleArray(out *types.ParsedInputData, params []byte, arrayHeadWord int, bytesWordIdxInStruct []int) (*types.ParsedInputData, error) {
	arrOff, err := ReadOffset(params, arrayHeadWord)
	if err != nil {
		return nil, fmt.Errorf("multicall: tuple array offset: %w", err)
	}
	count, err := ReadArrayLength(params, arrOff)
	if err != nil {
		return nil, fmt.Errorf("multicall: tuple array length: %w", err)
	}
	// Each struct has at least one dynamic field, so the array is encoded as
	// a list of offsets pointing to each struct.
	headBase := arrOff + wordSize
	for i := range count {
		relOff, err := ReadOffsetAt(params, headBase+i*wordSize)
		if err != nil {
			return nil, fmt.Errorf("multicall: tuple[%d] offset: %w", i, err)
		}
		structBase := headBase + relOff
		// Read the bytes field from inside the struct. Each tuple member
		// is a fixed-size head word; we treat structBase as the struct's
		// head start and find the bytes offset there.
		for _, wIdx := range bytesWordIdxInStruct {
			bytesRelOff, err := ReadOffsetAt(params, structBase+wIdx*wordSize)
			if err != nil {
				return nil, fmt.Errorf("multicall: tuple[%d] bytes offset: %w", i, err)
			}
			data, err := ReadDynamicBytesAt(params, structBase+bytesRelOff)
			if err != nil {
				return nil, fmt.Errorf("multicall: tuple[%d] bytes: %w", i, err)
			}
			d.appendSub(out, data)
		}
	}
	return out, nil
}

// appendSub re-parses inner calldata via the sub-parser and merges any
// transfers it produced. Errors and empty selectors are tolerated so a
// single malformed entry doesn't abort the whole batch.
func (d *MulticallDecoder) appendSub(out *types.ParsedInputData, raw []byte) {
	if len(raw) < 4 {
		return
	}
	inner, err := d.sub.ParseBytes(raw)
	if err != nil || inner == nil {
		return
	}
	out.Transfers = append(out.Transfers, inner.Transfers...)
}
