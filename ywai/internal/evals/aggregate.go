package evals

import "sort"

// ModelTaskSummary is one model's roll-up over every stored run of one task: the
// numbers the comparison table ranks by. Averages cover only answered attempts —
// an errored or empty response is a missing measurement, not a zero — and
// CostKnown is true only when every counted attempt had a priced model, so a
// partial TotalCostUSD is visible instead of quietly wrong.
type ModelTaskSummary struct {
	Model        string  `json:"model"`
	Attempts     int     `json:"attempts"`
	AvgWeighted  float64 `json:"avgWeighted"`
	HardRate     float64 `json:"hardRate"`
	AvgTurns     float64 `json:"avgTurns"`
	AvgSeconds   float64 `json:"avgSeconds"`
	TokensIn     int64   `json:"tokensIn"`
	TokensOut    int64   `json:"tokensOut"`
	TotalCostUSD float64 `json:"totalCostUsd"`
	CostKnown    bool    `json:"costKnown"`
}

// aggAcc accumulates one model's counted attempts before averaging.
type aggAcc struct {
	attempts     int
	sumWeighted  float64
	hardHits     int
	sumTurns     int
	sumSeconds   float64
	sumTokensIn  int64
	sumTokensOut int64
	sumCostUSD   float64
	allCostKnown bool
}

// Aggregate reduces the runs of one task into per-model summaries, best model
// first: weighted score desc, then hard-expectation rate desc, then turns asc —
// score outranks speed, and between equals the leaner trace does. Runs of other
// tasks are ignored; the model name breaks a full tie so the order is stable.
func Aggregate(runs []Run, taskID string) []ModelTaskSummary {
	byModel := map[string]*aggAcc{}
	for _, run := range runs {
		if run.TaskID != taskID {
			continue
		}
		for _, a := range run.Attempts {
			if !a.Score.Answered {
				continue
			}
			acc := byModel[a.Model]
			if acc == nil {
				acc = &aggAcc{allCostKnown: true}
				byModel[a.Model] = acc
			}
			acc.attempts++
			acc.sumWeighted += a.Score.Weighted
			if a.Score.GotHard {
				acc.hardHits++
			}
			acc.sumTurns += a.Metrics.Turns
			acc.sumSeconds += a.Seconds
			acc.sumTokensIn += a.Metrics.TokensIn
			acc.sumTokensOut += a.Metrics.TokensOut
			acc.sumCostUSD += a.CostUSD // unknown models price as (0, false), so summing is safe
			acc.allCostKnown = acc.allCostKnown && a.CostKnown
		}
	}

	out := make([]ModelTaskSummary, 0, len(byModel))
	for model, acc := range byModel {
		n := float64(acc.attempts)
		out = append(out, ModelTaskSummary{
			Model:        model,
			Attempts:     acc.attempts,
			AvgWeighted:  acc.sumWeighted / n,
			HardRate:     float64(acc.hardHits) / n,
			AvgTurns:     float64(acc.sumTurns) / n,
			AvgSeconds:   acc.sumSeconds / n,
			TokensIn:     acc.sumTokensIn,
			TokensOut:    acc.sumTokensOut,
			TotalCostUSD: acc.sumCostUSD,
			CostKnown:    acc.allCostKnown,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AvgWeighted != out[j].AvgWeighted {
			return out[i].AvgWeighted > out[j].AvgWeighted
		}
		if out[i].HardRate != out[j].HardRate {
			return out[i].HardRate > out[j].HardRate
		}
		if out[i].AvgTurns != out[j].AvgTurns {
			return out[i].AvgTurns < out[j].AvgTurns
		}
		return out[i].Model < out[j].Model
	})
	return out
}
