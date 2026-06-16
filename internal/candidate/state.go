package candidate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	stateVersion      = 1
	codeAgentSource   = "codeagent"
	pendingStatus     = "pending"
	committedStatus   = "committed"
	atomicFileMode    = 0o644
	atomicDirFileMode = 0o755
)

func LoadState(path string) (State, error) {
	state := defaultState()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return State{}, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	if state.Version == 0 {
		state.Version = stateVersion
	}
	if state.Source == "" {
		state.Source = codeAgentSource
	}
	if state.Files == nil {
		state.Files = map[string]FileState{}
	}
	return state, nil
}

func SaveStateAtomic(path string, state State) error {
	if state.Version == 0 {
		state.Version = stateVersion
	}
	if state.Source == "" {
		state.Source = codeAgentSource
	}
	if state.Files == nil {
		state.Files = map[string]FileState{}
	}
	return writeJSONAtomic(path, state)
}

func LoadPending(path string) (Pending, error) {
	var pending Pending
	data, err := os.ReadFile(path)
	if err != nil {
		return Pending{}, err
	}
	if err := json.Unmarshal(data, &pending); err != nil {
		return Pending{}, err
	}
	if pending.Version == 0 {
		pending.Version = stateVersion
	}
	if pending.Source == "" {
		pending.Source = codeAgentSource
	}
	if pending.StateUpdates == nil {
		pending.StateUpdates = map[string]FileState{}
	}
	if pending.BaseState == nil {
		pending.BaseState = map[string]FileState{}
	}
	return pending, nil
}

func SavePendingAtomic(path string, pending Pending) error {
	if pending.Version == 0 {
		pending.Version = stateVersion
	}
	if pending.Source == "" {
		pending.Source = codeAgentSource
	}
	if pending.StateUpdates == nil {
		pending.StateUpdates = map[string]FileState{}
	}
	if pending.BaseState == nil {
		pending.BaseState = map[string]FileState{}
	}
	return writeJSONAtomic(path, pending)
}

func defaultState() State {
	return State{
		Version: stateVersion,
		Source:  codeAgentSource,
		Files:   map[string]FileState{},
	}
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), atomicDirFileMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(atomicFileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}
