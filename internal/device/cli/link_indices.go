package cli

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

var errLinkIndices = errors.New("cannot read kernel link indices")

func (d *Device) LinkIndices(ctx context.Context) (map[string]int, error) {
	out, err := d.run(ctx, TierRead, "ip", "-o", "link", "show")
	if err != nil {
		return nil, errLinkIndices
	}
	return parseLinkIndices(out)
}

func parseLinkIndices(out string) (map[string]int, error) {
	byName := make(map[string]int)
	indices := make(map[int]struct{})
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		rawIndex, tail, hasIndex := strings.Cut(line, ": ")
		displayName, _, hasName := strings.Cut(tail, ":")
		name, peer, decorated := strings.Cut(displayName, "@")
		index, err := strconv.Atoi(rawIndex)
		if !hasIndex || !hasName || err != nil || index <= 0 ||
			name == "" || (decorated && peer == "") || strings.ContainsAny(displayName, " \t\r") {
			return nil, errLinkIndices
		}
		if _, duplicate := indices[index]; duplicate || byName[name] != 0 {
			return nil, errLinkIndices
		}
		indices[index] = struct{}{}
		byName[name] = index
	}
	return byName, nil
}
