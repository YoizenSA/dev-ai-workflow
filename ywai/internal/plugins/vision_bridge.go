package plugins

import (
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallVisionBridge vendors the vision-bridge opencode plugin and registers it
// in the config's "plugins" array. The plugin auto-analyzes attached images via
// TokenBank vision models when the active chat model does not support image input.
func InstallVisionBridge(configPath string) error {
	bundle, err := config.VisionBridgeBundlePath()
	if err != nil {
		return err
	}
	return installVisionBridgeWithBundle(configPath, bundle)
}

func installVisionBridgeWithBundle(configPath, bundleSrc string) error {
	if agent.OpenCodeIsV2() {
		return installVendorPluginV2(configPath, bundleSrc, config.VisionBridgeBundleName)
	}

	return installVendoredV1(configPath, bundleSrc, config.VisionBridgeBundleName)
}
