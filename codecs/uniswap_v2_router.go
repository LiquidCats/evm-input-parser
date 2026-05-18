package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// UniswapV2RouterDecoder covers the V2 router functions that pay ETH out to
// a caller-supplied recipient:
//
//	swapExactTokensForETH(uint amountIn, uint amountOutMin, address[] path, address to, uint deadline)
//	    0x18cbafe5
//	swapTokensForExactETH(uint amountOut, uint amountInMax, address[] path, address to, uint deadline)
//	    0x4a25d94a
//	swapExactTokensForETHSupportingFeeOnTransferTokens(uint amountIn, uint amountOutMin, address[] path, address to, uint deadline)
//	    0x791ac947
//
// In all three, the recipient is encoded directly in calldata (word index 3).
// `swapTokensForExactETH` also pins the exact ETH output (`amountOut`), while
// the other two only bound it from below via `amountOutMin`.
type UniswapV2RouterDecoder struct{}

func NewUniswapV2RouterDecoder() *UniswapV2RouterDecoder { return &UniswapV2RouterDecoder{} }

var (
	selV2SwapExactTokensForETH    = types.Selector{0x18, 0xcb, 0xaf, 0xe5}
	selV2SwapTokensForExactETH    = types.Selector{0x4a, 0x25, 0xd9, 0x4a}
	selV2SwapExactTokensForETHFOT = types.Selector{0x79, 0x1a, 0xc9, 0x47}
)

func (d *UniswapV2RouterDecoder) CanDecode(s types.Selector) bool {
	return s == selV2SwapExactTokensForETH ||
		s == selV2SwapTokensForExactETH ||
		s == selV2SwapExactTokensForETHFOT
}

func (d *UniswapV2RouterDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if !d.CanDecode(sel) {
		return nil, fmt.Errorf("uniswap_v2_router: unsupported selector %s", sel)
	}

	// All three signatures share layout for the first 5 head words:
	//   word 0: amount-in-like
	//   word 1: amount-out-like
	//   word 2: offset to path[]
	//   word 3: address `to`
	//   word 4: deadline
	recipient, err := ReadAddress(params, 3)
	if err != nil {
		return nil, fmt.Errorf("uniswap_v2_router: recipient: %w", err)
	}

	// `swapTokensForExactETH` pins the exact ETH output in word 0.
	// The other variants only know a minimum in word 1.
	if sel == selV2SwapTokensForExactETH {
		amountOut, err := ReadUint256(params, 0)
		if err != nil {
			return nil, fmt.Errorf("uniswap_v2_router: amountOut: %w", err)
		}
		return &types.ParsedInputData{
			Selector: sel,
			Transfers: []types.Transfer{{
				To:         recipient,
				Value:      amountOut,
				Confidence: types.Likely,
			}},
		}, nil
	}

	amountOutMin, err := ReadUint256(params, 1)
	if err != nil {
		return nil, fmt.Errorf("uniswap_v2_router: amountOutMin: %w", err)
	}
	return &types.ParsedInputData{
		Selector: sel,
		Transfers: []types.Transfer{{
			To:         recipient,
			Value:      amountOutMin,
			Confidence: types.Likely,
		}},
	}, nil
}
