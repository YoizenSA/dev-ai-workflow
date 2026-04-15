package installer

import (
	"io"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer/api"
	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer/version"
	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/ui"
)

type Flags struct {
	All                   bool
	InstallGA             bool
	InstallSDD            bool
	InstallVSCode         bool
	InstallGlobal         bool
	InstallExt            bool
	SkipGA                bool
	SkipSDD               bool
	SkipVSCode            bool
	SkipHooks             bool
	Provider              string
	DefaultModel          string
	Target                string
	ProjectType           string
	Version               string
	Channel               string
	UpdateAll             bool
	SelfUpdate            bool
	SDDProfiles           bool
	UpdateGlobalAgents    bool
	SkipGlobalAgentsUpdate bool
	GlobalOnly            bool // When true, only update global tools, don't write to repo
	Force                 bool
	Silent                bool
	DryRun                bool
	Help                  bool
	ListTypes             bool
	ListExtensions        bool
	ListInstallableSkills bool
	NonInteractive        bool
	VersionFlag           bool
	Sync                  bool
	InstallSkill          string
	InstallSkills         []string
	BuildVersion          string
	Output                io.Writer
}

type ProjectTypeOption struct {
	Name        string
	Description string
}

type ProjectType struct {
	Description  string              `json:"description"`
	AgentsMD     string              `json:"agents_md"`
	ReviewMD     string              `json:"review_md"`
	LefthookYML  string              `json:"lefthook_yml"`
	GlobalAgents []string            `json:"global_agents"`
	Skills       []string            `json:"skills"`
	Extensions   map[string][]string `json:"extensions"`
}

type TypesConfig struct {
	Types      map[string]ProjectType `json:"types"`
	BaseConfig BaseConfig             `json:"base_config"`
	Default    string                 `json:"default"`
}

type BaseConfig struct {
	Description           string              `json:"description"`
	AppendAgentsTemplates []string            `json:"append_agents_templates"`
	Extensions            map[string][]string `json:"extensions"`
	CopySharedSkills      bool                `json:"copy_shared_skills"`
	CopyCommands          bool                `json:"copy_commands"`
	InitGA                bool                `json:"init_ga"`
	OptionalHooks         bool                `json:"optional_hooks"`
}

type Installer struct {
	flags           *Flags
	logger          *ui.Logger
	targetDir       string
	projectType     string
	provider        string
	version         string
	channel         string
	buildVersion    string
	apiClient       *api.GitHubAPI
	versionResolver *version.Resolver
	out             io.Writer
}

type PrerequisiteCheck struct {
	Name      string
	Available bool
	Version   string
	Required  bool
}

const (
	GA_REPO         = "https://github.com/Yoizen/dev-ai-workflow.git"
	DEFAULT_VERSION = "stable"
	DEFAULT_CHANNEL = "stable"
)
