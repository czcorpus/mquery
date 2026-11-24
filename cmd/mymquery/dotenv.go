package main

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// loadDotEnv reads simple KEY=VALUE lines from a .env file.
// Supported: comments (#), blank lines, optional `export ` prefix,
// single/double-quoted values. A missing file is not an error.
func loadDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ans := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		} else if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		if k != "" {
			ans[k] = v
		}
	}
	return ans, sc.Err()
}

// envLookup returns the process env value if set, otherwise the .env value.
func envLookup(dotenv map[string]string, key string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return dotenv[key]
}
