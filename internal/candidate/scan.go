package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func ScanCodeAgent(cfg CodeAgentConfig, opts ScanOptions) (ScanResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	initialDays := choosePositive(opts.InitialDays, cfg.InitialDays, defaultInitialDays)
	maxRecords := choosePositive(opts.MaxRecordsPerRun, cfg.MaxRecordsPerRun, defaultMaxRecordsPerRun)

	state, err := LoadState(cfg.StatePath)
	if err != nil {
		return ScanResult{}, err
	}

	var records []Record
	var warnings []Warning
	fileUpdates := map[string]FileState{}
	baseState := map[string]FileState{}

	if cfg.Enabled {
		for _, agent := range cfg.Agents {
			if !agent.Enabled {
				continue
			}
			paths, pathWarnings := expandAgentPaths(agent)
			warnings = append(warnings, pathWarnings...)
			for _, path := range paths {
				previous := state.Files[path]
				fileRecords, update, fileWarnings, err := scanOneFile(agent, path, previous, now)
				warnings = append(warnings, fileWarnings...)
				if err != nil {
					return ScanResult{}, err
				}
				if update != nil {
					fileUpdates[path] = *update
					baseState[path] = previous
				}
				records = append(records, fileRecords...)
			}
		}
	}

	records = filterFirstRun(records, state, initialDays, now)
	records = keepLatestRecords(records, maxRecords)
	stateUpdates := stateUpdatesForPendingRecords(records, fileUpdates, now)
	baseState = baseStateForUpdates(baseState, stateUpdates)

	pending := Pending{
		Version:   stateVersion,
		Source:    codeAgentSource,
		CreatedAt: now.UTC().Format(time.RFC3339),
		Status:    pendingStatus,
		Config: PendingConfig{
			ConfigPath:  opts.ConfigPath,
			WikiRoot:    opts.WikiRoot,
			StatePath:   cfg.StatePath,
			PendingDir:  cfg.PendingDir,
			RunLogPath:  cfg.RunLogPath,
			SnapshotDir: cfg.SnapshotDir,
		},
		Limits: PendingLimits{
			InitialDays:      initialDays,
			MaxRecordsPerRun: maxRecords,
		},
		Records:      records,
		StateUpdates: stateUpdates,
		BaseState:    baseState,
		Warnings:     warnings,
	}
	pendingPath, err := nextPendingPath(cfg.PendingDir, now)
	if err != nil {
		return ScanResult{}, err
	}
	if err := SavePendingAtomic(pendingPath, pending); err != nil {
		return ScanResult{}, err
	}
	_ = appendRunLog(cfg.RunLogPath, "pending_created", now, map[string]any{
		"pending_path": pendingPath,
		"records":      len(records),
		"warnings":     len(warnings),
	})

	return ScanResult{
		PendingPath: pendingPath,
		StatePath:   cfg.StatePath,
		RunLogPath:  cfg.RunLogPath,
		SnapshotDir: cfg.SnapshotDir,
		Records:     summarizeRecords(records),
		Limits:      pending.Limits,
		Warnings:    warnings,
	}, nil
}

func CommitCodeAgent(pendingPath, reviewDocURL, snapshotPath string, now time.Time) (CommitResult, error) {
	if strings.TrimSpace(reviewDocURL) == "" {
		return CommitResult{}, errors.New("review doc URL is required")
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CommitResult{}, fmt.Errorf("snapshot file does not exist: %s", snapshotPath)
		}
		return CommitResult{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}

	pending, err := LoadPending(pendingPath)
	if err != nil {
		return CommitResult{}, err
	}
	if pending.Status != pendingStatus {
		return CommitResult{}, fmt.Errorf("pending status must be %q, got %q", pendingStatus, pending.Status)
	}

	state, err := LoadState(pending.Config.StatePath)
	if err != nil {
		return CommitResult{}, err
	}
	if state.Files == nil {
		state.Files = map[string]FileState{}
	}
	for path, base := range pending.BaseState {
		current := state.Files[path]
		if !fileStateMatchesBase(current, base) {
			return CommitResult{}, fmt.Errorf("stale pending %s: state for %s changed since scan", pendingPath, path)
		}
	}
	for path, update := range pending.StateUpdates {
		state.Files[path] = update
	}
	committedAt := now.UTC().Format(time.RFC3339)
	state.UpdatedAt = committedAt
	if err := SaveStateAtomic(pending.Config.StatePath, state); err != nil {
		return CommitResult{}, err
	}

	pending.Status = committedStatus
	pending.ReviewDocURL = reviewDocURL
	pending.SnapshotPath = snapshotPath
	pending.CommittedAt = committedAt
	if err := SavePendingAtomic(pendingPath, pending); err != nil {
		return CommitResult{}, err
	}
	_ = appendRunLog(pending.Config.RunLogPath, "committed", now, map[string]any{
		"pending_path":   pendingPath,
		"review_doc_url": reviewDocURL,
		"snapshot_path":  snapshotPath,
	})

	return CommitResult{
		PendingPath:  pendingPath,
		StatePath:    pending.Config.StatePath,
		ReviewDocURL: reviewDocURL,
		SnapshotPath: snapshotPath,
		CommittedAt:  committedAt,
	}, nil
}

