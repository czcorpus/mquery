package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvFileArgs(t *testing.T) {
	cases := []struct {
		args []string
		path string
		skip bool
	}{
		{nil, ".env", false},
		{[]string{"-no-env-file"}, ".env", true},
		{[]string{"--no-env-file=false"}, ".env", false},
		{[]string{"-api", "http://x", "-env-file", "a.env"}, "a.env", false},
		{[]string{"--env-file=b.env", "-no-env-file"}, "b.env", true},
		{[]string{"--", "-no-env-file"}, ".env", false},
	}
	for _, c := range cases {
		p, s := envFileArgs(c.args)
		if p != c.path || s != c.skip {
			t.Errorf("%v: got (%q, %v), want (%q, %v)", c.args, p, s, c.path, c.skip)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	data := "# comment\n\nAPI_TOKEN=\"tok en\"\nexport API_URL=https://x.cz/mquery # note\nLOGIN_URL='https://x.cz/login'\nbroken line\n"
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := loadDotEnv(p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"API_TOKEN": "tok en",
		"API_URL":   "https://x.cz/mquery",
		"LOGIN_URL": "https://x.cz/login",
	}
	if len(m) != len(want) {
		t.Fatalf("got %v", m)
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s: got %q, want %q", k, m[k], v)
		}
	}
	if m, err := loadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil || len(m) != 0 {
		t.Fatalf("missing file: %v %v", m, err)
	}
}
