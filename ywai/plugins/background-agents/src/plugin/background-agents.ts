/**
 * background-agents
 * Async delegation system for OpenCode — v2 only.
 *
 * Delegates tasks to sub-agents running in isolated sessions. All agent
 * outputs are persisted to storage, the supervisor receives only key
 * references.
 *
 * Based on oh-my-opencode by @code-yeongyu (MIT License)
 * https://github.com/code-yeongyu/oh-my-opencode
 *
 * Architecture: v2's built-in `subagent` tool is the LAUNCH transport (it owns
 * session creation, agent/model resolution, permissions, parent linking and
 * the child run loop). This plugin is the supervision layer the native tool
 * lacks — parent notifications, steer/stop/peek/status, watchdog timeouts,
 * crash recovery, persisted artifacts, anti-recursion. See v2.ts.
 *
 * The v1 host surface (server() hooks, the delegate/delegation_* tool map,
 * the task-tool guard, compaction hooks, the flavor marker) was removed:
 * ywai installs this plugin only under opencode2.
 */

import { formatDelegationContext } from "./context"
import { DelegationManager } from "./delegation-manager"
import { deserializeDelegation, serializeDelegation } from "./state"
import { setupV2 } from "./v2"
import type { V2PluginContext } from "../../../shared/v2"

/**
 * v2-only entry: v2 reads `id` and `setup()` from the default export and
 * rejects function-shaped or v1-hook exports at load.
 */
export default Object.assign(
	{
		id: "ywai-background-agents",
		setup: (ctx: V2PluginContext) => setupV2(ctx),
	},
	{
		testInternals: {
			DelegationManager,
			formatDelegationContext,
			serializeDelegation,
			deserializeDelegation,
		},
	} as const,
)
