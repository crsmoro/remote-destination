package main

import (
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"waze-remoto/internal/api"
	"waze-remoto/internal/store"
)

func main() {
	file := os.Getenv("DEVICE_KEY_FILE")
	if file == "" {
		log.Fatal("DEVICE_KEY_FILE is required")
	}
	raw, e := os.ReadFile(file)
	if e != nil {
		log.Fatal("cannot read device key file")
	}
	key := strings.TrimSpace(string(raw))
	decoded, e := base64.RawURLEncoding.DecodeString(key)
	if e != nil || len(decoded) != 32 {
		log.Fatal("device key must be 32 bytes encoded as unpadded base64url")
	}
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "/data/destinations.db"
	}
	s, e := store.Open(path)
	if e != nil {
		log.Fatal("cannot open database: ", e)
	}
	defer s.Close()
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	var handler http.Handler = api.New(s, key)
	if clientID, configured := os.LookupEnv("AUTH0_AGENT_CLIENT_ID"); configured {
		handler, e = api.NewWithAgent(s, key, clientID)
		if e != nil {
			log.Fatal("invalid AUTH0_AGENT_CLIENT_ID")
		}
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192}
	log.Printf("listening on %s; user routes require a trusted Auth0 proxy", addr)
	log.Fatal(server.ListenAndServe())
}
