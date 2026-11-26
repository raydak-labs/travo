package mock

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

//go:embed testdata/net-counters.txt
var capturedCounters string

func (*Device) ReadInterfaceCounters(ctx context.Context) (map[string]device.InterfaceCounters, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Keep the last captured counter for each interface. The mock has no live traffic.
	counters := make(map[string]device.InterfaceCounters)
	for line := range strings.SplitSeq(strings.TrimSpace(capturedCounters), "\n") {
		var name string
		var seconds int64
		var value device.InterfaceCounters
		if _, err := fmt.Sscan(line, &seconds, &name, &value.RXBytes, &value.TXBytes); err != nil {
			return nil, err
		}
		counters[name] = value
	}
	return counters, nil
}