func StatusCodeAgent(cfg CodeAgentConfig) (StatusResult, error) {
	state, err := LoadState(cfg.StatePath)
	if err != nil {
		return StatusResult{}, err
	}
	return StatusResult{
		Enabled:       cfg.Enabled,
		StatePath:     cfg.StatePath,
		PendingDir:    cfg.PendingDir,
		RunLogPath:    cfg.RunLogPath,
		SnapshotDir:   cfg.SnapshotDir,
		TrackedFiles:  len(state.Files),
		LastUpdatedAt: state.UpdatedAt,
		Agents:        cfg.Agents,
	}, nil
}

func scanOneFile(agent AgentConfig, path string, previous FileState, now time.Time) ([]Record, *FileState, []Warning, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, nil, err
	}
	currentFileID := fileID(info)
	currentTailHash, err := tailHash(path)
	if err != nil {
		return nil, nil, nil, err
	}

	startByte := int64(0)
	startLine := 0
	var warnings []Warning
	if previous.FileID != "" {
		boundaryTrusted, err := processedBoundaryMatches(path, previous)
		if err != nil {
			return nil, nil, nil, err
		}
		switch {
		case previous.FileID == currentFileID && info.Size() == previous.ProcessedBytes && previous.TailHash == currentTailHash && boundaryTrusted:
			return nil, nil, nil, nil
		case previous.FileID == currentFileID && info.Size() > previous.ProcessedBytes && boundaryTrusted:
			startByte = previous.ProcessedBytes
			startLine = previous.ProcessedLines
		default:
			warnings = append(warnings, Warning{Code: "SOURCE_FILE_RESET", Message: "source file was truncated, replaced, or rewritten; scanning from start", Path: path})
		}
	}

	records, parseWarnings, err := ParseJSONLFile(agent, path, startByte, startLine)
	warnings = append(warnings, parseWarnings...)
	if err != nil {
		return nil, nil, warnings, err
	}
	lineCount, err := countLines(path)
	if err != nil {
		return nil, nil, warnings, err
	}
	mtime := info.ModTime().UTC().Format(time.RFC3339)
	lastScannedAt := now.UTC().Format(time.RFC3339)
	update := FileState{
		Agent:          agent.Name,
		Type:           agent.Type,
		FileID:         currentFileID,
		Size:           info.Size(),
		MTime:          mtime,
		ProcessedLines: lineCount,
		ProcessedBytes: info.Size(),
		TailHash:       currentTailHash,
		BoundaryHash:   mustBoundaryHash(path, info.Size()),
		LastScannedAt:  lastScannedAt,
	}
	return records, &update, warnings, nil
}

func choosePositive(override, configured, fallback int) int {
	if override > 0 {
		return override
	}
	if configured > 0 {
		return configured
	}
	return fallback
}

