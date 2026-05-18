package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// UniswapV3PaymentsDecoder decodes the periphery payment helpers exposed by
// Uniswap V3's SwapRouter / SwapRouter02:
//
//	unwrapWETH9(uint256 amountMinimum, address recipient)
//	    0x49404b7c
//	unwrapWETH9WithFee(uint256 amountMinimum, address recipient, uint256 feeBips, address feeRecipient)
//	    0x9b2c0a37
//	refundETH()
//	    0x12210e8a
//
// `unwrapWETH9` releases the router's WETH balance as ETH to `recipient`; the
// actual amount is the router's runtime WETH balance, so we record the
// caller-supplied minimum with Likely confidence. `unwrapWETH9WithFee` adds a
// fee split — both the recipient and the fee recipient receive ETH.
//
// `refundETH()` sweeps the router's remaining ETH to msg.sender, which is not
// available from calldata; we still surface it as a zero-value Possible
// hint so consumers see that the call produces a refund.
type UniswapV3PaymentsDecoder struct{}

func NewUniswapV3PaymentsDecoder() *UniswapV3PaymentsDecoder { return &UniswapV3PaymentsDecoder{} }

var (
	selUnwrapWETH9        = types.Selector{0x49, 0x40, 0x4b, 0x7c}
	selUnwrapWETH9WithFee = types.Selector{0x9b, 0x2c, 0x0a, 0x37}
	selRefundETH          = types.Selector{0x12, 0x21, 0x0e, 0x8a}
)

func (d *UniswapV3PaymentsDecoder) CanDecode(s types.Selector) bool {
	return s == selUnwrapWETH9 || s == selUnwrapWETH9WithFee || s == selRefundETH
}

func (d *UniswapV3PaymentsDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	switch sel {
	case selUnwrapWETH9:
		amountMin, err := ReadUint256(params, 0)
		if err != nil {
			return nil, fmt.Errorf("uniswap_v3.unwrapWETH9 amountMin: %w", err)
		}
		recipient, err := ReadAddress(params, 1)
		if err != nil {
			return nil, fmt.Errorf("uniswap_v3.unwrapWETH9 recipient: %w", err)
		}
		return &types.ParsedInputData{
			Selector: sel,
			Transfers: []types.Transfer{{
				To:         recipient,
				Value:      amountMin,
				Confidence: types.Likely,
			}},
		}, nil

	case selUnwrapWETH9WithFee:
		amountMin, err := ReadUint256(params, 0)
		if err != nil {
			return nil, fmt.Errorf("uniswap_v3.unwrapWETH9WithFee amountMin: %w", err)
		}
		recipient, err := ReadAddress(params, 1)
		if err != nil {
			return nil, fmt.Errorf("uniswap_v3.unwrapWETH9WithFee recipient: %w", err)
		}
		feeRecipient, err := ReadAddress(params, 3)
		if err != nil {
			return nil, fmt.Errorf("uniswap_v3.unwrapWETH9WithFee feeRecipient: %w", err)
		}
		return &types.ParsedInputData{
			Selector: sel,
			Transfers: []types.Transfer{
				{To: recipient, Value: amountMin, Confidence: types.Likely},
				{To: feeRecipient, Confidence: types.Possible},
			},
		}, nil

	case selRefundETH:
		return &types.ParsedInputData{
			Selector: sel,
			Transfers: []types.Transfer{{Confidence: types.Possible}},
		}, nil
	}

	return nil, fmt.Errorf("uniswap_v3_payments: unsupported selector %s", sel)
}
