package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// orcaStatusBundleName is the status plugin Orca deploys into the OpenCode
// config it manages. ywai does not own the file and there is no source for it
// in this repo — the deployed JS is the only copy.
const orcaStatusBundleName = "orca-opencode-status.js"

// orcaStatusShimMarker makes the repair idempotent and greppable. Orca
// redeploys the plugin on its own updates, so the patch is re-applied on the
// next install rather than being a one-time migration.
const orcaStatusShimMarker = "ywai:orca-status-v2-shim"

// orcaStatusV1Export is the default export Orca ships. It is matched exactly:
// a fuzzy match on someone else's file risks corrupting a plugin we cannot
// rebuild, so an unrecognised export is left alone and reported instead.
const orcaStatusV1Export = `export default {
  id: "orca-opencode-status",
  server: OrcaOpenCodeStatusPlugin,
};`

// orcaStatusV2Export replaces it with a dual export.
//
// Two things are wrong under OpenCode v2 and both are fixed here:
//
//   - The loader rejects a default export that carries only server(); it wants
//     id plus setup (or effect). server() is kept so the older factory loader
//     still resolves the same instance.
//   - v2 dropped message.updated, which the plugin uses to cache a message's
//     role. Without it the cache never fills and user prompts silently vanish
//     from the status Orca shows — a plugin that loads and reports nothing is
//     worse than one that fails loudly, so the shim synthesises the event from
//     v2's session.usage.updated / session.step.ended telemetry.
//
// The factory reads client.session.get / client.session.list and nothing else,
// so the v1-shaped client below only has to cover those two.
const orcaStatusV2Export = `// ` + orcaStatusShimMarker + `
// Added by ywai: OpenCode v2 needs a setup() export, and it no longer emits
// message.updated. Re-applied on every ywai install, so an Orca redeploy that
// drops it is repaired rather than silently breaking the status bar.
function ywaiV1ShapedClient(ctx) {
  const session = {
    async get(input) {
      const id = input && input.path && input.path.id;
      if (!id || !ctx || !ctx.session || typeof ctx.session.get !== "function") {
        return { data: undefined };
      }
      try {
        const result = await ctx.session.get({ sessionID: id });
        return { data: result && result.data !== undefined ? result.data : result };
      } catch {
        return { data: undefined };
      }
    },
    async list() {
      if (!ctx || !ctx.session || typeof ctx.session.list !== "function") {
        return { data: [] };
      }
      try {
        const result = await ctx.session.list({});
        const data = result && result.data !== undefined ? result.data : result;
        return { data: Array.isArray(data) ? data : [] };
      } catch {
        return { data: [] };
      }
    },
  };
  return { session };
}

// v2 has no message.updated. Its role telemetry arrives on session.usage.updated
// and session.step.ended, so rebuild the shape the plugin already knows.
function ywaiSynthesizeV1Events(event) {
  const out = [event];
  const type = event && event.type;
  if (type !== "session.usage.updated" && type !== "session.step.ended") return out;
  const props = (event && (event.properties || event.data)) || {};
  const info = props.info || props.message;
  if (info && info.id) {
    out.push({ type: "message.updated", properties: { info } });
  }
  return out;
}

async function ywaiSetupV2(ctx) {
  const instance = await OrcaOpenCodeStatusPlugin(ywaiV1ShapedClient(ctx));
  if (!instance || typeof instance.event !== "function") return;
  if (!ctx || !ctx.event || typeof ctx.event.subscribe !== "function") return;

  const controller = new AbortController();
  void (async () => {
    try {
      for await (const raw of ctx.event.subscribe({ signal: controller.signal })) {
        for (const event of ywaiSynthesizeV1Events(raw)) {
          try {
            await instance.event({ event });
          } catch {
            // One bad event must not tear down the subscription: the status
            // bar would go stale for the rest of the session.
          }
        }
      }
    } catch {
      // Aborted on teardown.
    }
  })();

  return () => controller.abort();
}

export default {
  id: "orca-opencode-status",
  setup: ywaiSetupV2,
  server: OrcaOpenCodeStatusPlugin,
};`

// RepairOrcaStatusPluginV2 rewrites Orca's status plugin so OpenCode v2 loads
// it. It reports whether it changed the file.
//
// This repairs a file ywai does not own, which is deliberate: Orca deploys the
// plugin into the config directory ywai manages, and on v2 it fails to load at
// all. Leaving it broken means the user's status bar stays dead until Orca
// ships its own fix.
func RepairOrcaStatusPluginV2(configPath string) (bool, error) {
	if !agent.OpenCodeIsV2() {
		return false, nil
	}

	path := filepath.Join(filepath.Dir(configPath), autoDiscoveredPluginsSubdir, orcaStatusBundleName)
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil // Orca is not installed here.
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	body := string(src)
	if strings.Contains(body, orcaStatusShimMarker) {
		return false, nil // already repaired
	}
	if !strings.Contains(body, orcaStatusV1Export) {
		// Orca changed the export. Patching a shape we do not recognise could
		// corrupt a plugin with no source to rebuild from, so stop here.
		return false, fmt.Errorf("unrecognised default export in %s: not patching", path)
	}

	if err := os.WriteFile(path+".ywai-bak", src, 0o644); err != nil {
		return false, fmt.Errorf("back up %s: %w", path, err)
	}
	patched := strings.Replace(body, orcaStatusV1Export, orcaStatusV2Export, 1)
	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
