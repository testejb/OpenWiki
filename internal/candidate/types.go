package candidate

type AgentConfig struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Paths   []string `json:"paths"`
	Enabled bool     `json:"enabled"`
}

type CodeAgentConfig struct {
	Enabled          bool          `json:"enabled"`
	StatePath        string        `json:"state_path"`
	PendingDir       string        `json:"pending_dir"`
	RunLogPath       string        `json:"run_log_path"`
	SnapshotDir      string        `json:"snapshot_dir"`
	InitialDays      int           `json:"initial_days"`
	MaxRecordsPerRun int           `json:"max_records_per_run"`
	Agents           []AgentConfig `json:"agents"`
}
