package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func (*Device) ReadInterfaceCounters(ctx context.Context) (map[string]device.InterfaceCounters, error) {
	return readInterfaceCounters(ctx, "/sys/class/net")
}

func readInterfaceCounters(ctx context.Context, root string) (map[string]device.InterfaceCounters, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string]device.InterfaceCounters, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := entry.Name()
		path := filepath.Join(root, name)
		counters, err := readCounters(path)
		if errors.Is(err, os.ErrNotExist) {
			if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
				continue
			}
		}
		if err != nil {
			return nil, fmt.Errorf("interface counters for %s: %w", name, err)
		}
		snapshot[name] = counters
	}
	return snapshot, ctx.Err()
}

func readCounters(path string) (device.InterfaceCounters, error) {
	rx, err := readCounter(filepath.Join(path, "statistics", "rx_bytes"))
	if err != nil {
		return device.InterfaceCounters{}, err
	}
	tx, err := readCounter(filepath.Join(path, "statistics", "tx_bytes"))
	return device.InterfaceCounters{RXBytes: rx, TXBytes: tx}, err
}

func readCounter(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}
