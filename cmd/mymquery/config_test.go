package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
	require.NoError(t, os.Chmod(path, mode))
}

func TestConfigFilesOrder(t *testing.T) {
	loc := configLocations{systemDirs: []string{"/etc/a", "/etc/b"}, userDir: "/home/u/.config", useLocal: true}
	assert.Equal(t, []string{
		"/etc/b/mymquery/env", "/etc/a/mymquery/env", // the first system dir is the most important
		"/home/u/.config/mymquery/env",
		".env",
	}, loc.files())

	// an explicit file replaces ./.env
	loc.explicit = "/tmp/x.env"
	assert.Equal(t, "/tmp/x.env", loc.files()[3])
	// an installed binary does not read ./.env
	loc = configLocations{userDir: "/home/u/.config"}
	assert.Equal(t, []string{"/home/u/.config/mymquery/env"}, loc.files())
}

func TestLoadConfigMerge(t *testing.T) {
	dir := t.TempDir()
	sys := filepath.Join(dir, "etc")
	user := filepath.Join(dir, "home")
	writeFile(t, filepath.Join(sys, "mymquery", "env"), "API_URL=http://sys\nCQLIZER_MCP=http://cq\n", 0o644)
	writeFile(t, filepath.Join(user, "mymquery", "env"), "API_URL=http://user\nMISTRAL_API_KEY=k\n", 0o600)
	values, warnings, err := loadConfig(configLocations{systemDirs: []string{sys}, userDir: user})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"API_URL": "http://user", "CQLIZER_MCP": "http://cq", "MISTRAL_API_KEY": "k",
	}, values)
	assert.Empty(t, warnings)

	// a missing explicit file is an error (other files are optional)
	_, _, err = loadConfig(configLocations{explicit: filepath.Join(dir, "missing.env")})
	assert.Error(t, err)
}

func TestLoadConfigReadableSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mymquery", "env")
	writeFile(t, path, "MISTRAL_API_KEY=secret\n", 0o644)
	_, warnings, err := loadConfig(configLocations{userDir: dir})
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "contains MISTRAL_API_KEY but other users can read it")
	assert.NotContains(t, warnings[0], "secret")

	// no secrets => no warning
	writeFile(t, path, "API_URL=http://x\n", 0o644)
	_, warnings, _ = loadConfig(configLocations{userDir: dir})
	assert.Empty(t, warnings)
}

func TestUpdateConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mymquery", "env")
	// a new file (incl. the directory) readable by the owner only
	require.NoError(t, updateConfigFile(path, map[string]string{"LLM_BACKEND": "mistral", "MISTRAL_API_KEY": "k1"}))
	st, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	dirSt, _ := os.Stat(filepath.Dir(path))
	assert.Equal(t, os.FileMode(0o700), dirSt.Mode().Perm())

	// existing values are replaced in place, other lines are kept
	writeFile(t, path, "# my config\nAPI_URL=http://x\nexport MISTRAL_API_KEY=old\n", 0o600)
	require.NoError(t, updateConfigFile(path, map[string]string{
		"MISTRAL_API_KEY": "new", "LLM_MODEL": "a b,c", "X": "it's",
	}))
	data, _ := os.ReadFile(path)
	assert.Equal(t, "# my config\nAPI_URL=http://x\nMISTRAL_API_KEY=new\nLLM_MODEL='a b,c'\nX=\"it's\"\n", string(data))
	values, err := loadDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, "a b,c", values["LLM_MODEL"])
	assert.Equal(t, "it's", values["X"])
}
