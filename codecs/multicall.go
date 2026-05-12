package codec

import (
	"fmt"
	"math/big"

	"github.com/LiquidCats/evm-input-parser/types"
)

type Multicall3Parser struct{}

var (
	// aggregate((address,bytes)[])
	multicall3Aggregate = types.Selector{0x25, 0x2d, 0xba, 0x42}
	// tryAggregate(bool,(address,bytes)[])
	multicall3TryAggregate = types.Selector{0xbc, 0xe3, 0x8b, 0xd7}
	// aggregate3((address,bool,bytes)[])
	multicall3Aggregate3 = types.Selector{0x82, 0xad, 0x56, 0xcb}
	// aggregate3Value((address,bool,uint256,bytes)[])
	multicall3Aggregate3Value = types.Selector{0x17, 0x4d, 0xea, 0x71}
	// multicall(bytes[])
	multicallBytesArray = types.Selector{0xac, 0x96, 0x50, 0xd8}
	// multicall(uint256,bytes[])
	multicallDeadlineBytesArray = types.Selector{0x5a, 0xe4, 0x01, 0xdc}
)

func (p *Multicall3Parser) CanParse(sel types.Selector) bool {
	switch sel {
	case multicall3Aggregate, multicall3TryAggregate,
		multicall3Aggregate3, multicall3Aggregate3Value,
		multicallBytesArray, multicallDeadlineBytesArray:
		return true
	}
	return false
}

func (p *Multicall3Parser) Parse(ctx types.CallContext, selector types.Selector, params types.InputParams) (*types.DecodeResult, error) {
	switch selector {
	case multicall3Aggregate:
		return p.parseAggregate(ctx, params)
	case multicall3TryAggregate:
		return p.parseTryAggregate(ctx, params)
	case multicall3Aggregate3:
		return p.parseAggregate3(ctx, params)
	case multicall3Aggregate3Value:
		return p.parseAggregate3Value(ctx, params)
	case multicallBytesArray:
		return p.parseSameTargetMulticall(ctx, params, 0)
	case multicallDeadlineBytesArray:
		return p.parseSameTargetMulticall(ctx, params, 1)
	}
	return nil, nil
}

func (p *Multicall3Parser) parseAggregate(ctx types.CallContext, params []byte) (*types.DecodeResult, error) {
	return p.parseDynamicCallArray(ctx, params, 0, 1, -1)
}

func (p *Multicall3Parser) parseTryAggregate(ctx types.CallContext, params []byte) (*types.DecodeResult, error) {
	return p.parseDynamicCallArray(ctx, params, 1, 1, -1)
}

func (p *Multicall3Parser) parseAggregate3(ctx types.CallContext, params []byte) (*types.DecodeResult, error) {
	return p.parseDynamicCallArray(ctx, params, 0, 2, -1)
}

func (p *Multicall3Parser) parseAggregate3Value(ctx types.CallContext, params []byte) (*types.DecodeResult, error) {
	return p.parseDynamicCallArray(ctx, params, 0, 3, 2)
}

func (p *Multicall3Parser) parseSameTargetMulticall(ctx types.CallContext, params []byte, arrayOffsetWord int) (*types.DecodeResult, error) {
	arrayOffset, err := ReadOffset(params, arrayOffsetWord)
	if err != nil {
		return nil, fmt.Errorf("multicall(bytes[]): %w", err)
	}
	calls, err := ReadBytesArrayElements(params, arrayOffset)
	if err != nil {
		return nil, fmt.Errorf("multicall(bytes[]): %w", err)
	}

	result := &types.DecodeResult{}
	for _, callData := range calls {
		if len(callData) < 4 {
			continue
		}
		result.ChildCalls = append(result.ChildCalls, types.ChildCall{
			Context: ctx.Call(ctx.To, big.NewInt(0)),
			Data:    callData,
		})
	}
	return result, nil
}

// parseDynamicCallArray parses Multicall3 arrays whose elements are dynamic
// tuples starting with target address and ending with calldata bytes. dataWord
// and valueWord are tuple word indexes; valueWord < 0 means no explicit ETH.
func (p *Multicall3Parser) parseDynamicCallArray(ctx types.CallContext, params []byte, arrayOffsetWord int, dataWord int, valueWord int) (*types.DecodeResult, error) {
	arrayOffset, err := ReadOffset(params, arrayOffsetWord)
	if err != nil {
		return nil, fmt.Errorf("multicall3: read calls offset: %w", err)
	}
	count, err := ReadArrayLength(params, arrayOffset)
	if err != nil {
		return nil, fmt.Errorf("multicall3: read calls length: %w", err)
	}

	result := &types.DecodeResult{}
	tupleHeadBase := arrayOffset + wordSize

	for i := range count {
		ptrOffset := tupleHeadBase + i*wordSize
		relOffset, err := ReadOffsetAt(params, ptrOffset)
		if err != nil {
			return nil, fmt.Errorf("multicall3 call %d: read tuple offset: %w", i, err)
		}
		tupleOffset := tupleHeadBase + relOffset

		target, err := ReadAddressAt(params, tupleOffset)
		if err != nil {
			return nil, fmt.Errorf("multicall3 call %d: read target: %w", i, err)
		}

		value := big.NewInt(0)
		if valueWord >= 0 {
			value, err = ReadUint256At(params, tupleOffset+valueWord*wordSize)
			if err != nil {
				return nil, fmt.Errorf("multicall3 call %d: read value: %w", i, err)
			}
			if value.Sign() > 0 {
				result.Transfers = append(result.Transfers, types.Transfer{
					From:       ctx.To,
					To:         target,
					Value:      types.CopyBigInt(value),
					Confidence: types.Deterministic,
				})
			}
		}

		dataRelOffset, err := ReadOffsetAt(params, tupleOffset+dataWord*wordSize)
		if err != nil {
			return nil, fmt.Errorf("multicall3 call %d: read calldata offset: %w", i, err)
		}
		callData, err := ReadDynamicBytesAt(params, tupleOffset+dataRelOffset)
		if err != nil {
			return nil, fmt.Errorf("multicall3 call %d: read calldata: %w", i, err)
		}
		if len(callData) < 4 {
			continue
		}

		result.ChildCalls = append(result.ChildCalls, types.ChildCall{
			Context: ctx.Call(target, value),
			Data:    callData,
		})
	}

	return result, nil
}
