// Package system reads what the router is (model, firmware, uptime, memory, flash).
package system

import (
	"context"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

// Read calls `ubus system board` and `ubus system info` and parses both. Any failure is
// returned naming the call; there is never a partial result.
func Read(ctx context.Context, u device.Ubus) (facts.Board, facts.SystemInfo, error) {
	rawBoard, err := u.Call(ctx, "system", "board", nil)
	if err != nil {
		return facts.Board{}, facts.SystemInfo{}, fmt.Errorf("ubus system board: %w", err)
	}
	board, err := facts.ParseBoard(rawBoard)
	if err != nil {
		return facts.Board{}, facts.SystemInfo{}, fmt.Errorf("ubus system board: %w", err)
	}
	rawInfo, err := u.Call(ctx, "system", "info", nil)
	if err != nil {
		return facts.Board{}, facts.SystemInfo{}, fmt.Errorf("ubus system info: %w", err)
	}
	info, err := facts.ParseSystemInfo(rawInfo)
	if err != nil {
		return facts.Board{}, facts.SystemInfo{}, fmt.Errorf("ubus system info: %w", err)
	}
	return board, info, nil
}
