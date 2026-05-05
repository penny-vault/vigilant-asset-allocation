// Copyright 2021-2026
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vaa

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/penny-vault/pvbt/asset"
	"github.com/penny-vault/pvbt/data"
	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
	"github.com/penny-vault/pvbt/universe"
)

//go:embed README.md
var description string

// VigilantAssetAllocation implements Keller & Keuning's VAA (2017).
//
// VAA scores assets with the recency-weighted 13612W momentum
//
//	12*Pct(1) + 4*Pct(3) + 2*Pct(6) + Pct(12)
//
// and uses generalized breadth momentum (CBF = min(1, n_bad / B)) to
// blend between an offensive sleeve (top TopO equal-weighted) and a
// defensive sleeve (best-of by score, single asset).
type VigilantAssetAllocation struct {
	OffensiveUniverse universe.Universe `pvbt:"offensive-universe" desc:"Offensive (risky) assets to select from" default:"SPY,EFA,EEM,AGG" suggest:"VAA-G4=SPY,EFA,EEM,AGG|VAA-G12=SPY,IWM,QQQ,VGK,EWJ,EEM,VNQ,GSG,GLD,HYG,LQD,TLT"`
	DefensiveUniverse universe.Universe `pvbt:"defensive-universe" desc:"Defensive assets considered when breadth is bad" default:"LQD,IEF,SHY" suggest:"VAA-G4=LQD,IEF,SHY|VAA-G12=LQD,IEF,SHY"`
	TopO              int               `pvbt:"top-offensive" desc:"Number of top offensive assets to hold when fully offensive" default:"1" suggest:"VAA-G4=1|VAA-G12=5"`
	BreadthThreshold  int               `pvbt:"breadth-threshold" desc:"Number of bad offensive assets that triggers full defensive (B in CBF=min(1,n_bad/B))" default:"1" suggest:"VAA-G4=1|VAA-G12=4"`
}

func (s *VigilantAssetAllocation) Name() string {
	return "Vigilant Asset Allocation"
}

func (s *VigilantAssetAllocation) Setup(_ *engine.Engine) {}

func (s *VigilantAssetAllocation) Describe() engine.StrategyDescription {
	return engine.StrategyDescription{
		ShortCode:   "vaa",
		Description: description,
		Source:      "https://papers.ssrn.com/sol3/papers.cfm?abstract_id=3002624",
		Version:     "0.1.0",
		VersionDate: time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		Schedule:    "@monthend",
		Benchmark:   "SPY",
	}
}

func (s *VigilantAssetAllocation) Compute(ctx context.Context, eng *engine.Engine, _ portfolio.Portfolio, batch *portfolio.Batch) error {
	// Fetch a 14-month window of daily closes; downsample to monthly.
	// Need 13 monthly points to compute Pct(12); 14 months ensures we
	// always get at least 13 after downsampling regardless of where the
	// window start falls within a month.
	offensiveDF, err := s.OffensiveUniverse.Window(ctx, portfolio.Months(14), data.AdjClose)
	if err != nil {
		return fmt.Errorf("failed to fetch offensive universe prices: %w", err)
	}

	defensiveDF, err := s.DefensiveUniverse.Window(ctx, portfolio.Months(14), data.AdjClose)
	if err != nil {
		return fmt.Errorf("failed to fetch defensive universe prices: %w", err)
	}

	offensiveMonthly := offensiveDF.Downsample(data.Monthly).Last()
	defensiveMonthly := defensiveDF.Downsample(data.Monthly).Last()

	if offensiveMonthly.Len() < 13 || defensiveMonthly.Len() < 13 {
		return nil
	}

	offensiveMom := momentum13612W(offensiveMonthly).Drop(math.NaN()).Last()
	defensiveMom := momentum13612W(defensiveMonthly).Drop(math.NaN()).Last()

	if offensiveMom.Len() == 0 || defensiveMom.Len() == 0 {
		return nil
	}

	for _, a := range offensiveMom.AssetList() {
		for _, m := range offensiveMom.MetricList() {
			v := offensiveMom.Value(a, m)
			if !math.IsNaN(v) {
				batch.Annotate(a.Ticker+"/"+string(m), strconv.FormatFloat(v, 'f', -1, 64))
			}
		}
	}

	for _, a := range defensiveMom.AssetList() {
		for _, m := range defensiveMom.MetricList() {
			v := defensiveMom.Value(a, m)
			if !math.IsNaN(v) {
				batch.Annotate(a.Ticker+"/"+string(m), strconv.FormatFloat(v, 'f', -1, 64))
			}
		}
	}

	offensiveScores := sortByScore(offensiveMom)
	defensiveScores := sortByScore(defensiveMom)

	// Count "bad" offensive assets (score <= 0). Generalized breadth momentum:
	// CBF = min(1, n_bad / B). CBF goes to defensive, (1-CBF) to offensive.
	nBad := 0

	for _, sc := range offensiveScores {
		if sc.score <= 0 {
			nBad++
		}
	}

	b := s.BreadthThreshold
	if b < 1 {
		b = 1
	}

	cbf := math.Min(1.0, float64(nBad)/float64(b))
	defensiveWeight := cbf
	offensiveWeight := 1.0 - cbf

	topO := s.TopO
	if topO < 1 {
		topO = 1
	}

	if topO > len(offensiveScores) {
		topO = len(offensiveScores)
	}

	members := make(map[asset.Asset]float64)

	if offensiveWeight > 0 && topO > 0 {
		perAsset := offensiveWeight / float64(topO)
		for _, sc := range offensiveScores[:topO] {
			members[sc.a] += perAsset
		}
	}

	if defensiveWeight > 0 && len(defensiveScores) > 0 {
		members[defensiveScores[0].a] += defensiveWeight
	}

	regime := "offensive"

	switch {
	case cbf >= 1.0:
		regime = "defensive"
	case cbf > 0:
		regime = "mixed"
	}

	justification := fmt.Sprintf(
		"regime=%s n_bad=%d B=%d cbf=%.2f topO=%d",
		regime, nBad, b, cbf, topO,
	)

	batch.Annotate("regime", regime)
	batch.Annotate("n_bad", strconv.Itoa(nBad))
	batch.Annotate("cbf", strconv.FormatFloat(cbf, 'f', -1, 64))
	batch.Annotate("justification", justification)

	allocation := portfolio.Allocation{
		Date:          eng.CurrentDate(),
		Members:       members,
		Justification: justification,
	}

	if err := batch.RebalanceTo(ctx, allocation); err != nil {
		return fmt.Errorf("rebalance failed: %w", err)
	}

	return nil
}

// momentum13612W computes the recency-weighted 13612W momentum:
//
//	12*RET(1) + 4*RET(3) + 2*RET(6) + RET(12)
//
// where RET(n) = p0/pn - 1 (n-month return). This is the canonical Keller
// VAA momentum signal.
func momentum13612W(df *data.DataFrame) *data.DataFrame {
	ret1 := df.Pct(1).MulScalar(12)
	ret3 := df.Pct(3).MulScalar(4)
	ret6 := df.Pct(6).MulScalar(2)
	ret12 := df.Pct(12)

	return ret1.Add(ret3).Add(ret6).Add(ret12)
}

type assetScore struct {
	a     asset.Asset
	score float64
}

func sortByScore(mom *data.DataFrame) []assetScore {
	scores := make([]assetScore, 0, len(mom.AssetList()))
	for _, a := range mom.AssetList() {
		scores = append(scores, assetScore{a: a, score: mom.Value(a, data.AdjClose)})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	return scores
}
