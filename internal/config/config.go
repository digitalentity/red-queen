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
	ProjectID       string `mapstructure:"project_id"`
	Location        string `mapstructure:"location"`
	Endpoint        string `mapstructure:"endpoint"`
	APIKey          string `mapstructure:"api_key"`
	MaxArtifactSize int64  `mapstructure:"max_artifact_size"`
}

type StorageConfig struct {
	Provider string      `mapstructure:"provider"`
	Local    LocalConfig `mapstructure:"local"`
	S3       S3Config    `mapstructure:"s3"`
}

type LocalConfig struct {
	RootPath string `mapstructure:"root_path"`
}

type S3Config struct {
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
}

type NotifyConfig struct {
	Type            string `mapstructure:"type"`
	Enabled         bool   `mapstructure:"enabled"`
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
