package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
log_level: debug
concurrency: 5
ftp:
  port: 2121
  user: testuser
zones:
  - name: "Entry"
    cameras:
      - ip: "1.1.1.1"
`
	tmpFile, err := os.CreateTemp("", "config*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	t.Run("Load from file", func(t *testing.T) {
		cfg, err := LoadConfig(tmpFile.Name())
		require.NoError(t, err)

		assert.Equal(t, "debug", cfg.LogLevel)
		assert.Equal(t, 5, cfg.Concurrency)
		assert.Equal(t, 2121, cfg.FTP.Port)
		assert.Equal(t, "testuser", cfg.FTP.User)
		assert.Len(t, cfg.Zones, 1)
		assert.Equal(t, "Entry", cfg.Zones[0].Name)
	})

	t.Run("Environment variables are ignored", func(t *testing.T) {
		os.Setenv("RED_QUEEN_LOG_LEVEL", "warn")
		os.Setenv("RED_QUEEN_FTP_PORT", "9999")
		defer os.Unsetenv("RED_QUEEN_LOG_LEVEL")
		defer os.Unsetenv("RED_QUEEN_FTP_PORT")

		cfg, err := LoadConfig(tmpFile.Name())
		require.NoError(t, err)

		// Values should still come from the file, not environment
		assert.Equal(t, "debug", cfg.LogLevel)
		assert.Equal(t, 2121, cfg.FTP.Port)
	})

	t.Run("Fail when file is missing", func(t *testing.T) {
		_, err := LoadConfig("non-existent.yaml")
		assert.Error(t, err, "LoadConfig should fail if the specified file does not exist")
	})
}
