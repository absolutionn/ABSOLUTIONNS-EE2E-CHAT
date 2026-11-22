package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"e2ee-chat/internal/room"
	"github.com/coder/websocket"
)

func createTestRoom(t *testing.T, baseURL string) string {
	t.Helper()
	resp, err := http.Post(baseURL+"/api/rooms", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body["roomId"]
}

func TestIntegrationRelayAndRejections(t *testing.T) {
	mgr := room.NewManager()
	ts := httptest.NewServer(New(mgr, "").Router())
	defer ts.Close()

	wsBase := "ws" + strings.TrimPrefix(ts.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Missing room -> 404
	_, resp, err := websocket.Dial(ctx, wsBase+"/ws?room=missing", nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing room, got err=%v resp=%v", err, resp)
	}

	roomID := createTestRoom(t, ts.URL)
	c1, _, err := websocket.Dial(ctx, wsBase+"/ws?room="+roomID, nil)
	if err != nil {
		t.Fatalf("c1 dial: %v", err)
	}
	defer c1.CloseNow()

	c2, _, err := websocket.Dial(ctx, wsBase+"/ws?room="+roomID, nil)
	if err != nil {
		t.Fatalf("c2 dial: %v", err)
	}
	defer c2.CloseNow()

	// 3rd participant -> 403 Forbidden (FR-3)
	_, resp3, err := websocket.Dial(ctx, wsBase+"/ws?room="+roomID, nil)
	if err == nil || resp3 == nil || resp3.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for 3rd participant, got err=%v resp=%v", err, resp3)
	}

	// Drain initial status frames
	_, _, _ = c1.Read(ctx) // peers:1
	_, _, _ = c1.Read(ctx) // peers:2
	_, _, _ = c2.Read(ctx) // peers:2

	// Ciphertext relay + replay rejection (NFR-3)
	msg := `{"type":"msg","id":"nonce-1","data":{"iv":"nonce-1","ct":"abc"}}`
	if err := c1.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		t.Fatal(err)
	}
	_, got, err := c2.Read(ctx)
	if err != nil || string(got) != msg {
		t.Fatalf("peer got %q (err=%v), want %q", got, err, msg)
	}

	// Send duplicate nonce-1 -> server rejects with replay_detected
	if err := c1.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		t.Fatal(err)
	}
	_, errFrame, err := c1.Read(ctx)
	if err != nil || !strings.Contains(string(errFrame), "replay_detected") {
		t.Fatalf("expected replay_detected frame, got %q (err=%v)", errFrame, err)
	}

	// Close both -> RAM cleaned
	_ = c1.Close(websocket.StatusNormalClosure, "")
	_ = c2.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && mgr.ActiveRooms() > 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if mgr.ActiveRooms() != 0 {
		t.Fatalf("expected 0 active rooms after disconnect, got %d", mgr.ActiveRooms())
	}
}

func TestConcurrentJoinRaceSafety(t *testing.T) {
	mgr := room.NewManager()
	ts := httptest.NewServer(New(mgr, "").Router())
	defer ts.Close()

	wsBase := "ws" + strings.TrimPrefix(ts.URL, "http")
	roomID := createTestRoom(t, ts.URL)

	const total = 20
	var okCount, forbiddenCount atomic.Int32
	var wg sync.WaitGroup
	conns := make(chan *websocket.Conn, total)

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			c, resp, err := websocket.Dial(ctx, wsBase+"/ws?room="+roomID, nil)
			if err == nil {
				okCount.Add(1)
				conns <- c
				return
			}
			if resp != nil && resp.StatusCode == http.StatusForbidden {
				forbiddenCount.Add(1)
			}
		}()
	}
	wg.Wait()
	close(conns)
	for c := range conns {
		_ = c.Close(websocket.StatusNormalClosure, "")
	}

	if okCount.Load() != 2 || forbiddenCount.Load() != total-2 {
		t.Fatalf("race result: ok=%d forbidden=%d, want 2 and %d", okCount.Load(), forbiddenCount.Load(), total-2)
	}
}
