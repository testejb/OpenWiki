package candidate

import "time"

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

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
}

type Record struct {
	RecordID   string   `json:"record_id"`
	Agent      string   `json:"agent"`
	Type       string   `json:"type"`
	SourceFile string   `json:"source_file"`
	LineStart  int      `json:"line_start"`
	LineEnd    int      `json:"line_end"`
	ByteStart  int64    `json:"byte_start"`
	ByteEnd    int64    `json:"byte_end"`
	Timestamp  string   `json:"timestamp,omitempty"`
	SessionID  string   `json:"session_id,omitempty"`
	MessageID  string   `json:"message_id,omitempty"`
	Intent     string   `json:"intent,omitempty"`
	Actions    []string `json:"actions,omitempty"`
	Outcome    string   `json:"outcome,omitempty"`
	Learned    []string `json:"learned,omitempty"`
	Text       string   `json:"text"`
}

type FileState struct {
	Agent          string `json:"agent"`
	Type           string `json:"type"`
	FileID         string `json:"file_id"`
	Size           int64  `json:"size"`
	MTime          string `json:"mtime"`
	ProcessedLines int    `json:"processed_lines"`
	ProcessedBytes int64  `json:"processed_bytes"`
	TailHash       string `json:"tail_hash"`
	BoundaryHash   string `json:"boundary_hash,omitempty"`
	LastScannedAt  string `json:"last_scanned_at"`
}

type State struct {
	Version   int                  `json:"version"`
	Source    string               `json:"source"`
	UpdatedAt string               `json:"updated_at"`
	Files     map[string]FileState `json:"files"`
	Backlog   []Record             `json:"backlog,omitempty"`
}

type Pending struct {
	Version          int                  `json:"version"`
	Source           string               `json:"source"`
	CreatedAt        string               `json:"created_at"`
	Status           string               `json:"status"`
	Config           PendingConfig        `json:"config"`
	Limits           PendingLimits        `json:"limits"`
	Records          []Record             `json:"records"`
	StateUpdates     map[string]FileState `json:"state_updates"`
	BaseState        map[string]FileState `json:"base_state,omitempty"`
	BaseBacklogHash  string               `json:"base_backlog_hash,omitempty"`
	BacklogUpdateSet bool                 `json:"backlog_update_set,omitempty"`
	BacklogUpdate    []Record             `json:"backlog_update,omitempty"`
	Warnings         []Warning            `json:"warnings,omitempty"`
	ReviewDocURL     string               `json:"review_doc_url,omitempty"`
	SnapshotPath     string               `json:"snapshot_path,omitempty"`
	CommittedAt      string               `json:"committed_at,omitempty"`
}

type PendingConfig struct {
	ConfigPath  string `json:"config_path"`
	WikiRoot    string `json:"wiki_root"`
	StatePath   string `json:"state_path"`
	PendingDir  string `json:"pending_dir"`
	RunLogPath  string `json:"run_log_path"`
	SnapshotDir string `json:"snapshot_dir"`
}

type PendingLimits struct {
	InitialDays      int `json:"initial_days"`
	MaxRecordsPerRun int `json:"max_records_per_run"`
}

type ScanOptions struct {
	ConfigPath       string
	WikiRoot         string
	Now              time.Time
	InitialDays      int
	MaxRecordsPerRun int
}

type ScanResult struct {
	PendingPath string            `json:"pending_path"`
	StatePath   string            `json:"state_path"`
	RunLogPath  string            `json:"run_log_path"`
	SnapshotDir string            `json:"snapshot_dir"`
	Records     ScanRecordSummary `json:"records"`
	Limits      PendingLimits     `json:"limits"`
	Warnings    []Warning         `json:"warnings,omitempty"`
}

type ScanRecordSummary struct {
	Total   int            `json:"total"`
	ByAgent map[string]int `json:"by_agent"`
}

type CommitResult struct {
	PendingPath  string `json:"pending_path"`
	StatePath    string `json:"state_path"`
	ReviewDocURL string `json:"review_doc_url"`
	SnapshotPath string `json:"snapshot_path"`
	CommittedAt  string `json:"committed_at"`
}

type StatusResult struct {
	Enabled       bool          `json:"enabled"`
	StatePath     string        `json:"state_path"`
	PendingDir    string        `json:"pending_dir"`
	RunLogPath    string        `json:"run_log_path"`
	SnapshotDir   string        `json:"snapshot_dir"`
	TrackedFiles  int           `json:"tracked_files"`
	LastUpdatedAt string        `json:"last_updated_at,omitempty"`
	Agents        []AgentConfig `json:"agents"`
	Warnings      []Warning     `json:"warnings,omitempty"`
}
