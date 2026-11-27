package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// configured application files (XDG Base Directory Specification)
const (
	configAppDir   = "mymquery"
	configFileName = "env"
	localEnvFile   = ".env"
)

// secretConfigKeys are variables which must not be readable by other users
var secretConfigKeys = []string{"API_TOKEN", "GEMINI_API_KEY", "MISTRAL_API_KEY", "OPENAI_API_KEY"}

// configLocations describes where configuration files are looked for
type configLocations struct {
	// systemDirs are XDG_CONFIG_DIRS (the first one is the most important)
	systemDirs []string
	// userDir is XDG_CONFIG_HOME
	userDir string
	// explicit is a file set via -env-file ("" = none)
	explicit string
	// useLocal enables ./.env (in a development setup only, see localEnvApplies)
	useLocal bool
}

// defaultConfigLocations returns the locations according to the environment
func defaultConfigLocations(explicit string) configLocations {
	loc := configLocations{explicit: explicit}
	dirs := os.Getenv("XDG_CONFIG_DIRS")
	if dirs == "" {
		dirs = "/etc/xdg"
	}
	for _, d := range filepath.SplitList(dirs) {
		if filepath.IsAbs(d) {
			loc.systemDirs = append(loc.systemDirs, d)
		}
	}
	loc.userDir = os.Getenv("XDG_CONFIG_HOME")
	if loc.userDir == "" || !filepath.IsAbs(loc.userDir) {
		if home, err := os.UserHomeDir(); err == nil {
			loc.userDir = filepath.Join(home, ".config")
		}
	}
	loc.useLocal = explicit == "" && localEnvApplies()
	return loc
}

// localEnvApplies tells whether ./.env is used: only when the binary is run
// from the current directory (e.g. `./mymquery` in the source tree). An
// installed binary (e.g. in /usr/local/bin) ignores .env files in whatever
// directory the user is in.
func localEnvApplies() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	return filepath.Dir(exe) == cwd
}

// userConfigPath returns the user's configuration file ("" if unknown)
func (loc configLocations) userConfigPath() string {
	if loc.userDir == "" {
		return ""
	}
	return filepath.Join(loc.userDir, configAppDir, configFileName)
}

// files returns configuration files in the order of increasing precedence
// (a value from a later file overrides the earlier ones)
func (loc configLocations) files() []string {
	var ans []string
	for _, d := range slices.Backward(loc.systemDirs) {
		ans = append(ans, filepath.Join(d, configAppDir, configFileName))
	}
	if p := loc.userConfigPath(); p != "" {
		ans = append(ans, p)
	}
	if loc.explicit != "" {
		ans = append(ans, loc.explicit)
	} else if loc.useLocal {
		ans = append(ans, localEnvFile)
	}
	return ans
}

// loadConfig reads and merges the configuration files (missing ones are
// skipped, except for an explicit one). It returns warnings about files
// with secrets readable by other users.
func loadConfig(loc configLocations) (values map[string]string, warnings []string, err error) {
	values = make(map[string]string)
	for _, path := range loc.files() {
		if path == loc.explicit {
			if _, err := os.Stat(path); err != nil {
				return nil, nil, fmt.Errorf("env file %s: %w", path, err)
			}
		}
		file, err := loadDotEnv(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
		if w := checkSecretsReadable(path, file); w != "" {
			warnings = append(warnings, w)
		}
		for k, v := range file {
			values[k] = v
		}
	}
	return values, warnings, nil
}

// checkSecretsReadable returns a warning if a file with secrets can be read
// by other users
func checkSecretsReadable(path string, values map[string]string) string {
	var secrets []string
	for _, k := range secretConfigKeys {
		if values[k] != "" {
			secrets = append(secrets, k)
		}
	}
	if len(secrets) == 0 {
		return ""
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm()&0o077 == 0 {
		return ""
	}
	return fmt.Sprintf("%s contains %s but other users can read it (fix: chmod 600 %s)",
		path, strings.Join(secrets, ", "), path)
}

// updateConfigFile sets variables in a configuration file (other lines,
// incl. comments, are kept). The file is readable only by its owner.
func updateConfigFile(path string, values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	done := make(map[string]bool)
	for i, line := range lines {
		k, _, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "export "), "=")
		k = strings.TrimSpace(k)
		if v, set := values[k]; ok && set && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			lines[i] = k + "=" + quoteConfigValue(v)
			done[k] = true
		}
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		if !done[k] {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		lines = append(lines, k+"="+quoteConfigValue(values[k]))
	}
	// write a new file and rename it, so a failure does not destroy the old one
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+configFileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func quoteConfigValue(v string) string {
	switch {
	case !strings.ContainsAny(v, " #\"'"):
		return v
	case strings.Contains(v, "'"):
		return `"` + v + `"`
	}
	return "'" + v + "'"
}
