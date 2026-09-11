package evals

// Weighting turns the per-expectation flags into comparable numbers so a weighted
// score can rank answers that hit a different mix of hard and soft expectations.
// The rules:
//
//   - an explicit Weight always wins, so a task author can make one expectation
//     matter more than any default;
//   - otherwise a hard expectation weighs 2 and a soft one 1, because the hard
//     expectation is what separates a real trace from a shallow one;
//   - history is never backfilled: runs scored before weighting carry no weighted
//     value at all, so old and new results are not compared by accident.
const (
	defaultHardWeight = 2
	defaultSoftWeight = 1
)

// effectiveWeight is the pull one expectation has on the weighted score.
func effectiveWeight(e Expectation) int {
	switch {
	case e.Weight > 0:
		return e.Weight
	case e.Hard:
		return defaultHardWeight
	default:
		return defaultSoftWeight
	}
}

// weightedRatio turns the accumulated weights into the weighted score, a share
// from 0 to 1. An unanswered attempt keeps hitWeight at 0, so it scores 0 without
// ever counting an errored response as a wrong answer.
func weightedRatio(hitWeight, totalWeight int) float64 {
	if totalWeight == 0 {
		return 0
	}
	return float64(hitWeight) / float64(totalWeight)
}
