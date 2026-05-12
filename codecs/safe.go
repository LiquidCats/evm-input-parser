package codec

import (
	"fmt"
	"math/big"

	"github.com/LiquidCats/evm-input-parser/types"
)

const (
	safeOperationCall         = 0
	safeOperationDelegateCall = 1
)

type SafeParser struct{}

var (
	// execTransaction(address,uint256,bytes,uint8,uint256,uint256,uint256,address,address,bytes)
	safeExecTransaction = types.Selector{0x6a, 0x76, 0x12, 0x02}
	// multiSend(bytes)
	safeMultiSend = types.Selector{0x8d, 0x80, 0xff, 0x0a}
)

func (p *SafeParser) CanParse(sel types.Selector) bool {
	switch sel {
	case safeExecTransaction, safeMultiSend:
		return true
	}
	return false
}

func (p *SafeParser) Parse(ctx types.CallContext, selector types.Selector, params types.InputParams) (*types.DecodeResult, error) {
	switch selector {
	case safeExecTransaction:
		return p.parseExecTransaction(ctx, params)
	case safeMultiSend:
		return p.parseMultiSend(ctx, params)
	}
	return nil, nil
}

func (p *SafeParser) parseExecTransaction(ctx types.CallContext, params []byte) (*types.DecodeResult, error) {
	target, err := ReadAddress(params, 0)
	if err != nil {
		return nil, fmt.Errorf("safe.execTransaction: read target: %w", err)
	}
	value, err := ReadUint256(params, 1)
	if err != nil {
		return nil, fmt.Errorf("safe.execTransaction: read value: %w", err)
	}
	operation, err := ReadUint256(params, 3)
	if err != nil {
		return nil, fmt.Errorf("safe.execTransaction: read operation: %w", err)
	}
	if !operation.IsInt64() {
		return nil, fmt.Errorf("safe.execTransaction: operation too large: %s", operation)
	}
	callData, err := ReadDynamicBytes(params, 2)
	if err != nil {
		return nil, fmt.Errorf("safe.execTransaction: read data: %w", err)
	}

	result := &types.DecodeResult{}
	switch operation.Int64() {
	case safeOperationCall:
		if value.Sign() > 0 {
			result.Transfers = append(result.Transfers, types.Transfer{
				From:       ctx.To,
				To:         target,
				Value:      types.CopyBigInt(value),
				Confidence: types.Deterministic,
			})
		}
		if len(callData) >= 4 {
			result.ChildCalls = append(result.ChildCalls, types.ChildCall{
				Context: ctx.Call(target, value),
				Data:    callData,
			})
		}
	case safeOperationDelegateCall:
		// Delegatecall executes library code in the Safe context. This is how Safe
		// commonly executes MultiSend, so keep ctx.To as the ETH source.
		if len(callData) >= 4 {
			result.ChildCalls = append(result.ChildCalls, types.ChildCall{
				Context: ctx.DelegateCall(),
				Data:    callData,
			})
		}
	}

	return result, nil
}

func (p *SafeParser) parseMultiSend(ctx types.CallContext, params []byte) (*types.DecodeResult, error) {
	transactions, err := ReadDynamicBytes(params, 0)
	if err != nil {
		return nil, fmt.Errorf("safe.multiSend: read transactions: %w", err)
	}

	result := &types.DecodeResult{}
	for offset, index := 0, 0; offset < len(transactions); index++ {
		if len(transactions)-offset < 85 {
			return nil, fmt.Errorf("safe.multiSend tx %d: truncated header", index)
		}

		operation := transactions[offset]
		offset++

		var target types.Address
		copy(target[:], transactions[offset:offset+20])
		offset += 20

		value := new(big.Int).SetBytes(transactions[offset : offset+32])
		offset += 32

		dataLength := new(big.Int).SetBytes(transactions[offset : offset+32])
		offset += 32
		if !dataLength.IsInt64() {
			return nil, fmt.Errorf("safe.multiSend tx %d: data length too large: %s", index, dataLength)
		}
		dataLen := int(dataLength.Int64())
		if dataLen < 0 || offset+dataLen > len(transactions) {
			return nil, fmt.Errorf("safe.multiSend tx %d: data out of bounds", index)
		}
		callData := transactions[offset : offset+dataLen]
		offset += dataLen

		switch operation {
		case safeOperationCall:
			if value.Sign() > 0 {
				result.Transfers = append(result.Transfers, types.Transfer{
					From:       ctx.To,
					To:         target,
					Value:      types.CopyBigInt(value),
					Confidence: types.Deterministic,
				})
			}
			if len(callData) >= 4 {
				result.ChildCalls = append(result.ChildCalls, types.ChildCall{
					Context: ctx.Call(target, value),
					Data:    callData,
				})
			}
		case safeOperationDelegateCall:
			if len(callData) >= 4 {
				result.ChildCalls = append(result.ChildCalls, types.ChildCall{
					Context: ctx.DelegateCall(),
					Data:    callData,
				})
			}
		default:
			return nil, fmt.Errorf("safe.multiSend tx %d: unknown operation %d", index, operation)
		}
	}

	return result, nil
}
