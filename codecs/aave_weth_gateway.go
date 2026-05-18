package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// AaveWETHGatewayDecoder decodes Aave's WETHGateway helper, which lets users
// withdraw underlying ETH (rather than aWETH) from the lending pool:
//
//	withdrawETH(address lendingPool, uint256 amount, address to)   0x80500d20
//
// The gateway pulls aWETH from msg.sender, redeems WETH from the pool, and
// transfers raw ETH to `to`. Both the recipient and the amount are encoded
// in calldata, so this is a deterministic transfer.
type AaveWETHGatewayDecoder struct{}

func NewAaveWETHGatewayDecoder() *AaveWETHGatewayDecoder { return &AaveWETHGatewayDecoder{} }

var selAaveWithdrawETH = types.Selector{0x80, 0x50, 0x0d, 0x20}

func (d *AaveWETHGatewayDecoder) CanDecode(s types.Selector) bool {
	return s == selAaveWithdrawETH
}

func (d *AaveWETHGatewayDecoder) Decode(sel types.Selector, params types.InputParams) (*types.ParsedInputData, error) {
	if sel != selAaveWithdrawETH {
		return nil, fmt.Errorf("aave_weth_gateway: unsupported selector %s", sel)
	}

	// word 0 is the lendingPool address — irrelevant for the ETH transfer.
	amount, err := ReadUint256(params, 1)
	if err != nil {
		return nil, fmt.Errorf("aave_weth_gateway: amount: %w", err)
	}
	to, err := ReadAddress(params, 2)
	if err != nil {
		return nil, fmt.Errorf("aave_weth_gateway: to: %w", err)
	}

	return &types.ParsedInputData{
		Selector: sel,
		Transfers: []types.Transfer{{
			To:         to,
			Value:      amount,
			Confidence: types.Deterministic,
		}},
	}, nil
}
