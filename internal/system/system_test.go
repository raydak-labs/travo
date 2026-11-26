package system

import (
	"context"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

func TestReadSeededMock(t *testing.T) {
	d := mock.New()
	mock.SeedSystem(d)
	b, i, err := Read(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if b.Model != "GL.iNet GL-AXT1800" || i.MemTotal != 428646400 {
		t.Fatalf("%+v %+v", b, i)
	}
}

func TestReadNamesTheFailingCall(t *testing.T) {
	d := mock.New()
	if _, _, err := Read(context.Background(), d); err == nil || !strings.Contains(err.Error(), "system board") {
		t.Fatalf("board: %v", err)
	}
	d.SetUbusReply("system", "board", []byte(`{"model":"m","kernel":"k","release":{"version":"1"}}`))
	if _, _, err := Read(context.Background(), d); err == nil || !strings.Contains(err.Error(), "system info") {
		t.Fatalf("info: %v", err)
	}
	d.SetUbusReply("system", "info", []byte(`{}`))
	_, _, err := Read(context.Background(), d)
	if err == nil || !strings.Contains(err.Error(), "system info") {
		t.Fatalf("parse: %v", err)
	}
}
