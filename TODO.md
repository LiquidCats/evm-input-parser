1. Uncovered — calldata-derivable internal ETH transfers worth adding

| Selector | Function | Why it matters |
|---|---|---|
| `0x3593564c` | `execute(bytes commands, bytes[] inputs, uint256 deadline)` | Uniswap Universal Router. The `UNWRAP_WETH`, `SWEEP`, `PAY_PORTION`, `TRANSFER` commands all move ETH; needs its own opcode interpreter, not generic multicall recursion. |
| `0xdb006a75` | `redeem(uint256)` | Compound cETH → msg.sender (Likely) |
| `0x852a12e3` | `redeemUnderlying(uint256)` | Compound cETH → msg.sender (Likely) |
| `0x415565b0` | `transformERC20(address inputToken, address outputToken, uint256 inputAmount, uint256 minOutput, (uint32,bytes)[] transformations)` | 0x Exchange Proxy; if `outputToken == 0xEee…EeE` then `taker` (msg.sender) receives ETH |
| `0x12aa3caf` | `swap(address executor, (address srcToken, address dstToken, address payable srcReceiver, address payable dstReceiver, ...))` | 1inch AggregationRouter v5; `dstReceiver` + `dstToken == eeeE…` is the deterministic ETH leg |
| `0x0502b1c5` / `0x2e95b6c8` | `unoswap*` family | 1inch direct swaps to ETH |
| `0xfb0f3ee1` | `fulfillBasicOrder(BasicOrderParameters)` | Seaport: ETH payment to seller(s) + fees, very common on OpenSea |
| `0xed98a574` / `0xb3a34c4c` | `fulfillAvailableOrders` / `fulfillOrder` | Seaport multi-order forms |
| `0x33b3da80` | `claimWithdrawal(uint256)` / `claimWithdrawalsTo` | Lido WithdrawalQueue — ETH out to claimer/recipient |
| `0x205c2878` | `withdrawTo(address,uint256)` | Some WETH-like wrappers expose this explicitly — fully Deterministic |
| `0xe9e05c42` | `depositTransaction(address to, uint256 value, …)` | Optimism portal; deterministic L1→L2 ETH bridge |
| `0x439370b1` | `depositEth(uint256,uint256)` / Arbitrum inbox variants | Arbitrum bridge ETH entry |

## 2. ERC20 / NFT transfers — out of scope (logs are the source of truth)

| Selector | Function |
|---|---|
| `0xa9059cbb` | `transfer(address,uint256)` |
| `0x23b872dd` | `transferFrom(address,address,uint256)` |
| `0x4000aea0` | `transferAndCall(address,uint256,bytes)` (ERC677) |
| `0x42842e0e` | `safeTransferFrom(address,address,uint256)` (ERC721) |
| `0xb88d4fde` | `safeTransferFrom(address,address,uint256,bytes)` (ERC721) |
| `0xf242432a` | `safeTransferFrom(...)` (ERC1155 single) |
| `0x2eb2c2d6` | `safeBatchTransferFrom(...)` (ERC1155 batch) |
| `0x51ba317a` | `disperseToken(address,address[],uint256[])` |
| `0xc73a2d60` | `disperseTokenSimple(address,address[],uint256[])` |
| `0x38ed1739` | UniV2 `swapExactTokensForTokens` |
| `0x8803dbee` | UniV2 `swapTokensForExactTokens` |
| `0x04e45aaf` / `0x5023b4df` / `0xc04b8d59` / `0xf28c0498` | UniV3 `exactInputSingle`/`exactOutputSingle`/`exactInput`/`exactOutput` (only matter for ETH when paired with `unwrapWETH9` via multicall — already covered transitively) |
| `0xfb3bdb41` | UniV2 `swapETHForExactTokens` (ETH is in `msg.value`, not calldata-derivable) |
| `0x7ff36ab5` | UniV2 `swapExactETHForTokens` (same — `msg.value`) |

## 3. Should NOT be parsed — no calldata-derivable ETH transfer

These are either non-transfer (config/view/approval), or the ETH amount lives in `msg.value` and is invisible to calldata-only parsing. The heuristic fallback should be allowed to silently produce nothing for these — they're noise, not signal.

| Selector | Function | Reason |
|---|---|---|
| `0x095ea7b3` | `approve(address,uint256)` | not a transfer |
| `0xa22cb465` | `setApprovalForAll(address,bool)` | not a transfer |
| `0x39509351` / `0xa457c2d7` | `increaseAllowance` / `decreaseAllowance` | not a transfer |
| `0xd505accf` | `permit(...)` (EIP-2612) | signed approval |
| `0x8fcbaf0c` | `permit(...)` (DAI-style) | signed approval |
| `0x06fdde03` / `0x95d89b41` / `0x313ce567` | `name` / `symbol` / `decimals` | view |
| `0x70a08231` / `0xdd62ed3e` / `0x18160ddd` | `balanceOf` / `allowance` / `totalSupply` | view |
| `0x8da5cb5b` / `0xf2fde38b` / `0x715018a6` | `owner` / `transferOwnership` / `renounceOwnership` | admin |
| `0x8456cb59` / `0x3f4ba83a` | `pause` / `unpause` | admin |
| `0x8129fc1c` | `initialize()` (and `initialize(...)` variants) | proxy init |
| `0xd0e30db0` | WETH `deposit()` | ETH is in `msg.value`, not calldata |
| `0x1249c58b` | cETH `mint()` | same — ETH is `msg.value` |
| `0xa0712d68` | `mint(uint256)` | mints tokens, no ETH |
| `0x42966c68` | `burn(uint256)` | burns tokens, no ETH |
| `0x22895118` | Beacon `deposit(bytes,bytes,bytes,bytes32)` | ETH is `msg.value` (32 ETH per call) |

### One operational note