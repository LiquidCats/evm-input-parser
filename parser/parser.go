package parser

import (
	"encoding/hex"
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

type Decoder interface {
	CanDecode(selector types.Selector) bool
	Decode(selector types.Selector, params types.InputParams) (*types.ParsedInputData, error)
}

type Parser struct {
	decoders []Decoder
	maxDepth int
}

func New(codecs ...Decoder) *Parser {
	return &Parser{
		decoders: codecs,
	}
}

// Parse decodes calldata without caller/callee context. Transfer From fields are
// zero when the source contract cannot be known from calldata alone.
func (p *Parser) Parse(data types.RawInputData) (*types.ParsedInputData, error) {
	hexData, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("invalid hex data: %w", err)
	}

	sel := types.Selector(hexData[:4])
	params := types.InputParams(hexData[4:])

	for _, codec := range p.decoders {
		if !codec.CanDecode(sel) {
			continue
		}

		result, err := codec.Decode(sel, params)
		if err != nil {
			return nil, fmt.Errorf("decoding failed: %w", err)
		}
		return result, nil
	}

	return nil, fmt.Errorf("no decoder found for selector: %s", sel)
}
