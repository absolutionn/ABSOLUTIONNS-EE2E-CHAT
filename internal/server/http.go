package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"e2ee-chat/internal/room"
	"github.com/coder/websocket"
)

type Envelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type Server struct {
	mgr       *room.Manager
	webDir    string
	clientSeq atomic.Uint64
}

func New(mgr *room.Manager, webDir string) *Server { return &Server{mgr: mgr, webDir: webDir} }

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/rooms", s.handleCreateRoom)
	mux.HandleFunc("GET /ws", s.handleWS)
	if s.webDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(s.webDir)))
	}
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, _ *http.Request) {
	id, err := s.mgr.CreateRoom()
	if err != nil {
		http.Error(w, "failed to create room", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"roomId": id,
		"url":    "/?room=" + id,
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		http.Error(w, "missing room id", http.StatusBadRequest)
		return
	}
	clientID := "c-" + strconv.FormatUint(s.clientSeq.Add(1), 10)
	ch, err := s.mgr.Join(roomID, clientID)
	if err != nil {
		switch {
		case errors.Is(err, room.ErrRoomNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, room.ErrRoomFull):
			http.Error(w, err.Error(), http.StatusForbidden)
		default:
			http.Error(w, "join failed", http.StatusInternalServerError)
		}
		return
	}
	defer s.mgr.Leave(roomID, clientID)

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		for msg := range ch {
			wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			wcancel()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			writeErr(ctx, conn, "invalid_json")
			continue
		}
		if env.Type != "pubkey" && env.Type != "msg" {
			writeErr(ctx, conn, "unknown_type")
			continue
		}
		msgID := ""
		if env.Type == "msg" {
			if env.ID == "" {
				writeErr(ctx, conn, "missing_id")
				continue
			}
			msgID = env.ID
		}
		if err := s.mgr.Relay(roomID, clientID, msgID, data); err != nil {
			if errors.Is(err, room.ErrReplayDetected) {
				writeErr(ctx, conn, "replay_detected")
			} else {
				writeErr(ctx, conn, err.Error())
			}
		}
	}
}

func writeErr(ctx context.Context, conn *websocket.Conn, code string) {
	wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	payload, _ := json.Marshal(map[string]string{"type": "error", "error": code})
	_ = conn.Write(wctx, websocket.MessageText, payload)
}
