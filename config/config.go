package config

import (
	"fmt"
	"log"
	"strconv"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	Output   string `mapstructure:"output"`
	FilePath string `mapstructure:"file_path"`
}

type RelayConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	URL        string `mapstructure:"url"`
	MaxClients int    `mapstructure:"max_clients"`
}

func (r RelayConfig) GetAddress() string {
	return r.Host + ":" + strconv.Itoa(r.Port)
}

type AuthConfig struct {
	NoAuth bool   `mapstructure:"no_auth"`
	Cert   string `mapstructure:"cert"`
	Key    string `mapstructure:"key"`
}

type StorageConfig struct {
	Type     StorageType `mapstructure:"type"`
	Host     string      `mapstructure:"host"`
	Port     int         `mapstructure:"port"`
	User     string      `mapstructure:"user"`
	Password string      `mapstructure:"password"`
	Name     string      `mapstructure:"name"`
}

func (sc *StorageConfig) GetConnectionString() string {
	switch sc.Type {
	case Postgres:
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", sc.Host, sc.Port, sc.User, sc.Password, sc.Name)
	case MySQL:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", sc.User, sc.Password, sc.Host, sc.Port, sc.Name)
	case SQLite:
		return fmt.Sprintf("file:%s", sc.Name)
	default:
		return ""
	}
}

type ClientConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

type Config struct {
	Logging LoggingConfig `mapstructure:"logging"`
	Relay   RelayConfig   `mapstructure:"relay"`
	Storage StorageConfig `mapstructure:"storage"`
	Clients ClientConfig  `mapstructure:"clients"`
	Auth    AuthConfig    `mapstructure:"auth"`
}

func LoadConfig() (*Config, error) {
	// Define command line flags
	pflag.String("logging.log_level", "", "log level")
	pflag.String("logging.format", "", "log output format: \"text\", \"json\"")
	pflag.String("logging.output", "", "where to send log output:  \"stdout\", \"stderr\", \"file\"")
	pflag.String("logging.file_path", "", "only used if output is \"file\"")
	pflag.String("relay.host", "", "Relay host")
	pflag.Int("relay.port", 0, "Relay port")
	pflag.String("relay.url", "", "Relay URL")
	pflag.Int("relay.max_clients", 0, "Relay max clients")
	pflag.String("storage.type", "", "Storage type")
	pflag.String("storage.host", "", "Storage host")
	pflag.Int("storage.port", 0, "Storage port")
	pflag.String("storage.user", "", "Storage user")
	pflag.String("storage.password", "", "Storage password")
	pflag.String("storage.name", "", "Storage password")
	pflag.Bool("auth.no_auth", true, "Don't use TLS, relay is behind a proxy")
	pflag.String("auth.cert", "", "Path to server.cert password")
	pflag.String("auth.key", "", "Path to server.key password")

	pflag.Parse()

	// Initialize Viper
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("yaml")   // required if the config file does not have the extension in the name
	viper.AddConfigPath(".")      // optionally look for config in the working directory

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
		return nil, err
	}

	// Bind environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("nostrodomo") // add a prefix to avoid conflicts
	viper.BindEnv("logging.log_level")
	viper.BindEnv("logging.format")
	viper.BindEnv("logging.output")
	viper.BindEnv("logging.file_path")
	viper.BindEnv("relay.host")
	viper.BindEnv("relay.port")
	viper.BindEnv("relay.max_clients")
	viper.BindEnv("storage.type")
	viper.BindEnv("storage.host")
	viper.BindEnv("storage.port")
	viper.BindEnv("storage.user")
	viper.BindEnv("storage.password")
	viper.BindEnv("storage.name")
	viper.BindEnv("auth.no_auth")
	viper.BindEnv("auth.cert")
	viper.BindEnv("auth.key")

	// Bind command line flags
	viper.BindPFlags(pflag.CommandLine)

	// Unmarshal the config into the struct
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Unable to unmarshal config into struct: %v", err)
		return nil, err
	}

	return &config, nil
}
