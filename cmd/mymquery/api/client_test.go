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
