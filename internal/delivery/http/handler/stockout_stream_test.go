package handler

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	usecaseStockOut "chawy-erp-api/internal/usecase/stockout"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// The stream sends a change event to an open connection when a batch is saved.
func TestStockOutStreamSendsChangeEvents(t *testing.T) {
	broker := usecaseStockOut.NewBroker()
	h := NewStockOutHandler(nil, broker)
	// A short heartbeat lets the writer notice the client left, so Shutdown returns.
	h.heartbeat = 50 * time.Millisecond
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/stock/stream", h.Stream)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.Listener(ln) }()
	defer func() { _ = app.ShutdownWithTimeout(2 * time.Second) }()

	res, err := http.Get("http://" + ln.Addr().String() + "/stock/stream")
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, "text/event-stream", res.Header.Get("Content-Type"))

	r := bufio.NewReader(res.Body)
	readUntil := func(want string) string {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			line, err := r.ReadString('\n')
			require.NoError(t, err)
			if strings.Contains(line, want) {
				return line
			}
		}
		t.Fatalf("no %q in stream", want)
		return ""
	}
	readUntil(": connected")
	require.Eventually(t, func() bool { return broker.Subscribers() == 1 }, 2*time.Second, 10*time.Millisecond)

	broker.Publish(usecaseStockOut.Event{Type: "batch", BatchID: 7, Date: "2026-11-26"})
	readUntil("event: change")
	data := readUntil("data:")
	require.Contains(t, data, `"batchId":7`)
}
