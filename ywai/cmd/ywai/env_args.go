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
	// Workflow names an exported workflow to run instead of a plain prompt;
	// the rest of the line is the task text.
	Workflow string
	Pass     []string // other opencode2 flags, forwarded as-is
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
		case "--model", "-m", "--agent", "-a", "--workflow", "-w":
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
			switch name {
			case "--model", "-m":
				o.Model = val
			case "--agent", "-a":
				o.Agent = val
			default:
				o.Workflow = val
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

// resolveWorkflowRun turns `ywai <env> <workflow> "task"` into a normal
// prompt run against the workflow's orchestrator agent, mirroring the Run
// button in the Workflow Studio. The workflow comes from --workflow or from
// the first word of the line (with or without a leading slash); everything
// after it is the task text. Only real workflows (frontmatter agent:) claim
// the first word: plain commands keep their text as a literal prompt and run
// through the explicit --workflow form, which uses the command body.
//
// lookup reports the command's agent and prompt body (ok=false when the env
// has no such command) and list names what is available, for the error message.
func resolveWorkflowRun(prompt []string, o envRunOpts, lookup func(string) (string, string, bool), list func() []string) ([]string, envRunOpts, error) {
	name := strings.TrimSpace(o.Workflow)
	switch {
	case name != "":
		// Explicit flag: the whole line is the task.
	case len(prompt) > 0:
		first := strings.TrimPrefix(prompt[0], "/")
		// Only a workflow with its own orchestrator may be claimed by a bare
		// word; a plain prompt (or a plain command) must never be hijacked.
		if agent, _, ok := lookup(first); !ok || agent == "" {
			return prompt, o, nil
		}
		name, prompt = first, prompt[1:]
	default:
		return prompt, o, nil
	}

	agent, body, ok := lookup(name)
	if !ok {
		available := list()
		if len(available) == 0 {
			return nil, o, fmt.Errorf("env has no workflow %q exported yet — export it from the Workflow Studio (Export → env) or run `ywai install --profile <env> --agent opencode`", name)
		}
		return nil, o, fmt.Errorf("unknown workflow %q in this env; available: %s", name, strings.Join(available, ", "))
	}
	o.Workflow = name
	task := strings.Join(prompt, " ")
	if agent == "" {
		// Plain command: the file body is the prompt, with the task text
		// where $ARGUMENTS sits (what the TUI does when you invoke /name).
		// It runs with the env default agent.
		if body == "" {
			return nil, o, fmt.Errorf("command %q has no prompt body to run", name)
		}
		if strings.Contains(body, "$ARGUMENTS") {
			body = strings.ReplaceAll(body, "$ARGUMENTS", task)
		} else if task != "" {
			body += "\n\n" + task
		}
		return []string{strings.TrimSpace(body)}, o, nil
	}
	// An explicit --agent wins: the user asked for that agent by name.
	if o.Agent == "" {
		o.Agent = agent
	}
	if len(prompt) == 0 {
		prompt = []string{"Run the workflow."}
	}
	return prompt, o, nil
}

const envRunHelp = `Run opencode inside the environment.

  ywai %[1]s                                   open the TUI
  ywai %[1]s "task"                            headless run, prints the answer
  ywai %[1]s "task" --model opencode-go/glm-5.3-flash
                                                pick the model as provider/model (also --model=… / --model:…, -m)
  ywai %[1]s "task" --agent ask                pick the agent (also -a)
  ywai %[1]s "task" --auto                     auto-approve permissions (unattended runs)
  ywai %[1]s -c                                continue the last session in the TUI
  ywai %[1]s <workflow> "free text"            run an exported workflow with its orchestrator
                                                (also --workflow <name> / -w, and /<workflow>)

Other opencode2 flags (--file, --session, --title, --format…) pass through.
Use -- to send text that starts with a dash as prompt.
`
