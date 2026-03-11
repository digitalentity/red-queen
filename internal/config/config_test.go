package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	validBase := func() *Config {
		return &Config{
			FTP:   FTPConfig{TempDir: "/tmp/uploads"},
			Zones: []ZoneConfig{{Name: "front-door", Cameras: []CameraConfig{{IP: "192.168.1.10"}}}},
			Detection: DetectionConfig{
				Analysis: AnalyzerConfig{Provider: "gemini-ai"},
			},
		}
	}

	t.Run("Valid config passes", func(t *testing.T) {
		assert.NoError(t, validBase().Validate())
	})

	t.Run("Missing analysis provider", func(t *testing.T) {
		cfg := validBase()
		cfg.Detection.Analysis.Provider = ""
		assert.ErrorContains(t, cfg.Validate(), "analysis.provider")
	})

	t.Run("Prefilter missing provider", func(t *testing.T) {
		cfg := validBase()
		cfg.Detection.Prefilter = &AnalyzerConfig{Provider: ""}
		assert.ErrorContains(t, cfg.Validate(), "prefilter.provider")
	})

	t.Run("Missing temp_dir", func(t *testing.T) {
		cfg := validBase()
		cfg.FTP.TempDir = ""
		assert.ErrorContains(t, cfg.Validate(), "temp_dir")
	})

	t.Run("No zones", func(t *testing.T) {
		cfg := validBase()
		cfg.Zones = nil
		assert.ErrorContains(t, cfg.Validate(), "zone")
	})

	t.Run("Empty zone name", func(t *testing.T) {
		cfg := validBase()
		cfg.Zones = []ZoneConfig{{Name: ""}}
		assert.ErrorContains(t, cfg.Validate(), "zone name")
	})

	t.Run("Zone name with slash", func(t *testing.T) {
		cfg := validBase()
		cfg.Zones = []ZoneConfig{{Name: "../../etc"}}
		assert.ErrorContains(t, cfg.Validate(), "invalid zone name")
	})

	t.Run("Zone name with space", func(t *testing.T) {
		cfg := validBase()
		cfg.Zones = []ZoneConfig{{Name: "front door"}}
		assert.ErrorContains(t, cfg.Validate(), "invalid zone name")
	})

	t.Run("Zone name with hyphens and underscores is valid", func(t *testing.T) {
		cfg := validBase()
		cfg.Zones = []ZoneConfig{{Name: "Zone_1-A"}}
		assert.NoError(t, cfg.Validate())
	})
}

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

	t.Run("Load with Detection config", func(t *testing.T) {
		configContent := `
detection:
  analysis:
    provider: gemini-ai
    max_artifact_size: 1024
`
		tmpFile, err := os.CreateTemp("", "mlconfig*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, _ = tmpFile.WriteString(configContent)
		tmpFile.Close()

		cfg, err := LoadConfig(tmpFile.Name())
		require.NoError(t, err)

		assert.Equal(t, "gemini-ai", cfg.Detection.Analysis.Provider)
		assert.Equal(t, int64(1024), cfg.Detection.Analysis.MaxArtifactSize)
	})

	t.Run("Load with prefilter config", func(t *testing.T) {
		configContent := `
detection:
  prefilter:
    provider: yolo-onnx
    frame_interval: 500ms
  analysis:
    provider: gemini-ai
`
		tmpFile, err := os.CreateTemp("", "prefilter*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, _ = tmpFile.WriteString(configContent)
		tmpFile.Close()

		cfg, err := LoadConfig(tmpFile.Name())
		require.NoError(t, err)

		assert.Equal(t, "yolo-onnx", cfg.Detection.Prefilter.Provider)
		assert.Equal(t, 500*time.Millisecond, cfg.Detection.Prefilter.FrameInterval)
	})
}
