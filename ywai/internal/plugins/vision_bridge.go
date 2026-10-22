package plugins

import (
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallVisionBridge vendors the vision-bridge opencode plugin and registers it
// in the config's "plugins" array. The plugin auto-analyzes attached images via
// TokenBank vision models when the active chat model does not support image input.
func InstallVisionBridge(configPath string) error {
	return installVendorJS("", configPath, ManifestEntry{Bundle: "vision-bridge"})
}

func installVisionBridgeWithBundle(configPath, bundleSrc string) error {
	return installVendorPluginV2(configPath, bundleSrc, config.VisionBridgeBundleName)
}
