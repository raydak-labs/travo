package sshkeys

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

// Throwaway public keys from `ssh-keygen -t <type>`; fingerprints from `ssh-keygen -lf -E sha256`.
const (
	ed25519 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKiO5ZRy/vDVzgKRHcpeswVz7ybMJPfsbY4JBskk4YXL test-ed25519"
	ecdsa   = "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBK8VP6+X8Tz20sb/tVQGu80Sdb36E5DaqW7l9tvP+MAz8xzdDWJ8C4pLutg57E+rRC5N7dtI81XLco8rcF9EBks= test-ecdsa"
	rsa     = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6hOjriT/5lLmsKaFG7hmxy/bpipA1Zx5Ynx6h6uC9pB5Nx4GaN2JruC5UN0x3kp6MfNcCkHUuyvJrNnQYaN3j1sh0yAsSwbQHIAYD3S7Ho0KfOH7H+hVN5jzxGWbHvtQTbYityirPyX4Fsn/I+gttgSOW+T/Ukg5kRwxGxil2UbTJFrrc6F2DPNqrz2/WPVb0NQ9xulyG3zHoQOOBA0tD77qNaAwS2SRHBZTvtq3SpzhuVT/a6KkvX4Q+2uTTaAEwoyVGLGXoyAuRaqLqXAeNPrOOLT7nc4oQPTkI1L9GaIE8O/ajC/iKiZtjrusnqFW/DDddOLqxz1I3xozyjYDIkUpHFI3wEQmps+MgN2M15MTbSzIRxcDpOq9SPfjzojal9mgjqsLoXuRWBnGfRAkO6kUj/hema5aNEkMgCqKYnUgyIyhKNDFvneOZpYMkxlt6sH3YYyAZZ7DVntb9/b9XA6PFaMFBbj8fHPf3CyVi5Ddm74raHRDjAOQTJRnc5IE= test-rsa"
)

func TestParseAcceptsRealKeysWithOpenSSHFingerprints(t *testing.T) {
	for line, fp := range map[string]string{
		ed25519: "SHA256:wDQ6rSZgjr4cNqg9V83faMCsq8u5Q3JFikiOxv5Rw84",
		ecdsa:   "SHA256:YVgFL0Bcn4mZa9IXqGu+IKMyQ7zEur/l+zmy3CqC+QI",
		rsa:     "SHA256:/mIKH+YxiQUJAXY+HqkwsyxL5aJ+3a/VjUTf17gPPVc",
	} {
		k, err := Parse(line)
		if err != nil {
			t.Fatalf("%s: %v", line[:20], err)
		}
		if k.Fingerprint != fp || len(k.ID) != 64 || !strings.HasPrefix(k.Comment, "test-") {
			t.Fatalf("%+v", k)
		}
	}
}

func TestParseRefuses(t *testing.T) {
	blob := strings.Fields(ed25519)[1]
	for name, line := range map[string]string{
		"empty":           "",
		"type only":       "ssh-ed25519",
		"unknown type":    "ssh-dss " + blob,
		"options":         `command="/bin/sh" ` + ed25519,
		"from option":     `from="10.0.0.1" ` + ed25519,
		"newline":         ed25519 + "\nssh-ed25519 " + blob,
		"carriage return": ed25519 + "\r",
		"nul":             ed25519 + "\x00",
		"bad base64":      "ssh-ed25519 !!!",
		"type mismatch":   "ssh-rsa " + blob,
		"control in note": "ssh-ed25519 " + blob + " a\x1bb",
		"too long":        "ssh-ed25519 " + blob + " " + strings.Repeat("x", maxLine),
		"truncated blob":  "ssh-ed25519 AAAA",
	} {
		if _, err := Parse(line); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

func service(content string) (*Service, *mock.Device) {
	d := mock.New()
	if content != "" {
		_ = d.WriteAuthorizedKeys(context.Background(), []byte(content))
	}
	return &Service{File: d}, d
}

func file(t *testing.T, d *mock.Device) string {
	t.Helper()
	b, err := d.ReadAuthorizedKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAddAppendsOneLineAndKeepsForeignLines(t *testing.T) {
	foreign := `# managed by hand` + "\n" + `command="uptime" ` + rsa + "\n"
	s, d := service(foreign)
	ctx := context.Background()
	k, err := s.Add(ctx, "  "+ed25519+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := file(t, d), foreign+ed25519+"\n"; got != want {
		t.Fatalf("file\n%q\nwant\n%q", got, want)
	}
	l, _ := s.List(ctx)
	if len(l.Keys) != 1 || l.Keys[0].ID != k.ID || l.Unmanaged != 1 {
		t.Fatalf("%+v", l)
	}
	if _, err := s.Add(ctx, ed25519); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate: %v", err)
	}
	// the same key with another comment is still the same key
	if _, err := s.Add(ctx, strings.TrimSuffix(ed25519, "test-ed25519")+"laptop"); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate with comment: %v", err)
	}
}

func TestDeleteRemovesOnlyThatKey(t *testing.T) {
	foreign := `from="10.0.0.0/8" ` + ecdsa + "\n"
	s, d := service(foreign + ed25519 + "\n" + rsa + "\n")
	ctx := context.Background()
	k, _ := Parse(ed25519)
	if err := s.Delete(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	if got := file(t, d); got != foreign+rsa+"\n" {
		t.Fatalf("%q", got)
	}
	if err := s.Delete(ctx, k.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestEmptyFileAndLimit(t *testing.T) {
	s, _ := service("")
	ctx := context.Background()
	l, err := s.List(ctx)
	if err != nil || len(l.Keys) != 0 || l.Keys == nil {
		t.Fatalf("%+v %v", l, err)
	}
	var lines []string
	for i := range MaxKeys {
		blob := make([]byte, 0, 51)
		blob = append(blob, 0, 0, 0, 11)
		blob = append(blob, "ssh-ed25519"...)
		blob = append(blob, 0, 0, 0, 32)
		blob = append(blob, []byte(fmt.Sprintf("%032d", i))...)
		lines = append(lines, "ssh-ed25519 "+base64.StdEncoding.EncodeToString(blob))
	}
	s, _ = service(strings.Join(lines, "\n") + "\n")
	if _, err := s.Add(ctx, ed25519); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit: %v", err)
	}
}

func TestReadFailureIsAnError(t *testing.T) {
	s, d := service("")
	d.AuthorizedKeysErr = errors.New("eio")
	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("want error")
	}
	if _, err := s.Add(context.Background(), ed25519); err == nil {
		t.Fatal("want error")
	}
}
