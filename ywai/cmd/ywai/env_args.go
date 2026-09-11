package main

import (
	"fmt"
	"strings"
)

// envRunOpts are the ywai-level flags of `ywai <env> [prompt]`.
type envRunOpts struct {
	Model string
	Agent string
	Auto  bool
	Pass  []string // other opencode2 flags, forwarded as-is
}

// runValueFlags are opencode2 run/TUI flags that take a value, so the value
// is forwarded with the flag instead of being read as prompt text.
var runValueFlags = map[string]bool{
	"--session": true, "-s": true, "--file": true, "-f": true,
	"--title": true, "--format": true, "--server": true,
}

// parseEnvRunArgs splits `ywai <env>` args into prompt words and flags.
// --model/-m and --agent/-a accept "--model x", "--model=x" and
// "--model:x" anywhere on the line; "--" ends flag parsing (the rest is
// prompt text, even if it starts with a dash).
func parseEnvRunArgs(args []string) ([]string, envRunOpts, error) {
	var prompt []string
	var o envRunOpts
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			prompt = append(prompt, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			prompt = append(prompt, a)
			continue
		}
		name, val, hasVal := splitFlag(a)
		switch name {
		case "--model", "-m", "--agent", "-a":
			if !hasVal {
				if i+1 >= len(args) {
					return nil, o, fmt.Errorf("%s needs a value, e.g. --model opencode-go/glm-5.3-flash or --agent ask", name)
				}
				i++
				val = args[i]
			}
			if strings.TrimSpace(val) == "" {
				return nil, o, fmt.Errorf("%s needs a value", name)
			}
			if name == "--model" || name == "-m" {
				o.Model = val
			} else {
				o.Agent = val
			}
		case "--auto":
			o.Auto = true
		default:
			o.Pass = append(o.Pass, a)
			if runValueFlags[name] && !hasVal && i+1 < len(args) {
				i++
				o.Pass = append(o.Pass, args[i])
			}
		}
	}
	return prompt, o, nil
}

// splitFlag splits "--model=x" or "--model:x" into name and value.
func splitFlag(a string) (name, val string, ok bool) {
	if i := strings.IndexAny(a, "=:"); i > 0 {
		return a[:i], a[i+1:], true
	}
	return a, "", false
}

// wantsHelp reports a standalone -h/--help before any "--".
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// envOpencodeArgv builds the opencode2 arguments (after the binary): a prompt
// runs headless `run`; no prompt opens the TUI, which has no model/agent
// flags, so asking for them there is an error rather than silently ignored.
func envOpencodeArgv(prompt []string, o envRunOpts) ([]string, error) {
	if len(prompt) == 0 {
		if o.Model != "" || o.Agent != "" {
			return nil, fmt.Errorf("--model/--agent only apply to a prompt run (ywai <env> \"prompt\" --model opencode-go/glm-5.3-flash --agent ask); pick them inside the TUI instead")
		}
		var argv []string
		if o.Auto {
			argv = append(argv, "--auto")
		}
		return append(argv, o.Pass...), nil
	}
	argv := []string{"run"}
	if o.Model != "" {
		argv = append(argv, "--model", o.Model)
	}
	if o.Agent != "" {
		argv = append(argv, "--agent", o.Agent)
	}
	if o.Auto {
		argv = append(argv, "--auto")
	}
	argv = append(argv, o.Pass...)
	return append(argv, strings.Join(prompt, " ")), nil
}

const envRunHelp = `Run opencode inside the environment.

  ywai %[1]s                                   open the TUI
  ywai %[1]s "task"                            headless run, prints the answer
  ywai %[1]s "task" --model opencode-go/glm-5.3-flash
                                                pick the model as provider/model (also --model=… / --model:…, -m)
  ywai %[1]s "task" --agent ask                pick the agent (also -a)
  ywai %[1]s "task" --auto                     auto-approve permissions (unattended runs)
  ywai %[1]s -c                                continue the last session in the TUI

Other opencode2 flags (--file, --session, --title, --format…) pass through.
Use -- to send text that starts with a dash as prompt.
`
