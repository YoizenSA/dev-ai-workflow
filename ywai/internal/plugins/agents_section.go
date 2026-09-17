package plugins

import (
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// jevGateSection is the AGENTS.md block that teaches the agent how to use the
// jev-gate tools. It ships with the plugin because the rules are not optional:
// without them an agent can present a review it never ran, or restate a 0.8
// probability as a certainty.
//
//go:embed agents_sections/jev-gate.md
var jevGateSection string

// agentsSections maps a manifest entry's AgentsSection name to its body.
// Adding a section means adding a file and one line here, so an entry cannot
// name a section that does not exist.
var agentsSections = map[string]string{
	"jev-gate": jevGateSection,
}

// UpsertAgentsSection writes body into path between markers named by id,
// replacing an existing block with the same id and appending when there is
// none. ywai owns AGENTS.md and rewrites it on every install, so a plugin's
// section has to be re-applied after that rewrite rather than assumed to
// survive it.
//
// A missing AGENTS.md is created: an agent config can legitimately not have
// one yet when a plugin installs before the curated file is written.
func UpsertAgentsSection(path, id, body string) error {
	begin := fmt.Sprintf("<!-- BEGIN ywai:%s -->", id)
	end := fmt.Sprintf("<!-- END ywai:%s -->", id)
	block := begin + "\n" + strings.TrimSpace(body) + "\n" + end

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	content := string(existing)
	// (?s) so the block can span lines; the markers are literal, so a body
	// that merely mentions them in prose cannot open or close a block.
	pattern := regexp.MustCompile("(?s)" + regexp.QuoteMeta(begin) + ".*?" + regexp.QuoteMeta(end))
	if pattern.MatchString(content) {
		content = pattern.ReplaceAllLiteralString(content, block)
	} else {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if content != "" {
			content += "\n"
		}
		content += block + "\n"
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

// RemoveAgentsSection drops the block named by id, leaving the rest of the
// file untouched. Uninstalling a plugin must not leave instructions for tools
// that are no longer there.
func RemoveAgentsSection(path, id string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	begin := fmt.Sprintf("<!-- BEGIN ywai:%s -->", id)
	end := fmt.Sprintf("<!-- END ywai:%s -->", id)
	pattern := regexp.MustCompile("(?s)\n*" + regexp.QuoteMeta(begin) + ".*?" + regexp.QuoteMeta(end) + "\n*")
	content := pattern.ReplaceAllLiteralString(string(existing), "\n")
	return os.WriteFile(path, []byte(content), 0o644)
}
