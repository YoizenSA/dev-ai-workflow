package missions

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrServicesNotFound = fmt.Errorf("services.yaml not found")
	ErrInvalidServices  = fmt.Errorf("invalid services.yaml")
)

// ─── Services Manifest Parser ─────────────────────────────────────────────

// ServicesParser parses services.yaml files.
type ServicesParser struct {
	missionDir string
}

// NewServicesParser creates a new ServicesParser for the given mission directory.
func NewServicesParser(missionDir string) *ServicesParser {
	return &ServicesParser{missionDir: missionDir}
}

// LoadServices loads and parses the services manifest from services.yaml.
func (sp *ServicesParser) LoadServices() (*ServicesManifest, error) {
	servicesPath := filepath.Join(sp.missionDir, "services.yaml")

	content, err := os.ReadFile(servicesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrServicesNotFound, servicesPath)
		}
		return nil, fmt.Errorf("read services file: %w", err)
	}

	return sp.parseServices(content)
}

// parseServices parses services.yaml content into a ServicesManifest.
func (sp *ServicesParser) parseServices(content []byte) (*ServicesManifest, error) {
	var manifest ServicesManifest

	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidServices, err)
	}

	// Validate that commands and services maps are initialized
	if manifest.Commands == nil {
		manifest.Commands = make(map[string]string)
	}
	if manifest.Services == nil {
		manifest.Services = make(map[string]ServiceDef)
	}

	return &manifest, nil
}

// GetCommand returns a command by name.
func (sm *ServicesManifest) GetCommand(name string) (string, bool) {
	cmd, ok := sm.Commands[name]
	return cmd, ok
}

// GetService returns a service definition by name.
func (sm *ServicesManifest) GetService(name string) (*ServiceDef, bool) {
	svc, ok := sm.Services[name]
	return &svc, ok
}

// GetServiceDependencies returns all services that depend on the given service.
func (sm *ServicesManifest) GetServiceDependencies(serviceName string) []string {
	var deps []string
	for name, svc := range sm.Services {
		for _, dep := range svc.DependsOn {
			if dep == serviceName {
				deps = append(deps, name)
			}
		}
	}
	return deps
}

// GetStartupOrder returns services in dependency order (services with no deps first).
func (sm *ServicesManifest) GetStartupOrder() []string {
	// Simple topological sort based on depends_on
	visited := make(map[string]bool)
	order := []string{}

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		svc, ok := sm.Services[name]
		if !ok {
			return
		}

		// Visit dependencies first
		for _, dep := range svc.DependsOn {
			visit(dep)
		}

		order = append(order, name)
	}

	for name := range sm.Services {
		visit(name)
	}

	return order
}

// CheckPortConflicts checks if any services have conflicting port assignments.
func (sm *ServicesManifest) CheckPortConflicts() map[int][]string {
	portMap := make(map[int][]string)

	for name, svc := range sm.Services {
		if svc.Port > 0 {
			portMap[svc.Port] = append(portMap[svc.Port], name)
		}
	}

	// Filter to only conflicting ports
	conflicts := make(map[int][]string)
	for port, services := range portMap {
		if len(services) > 1 {
			conflicts[port] = services
		}
	}

	return conflicts
}
