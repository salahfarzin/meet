package e2e

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestWaitForServer_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := WaitForServer(server.URL, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWaitForServer_NeverReady(t *testing.T) {
	err := WaitForServer("http://127.0.0.1:1", 1)
	if err == nil {
		t.Fatal("expected an error when server never becomes ready")
	}
}

func TestDecodeJSON_Success(t *testing.T) {
	resp := &APIResponse{Body: []byte(`{"foo":"bar"}`)}
	var v map[string]string
	resp.DecodeJSON(t, &v)
	if v["foo"] != "bar" {
		t.Fatalf("expected foo=bar, got %v", v)
	}
}

func TestDecodeJSON_Invalid(t *testing.T) {
	ft := &testing.T{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = recover() }()
		resp := &APIResponse{Body: []byte("not json")}
		var v map[string]interface{}
		resp.DecodeJSON(ft, &v)
	}()
	wg.Wait()

	if !ft.Failed() {
		t.Fatal("expected DecodeJSON to fail on invalid JSON")
	}
}
