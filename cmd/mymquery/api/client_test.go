package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginSendsSessionCookie(t *testing.T) {
	var gotCookie string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("personal_access_token") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "sess", Value: "abc", Path: "/"})
	})
	mux.HandleFunc("GET /api/corplist", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("sess"); err == nil {
			gotCookie = c.Value
		}
		w.Write([]byte(`{"corpora":[]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewClient(srv.URL+"/api", nil, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(context.Background(), srv.URL+"/login", "wrong"); err == nil {
		t.Fatal("expected login failure for a wrong token")
	}
	if c.Authenticated() {
		t.Fatal("must not be authenticated after failed login")
	}
	if err := c.Login(context.Background(), srv.URL+"/login", "secret"); err != nil {
		t.Fatal(err)
	}
	if !c.Authenticated() {
		t.Fatal("expected authenticated client")
	}
	if _, err := c.Corplist(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if gotCookie != "abc" {
		t.Fatalf("session cookie not sent, got %q", gotCookie)
	}
}

func TestServerInfoUsesRootPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"MQuery","versionInfo":{"version":"1.2.3","buildDate":"2026-01-01","lastCommit":"abc"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewClient(srv.URL+"/api", nil, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	info, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "MQuery" || info.VersionInfo.Version != "1.2.3" || info.VersionInfo.LastCommit != "abc" {
		t.Fatalf("unexpected server info: %+v", info)
	}
}

func TestConcordanceArgsContextStruct(t *testing.T) {
	if v := (ConcordanceArgs{Query: "x"}).values(); v.Has("contextStruct") {
		t.Error("contextStruct should be omitted by default")
	}
	if v := (ConcordanceArgs{Query: "x", ContextStruct: "p"}).values(); v.Get("contextStruct") != "p" {
		t.Errorf("unexpected contextStruct %q", v.Get("contextStruct"))
	}
	v := ConcordanceArgs{Query: "x", ContextStruct: "p", TokenContext: true}.values()
	if !v.Has("contextStruct") || v.Get("contextStruct") != "" {
		t.Errorf("TokenContext must send an empty contextStruct, got %v", v)
	}
}

func TestConcordanceArgsShowMarkup(t *testing.T) {
	if v := (ConcordanceArgs{Query: "x"}).values(); v.Has("showMarkup") {
		t.Error("showMarkup should be omitted by default")
	}
	if v := (ConcordanceArgs{Query: "x", ShowMarkup: true}).values(); v.Get("showMarkup") != "1" {
		t.Errorf("unexpected showMarkup %q", v.Get("showMarkup"))
	}
}
