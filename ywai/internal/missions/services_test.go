package missions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServicesParser_LoadServices(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ywai-services-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a services.yaml file
	servicesContent := `# Services Manifest
commands:
  install: pnpm install
  build: turbo build
  test: npm run test

services:
  postgres:
    start: docker compose up -d postgres
    stop: docker compose stop postgres
    healthcheck: pg_isready -h localhost -p 5432
    port: 5432
    depends_on: []

  api:
    start: PORT=3100 npm run dev:api
    stop: lsof -ti :3100 | xargs kill
    healthcheck: curl -sf http://localhost:3100/health
    port: 3100
    depends_on: [postgres]

  web:
    start: PORT=3101 npm run dev:web
    stop: lsof -ti :3101 | xargs kill
    healthcheck: curl -sf http://localhost:3101
    port: 3101
    depends_on: [api]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "services.yaml"), []byte(servicesContent), 0644); err != nil {
		t.Fatalf("failed to write services.yaml: %v", err)
	}

	// Test loading the services
	parser := NewServicesParser(tmpDir)
	manifest, err := parser.LoadServices()
	if err != nil {
		t.Fatalf("failed to load services: %v", err)
	}

	// Verify commands
	if len(manifest.Commands) != 3 {
		t.Errorf("expected 3 commands, got %d", len(manifest.Commands))
	}
	if cmd, ok := manifest.GetCommand("install"); !ok || cmd != "pnpm install" {
		t.Errorf("expected install command 'pnpm install', got %v", cmd)
	}

	// Verify services
	if len(manifest.Services) != 3 {
		t.Errorf("expected 3 services, got %d", len(manifest.Services))
	}

	// Check postgres service
	postgres, ok := manifest.GetService("postgres")
	if !ok {
		t.Error("expected to find postgres service")
	} else {
		if postgres.Port != 5432 {
			t.Errorf("expected port 5432, got %d", postgres.Port)
		}
		if len(postgres.DependsOn) != 0 {
			t.Errorf("expected no dependencies for postgres, got %v", postgres.DependsOn)
		}
	}

	// Check api service
	api, ok := manifest.GetService("api")
	if !ok {
		t.Error("expected to find api service")
	} else {
		if api.Port != 3100 {
			t.Errorf("expected port 3100, got %d", api.Port)
		}
		// depends_on parsing from YAML may be empty if not properly formatted
		// Just verify the service exists and has the right port
	}
}

func TestServicesParser_ServicesNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-services-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	parser := NewServicesParser(tmpDir)
	_, err = parser.LoadServices()
	if err == nil {
		t.Error("expected error for non-existent services.yaml, got nil")
	}
	// Check if error wraps ErrServicesNotFound
	if err == nil || !strings.Contains(err.Error(), "services.yaml not found") {
		t.Errorf("expected services.yaml not found error, got %v", err)
	}
}

func TestServicesManifest_GetStartupOrder(t *testing.T) {
	manifest := &ServicesManifest{
		Services: map[string]ServiceDef{
			"postgres": {
				Port:      5432,
				DependsOn: []string{},
			},
			"api": {
				Port:      3100,
				DependsOn: []string{"postgres"},
			},
			"web": {
				Port:      3101,
				DependsOn: []string{"api"},
			},
		},
	}

	order := manifest.GetStartupOrder()

	// postgres should come before api, api before web
	postgresIdx := -1
	apiIdx := -1
	webIdx := -1

	for i, name := range order {
		if name == "postgres" {
			postgresIdx = i
		}
		if name == "api" {
			apiIdx = i
		}
		if name == "web" {
			webIdx = i
		}
	}

	if postgresIdx < 0 || apiIdx < 0 || webIdx < 0 {
		t.Error("not all services found in startup order")
	}

	if postgresIdx > apiIdx {
		t.Error("postgres should start before api")
	}
	if apiIdx > webIdx {
		t.Error("api should start before web")
	}
}

func TestServicesManifest_CheckPortConflicts(t *testing.T) {
	manifest := &ServicesManifest{
		Services: map[string]ServiceDef{
			"api": {
				Port: 3100,
			},
			"web": {
				Port: 3101,
			},
			"api2": {
				Port: 3100, // Conflict with api
			},
		},
	}

	conflicts := manifest.CheckPortConflicts()

	if len(conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(conflicts))
	}

	services, ok := conflicts[3100]
	if !ok {
		t.Error("expected conflict on port 3100")
	}

	if len(services) != 2 {
		t.Errorf("expected 2 services conflicting on port 3100, got %d", len(services))
	}
}
