package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	LogLevel       string           `mapstructure:"log_level"`
	Concurrency    int              `mapstructure:"concurrency"`
	ProcessTimeout string           `mapstructure:"process_timeout"` // e.g. "5m"
	HTTPClient     HTTPClientConfig `mapstructure:"http_client"`
	FTP            FTPConfig        `mapstructure:"ftp"`
	Zones          []ZoneConfig     `mapstructure:"zones"`
	ML             MLConfig         `mapstructure:"ml"`
	Storage        StorageConfig    `mapstructure:"storage"`
	Notifications  []NotifyConfig   `mapstructure:"notifications"`
	API            APIConfig        `mapstructure:"api"`
}

type HTTPClientConfig struct {
	Timeout string `mapstructure:"timeout"` // Default 30s
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

type MLConfig struct {
	Provider        string   `mapstructure:"provider"`
	ModelName       string   `mapstructure:"model_name"`
	ProjectID       string   `mapstructure:"project_id"`
	Location        string   `mapstructure:"location"`
	Endpoint        string   `mapstructure:"endpoint"`
	APIKey          string   `mapstructure:"api_key"`
	Threshold       float64  `mapstructure:"threshold"`
	TargetObjects   []string `mapstructure:"target_objects"`
	MaxArtifactSize int64    `mapstructure:"max_artifact_size"` // Max size in bytes
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
	Type    string `mapstructure:"type"`
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`     // Used by Webhook
	Channel string `mapstructure:"channel"` // Used by Slack
	Token   string `mapstructure:"token"`   // Used by Slack/Telegram
	HomeyID string `mapstructure:"homey_id"` // Used by Homey
	Event   string `mapstructure:"event"`    // Used by Homey
	ChatID  int64  `mapstructure:"chat_id"`  // Used by Telegram
}

type APIConfig struct {
	Port    int  `mapstructure:"port"`
	Enabled bool `mapstructure:"enabled"`
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
