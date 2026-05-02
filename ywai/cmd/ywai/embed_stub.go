//go:build !embedded

package main

import "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"

func init() {
	config.RegisterEmbeddedProviders(nil, nil)
}
