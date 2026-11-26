package adguard

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// ProbeAddr is where dnsmasq forwards (the `adguard` layer's server).
const ProbeAddr = LoopbackDNS + ":" + DNSPort

// ProbeZone parents the fresh names. Each probe asks a random label below it, so no cache
// (AGH's or upstream's) can answer and the reply proves AGH reached its upstream (invariants §8).
const ProbeZone = "example.com"

// DNS reply codes the probe distinguishes.
const (
	rcodeNoError  = 0
	rcodeNXDomain = 3
)

// ErrProbe is a failed liveness probe: no answer, or an answer that is not a resolution.
var ErrProbe = errors.New("AdGuard Home did not resolve the probe name")

// BuildQuery is a minimal RFC 1035 query: one question, type A, class IN, recursion desired.
func BuildQuery(id uint16, name string) ([]byte, error) {
	b := make([]byte, 12, 64)
	binary.BigEndian.PutUint16(b[0:], id)
	b[2] = 0x01                          // RD
	binary.BigEndian.PutUint16(b[4:], 1) // QDCOUNT
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if label == "" || len(label) > 63 {
			return nil, fmt.Errorf("bad DNS label in %q", name)
		}
		b = append(b, byte(len(label)))
		b = append(b, label...)
	}
	return append(b, 0, 0, 1, 0, 1), nil // root, QTYPE A, QCLASS IN
}

// ParseReplyCode checks the header of a reply to query id and returns its RCODE.
func ParseReplyCode(id uint16, b []byte) (int, error) {
	if len(b) < 12 {
		return 0, fmt.Errorf("%w: short reply (%d bytes)", ErrProbe, len(b))
	}
	if binary.BigEndian.Uint16(b[0:]) != id {
		return 0, fmt.Errorf("%w: reply id mismatch", ErrProbe)
	}
	if b[2]&0x80 == 0 {
		return 0, fmt.Errorf("%w: not a reply", ErrProbe)
	}
	return int(b[3] & 0x0f), nil
}

// Alive classifies an RCODE: NOERROR and NXDOMAIN are resolutions; SERVFAIL (upstream
// unreachable) and REFUSED are failures.
func Alive(rcode int) error {
	if rcode == rcodeNoError || rcode == rcodeNXDomain {
		return nil
	}
	return fmt.Errorf("%w: rcode %d", ErrProbe, rcode)
}

// UDPProbe asks addr for a fresh random name under ProbeZone and waits up to timeout.
func UDPProbe(addr string, timeout time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		var r [10]byte
		if _, err := rand.Read(r[:]); err != nil {
			return err
		}
		id := binary.BigEndian.Uint16(r[:2])
		q, err := BuildQuery(id, "travo-"+hex.EncodeToString(r[2:])+"."+ProbeZone)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		var d net.Dialer
		conn, err := d.DialContext(ctx, "udp", addr)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrProbe, err)
		}
		defer conn.Close()
		if dl, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(dl)
		}
		if _, err := conn.Write(q); err != nil {
			return fmt.Errorf("%w: %v", ErrProbe, err)
		}
		buf := make([]byte, 1500)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrProbe, err)
			}
			code, err := ParseReplyCode(id, buf[:n])
			if err != nil {
				continue // a stray datagram; keep waiting for ours until the deadline
			}
			return Alive(code)
		}
	}
}
