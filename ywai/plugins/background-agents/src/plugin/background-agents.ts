/**
 * background-agents — supervision layer over OpenCode v2's native background
 * `subagent` (stop, watchdog timeout, steer, live status, post-compaction read).
 * Launch and completion notification are native. See v2.ts.
 *
 * v2 reads `id` and `setup()` from the default export.
 */

import { setupV2 } from "./v2"
import type { V2PluginContext } from "../../../shared/v2"

export default {
	id: "ywai-background-agents",
	setup: (ctx: V2PluginContext) => setupV2(ctx),
}