func expandAgentPaths(agent AgentConfig) ([]string, []Warning) {
	seen := map[string]bool{}
	var paths []string
	var warnings []Warning
	for _, pattern := range agent.Paths {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			warnings = append(warnings, Warning{Code: "INVALID_GLOB", Message: err.Error(), Path: pattern})
			continue
		}
		if len(matches) == 0 {
			if hasGlobMeta(pattern) {
				warnings = append(warnings, Warning{Code: "NO_MATCHING_FILES", Message: "no files matched source pattern", Path: pattern})
				continue
			}
			if _, err := os.Stat(pattern); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					warnings = append(warnings, Warning{Code: "SOURCE_FILE_NOT_FOUND", Message: "source file not found", Path: pattern})
					continue
				}
				warnings = append(warnings, Warning{Code: "SOURCE_FILE_STAT_FAILED", Message: err.Error(), Path: pattern})
				continue
			}
			matches = []string{pattern}
		}
		for _, match := range matches {
			if seen[match] {
				continue
			}
			seen[match] = true
			paths = append(paths, match)
		}
	}
	sort.Strings(paths)
	return paths, warnings
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func filterFirstRun(records []Record, state State, initialDays int, now time.Time) []Record {
	if initialDays <= 0 {
		return records
	}
	cutoff := now.AddDate(0, 0, -initialDays)
	filtered := records[:0]
	for _, record := range records {
		if _, known := state.Files[record.SourceFile]; known {
			filtered = append(filtered, record)
			continue
		}
		if record.Timestamp == "" {
			filtered = append(filtered, record)
			continue
		}
		parsed, err := time.Parse(time.RFC3339, record.Timestamp)
		if err != nil || !parsed.Before(cutoff) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func keepLatestRecords(records []Record, maxRecords int) []Record {
	if maxRecords <= 0 || len(records) <= maxRecords {
		return records
	}
	ordered := append([]Record(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return recordLess(ordered[i], ordered[j])
	})
	return append([]Record(nil), ordered[len(ordered)-maxRecords:]...)
}

func recordLess(left, right Record) bool {
	leftTime, leftOK := recordTimestamp(left)
	rightTime, rightOK := recordTimestamp(right)
	if leftOK && rightOK && !leftTime.Equal(rightTime) {
		return leftTime.Before(rightTime)
	}
	if leftOK != rightOK {
		return !leftOK
	}
	if left.SourceFile != right.SourceFile {
		return left.SourceFile < right.SourceFile
	}
	if left.LineStart != right.LineStart {
		return left.LineStart < right.LineStart
	}
	if left.ByteStart != right.ByteStart {
		return left.ByteStart < right.ByteStart
	}
	return left.RecordID < right.RecordID
}

func recordTimestamp(record Record) (time.Time, bool) {
	if record.Timestamp != "" {
		if parsed, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func nextPendingPath(pendingDir string, now time.Time) (string, error) {
	if err := os.MkdirAll(pendingDir, atomicDirFileMode); err != nil {
		return "", err
	}
	prefix := now.UTC().Format("20060102T150405.000000000Z") + "-scan-"
	file, err := os.CreateTemp(pendingDir, prefix+"*.json")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func stateUpdatesForPendingRecords(records []Record, fileUpdates map[string]FileState, now time.Time) map[string]FileState {
	updates := map[string]FileState{}
	for _, record := range records {
		base, ok := fileUpdates[record.SourceFile]
		if !ok {
			continue
		}
		if existing, ok := updates[record.SourceFile]; ok && existing.ProcessedBytes >= record.ByteEnd {
			continue
		}
		update := base
		update.ProcessedBytes = record.ByteEnd
		update.ProcessedLines = record.LineEnd
		update.LastScannedAt = now.UTC().Format(time.RFC3339)
		if boundary, err := boundaryHash(record.SourceFile, record.ByteEnd); err == nil {
			update.BoundaryHash = boundary
		}
		updates[record.SourceFile] = update
	}
	return updates
}

func baseStateForUpdates(baseState map[string]FileState, stateUpdates map[string]FileState) map[string]FileState {
	filtered := map[string]FileState{}
	for path := range stateUpdates {
		filtered[path] = baseState[path]
	}
	return filtered
}

func fileStateMatchesBase(current, base FileState) bool {
	if base.FileID == "" && base.ProcessedBytes == 0 && base.ProcessedLines == 0 && base.BoundaryHash == "" {
		return current.FileID == "" && current.ProcessedBytes == 0 && current.ProcessedLines == 0 && current.BoundaryHash == ""
	}
	return current.FileID == base.FileID &&
		current.ProcessedBytes == base.ProcessedBytes &&
		current.ProcessedLines == base.ProcessedLines &&
		current.BoundaryHash == base.BoundaryHash
}

func summarizeRecords(records []Record) ScanRecordSummary {
	summary := ScanRecordSummary{Total: len(records), ByAgent: map[string]int{}}
	for _, record := range records {
		summary.ByAgent[record.Agent]++
	}
	return summary
}

func fileID(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", uint64(stat.Dev), uint64(stat.Ino))
	}
	return fmt.Sprintf("%s:%d", info.Name(), info.ModTime().UnixNano())
}

func processedBoundaryMatches(path string, previous FileState) (bool, error) {
	if previous.BoundaryHash == "" {
		return true, nil
	}
	current, err := boundaryHash(path, previous.ProcessedBytes)
	if err != nil {
		return false, err
	}
	return current == previous.BoundaryHash, nil
}

func mustBoundaryHash(path string, byteEnd int64) string {
	hash, err := boundaryHash(path, byteEnd)
	if err != nil {
		return ""
	}
	return hash
}

func boundaryHash(path string, byteEnd int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if byteEnd > info.Size() {
		return "", nil
	}
	const boundarySize int64 = 4096
	start := int64(0)
	if byteEnd > boundarySize {
		start = byteEnd - boundarySize
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.CopyN(hash, file, byteEnd-start); err != nil && err != io.EOF {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func tailHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	const tailSize int64 = 4096
	start := int64(0)
	if info.Size() > tailSize {
		start = info.Size() - tailSize
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	buffer := make([]byte, 32*1024)
	lines := 0
	var lastByte byte
	readAny := false
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			readAny = true
			chunk := buffer[:n]
			for _, b := range chunk {
				if b == '\n' {
					lines++
				}
			}
			lastByte = chunk[n-1]
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	if readAny && lastByte != '\n' {
		lines++
	}
	return lines, nil
}
