package txn

import (
	"context"
	"time"
)

// BeginWindow is Begin with a longer confirm window for one transaction. A change that moves
// the caller's own address (ADR 0028) needs time to reconnect and log in again before the
// confirm can arrive. window shorter than the manager's Window is refused (ErrBadWindow): a
// caller may only extend the user's time, never shorten it.
func (m *Manager) BeginWindow(ctx context.Context, summary string, configs []string, window time.Duration, stage func(context.Context) error, probe Probe) (Status, error) {
	return m.begin(ctx, summary, configs, false, window, stage, probe)
}
