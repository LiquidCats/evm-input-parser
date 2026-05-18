package codec

import (
	"fmt"

	"github.com/LiquidCats/evm-input-parser/types"
)

// Decoder is the contract every codec in this package satisfies. It is
// re-declared here (in addition to the identical interface owned by the
// parser package) so the codecs package is self-contained and can build
// its own decoder pipelines for recursion.
type Decoder interface {
	CanDecode(types.Selector) bool
	Decode(types.Selector, types.InputParams) (*types.ParsedInputData, error)
}

// Chain is the in-package realisation of SubParser: a flat list of
// decoders tried in registration order, returning the first match. It
// mirrors what *parser.Parser does, so wrapper decoders (multicall,
// multicall3) can recurse using either a Chain assembled here or the
// outer *parser.Parser — both satisfy SubParser.
type Chain struct {
	decoders []Decoder
}

func NewChain(decoders ...Decoder) *Chain {
	return &Chain{decoders: decoders}
}

// Add appends a decoder to the end of the chain. Useful when a wrapper
// decoder (e.g. MulticallDecoder) needs to be constructed before the
// chain that contains it.
func (c *Chain) Add(d Decoder) { c.decoders = append(c.decoders, d) }

// ParseBytes dispatches raw calldata (selector + params) to the first
// decoder whose CanDecode reports a match.
func (c *Chain) ParseBytes(raw []byte) (*types.ParsedInputData, error) {
	if len(raw) < 4 {
		return nil, fmt.Errorf("chain: calldata shorter than selector (%d bytes)", len(raw))
	}
	sel := types.SelectorFromBytes(raw[:4])
	params := types.InputParams(raw[4:])

	for _, dec := range c.decoders {
		if !dec.CanDecode(sel) {
			continue
		}
		return dec.Decode(sel, params)
	}
	return nil, fmt.Errorf("chain: no decoder for selector %s", sel)
}
