package doctor

// Status represents the health status of a check.
type Status string

const (
	StatusOK    Status = "OK"
	StatusWarn  Status = "WARN"
	StatusError Status = "ERROR"
	StatusInfo  Status = "INFO"
)

// Symbol returns the icon representation for console output.
func (s Status) Symbol() string {
	switch s {
	case StatusOK:
		return "✓"
	case StatusWarn:
		return "!"
	case StatusError:
		return "✗"
	case StatusInfo:
		return "ℹ"
	default:
		return "?"
	}
}

// Category identifies which subsystem a check belongs to.
type Category string

const (
	CategoryDependencies Category = "Dependencies"
	CategorySystemd      Category = "Systemd & Services"
	CategoryHardware     Category = "Display & Permissions"
	CategoryConfig       Category = "Configuration & Crontab"
	CategoryGit          Category = "Git Fleet Sync"
)

// CheckResult represents the outcome of a diagnostic check.
type CheckResult struct {
	Category   Category `json:"category"`
	Name       string   `json:"name"`
	Status     Status   `json:"status"`
	Message    string   `json:"message"`
	Detail     string   `json:"detail,omitempty"`
	Fixable    bool     `json:"fixable,omitempty"`
	FixHint    string   `json:"fix_hint,omitempty"`
	FixApplied bool     `json:"fix_applied,omitempty"`
}

// Options controls doctor execution parameters.
type Options struct {
	Fix             bool   `json:"fix"`
	JSON            bool   `json:"json"`
	Verbose         bool   `json:"verbose"`
	AgentConfigPath string `json:"agent_config_path,omitempty"`
	ScreenPath      string `json:"screen_path,omitempty"`
}

// Summary summarizes the result counts across all checks.
type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
	Fixed   int `json:"fixed"`
}

// Report contains all results and overall summary for doctor output.
type Report struct {
	Results []CheckResult `json:"results"`
	Summary Summary       `json:"summary"`
}
