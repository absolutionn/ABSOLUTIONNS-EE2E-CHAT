package room

import (
	"errors"
	"testing"
)

func TestCreateRoomUniqueness(t *testing.T) {
	mgr := NewManager()
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id, err := mgr.CreateRoom()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(id) != 32 {
			t.Fatalf("expected 32-char hex ID, got %d", len(id))
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate room ID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestJoinBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		roomExists bool
		preJoined  int
		wantErr    error
	}{
		{"missing room", false, 0, ErrRoomNotFound},
		{"first participant", true, 0, nil},
		{"second participant (boundary)", true, 1, nil},
		{"third participant blocked", true, 2, ErrRoomFull},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewManager()
			roomID := "nonexistent"
			if tc.roomExists {
				var err error
				roomID, err = mgr.CreateRoom()
				if err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < tc.preJoined; i++ {
				if _, err := mgr.Join(roomID, "pre-"+string(rune('a'+i))); err != nil {
					t.Fatalf("setup join failed: %v", err)
				}
			}
			_, err := mgr.Join(roomID, "target")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got err %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRelayAndReplayProtection(t *testing.T) {
	mgr := NewManager()
	roomID, _ := mgr.CreateRoom()
	_, _ = mgr.Join(roomID, "c1")
	ch2, _ := mgr.Join(roomID, "c2")
	// Drain initial status messages
	<-ch2

	if err := mgr.Relay(roomID, "c1", "iv-1", nil); !errors.Is(err, ErrEmptyPayload) {
		t.Fatalf("expected ErrEmptyPayload, got %v", err)
	}
	if err := mgr.Relay("bad-room", "c1", "iv-1", []byte("x")); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound, got %v", err)
	}

	msg := []byte(`{"type":"msg","id":"iv-1","data":"cipher"}`)
	if err := mgr.Relay(roomID, "c1", "iv-1", msg); err != nil {
		t.Fatalf("first relay failed: %v", err)
	}
	got := <-ch2
	if string(got) != string(msg) {
		t.Fatalf("got %q, want %q", got, msg)
	}

	// NFR-3: Replay with identical msgID must fail
	if err := mgr.Relay(roomID, "c1", "iv-1", msg); !errors.Is(err, ErrReplayDetected) {
		t.Fatalf("expected ErrReplayDetected, got %v", err)
	}
}

func TestLeaveCleansUpRAM(t *testing.T) {
	mgr := NewManager()
	roomID, _ := mgr.CreateRoom()
	_, _ = mgr.Join(roomID, "c1")
	_, _ = mgr.Join(roomID, "c2")

	mgr.Leave(roomID, "c1")
	if mgr.ActiveRooms() != 1 {
		t.Fatalf("expected 1 active room, got %d", mgr.ActiveRooms())
	}
	mgr.Leave(roomID, "c2")
	if mgr.ActiveRooms() != 0 {
		t.Fatalf("expected 0 active rooms after both leave, got %d", mgr.ActiveRooms())
	}
}
