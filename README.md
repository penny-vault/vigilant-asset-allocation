# Vigilant Asset Allocation

The **Vigilant Asset Allocation (VAA)** strategy was developed by [Wouter Keller](https://papers.ssrn.com/sol3/cf_dev/AbsByAuth.cfm?per_id=1935527) and Jan Willem Keuning. It is based on their 2017 paper [Breadth Momentum and Vigilant Asset Allocation (VAA): Winning More by Losing Less](https://papers.ssrn.com/sol3/papers.cfm?abstract_id=3002624). VAA scores assets with a recency-weighted 13612W momentum signal and uses generalized breadth momentum to blend between an offensive sleeve (risky assets) and a defensive sleeve (bonds) depending on how many offensive assets have negative momentum.

The strategy ships in two named variants: **VAA-G4** (aggressive, holds one asset) and **VAA-G12** (balanced, holds up to five assets and partial defensive).

## Rules

**VAA-G4 (Aggressive, default):**
- **Offensive**: SPY, EFA, EEM, AGG
- **Defensive**: LQD, IEF, SHY
- TopO = 1, BreadthThreshold (B) = 1

**VAA-G12 (Balanced):**
- **Offensive**: SPY, IWM, QQQ, VGK, EWJ, EEM, VNQ, GSG, GLD, HYG, LQD, TLT
- **Defensive**: LQD, IEF, SHY
- TopO = 5, BreadthThreshold (B) = 4

1. On the last trading day of the month, compute the 13612W momentum for every asset in the offensive and defensive universes:
   - `13612W = 12 * (p0/p1 - 1) + 4 * (p0/p3 - 1) + 2 * (p0/p6 - 1) + (p0/p12 - 1)`
   - This is a recency-weighted blend of 1, 3, 6, and 12-month returns.
2. Count the number of "bad" offensive assets (score ≤ 0) as `n_bad`.
3. Compute the Cash-Bond Fraction (breadth momentum):
   - `CBF = min(1, n_bad / B)`
4. **Offensive sleeve** receives weight `1 - CBF`, allocated equal-weight to the top `TopO` offensive assets ranked by 13612W.
5. **Defensive sleeve** receives weight `CBF`, allocated entirely to the single best defensive asset by 13612W.
6. Hold all positions until the close of the following month, then re-rank and rebalance.

For VAA-G4, B=1 means any single bad offensive asset triggers full defensive. For VAA-G12, B=4 means the defensive allocation scales linearly with the number of bad offensive assets (1 bad = 25% defensive, 2 bad = 50%, 3 bad = 75%, 4 or more = 100%).

## Parameters

- **OffensiveUniverse** -- comma-separated tickers for the offensive sleeve.
- **DefensiveUniverse** -- comma-separated tickers for the defensive sleeve.
- **TopO** -- number of top offensive assets to hold when fully offensive (1 for G4, 5 for G12).
- **BreadthThreshold** -- the B in `CBF = min(1, n_bad / B)`. 1 for G4, 4 for G12.

Use `--preset VAA-G4` or `--preset VAA-G12` to select a named variant.

## Notes

The 13612W momentum signal is heavily biased toward recent months (the 1-month return carries 12x the weight of the 12-month return). This makes VAA quicker to respond to market changes than typical 12-1 momentum strategies, but also leads to high portfolio turnover (Allocate Smartly reports ~700% annual turnover for the aggressive variant).

ETF inception dates limit the practical backtest window. For G4: AGG (2003), EEM (2003), EFA (2001) are the binding constraints. For G12: HYG (2007) and GSG (2006) are the binding constraints.

## References

- Keller, W. J. and Keuning, J. W. (2017). [Breadth Momentum and Vigilant Asset Allocation (VAA): Winning More by Losing Less](https://papers.ssrn.com/sol3/papers.cfm?abstract_id=3002624). SSRN.

## Assets Typically Held

| Ticker | Name                                                | Sector                              |
| ------ | --------------------------------------------------- | ----------------------------------- |
| SPY    | SPDR S&P 500 ETF                                    | Equity, U.S., Large Cap             |
| QQQ    | Invesco QQQ                                         | Equity, U.S., Large Cap             |
| IWM    | iShares Russell 2000 ETF                            | Equity, U.S., Small Cap             |
| EFA    | iShares MSCI EAFE ETF                               | Equity, Developed ex-U.S.           |
| VGK    | Vanguard FTSE Europe ETF                            | Equity, Europe, Large Cap           |
| EWJ    | iShares MSCI Japan ETF                              | Equity, Japan, Large Cap            |
| EEM    | iShares MSCI Emerging Markets ETF                   | Equity, Emerging Markets            |
| VNQ    | Vanguard Real Estate Index Fund ETF                 | Real Estate, U.S.                   |
| GSG    | iShares S&P GSCI Commodity-Indexed Trust            | Commodity, Diversified              |
| GLD    | SPDR Gold Trust                                     | Commodity, Gold                     |
| HYG    | iShares iBoxx $ High Yield Corporate Bond ETF       | Bond, U.S., High Yield              |
| LQD    | iShares iBoxx $ Investment Grade Corporate Bond ETF | Bond, U.S., Investment Grade        |
| AGG    | iShares Core U.S. Aggregate Bond ETF                | Bond, U.S., Aggregate               |
| TLT    | iShares 20+ Year Treasury Bond ETF                  | Bond, U.S., Long-Term               |
| IEF    | iShares 7-10 Year Treasury Bond ETF                 | Bond, U.S., Intermediate-Term       |
| SHY    | iShares 1-3 Year Treasury Bond ETF                  | Bond, U.S., Short-Term              |
