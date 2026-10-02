package api

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// A slow or stalled browser must not be able to fail the operation it is
// watching. Before the producer was decoupled from the socket, a full socket
// buffer blocked the command's output reader, which execx.Stream reports as a
// WaitDelay timeout — a failed install that actually succeeded on the device.
func TestStreamServiceAction_SlowClientDoesNotBlockAction(t *testing.T) {
	const lines = 3000
	actionDone := make(chan struct{})

	app := fiber.New()
	app.Post("/stream", func(c fiber.Ctx) error {
		return streamServiceAction(c, func(logFn func(string)) error {
			for range lines {
				logFn(strings.Repeat("x", 512))
			}
			close(actionDone)
			return nil
		})
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() { _ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("POST /stream HTTP/1.1\r\nHost: x\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")); err != nil {
		t.Fatal(err)
	}

	// Read a trickle, then effectively stall, so the server's socket buffer
	// fills and the writer goroutine blocks.
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 64)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	select {
	case <-actionDone:
		// The action finished without waiting for the client: log lines were
		// dropped, not blocked on.
	case <-time.After(20 * time.Second):
		t.Fatal("a slow client blocked the action: logFn is not decoupled from the socket")
	}

	_ = conn.Close()
	<-readDone
}
