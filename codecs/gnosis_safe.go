package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// GnosisSafeDecoder decodes Gnosis Safe execTransaction calls:
//
//	execTransaction(
//	    address to, uint256 value, bytes data, uint8 operation,
//	    uint256 safeTxGas, uint256 baseGas, uint256 gasPrice,
//	    address gasToken, address refundReceiver, bytes signatures
//	)   0x6a761202
//
// When value > 0 the Safe forwards that amount of ETH to `to`. The transfer
// is deterministic: both recipient and amount are encoded in calldata.
type GnosisSafeDecoder struct{}

func NewGnosisSafeDecoder() *GnosisSafeDecoder { return &GnosisSafeDecoder{} }

var selSafeExecTransaction = types.Selector{0x6a, 0x76, 0x12, 0x02}

func (d *GnosisSafeDecoder) CanDecode(s types.Selector) bool {
	return s == selSafeExecTransaction
}

func (d *GnosisSafeDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if sel != selSafeExecTransaction {
		return nil, fmt.Errorf("gnosis_safe: unsupported selector %s", sel)
	}

	to, err := ReadAddress(params, 0)
	if err != nil {
		return nil, fmt.Errorf("gnosis_safe: to: %w", err)
	}
	value, err := ReadUint256(params, 1)
	if err != nil {
		return nil, fmt.Errorf("gnosis_safe: value: %w", err)
	}

	out := &types.ParsedInputData{Selector: sel}
	if value.Sign() > 0 {
		out.Transfers = append(out.Transfers, types.Transfer{
			To:         to,
			Value:      value,
			Confidence: types.Deterministic,
		})
	}
	return out, nil
}
