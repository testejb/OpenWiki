package candidate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const maxJSONLParseWarnings = 20

// ParseJSONLFile parses candidate records from a code agent JSONL file.
func ParseJSONLFile(agent AgentConfig, path string, startByte int64, startLine int) ([]Record, []Warning, error) {
	if agent.Type != "traex-history" && agent.Type != "trae-ide-memory" {
		return nil, []Warning{{
			Code:    "UNSUPPORTED_AGENT_TYPE",
			Message: fmt.Sprintf("unsupported agent type: %s", agent.Type),
			Path:    path,
		}}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	if _, err := file.Seek(startByte, 0); err != nil {
		return nil, nil, err
	}

	reader := bufio.NewReader(file)

	var records []Record
	var warnings []Warning
	lineNo := startLine
	byteStart := startByte
	parseWarningCount := 0
	parseWarningsTruncated := false

	for {
		rawLine, err := reader.ReadBytes('\n')
		if len(rawLine) == 0 {
			if err == io.EOF {
				break
			}
			if err != nil {
				return records, warnings, err
			}
		}

		lineNo++
		byteEnd := byteStart + int64(len(rawLine))
		line := trimJSONLLineEnding(rawLine)

		record, ok, warning := parseJSONLLine(agent, path, line, lineNo, byteStart, byteEnd)
		if warning != nil {
			if warning.Code == "JSONL_PARSE_FAILED" {
				parseWarningCount++
				if parseWarningCount <= maxJSONLParseWarnings {
					warnings = append(warnings, *warning)
				} else {
					parseWarningsTruncated = true
				}
			} else {
				warnings = append(warnings, *warning)
			}
		}
		if ok {
			records = append(records, record)
		}

		byteStart = byteEnd
		if err == io.EOF {
			break
		}
		if err != nil {
			return records, warnings, err
		}
	}
	if parseWarningsTruncated {
		warnings = append(warnings, Warning{
			Code:    "JSONL_PARSE_WARNINGS_TRUNCATED",
			Message: fmt.Sprintf("suppressed %d additional JSONL parse warnings", parseWarningCount-maxJSONLParseWarnings),
			Path:    path,
		})
	}

	return records, warnings, nil
}

func trimJSONLLineEnding(line []byte) []byte {
	line = bytesTrimSuffix(line, '\n')
	line = bytesTrimSuffix(line, '\r')
	return line
}

func bytesTrimSuffix(line []byte, suffix byte) []byte {
	if len(line) > 0 && line[len(line)-1] == suffix {
		return line[:len(line)-1]
	}
	return line
}

func parseJSONLLine(agent AgentConfig, path string, line []byte, lineNo int, byteStart, byteEnd int64) (Record, bool, *Warning) {
	switch agent.Type {
	case "traex-history":
		return parseTraexHistoryLine(agent, path, line, lineNo, byteStart, byteEnd)
	case "trae-ide-memory":
		return parseTraeIDEMemoryLine(agent, path, line, lineNo, byteStart, byteEnd)
	default:
		return Record{}, false, &Warning{
			Code:    "UNSUPPORTED_AGENT_TYPE",
			Message: fmt.Sprintf("unsupported agent type: %s", agent.Type),
			Path:    path,
			Line:    lineNo,
		}
	}
}

func parseTraexHistoryLine(agent AgentConfig, path string, line []byte, lineNo int, byteStart, byteEnd int64) (Record, bool, *Warning) {
	var payload struct {
		SessionID string `json:"session_id"`
		Ts        int64  `json:"ts"`
		Text      string `json:"text"`
	}
	if err := json.Unmarshal(line, &payload); err != nil {
		return Record{}, false, jsonlParseWarning(path, lineNo, err)
	}

	text := strings.TrimSpace(payload.Text)
	if text == "" {
		return Record{}, false, nil
	}

	return baseRecord(agent, path, lineNo, byteStart, byteEnd, text, func(record *Record) {
		record.SessionID = payload.SessionID
		if payload.Ts != 0 {
			record.Timestamp = time.Unix(payload.Ts, 0).UTC().Format(time.RFC3339)
		}
	}), true, nil
}

func parseTraeIDEMemoryLine(agent AgentConfig, path string, line []byte, lineNo int, byteStart, byteEnd int64) (Record, bool, *Warning) {
	var payload struct {
		Intent             string   `json:"intent"`
		Actions            []string `json:"actions"`
		Outcome            string   `json:"outcome"`
		Learned            []string `json:"learned"`
		MessageSummaryTime string   `json:"message_summary_time"`
		MessageID          string   `json:"message_id"`
	}
	if err := json.Unmarshal(line, &payload); err != nil {
		return Record{}, false, jsonlParseWarning(path, lineNo, err)
	}

	text := buildMemorySummary(payload.Intent, payload.Actions, payload.Outcome, payload.Learned)
	if strings.TrimSpace(text) == "" {
		return Record{}, false, nil
	}

	return baseRecord(agent, path, lineNo, byteStart, byteEnd, text, func(record *Record) {
		record.MessageID = payload.MessageID
		record.Intent = payload.Intent
		record.Actions = payload.Actions
		record.Outcome = payload.Outcome
		record.Learned = payload.Learned
		record.Timestamp = parseMemoryTimestamp(payload.MessageSummaryTime)
	}), true, nil
}

func baseRecord(agent AgentConfig, path string, lineNo int, byteStart, byteEnd int64, text string, apply func(*Record)) Record {
	record := Record{
		RecordID:   fmt.Sprintf("%s:%s:line:%d", agent.Name, path, lineNo),
		Agent:      agent.Name,
		Type:       agent.Type,
		SourceFile: path,
		LineStart:  lineNo,
		LineEnd:    lineNo,
		ByteStart:  byteStart,
		ByteEnd:    byteEnd,
		Text:       text,
	}
	apply(&record)
	return record
}

func jsonlParseWarning(path string, lineNo int, err error) *Warning {
	return &Warning{
		Code:    "JSONL_PARSE_FAILED",
		Path:    path,
		Line:    lineNo,
		Message: err.Error(),
	}
}

func buildMemorySummary(intent string, actions []string, outcome string, learned []string) string {
	var parts []string
	if strings.TrimSpace(intent) != "" {
		parts = append(parts, "Intent: "+strings.TrimSpace(intent))
	}
	if len(actions) > 0 {
		parts = append(parts, "Actions: "+strings.Join(actions, "; "))
	}
	if strings.TrimSpace(outcome) != "" {
		parts = append(parts, "Outcome: "+strings.TrimSpace(outcome))
	}
	if len(learned) > 0 {
		parts = append(parts, "Learned: "+strings.Join(learned, "; "))
	}
	return strings.Join(parts, "\n")
}

func parseMemoryTimestamp(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
	} {
		parsed, err := time.ParseInLocation(layout, trimmed, time.Local)
		if err == nil {
			return parsed.Format(time.RFC3339)
		}
	}

	return value
}
