package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"waze-remoto/internal/api"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestSlowSequentialGoogleResponseReachesClient(t *testing.T) {
	// Extract the actual server timeout so this exercises main's configuration.
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var timeout time.Duration
	ast.Inspect(f, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		k, ok := kv.Key.(*ast.Ident)
		if !ok || k.Name != "WriteTimeout" {
			return true
		}
		mul, ok := kv.Value.(*ast.BinaryExpr)
		if !ok || mul.Op != token.MUL {
			t.Fatal("unexpected timeout expression")
		}
		lit, ok := mul.X.(*ast.BasicLit)
		if !ok {
			t.Fatal("unexpected timeout value")
		}
		nsec, e := strconv.Atoi(lit.Value)
		if e != nil {
			t.Fatal(e)
		}
		timeout = time.Duration(nsec) * time.Second
		return true
	})
	if timeout == 0 {
		t.Fatal("server WriteTimeout not found")
	}
	// Three network stages plus agent authentication need room for processing.
	if timeout < 40*time.Second || timeout > 60*time.Second {
		t.Fatalf("unsafe WriteTimeout: %s", timeout)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := api.New(s, "key")
	err = api.SetGeocoder(h, func(ctx context.Context, input string) ([]destination.Candidate, error) {
		// Simulate empty Geocoding then successful Places, each below its 8s limit.
		for range 2 {
			select {
			case <-time.After(7700 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return []destination.Candidate{{Label: "Mercado", Latitude: -26.3, Longitude: -48.8}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h, WriteTimeout: timeout}
	defer srv.Close()
	go srv.Serve(listener)
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Post("http://"+listener.Addr().String()+"/api/destinations", "application/json", strings.NewReader(`{"input":"Mercado"}`))
	if err != nil {
		t.Fatalf("response lost after sequential calls: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %s", resp.Status)
	}
	pending, err := s.Pending()
	if err != nil || pending == nil {
		t.Fatalf("pending=%v error=%v", pending, err)
	}
}
