package main

import (
	"log"
	"net/http"
	"os"

	"e2ee-chat/internal/room"
	"e2ee-chat/internal/server"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "./web"
	}
	srv := server.New(room.NewManager(), webDir)
	log.Printf("listening on http://%s", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}
