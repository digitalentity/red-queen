package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	LogLevel      string          `mapstructure:"log_level"`
	Concurrency   int             `mapstructure:"concurrency"`
	FTP           FTPConfig       `mapstructure:"ftp"`
	Zones         []ZoneConfig    `mapstructure:"zones"`
	ML            MLConfig        `mapstructure:"ml"`
	Storage       StorageConfig   `mapstructure:"storage"`
	Notifications []NotifyConfig  `mapstructure:"notifications"`
	API           APIConfig       `mapstructure:"api"`
}

type FTPConfig struct {
	ListenAddress string `mapstructure:"listen_address"`
	Port          int    `mapstructure:"port"`
	User          string `mapstructure:"user"`
	Password      string `mapstructure:"password"`
	TempDir       string `mapstructure:"temp_dir"`
}

type ZoneConfig struct {
	Name    string         `mapstructure:"name"`
	Cameras []CameraConfig `mapstructure:"cameras"`
}

type CameraConfig struct {
	IP string `mapstructure:"ip"`
}

type MLConfig struct {
	Provider      string   `mapstructure:"provider"`
	Endpoint      string   `mapstructure:"endpoint"`
	APIKey        string   `mapstructure:"api_key"`
	Threshold     float64  `mapstructure:"threshold"`
	TargetObjects []string `mapstructure:"target_objects"`
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
	Token   string `mapstructure:"token"`   // Used by Slack
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

	// Support environment variables
	v.SetEnvPrefix("RED_QUEEN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// It's okay if the config file is missing as long as env vars are provided,
		// but usually we expect a file for basic structure.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
