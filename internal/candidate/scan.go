package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	resetFiles := map[string]bool{}

	if cfg.Enabled {
		for _, agent := range cfg.Agents {
			if !agent.Enabled {
				continue
			}
			paths, pathWarnings := expandAgentPaths(agent)
			warnings = append(warnings, pathWarnings...)
			for _, path := range paths {
				previous := state.Files[path]
				fileRecords, update, reset, fileWarnings, err := scanOneFile(agent, path, previous, now)
				warnings = append(warnings, fileWarnings...)
				if err != nil {
					return ScanResult{}, err
				}
				if update != nil {
					fileUpdates[path] = *update
					baseState[path] = previous
				}
				if reset {
					resetFiles[path] = true
				}
				records = append(records, fileRecords...)
			}
		}
	}

	var firstRunWarnings []Warning
	records, firstRunWarnings = filterFirstRun(records, state, initialDays, now, fileUpdates, resetFiles)
	warnings = append(warnings, firstRunWarnings...)
	baseBacklogHash := hashRecords(state.Backlog)
	var backlogUpdate []Record
	backlogUpdateSet := false
	if cfg.Enabled {
		records = mergeRecords(state.Backlog, records)
		records, backlogUpdate = keepLatestRecords(records, maxRecords)
		backlogUpdateSet = true
	}
	stateUpdates := appendAllFileUpdates(fileUpdates)
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
		Records:          records,
		StateUpdates:     stateUpdates,
		BaseState:        baseState,
		BaseBacklogHash:  baseBacklogHash,
		BacklogUpdateSet: backlogUpdateSet,
		BacklogUpdate:    backlogUpdate,
		Warnings:         warnings,
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
	if pending.BacklogUpdateSet && pending.BaseBacklogHash != "" && hashRecords(state.Backlog) != pending.BaseBacklogHash {
		return CommitResult{}, fmt.Errorf("stale pending %s: backlog changed since scan", pendingPath)
	}
	for path, update := range pending.StateUpdates {
		state.Files[path] = update
	}
	if pending.BacklogUpdateSet {
		state.Backlog = append([]Record(nil), pending.BacklogUpdate...)
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

func scanOneFile(agent AgentConfig, path string, previous FileState, now time.Time) ([]Record, *FileState, bool, []Warning, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, false, nil, err
	}
	currentFileID := fileID(info)
	currentTailHash, err := tailHash(path)
	if err != nil {
		return nil, nil, false, nil, err
	}

	startByte := int64(0)
	startLine := 0
	reset := false
	var warnings []Warning
	if previous.FileID != "" {
		boundaryTrusted, err := processedBoundaryMatches(path, previous)
		if err != nil {
			return nil, nil, false, nil, err
		}
		switch {
		case previous.FileID == currentFileID && info.Size() == previous.ProcessedBytes && previous.TailHash == currentTailHash && boundaryTrusted:
			return nil, nil, false, nil, nil
		case previous.FileID == currentFileID && info.Size() > previous.ProcessedBytes && boundaryTrusted:
			startByte = previous.ProcessedBytes
			startLine = previous.ProcessedLines
		default:
			reset = true
			warnings = append(warnings, Warning{Code: "SOURCE_FILE_RESET", Message: "source file was truncated, replaced, or rewritten; scanning from start", Path: path})
		}
	}

	records, parseWarnings, err := ParseJSONLFile(agent, path, startByte, startLine)
	warnings = append(warnings, parseWarnings...)
	if err != nil {
		return nil, nil, reset, warnings, err
	}
	lineCount, err := countLines(path)
	if err != nil {
		return nil, nil, reset, warnings, err
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
	return records, &update, reset, warnings, nil
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
		matches, err := expandPathPattern(pattern)
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

func expandPathPattern(pattern string) ([]string, error) {
	if strings.Contains(pattern, "**") {
		return doublestarGlob(pattern)
	}
	return filepath.Glob(pattern)
}

func doublestarGlob(pattern string) ([]string, error) {
	cleanPattern := filepath.Clean(pattern)
	parts := strings.Split(cleanPattern, string(os.PathSeparator))
	starIndex := -1
	for i, part := range parts {
		if part == "**" {
			starIndex = i
			break
		}
	}
	if starIndex < 0 {
		return filepath.Glob(pattern)
	}
	if hasDoubleStarInsideSegment(parts) {
		return nil, fmt.Errorf("unsupported doublestar pattern: %s", pattern)
	}

	rootParts := parts[:starIndex]
	root := strings.Join(rootParts, string(os.PathSeparator))
	if filepath.IsAbs(cleanPattern) && root == "" {
		root = string(os.PathSeparator)
	}
	if root == "" {
		root = "."
	}
	suffix := filepath.Join(parts[starIndex+1:]...)
	if suffix == "" {
		suffix = "*"
	}

	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		candidate := relative
		if !strings.Contains(suffix, string(os.PathSeparator)) {
			candidate = filepath.Base(path)
		}
		matched, err := filepath.Match(suffix, candidate)
		if err != nil {
			return err
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

func hasDoubleStarInsideSegment(parts []string) bool {
	for _, part := range parts {
		if strings.Contains(part, "**") && part != "**" {
			return true
		}
	}
	return false
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func filterFirstRun(records []Record, state State, initialDays int, now time.Time, fileUpdates map[string]FileState, resetFiles map[string]bool) ([]Record, []Warning) {
	if initialDays <= 0 {
		return records, nil
	}
	cutoff := now.AddDate(0, 0, -initialDays)
	filtered := records[:0]
	var warnings []Warning
	warnedTimestamplessOldFiles := map[string]bool{}
	for _, record := range records {
		if _, known := state.Files[record.SourceFile]; known && !resetFiles[record.SourceFile] {
			filtered = append(filtered, record)
			continue
		}
		if record.Timestamp == "" {
			if fileMTimeWithinInitialWindow(record.SourceFile, fileUpdates, cutoff) {
				filtered = append(filtered, record)
				continue
			}
			if !warnedTimestamplessOldFiles[record.SourceFile] {
				warnings = append(warnings, Warning{
					Code:    "TIMESTAMPLESS_OLD_FILE_SKIPPED",
					Message: "timestampless records in old source file were skipped on first run",
					Path:    record.SourceFile,
				})
				warnedTimestamplessOldFiles[record.SourceFile] = true
			}
			continue
		}
		parsed, err := time.Parse(time.RFC3339, record.Timestamp)
		if err != nil || !parsed.Before(cutoff) {
			filtered = append(filtered, record)
		}
	}
	return filtered, warnings
}

func fileMTimeWithinInitialWindow(path string, fileUpdates map[string]FileState, cutoff time.Time) bool {
	if update, ok := fileUpdates[path]; ok && update.MTime != "" {
		if parsed, err := time.Parse(time.RFC3339, update.MTime); err == nil {
			return !parsed.Before(cutoff)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return !info.ModTime().Before(cutoff)
}

func keepLatestRecords(records []Record, maxRecords int) ([]Record, []Record) {
	ordered := append([]Record(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return recordLess(ordered[i], ordered[j])
	})
	if maxRecords <= 0 || len(ordered) <= maxRecords {
		return ordered, nil
	}
	cutoff := len(ordered) - maxRecords
	return append([]Record(nil), ordered[cutoff:]...), append([]Record(nil), ordered[:cutoff]...)
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

func mergeRecords(backlog, records []Record) []Record {
	if len(backlog) == 0 {
		return append([]Record(nil), records...)
	}
	merged := make([]Record, 0, len(backlog)+len(records))
	seen := map[string]bool{}
	for _, record := range backlog {
		key := recordKey(record)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, record)
	}
	for _, record := range records {
		key := recordKey(record)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, record)
	}
	return merged
}

func appendAllFileUpdates(fileUpdates map[string]FileState) map[string]FileState {
	updates := map[string]FileState{}
	for path, update := range fileUpdates {
		updates[path] = update
	}
	return updates
}

func hashRecords(records []Record) string {
	if len(records) == 0 {
		return "empty"
	}
	ordered := append([]Record(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return recordLess(ordered[i], ordered[j])
	})
	hash := sha256.New()
	encoder := json.NewEncoder(hash)
	for _, record := range ordered {
		_ = encoder.Encode(record)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func recordKey(record Record) string {
	return fmt.Sprintf("%s\x00%d\x00%d\x00%s", record.SourceFile, record.ByteStart, record.ByteEnd, record.RecordID)
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
