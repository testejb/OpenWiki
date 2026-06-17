package candidate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func appendRunLog(path, event string, now time.Time, fields map[string]any) error {
	if path == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	entry := map[string]any{
		"event": event,
		"time":  now.UTC().Format(time.RFC3339),
	}
	for key, value := range fields {
		entry[key] = value
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), atomicDirFileMode); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, atomicFileMode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}
