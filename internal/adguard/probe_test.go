package adguard

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestBuildQueryWireFormat(t *testing.T) {
	q, err := BuildQuery(0xbeef, "travo-ab.example.com")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xbe, 0xef, 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0,
		8, 't', 'r', 'a', 'v', 'o', '-', 'a', 'b', 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0,
		0, 1, 0, 1}
	if !bytes.Equal(q, want) {
		t.Fatalf("% x", q)
	}
	if _, err := BuildQuery(1, "a..b"); err == nil {
		t.Fatal("empty label accepted")
	}
}

func TestReplyClassification(t *testing.T) {
	reply := func(id uint16, flags2, flags3 byte) []byte {
		return []byte{byte(id >> 8), byte(id), flags2, flags3, 0, 1, 0, 0, 0, 0, 0, 0}
	}
	for _, tc := range []struct {
		name string
		b    []byte
		ok   bool
	}{
		{"noerror", reply(7, 0x81, 0x80), true},
		{"nxdomain", reply(7, 0x81, 0x83), true},
		{"servfail", reply(7, 0x81, 0x82), false},
		{"refused", reply(7, 0x81, 0x85), false},
		{"wrong id", reply(8, 0x81, 0x80), false},
		{"query not reply", reply(7, 0x01, 0x00), false},
		{"short", []byte{0, 7}, false},
	} {
		code, err := ParseReplyCode(7, tc.b)
		if err == nil {
			err = Alive(code)
		}
		if (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrProbe)) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// fakeDNS answers every query with rcode, echoing the id.
func fakeDNS(t *testing.T, rcode byte) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			out := append([]byte(nil), buf[:n]...)
			out[2] |= 0x80
			out[3] = 0x80 | rcode
			_, _ = pc.WriteTo(out, addr)
		}
	}()
	return pc.LocalAddr().String()
}

func TestUDPProbeAgainstLocalServer(t *testing.T) {
	if err := UDPProbe(fakeDNS(t, 3), time.Second)(context.Background()); err != nil {
		t.Fatal("NXDOMAIN for a fresh name is alive:", err)
	}
	if err := UDPProbe(fakeDNS(t, 5), time.Second)(context.Background()); !errors.Is(err, ErrProbe) {
		t.Fatal("REFUSED is a failure:", err)
	}
	// nothing listening: AGH stopped
	pc, _ := net.ListenPacket("udp", "127.0.0.1:0")
	addr := pc.LocalAddr().String()
	pc.Close()
	if err := UDPProbe(addr, 200*time.Millisecond)(context.Background()); !errors.Is(err, ErrProbe) {
		t.Fatal("no listener is a failure:", err)
	}
}
