package config

import (
	"fmt"
	"regexp"
	"time"

	"github.com/spf13/viper"
)

// validZoneName restricts zone names to characters that are safe for use in
// filesystem paths and log labels.
var validZoneName = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

type Config struct {
	LogLevel       string           `mapstructure:"log_level"`
	Concurrency    int              `mapstructure:"concurrency"`
	ProcessTimeout time.Duration    `mapstructure:"process_timeout"`
	HTTPClient     HTTPClientConfig `mapstructure:"http_client"`
	FTP            FTPConfig        `mapstructure:"ftp"`
	Zones          []ZoneConfig    `mapstructure:"zones"`
	Detection      DetectionConfig `mapstructure:"detection"`
	Storage        StorageConfig   `mapstructure:"storage"`
	Notifications  []NotifyConfig  `mapstructure:"notifications"`
	API            APIConfig       `mapstructure:"api"`
}

type HTTPClientConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

type FTPConfig struct {
	ListenAddress string `mapstructure:"listen_address"`
	Port          int    `mapstructure:"port"`
	User          string `mapstructure:"user"`
	Password      string `mapstructure:"password"`
	TempDir       string `mapstructure:"temp_dir"`
	RetainFiles   bool   `mapstructure:"retain_files"`
}

type ZoneConfig struct {
	Name    string         `mapstructure:"name"`
	Cameras []CameraConfig `mapstructure:"cameras"`
}

type CameraConfig struct {
	IP string `mapstructure:"ip"`
}

// DetectionConfig configures the multi-stage detection pipeline.
// Prefilter is optional; when nil the Analysis stage runs directly.
type DetectionConfig struct {
	Prefilter *AnalyzerConfig `mapstructure:"prefilter"`
	Analysis  *AnalyzerConfig `mapstructure:"analysis"`
}

// AnalyzerConfig holds all fields for any analyzer implementation.
// Fields that do not apply to a given provider are ignored.
type AnalyzerConfig struct {
	Provider string `mapstructure:"provider"` // "yolo-onnx" | "gemini-ai" | "always"

	// Common fields
	Threshold     float64  `mapstructure:"threshold"`
	TargetObjects []string `mapstructure:"target_objects"`

	// YOLO ONNX (provider: "yolo-onnx")
	ModelPath         string        `mapstructure:"model_path"`
	ExecutionProvider string        `mapstructure:"execution_provider"` // default: "cpu"
	FrameInterval     time.Duration `mapstructure:"frame_interval"`     // default: 0.25s

	// Gemini AI (provider: "gemini-ai")
	ModelName       string `mapstructure:"model_name"`
	APIKey          string `mapstructure:"api_key"`
	Endpoint        string `mapstructure:"endpoint"`
	MaxArtifactSize int64  `mapstructure:"max_artifact_size"`
}

type StorageConfig struct {
	AlwaysStore bool                  `mapstructure:"always_store"`
	Providers   []StorageProviderConfig `mapstructure:"providers"`
}

// StorageProviderConfig holds the configuration for a single storage backend.
// Fields that do not apply to a given type are ignored.
type StorageProviderConfig struct {
	Type        string       `mapstructure:"type"` // "local" | "google_drive"
	Local       LocalConfig  `mapstructure:"local"`
	GoogleDrive GDriveConfig `mapstructure:"google_drive"`
}

type LocalConfig struct {
	RootPath string `mapstructure:"root_path"`
}

// GDriveConfig configures the Google Drive storage backend.
// The target folder must be shared with the service account as Editor.
// Uploaded files are private and inherit the folder's sharing settings —
// the webViewLink included in notifications is only accessible to accounts
// with explicit folder access.
type GDriveConfig struct {
	// CredentialsFile is the path to a Google service account JSON key file.
	CredentialsFile string `mapstructure:"credentials_file"`
	// FolderID is the ID of the Drive folder to upload artifacts into.
	FolderID string `mapstructure:"folder_id"`
}

type NotifyConfig struct {
	Type            string `mapstructure:"type"`
	Enabled         bool   `mapstructure:"enabled"`
	Condition       string `mapstructure:"condition"`        // "on_threat" (default) | "always"
	URL             string `mapstructure:"url"`              // Used by Webhook; Telegram API base URL override
	ArtifactBaseURL string `mapstructure:"artifact_base_url"` // Used by Telegram: public base URL for artifact links
	Token           string `mapstructure:"token"`             // Used by Telegram
	HomeyID         string `mapstructure:"homey_id"`          // Used by Homey
	Event           string `mapstructure:"event"`             // Used by Homey
	ChatID          int64  `mapstructure:"chat_id"`           // Used by Telegram
}

type APIConfig struct {
	Port    int  `mapstructure:"port"`
	Enabled bool `mapstructure:"enabled"`
}

// Validate checks that required configuration fields are present and well-formed.
// It is separate from LoadConfig so that partial configs can still be loaded in
// tests; call it during application startup via app.New().
func (c *Config) Validate() error {
	if c.FTP.TempDir == "" {
		return fmt.Errorf("ftp.temp_dir must not be empty")
	}
	if len(c.Zones) == 0 {
		return fmt.Errorf("at least one zone must be configured under 'zones'")
	}
	for _, z := range c.Zones {
		if z.Name == "" {
			return fmt.Errorf("zone name must not be empty")
		}
		if !validZoneName.MatchString(z.Name) {
			return fmt.Errorf("invalid zone name %q: must contain only letters, digits, underscores, and hyphens", z.Name)
		}
	}

	for i, p := range c.Storage.Providers {
		switch p.Type {
		case "local":
			if p.Local.RootPath == "" {
				return fmt.Errorf("storage.providers[%d] (local): root_path must not be empty", i)
			}
		case "google_drive":
			if p.GoogleDrive.CredentialsFile == "" {
				return fmt.Errorf("storage.providers[%d] (google_drive): credentials_file must not be empty", i)
			}
			if p.GoogleDrive.FolderID == "" {
				return fmt.Errorf("storage.providers[%d] (google_drive): folder_id must not be empty", i)
			}
		case "":
			return fmt.Errorf("storage.providers[%d]: type must not be empty", i)
		}
	}

	for i, n := range c.Notifications {
		if !n.Enabled {
			continue
		}
		if n.Condition != "" && n.Condition != "on_threat" && n.Condition != "always" {
			return fmt.Errorf("notifications[%d]: invalid condition %q: must be 'on_threat' or 'always'", i, n.Condition)
		}
	}

	if c.Detection.Analysis == nil {
		return fmt.Errorf("detection.analysis section is required")
	}

	if c.Detection.Analysis.Provider == "" {
		return fmt.Errorf("detection.analysis.provider must not be empty")
	}

	if c.Detection.Prefilter != nil {
		if c.Detection.Prefilter.Provider == "" {
			return fmt.Errorf("detection.prefilter.provider must not be empty")
		}
	}

	return nil
}

// LoadConfig reads the configuration from the given file path and merges it with environment variables.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
