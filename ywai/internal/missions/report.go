package missions

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateMissionReport creates a REPORT.md for a completed mission.
// The report includes handoffs, verification results, logs, and evidence.
func GenerateMissionReport(store *MissionsStore, mission *Mission) error {
	if mission == nil {
		return fmt.Errorf("mission is nil")
	}

	reportDir := filepath.Join(store.MissionDir(mission.ID), "report")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}

	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Mission Report: %s\n\n", mission.Name))
	b.WriteString(fmt.Sprintf("**ID:** %s  \n", mission.ID))
	b.WriteString(fmt.Sprintf("**Status:** %s  \n", mission.Status))
	b.WriteString(fmt.Sprintf("**Project:** %s  \n", mission.Project))
	if mission.CompletedAt != nil {
		b.WriteString(fmt.Sprintf("**Completed:** %s  \n", mission.CompletedAt.Format(time.RFC3339)))
	}
	b.WriteString(fmt.Sprintf("**Features:** %d  \n", len(mission.Features)))
	b.WriteString("\n---\n\n")

	// Summary table
	b.WriteString("## Feature Summary\n\n")
	b.WriteString("| Feature | Status | Retries | Verify Runs |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, feat := range mission.Features {
		verifyPassed := 0
		verifyTotal := len(feat.VerifyRuns)
		for _, vr := range feat.VerifyRuns {
			if vr.Passed {
				verifyPassed++
			}
		}
		verifyStr := fmt.Sprintf("%d/%d", verifyPassed, verifyTotal)
		if verifyTotal == 0 {
			verifyStr = "-"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n",
			feat.ID, feat.Status, feat.RetryCount, verifyStr))
	}
	b.WriteString("\n---\n\n")

	// Detail per feature
	b.WriteString("## Feature Details\n\n")
	for i, feat := range mission.Features {
		b.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, feat.ID))
		b.WriteString(fmt.Sprintf("**Description:** %s  \n", feat.Description))
		b.WriteString(fmt.Sprintf("**Status:** %s  \n", feat.Status))
		b.WriteString(fmt.Sprintf("**Retry Count:** %d  \n", feat.RetryCount))

		if feat.LastError != "" {
			b.WriteString("\n**Last Error:**\n```\n")
			b.WriteString(truncateOutput(feat.LastError, 1024))
			b.WriteString("\n```\n\n")
		}

		// Verify runs
		if len(feat.VerifyRuns) > 0 {
			b.WriteString("#### Verification Results\n\n")
			for j, vr := range feat.VerifyRuns {
				b.WriteString(fmt.Sprintf("**Run %d** — %s  \n", j+1, statusEmoji(vr.Passed)))
				b.WriteString(fmt.Sprintf("- Time: %s  \n", vr.RunAt.Format(time.RFC3339)))
				if vr.Output != "" {
					b.WriteString("- Output:\n```\n")
					b.WriteString(truncateOutput(vr.Output, 512))
					b.WriteString("\n```\n")
				}
				b.WriteString("\n")
			}
		}

		// Handoff evidence
		handoff := readHandoffFromFile(store, mission.ID, feat.ID)
		if handoff != nil {
			b.WriteString("#### Handoff Evidence\n\n")
			b.WriteString(fmt.Sprintf("**Summary:** %s  \n", handoff.SalientSummary))
			b.WriteString(fmt.Sprintf("**Implemented:** %s  \n", handoff.WhatWasImplemented))
			if handoff.WhatWasLeftUndone != "" {
				b.WriteString(fmt.Sprintf("**Left undone:** %s  \n", handoff.WhatWasLeftUndone))
			}
			if len(handoff.Verification.CommandsRun) > 0 {
				b.WriteString("**Verification commands:**\n")
				for _, c := range handoff.Verification.CommandsRun {
					b.WriteString(fmt.Sprintf("- `%s` (exit: %d)\n", c.Command, c.ExitCode))
				}
			}
			if len(handoff.DiscoveredIssues) > 0 {
				b.WriteString("**Discovered issues:**\n")
				for _, iss := range handoff.DiscoveredIssues {
					b.WriteString(fmt.Sprintf("- %s\n", iss.Description))
				}
			}
			b.WriteString("\n")
		}

		b.WriteString("---\n\n")
	}

	// Milestone Validation section — render VerifyEvidence from last report per milestone
	if len(mission.Milestones) > 0 {
		b.WriteString("## Milestone Validation\n\n")
		for _, ms := range mission.Milestones {
			if len(ms.ValidationReports) > 0 {
				lastReport := ms.ValidationReports[len(ms.ValidationReports)-1]
				b.WriteString(fmt.Sprintf("### %s\n\n", ms.Name))
				b.WriteString(fmt.Sprintf("**Status:** %s  \n", statusEmoji(lastReport.Passed)))
				if len(lastReport.VerifyEvidence) > 0 {
					b.WriteString("\n#### Verification Evidence\n\n")
					for _, ve := range lastReport.VerifyEvidence {
						if ve.Passed {
							b.WriteString(fmt.Sprintf("- ✅ `%s`: passed\n", ve.FeatureID))
						} else {
							b.WriteString(fmt.Sprintf("- ❌ `%s`: failed — `%s`\n", ve.FeatureID, ve.FailedCommand))
						}
					}
					b.WriteString("\n")
				}
			}
		}
		b.WriteString("---\n\n")
	}

	reportPath := filepath.Join(reportDir, "REPORT.md")
	if err := os.WriteFile(reportPath, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	log.Printf("Mission report generated: %s", reportPath)
	return nil
}

// ReadFeatureHandoff reads the handoff for a specific feature from disk.
func ReadFeatureHandoff(store *MissionsStore, missionID, featureID string) *WorkerHandoff {
	return readHandoffFromFile(store, missionID, featureID)
}

// readHandoffFromFile reads a worker handoff from the mission's artifacts directory.
func readHandoffFromFile(store *MissionsStore, missionID, featureID string) *WorkerHandoff {
	handoffPath := filepath.Join(store.MissionDir(missionID), "workers", featureID, "handoff.json")
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		return nil
	}
	var handoff WorkerHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		return nil
	}
	return &handoff
}

// statusEmoji returns an emoji for a verification status.
func statusEmoji(passed bool) string {
	if passed {
		return "✅ Passed"
	}
	return "❌ Failed"
}
