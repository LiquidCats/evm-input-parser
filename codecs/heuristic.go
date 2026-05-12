package codec

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/LiquidCats/evm-input-parser/types"
)

const defaultMaxGuessedTransfers = 64

var (
	// withdraw(uint256). Common WETH-like contracts send native ETH back to
	// msg.sender for exactly the requested amount.
	nativeWithdraw = types.Selector{0x2e, 0x1a, 0x7d, 0x4d}

	ignoredTokenSelectors = map[types.Selector]struct{}{
		// ERC20/ERC721/ERC1155 movement and approval selectors. Token transfers
		// are intentionally excluded because reliable data comes from logs.
		{0xa9, 0x05, 0x9c, 0xbb}: {}, // transfer(address,uint256)
		{0x23, 0xb8, 0x72, 0xdd}: {}, // transferFrom(address,address,uint256)
		{0x09, 0x5e, 0xa7, 0xb3}: {}, // approve(address,uint256)
		{0x42, 0x84, 0x2e, 0x0e}: {}, // safeTransferFrom(address,address,uint256)
		{0xb8, 0x8d, 0x4f, 0xde}: {}, // safeTransferFrom(address,address,uint256,bytes)
		{0xf2, 0xeb, 0x54, 0x45}: {}, // safeTransferFrom(address,address,uint256,uint256,bytes)
		{0xa2, 0x2c, 0xb4, 0x65}: {}, // setApprovalForAll(address,bool)
	}
)

// HeuristicParser is the final fallback parser. It does not claim certainty:
// it emits Possible transfers when calldata contains canonical ABI
// address/value pairs that may represent native ETH movement.
type HeuristicParser struct {
	MaxCandidates int
}

func (p *HeuristicParser) CanParse(sel types.Selector) bool {
	_, ignored := ignoredTokenSelectors[sel]
	return !ignored
}

func (p *HeuristicParser) Parse(ctx types.CallContext, selector types.Selector, params types.InputParams) (*types.DecodeResult, error) {
	if selector == nativeWithdraw {
		return p.parseNativeWithdraw(ctx, params)
	}
	return &types.DecodeResult{Transfers: p.guessAddressValuePairs(ctx, params)}, nil
}

func (p *HeuristicParser) parseNativeWithdraw(ctx types.CallContext, params []byte) (*types.DecodeResult, error) {
	value, err := ReadUint256(params, 0)
	if err != nil {
		return nil, fmt.Errorf("withdraw(uint256): read value: %w", err)
	}
	if value.Sign() == 0 || ctx.From.IsZero() {
		return &types.DecodeResult{}, nil
	}
	return &types.DecodeResult{Transfers: []types.Transfer{{
		From:       ctx.To,
		To:         ctx.From,
		Value:      types.CopyBigInt(value),
		Confidence: types.Likely,
	}}}, nil
}

func (p *HeuristicParser) guessAddressValuePairs(ctx types.CallContext, params []byte) []types.Transfer {
	limit := p.MaxCandidates
	if limit <= 0 {
		limit = defaultMaxGuessedTransfers
	}

	var transfers []types.Transfer
	seen := make(map[string]struct{})
	words := len(params) / wordSize
	for wordIdx := 0; wordIdx < words && len(transfers) < limit; wordIdx++ {
		word := params[wordIdx*wordSize : (wordIdx+1)*wordSize]
		to, ok := canonicalABIAddress(word)
		if !ok {
			continue
		}

		for _, valueWordIdx := range []int{wordIdx + 1, wordIdx - 1} {
			if valueWordIdx < 0 || valueWordIdx >= words {
				continue
			}
			valueWord := params[valueWordIdx*wordSize : (valueWordIdx+1)*wordSize]
			value, ok := plausibleETHValue(params, valueWord)
			if !ok {
				continue
			}

			key := transferKey(ctx.To, to, value)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			transfers = append(transfers, types.Transfer{
				From:       ctx.To,
				To:         to,
				Value:      value,
				Confidence: types.Possible,
			})
			break
		}
	}
	return transfers
}

func canonicalABIAddress(word []byte) (types.Address, bool) {
	var addr types.Address
	if len(word) != wordSize || !bytes.Equal(word[:12], make([]byte, 12)) {
		return addr, false
	}
	copy(addr[:], word[12:])
	if addr.IsZero() || isPrecompileAddress(addr) {
		return types.Address{}, false
	}
	return addr, true
}

func plausibleETHValue(params []byte, word []byte) (*big.Int, bool) {
	if len(word) != wordSize {
		return nil, false
	}
	value := new(big.Int).SetBytes(word)
	if value.Sign() <= 0 || value.Cmp(maxPlausibleWeiValue()) > 0 {
		return nil, false
	}
	if value.Cmp(big.NewInt(1)) <= 0 {
		return nil, false
	}
	if looksLikeDynamicOffset(params, value) {
		return nil, false
	}
	return value, true
}

func looksLikeDynamicOffset(params []byte, value *big.Int) bool {
	if !value.IsInt64() {
		return false
	}
	offset := int(value.Int64())
	if offset%wordSize != 0 || offset < wordSize || offset+wordSize > len(params) {
		return false
	}
	length := new(big.Int).SetBytes(params[offset : offset+wordSize])
	return length.IsInt64() && length.Int64() >= 0 && offset+wordSize+int(length.Int64()) <= len(params)
}

func maxPlausibleWeiValue() *big.Int {
	// 1e12 ETH. Larger values are almost always IDs, bitmaps, or corrupt input.
	return new(big.Int).Mul(big.NewInt(1_000_000_000_000), big.NewInt(1_000_000_000_000_000_000))
}

func isPrecompileAddress(addr types.Address) bool {
	for i := 0; i < 19; i++ {
		if addr[i] != 0 {
			return false
		}
	}
	return addr[19] > 0 && addr[19] <= 10
}

func transferKey(from types.Address, to types.Address, value *big.Int) string {
	return from.Hex() + "|" + to.Hex() + "|" + value.String()
}
