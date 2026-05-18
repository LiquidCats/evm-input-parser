package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// DisperseDecoder decodes the Disperse.app helper contract:
//
//	disperseEther(address[] recipients, uint256[] values)   0xe63d38ed
//
// Every (recipient, value) pair is a deterministic ETH transfer originating
// from the Disperse contract.
type DisperseDecoder struct{}

func NewDisperseDecoder() *DisperseDecoder { return &DisperseDecoder{} }

var selDisperseEther = types.Selector{0xe6, 0x3d, 0x38, 0xed}

func (d *DisperseDecoder) CanDecode(s types.Selector) bool {
	return s == selDisperseEther
}

func (d *DisperseDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if sel != selDisperseEther {
		return nil, fmt.Errorf("disperse: unsupported selector %s", sel)
	}

	recipientsOff, err := ReadOffset(params, 0)
	if err != nil {
		return nil, fmt.Errorf("disperse: recipients offset: %w", err)
	}
	valuesOff, err := ReadOffset(params, 1)
	if err != nil {
		return nil, fmt.Errorf("disperse: values offset: %w", err)
	}

	recipientsLen, err := ReadArrayLength(params, recipientsOff)
	if err != nil {
		return nil, fmt.Errorf("disperse: recipients length: %w", err)
	}
	valuesLen, err := ReadArrayLength(params, valuesOff)
	if err != nil {
		return nil, fmt.Errorf("disperse: values length: %w", err)
	}
	if recipientsLen != valuesLen {
		return nil, fmt.Errorf("disperse: array length mismatch: %d vs %d", recipientsLen, valuesLen)
	}

	transfers := make([]types.Transfer, 0, recipientsLen)
	for i := range recipientsLen {
		addr, err := ReadAddressAt(params, recipientsOff+wordSize+i*wordSize)
		if err != nil {
			return nil, fmt.Errorf("disperse: recipient[%d]: %w", i, err)
		}
		val, err := ReadUint256At(params, valuesOff+wordSize+i*wordSize)
		if err != nil {
			return nil, fmt.Errorf("disperse: value[%d]: %w", i, err)
		}
		transfers = append(transfers, types.Transfer{
			To:         addr,
			Value:      val,
			Confidence: types.Deterministic,
		})
	}

	return &types.ParsedInputData{Selector: sel, Transfers: transfers}, nil
}
