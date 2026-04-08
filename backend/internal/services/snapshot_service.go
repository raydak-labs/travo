package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// ConfigSnapshot represents a saved configuration snapshot.
type ConfigSnapshot struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	Description string            `json:"description"`
	User        string            `json:"user"`
	Configs     map[string]string `json:"configs"` // config name -> content
}

// SnapshotService manages configuration snapshots for safety and recovery.
type SnapshotService struct {
	snapshotDir string
	uci         uci.UCI
}

// NewSnapshotService creates a new snapshot service.
func NewSnapshotService(snapshotDir string, u uci.UCI) *SnapshotService {
	return &SnapshotService{
		snapshotDir: snapshotDir,
		uci:         u,
	}
}

// CreateSnapshot captures the current state of specified UCI configurations.
func (s *SnapshotService) CreateSnapshot(description string, configNames []string, user string) (*ConfigSnapshot, error) {
	if err := os.MkdirAll(s.snapshotDir, 0750); err != nil {
		return nil, fmt.Errorf("create snapshot directory: %w", err)
	}

	snapshot := &ConfigSnapshot{
		ID:          fmt.Sprintf("snap-%d", time.Now().Unix()),
		Timestamp:   time.Now(),
		Description: description,
		User:        user,
		Configs:     make(map[string]string),
	}

	for _, configName := range configNames {
		content, err := s.readConfigFile(configName)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", configName, err)
		}
		snapshot.Configs[configName] = content
	}

	if err := s.saveSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}

	return snapshot, nil
}

// ListSnapshots returns all available snapshots sorted by timestamp (newest first).
func (s *SnapshotService) ListSnapshots() ([]*ConfigSnapshot, error) {
	entries, err := os.ReadDir(s.snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ConfigSnapshot{}, nil
		}
		return nil, err
	}

	var snapshots []*ConfigSnapshot
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		snapshot, err := s.loadSnapshot(entry.Name())
		if err != nil {
			continue // skip corrupted snapshots
		}
		snapshots = append(snapshots, snapshot)
	}

	// Sort by timestamp (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	return snapshots, nil
}

// RestoreSnapshot restores a configuration snapshot.
func (s *SnapshotService) RestoreSnapshot(snapshotID string) error {
	snapshot, err := s.loadSnapshot(snapshotID + ".json")
	if err != nil {
		return fmt.Errorf("load snapshot: %w", err)
	}

	for configName, content := range snapshot.Configs {
		if err := s.writeConfigFile(configName, content); err != nil {
			return fmt.Errorf("restore config %s: %w", configName, err)
		}
	}

	return nil
}

// DeleteSnapshot removes a snapshot.
func (s *SnapshotService) DeleteSnapshot(snapshotID string) error {
	path := filepath.Join(s.snapshotDir, snapshotID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetSnapshot returns a specific snapshot by ID.
func (s *SnapshotService) GetSnapshot(snapshotID string) (*ConfigSnapshot, error) {
	return s.loadSnapshot(snapshotID + ".json")
}

func (s *SnapshotService) readConfigFile(configName string) (string, error) {
	path := filepath.Join("/etc/config", configName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SnapshotService) writeConfigFile(configName, content string) error {
	path := filepath.Join("/etc/config", configName)
	return os.WriteFile(path, []byte(content), 0600)
}

func (s *SnapshotService) saveSnapshot(snapshot *ConfigSnapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.snapshotDir, snapshot.ID+".json")
	return os.WriteFile(path, data, 0600)
}

func (s *SnapshotService) loadSnapshot(filename string) (*ConfigSnapshot, error) {
	path := filepath.Join(s.snapshotDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var snapshot ConfigSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// AutoSaveBeforeChange creates an automatic snapshot before configuration changes.
func (s *SnapshotService) AutoSaveBeforeChange(description string, configNames []string) (*ConfigSnapshot, error) {
	return s.CreateSnapshot(fmt.Sprintf("[AUTO] %s", description), configNames, "system")
}

// CleanOldSnapshots removes snapshots older than the specified duration, keeping at least minSnapshots.
func (s *SnapshotService) CleanOldSnapshots(olderThan time.Duration, minSnapshots int) error {
	snapshots, err := s.ListSnapshots()
	if err != nil {
		return err
	}

	if len(snapshots) <= minSnapshots {
		return nil
	}

	cutoff := time.Now().Add(-olderThan)
	toDelete := 0
	for i, snapshot := range snapshots {
		if i >= minSnapshots && snapshot.Timestamp.Before(cutoff) {
			toDelete++
		}
	}

	for i := minSnapshots; i < len(snapshots) && i < minSnapshots+toDelete; i++ {
		snapshot := snapshots[i]
		if snapshot.Timestamp.Before(cutoff) {
			if err := s.DeleteSnapshot(snapshot.ID); err != nil {
				return err
			}
		}
	}

	return nil
}
