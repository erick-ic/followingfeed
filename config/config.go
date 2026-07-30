package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

type Config struct {
	Env    string       `mapstructure:"env" yaml:"env"`
	Server ServerConfig `mapstructure:"server" yaml:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql" yaml:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis" yaml:"redis"`
	JWT    JWTConfig    `mapstructure:"jwt" yaml:"jwt"`
	CORS   CORSConfig   `mapstructure:"cors" yaml:"cors"`
}

func Load() (Config, error) {
	if err := gotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("FOLLOWINGFEED")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	configFile := os.Getenv("FOLLOWINGFEED_CONFIG_FILE")
	if configFile == "" {
		configFile = "./config/dev.yaml"
	}
	v.SetConfigFile(configFile)

	// SetDefault 同时让 Viper 能够识别相应的环境变量，默认值。
	// 环境变量 > config/dev.yaml > SetDefault 默认值
	v.SetDefault("env", "development")
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.read_header_timeout", "5s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.shutdown_timeout", "15s")
	v.SetDefault("mysql.host", "")
	v.SetDefault("mysql.port", "3306")
	v.SetDefault("mysql.user", "")
	v.SetDefault("mysql.password", "")
	v.SetDefault("mysql.database", "")
	v.SetDefault("mysql.dsn", "")
	v.SetDefault("mysql.max_open_conns", 25)
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.conn_max_lifetime", "30m")
	v.SetDefault("redis.addr", "")
	v.SetDefault("jwt.access_token_key", "")
	v.SetDefault("jwt.refresh_token_key", "")
	v.SetDefault("cors.allowed_origins", []string{})

	if err := v.ReadInConfig(); err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	cfg.CORS.AllowedOrigins = v.GetStringSlice("cors.allowed_origins")

	if cfg.Server.Addr == "" || cfg.MySQL.Host == "" || cfg.MySQL.Database == "" || cfg.Redis.Addr == "" {
		return Config{}, errors.New("database config is incomplete")
	}
	if len(cfg.JWT.AccessTokenKey) < 32 || len(cfg.JWT.RefreshTokenKey) < 32 {
		return Config{}, errors.New("JWT signing keys must each contain at least 32 characters")
	}
	if cfg.Env == "production" && len(cfg.CORS.AllowedOrigins) == 0 {
		return Config{}, errors.New("production CORS allowed origins must not be empty")
	}

	return cfg, nil
}

type ServerConfig struct {
	Addr              string `mapstructure:"addr" yaml:"addr"`
	ReadTimeout       string `mapstructure:"read_timeout" yaml:"read_timeout"`
	ReadHeaderTimeout string `mapstructure:"read_header_timeout" yaml:"read_header_timeout"`
	WriteTimeout      string `mapstructure:"write_timeout" yaml:"write_timeout"`
	IdleTimeout       string `mapstructure:"idle_timeout" yaml:"idle_timeout"`
	ShutdownTimeout   string `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
}

type JWTConfig struct {
	AccessTokenKey  string `mapstructure:"access_token_key" yaml:"access_token_key"`
	RefreshTokenKey string `mapstructure:"refresh_token_key" yaml:"refresh_token_key"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins" yaml:"allowed_origins"`
}

type MySQLConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Database        string
	DSN             string
	MaxOpenConns    int    `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
}

func (mc *MySQLConfig) GetDSN() string {
	if mc.DSN != "" {
		return mc.DSN
	}
	textCode := "?charset=utf8mb4&parseTime=true&loc=Local"
	return mc.User + ":" + mc.Password + "@tcp(" + mc.Host + ":" + mc.Port + ")/" + mc.Database + textCode
}

type RedisConfig struct {
	Addr string
}
