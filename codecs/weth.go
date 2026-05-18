package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// WETHDecoder decodes WETH9 calldata patterns that move ETH out of the
// wrapper contract.
//
//	withdraw(uint256)   0x2e1a7d4d   WETH9 -> msg.sender
//
// The recipient (msg.sender) and the source (the WETH9 contract itself)
// cannot be derived from calldata alone, so the resulting Transfer carries
// the amount with Likely confidence and zero From/To.
type WETHDecoder struct{}

func NewWETHDecoder() *WETHDecoder { return &WETHDecoder{} }

var selWETHWithdraw = types.Selector{0x2e, 0x1a, 0x7d, 0x4d}

func (d *WETHDecoder) CanDecode(s types.Selector) bool {
	return s == selWETHWithdraw
}

func (d *WETHDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if sel != selWETHWithdraw {
		return nil, fmt.Errorf("weth: unsupported selector %s", sel)
	}
	amount, err := ReadUint256(params, 0)
	if err != nil {
		return nil, fmt.Errorf("weth.withdraw amount: %w", err)
	}
	return &types.ParsedInputData{
		Selector: sel,
		Transfers: []types.Transfer{{
			Value:      amount,
			Confidence: types.Likely,
		}},
	}, nil
}
