package codec

import "github.com/LiquidCats/evm-input-parser/types"

// SubParser is the recursive entry point used by wrapper decoders (multicall,
// multicall3) to decode inner calldata payloads. It operates on raw bytes —
// the selector is the first 4 bytes — so wrappers can hand their already
// decoded sub-payloads directly to it without a hex round-trip.
//
// *Chain in this package satisfies SubParser. Callers wiring this up with
// the outer *parser.Parser can wrap it with NewParserAdapter.
type SubParser interface {
	ParseBytes(data []byte) (*types.ParsedInputData, error)
}
