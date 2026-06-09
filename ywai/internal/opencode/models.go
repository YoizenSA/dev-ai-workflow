package opencode

// AgentInfo represents an agent from either server API or local config.
type AgentInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
}

// ModelInfo represents a model from either server API or local config.
type ModelInfo struct {
	ID       string `json:"id"`
	Provider string `json:"provider,omitempty"`
	Name     string `json:"name,omitempty"`
}

// ClientStatus indicates connectivity state of a Client.
type ClientStatus struct {
	Connected           bool     `json:"connected"`
	Source              string   `json:"source"` // "server" | "local"
	Version             string   `json:"version,omitempty"`
	ConnectedProviders  []string `json:"connectedProviders,omitempty"`
}
