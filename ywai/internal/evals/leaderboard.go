package evals

import (
	"sort"
	"time"
)

// maxTrend caps how many per-run points a leaderboard row keeps: a handful of
// recent results shows direction without redrawing the whole history.
const maxTrend = 5

// LeaderRow is one model's line on the leaderboard: its averages over every
// answered attempt in the window, plus a per-run trend of its last few
// results. Runs counts only the runs where the model answered at least once —
// a run it never answered is not a measurement of it.
type LeaderRow struct {
	Model       string    `json:"model"`
	Runs        int       `json:"runs"`
	AvgWeighted float64   `json:"avgWeighted"`
	HardRate    float64   `json:"hardRate"`
	AvgCostUSD  float64   `json:"avgCostUsd"`
	CostKnown   bool      `json:"costKnown"`
	Trend       []float64 `json:"trend"`
}

// lbRunSum accumulates the counted attempts of one run — or, in a model's
// totals, of a whole window — for one model. The counting rules follow
// Aggregate: answered attempts only, CostKnown is an AND over counted
// attempts, and unknown prices contribute their stored zero.
type lbRunSum struct {
	attempts     int
	sumWeighted  float64
	hardHits     int
	sumCostUSD   float64
	allCostKnown bool
}

// lbRunScore is one run's contribution to a model's trend.
type lbRunScore struct {
	at    time.Time
	runID string
	avg   float64
}

// lbModel folds every windowed run of one model into totals plus the per-run
// averages the trend needs — Aggregate reduces to a single summary, which
// cannot show direction over time.
type lbModel struct {
	totals lbRunSum
	perRun []lbRunScore
}

// BuildLeaderboard ranks the models seen in runs within the last days days
// (days <= 0 means all time), best first: AvgWeighted desc, then model name
// asc so equal scores hold a stable order. A non-empty taskID keeps only that
// task's runs; an empty one ranks across every task. Only answered attempts
// count, and an empty window yields empty (non-nil) rows. Pure function: no
// I/O, and now is injected so the window is testable.
func BuildLeaderboard(runs []Run, taskID string, days int, now time.Time) []LeaderRow {
	var cutoff time.Time
	if days > 0 {
		cutoff = now.AddDate(0, 0, -days)
	}
	byModel := map[string]*lbModel{}
	for _, run := range runs {
		if taskID != "" && run.TaskID != taskID {
			continue
		}
		if days > 0 && run.StartedAt.Before(cutoff) {
			continue
		}
		// Summing per run first keeps the trend per run: a model that answered
		// twice in one run contributes one trend point, not two.
		perModel := map[string]*lbRunSum{}
		for _, a := range run.Attempts {
			if !a.Score.Answered {
				continue
			}
			rs := perModel[a.Model]
			if rs == nil {
				rs = &lbRunSum{allCostKnown: true}
				perModel[a.Model] = rs
			}
			rs.attempts++
			rs.sumWeighted += a.Score.Weighted
			if a.Score.GotHard {
				rs.hardHits++
			}
			rs.sumCostUSD += a.CostUSD
			rs.allCostKnown = rs.allCostKnown && a.CostKnown
		}
		for model, rs := range perModel {
			m := byModel[model]
			if m == nil {
				m = &lbModel{totals: lbRunSum{allCostKnown: true}}
				byModel[model] = m
			}
			m.totals.attempts += rs.attempts
			m.totals.sumWeighted += rs.sumWeighted
			m.totals.hardHits += rs.hardHits
			m.totals.sumCostUSD += rs.sumCostUSD
			m.totals.allCostKnown = m.totals.allCostKnown && rs.allCostKnown
			m.perRun = append(m.perRun, lbRunScore{
				at:    run.StartedAt,
				runID: run.ID,
				avg:   rs.sumWeighted / float64(rs.attempts),
			})
		}
	}

	rows := make([]LeaderRow, 0, len(byModel))
	for model, m := range byModel {
		n := float64(m.totals.attempts)
		rows = append(rows, LeaderRow{
			Model:       model,
			Runs:        len(m.perRun),
			AvgWeighted: m.totals.sumWeighted / n,
			HardRate:    float64(m.totals.hardHits) / n,
			AvgCostUSD:  m.totals.sumCostUSD / n,
			CostKnown:   m.totals.allCostKnown,
			Trend:       trendOf(m.perRun),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AvgWeighted != rows[j].AvgWeighted {
			return rows[i].AvgWeighted > rows[j].AvgWeighted
		}
		return rows[i].Model < rows[j].Model
	})
	return rows
}

// trendOf keeps a model's last few per-run averages, oldest first, so a chart
// reads left to right through time. Run ID breaks StartedAt ties so two runs
// started in the same instant keep a fixed order.
func trendOf(perRun []lbRunScore) []float64 {
	sort.Slice(perRun, func(i, j int) bool {
		if !perRun[i].at.Equal(perRun[j].at) {
			return perRun[i].at.Before(perRun[j].at)
		}
		return perRun[i].runID < perRun[j].runID
	})
	if len(perRun) > maxTrend {
		perRun = perRun[len(perRun)-maxTrend:]
	}
	trend := make([]float64, 0, len(perRun))
	for _, rs := range perRun {
		trend = append(trend, rs.avg)
	}
	return trend
}
