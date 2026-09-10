/**
 * JSON error recovery.
 *
 * Models occasionally emit invalid JSON in tool arguments. The host then
 * returns a parse error as the tool output, and a model that does not
 * understand the failure often repeats the identical invalid call. This hook
 * appends a corrective instruction to the tool output when it matches a known
 * JSON parse failure, so the next attempt fixes syntax instead of looping.
 *
 * Excluded tools are the ones whose output legitimately quotes JSON errors
 * (a bash command printing one, a webfetch of an erroring API) — nagging
 * there is noise, not recovery.
 *
 * Ported from oh-my-opencode-slim (MIT), src/hooks/json-error-recovery.
 */

export const JSON_ERROR_TOOL_EXCLUDE_LIST = ["bash", "read", "glob", "webfetch", "gh_grep_searchgithub"] as const

export const JSON_ERROR_PATTERNS = [
	/json parse error/i,
	/failed to parse json/i,
	/invalid json/i,
	/malformed json/i,
	/unexpected end of json input/i,
	/syntaxerror:\s*unexpected token.*json/i,
	/json[^\n]*expected '\}'/i,
	/json[^\n]*unexpected eof/i,
] as const

const JSON_ERROR_REMINDER_MARKER = "[JSON PARSE ERROR - IMMEDIATE ACTION REQUIRED]"
const JSON_ERROR_EXCLUDED_TOOLS = new Set<string>(JSON_ERROR_TOOL_EXCLUDE_LIST)

export const JSON_ERROR_REMINDER = `
${JSON_ERROR_REMINDER_MARKER}

You sent invalid JSON arguments. The system could not parse your tool call.
STOP and do this NOW:

1. LOOK at the error message above to see what was expected vs what you sent.
2. CORRECT your JSON syntax (missing braces, unescaped quotes, trailing commas, etc).
3. RETRY the tool call with valid JSON.

DO NOT repeat the exact same invalid call.
`

interface ToolExecuteAfterInput {
	tool: string
	sessionID?: string
	callID?: string
}

interface ToolExecuteAfterOutput {
	output?: unknown
	title?: string
	metadata?: unknown
}

export interface JsonErrorRecoveryHooks {
	"tool.execute.after": (input: ToolExecuteAfterInput, output: ToolExecuteAfterOutput) => Promise<void>
}

export function createJsonErrorRecoveryHook(): JsonErrorRecoveryHooks {
	return {
		"tool.execute.after": async (input: ToolExecuteAfterInput, output: ToolExecuteAfterOutput): Promise<void> => {
			if (JSON_ERROR_EXCLUDED_TOOLS.has(input.tool.toLowerCase())) return
			if (typeof output.output !== "string") return
			if (output.output.includes(JSON_ERROR_REMINDER_MARKER)) return

			const hasJsonError = JSON_ERROR_PATTERNS.some((pattern) => pattern.test(output.output as string))

			if (hasJsonError) {
				output.output += `\n${JSON_ERROR_REMINDER}`
			}
		},
	}
}
