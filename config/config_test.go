package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAllowsMissingConfigFile(t *testing.T) {
	t.Setenv("FOLLOWINGFEED_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))

	var cfg Config
	require.NoError(t, load(&cfg))
	assert.Equal(t, ":8080", cfg.Server.Addr)
}

func TestLoadRejectsConfigReadError(t *testing.T) {
	t.Setenv("FOLLOWINGFEED_CONFIG_FILE", t.TempDir())

	var cfg Config
	err := load(&cfg)
	require.ErrorContains(t, err, "读取配置文件失败")
}

func TestLoadMigrationIgnoresUnrelatedConfig(t *testing.T) {
	t.Setenv("FOLLOWINGFEED_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("FOLLOWINGFEED_SERVER_READ_TIMEOUT", "invalid")
	t.Setenv("FOLLOWINGFEED_MIGRATION_DSN", "migrator:secret@tcp(mysql:3306)/followingfeed")

	_, err := LoadMigration()
	require.NoError(t, err)
}

func TestViperUnmarshalConfig(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("mysql.host", "127.0.0.1")
	v.Set("mysql.user", "followingfeed")
	v.Set("mysql.database", "followingfeed")
	v.Set("redis.addr", "127.0.0.1:6379")
	v.Set("jwt.access_token_key", "access-token-key-at-least-32-characters")
	v.Set("jwt.refresh_token_key", "refresh-token-key-at-least-32-characters")
	v.Set("cors.allowed_origins", "https://a.example,https://b.example")
	v.Set("server.trusted_proxies", "10.0.0.0/8,192.0.2.10")

	var cfg Config
	require.NoError(t, v.Unmarshal(&cfg))
	require.NoError(t, cfg.validate())
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, []string{"https://a.example", "https://b.example"}, cfg.CORS.AllowedOrigins)
	assert.Equal(t, []string{"10.0.0.0/8", "192.0.2.10"}, cfg.Server.TrustedProxies)
	assert.False(t, cfg.Swagger.Enabled)
	assert.Equal(t, 30*time.Minute, cfg.Migration.Timeout)
	assert.Equal(t, 30*time.Minute, cfg.MySQL.ConnMaxLifetime)
	assert.Equal(t, 3*time.Second, cfg.MySQL.ConnectTimeout)
	assert.Equal(t, 5*time.Second, cfg.MySQL.ReadTimeout)
	assert.Equal(t, 5*time.Second, cfg.MySQL.WriteTimeout)
	assert.Equal(t, 3*time.Second, cfg.MySQL.QueryTimeout)
	assert.Equal(t, time.Second, cfg.RateLimit.Window)
	assert.Equal(t, 100, cfg.RateLimit.Threshold)
}

func TestViperUnmarshalEnablesSwaggerUI(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("swagger.enabled", true)

	var cfg Config
	require.NoError(t, v.Unmarshal(&cfg))
	assert.True(t, cfg.Swagger.Enabled)
}

func TestViperUnmarshalRejectsInvalidDuration(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("server.read_timeout", "invalid")

	var cfg Config
	require.Error(t, v.Unmarshal(&cfg))
}

func TestValidateRejectsInvalidConnectionPool(t *testing.T) {
	cfg := validConfig()
	cfg.MySQL.MaxOpenConns = 2
	cfg.MySQL.MaxIdleConns = 3

	assert.EqualError(t, cfg.validate(), "MySQL 连接池配置无效")
}

func TestValidateDoesNotRequireMigrationConfig(t *testing.T) {
	require.NoError(t, validConfig().validate())
}

func TestValidateRejectsSharedJWTSigningKey(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.RefreshTokenKey = cfg.JWT.AccessTokenKey

	assert.EqualError(t, cfg.validate(), "访问令牌和刷新令牌不得使用相同签名密钥")
}

func TestValidateRejectsWildcardCORSOrigin(t *testing.T) {
	cfg := validConfig()
	cfg.CORS.AllowedOrigins = []string{"*"}

	assert.EqualError(t, cfg.validate(), "携带认证 Cookie 时 CORS 不允许使用通配来源")
}

func TestValidateRejectsInvalidTrustedProxy(t *testing.T) {
	cfg := validConfig()
	cfg.Server.TrustedProxies = []string{"proxy.internal"}

	assert.EqualError(t, cfg.validate(), "可信代理地址无效：proxy.internal")
}

func TestValidateRejectsInvalidRateLimit(t *testing.T) {
	cfg := validConfig()
	cfg.RateLimit.Threshold = 0

	assert.EqualError(t, cfg.validate(), "限流窗口和阈值必须大于 0")
}

func validConfig() Config {
	return Config{
		Auth: AuthConfig{SignupEnabled: true, PublishingEnabled: true, RateWindow: time.Minute, RateThreshold: 10, MaxConcurrent: 2},
		Server: ServerConfig{
			Addr: ":8080", ReadTimeout: time.Second, ReadHeaderTimeout: time.Second,
			WriteTimeout: time.Second, IdleTimeout: time.Second, ShutdownTimeout: time.Second,
		},
		Observability: ObservabilityConfig{MetricsAddr: ":8081"},
		MySQL: MySQLConfig{
			Host: "mysql", Port: "3306", User: "app", Database: "app",
			MaxOpenConns: 3, MaxIdleConns: 2, ConnMaxLifetime: time.Minute,
			ConnectTimeout: time.Second, ReadTimeout: time.Second,
			WriteTimeout: time.Second, QueryTimeout: time.Second,
		},
		Redis: RedisConfig{Addr: "redis:6379"},
		RateLimit: RateLimitConfig{
			Window: time.Second, Threshold: 100,
		},
		JWT: JWTConfig{
			AccessTokenKey:  "access-token-key-at-least-32-characters",
			RefreshTokenKey: "refresh-token-key-at-least-32-characters",
		},
	}
}

func TestMySQLDSNIncludesConfiguredTimeouts(t *testing.T) {
	cfg := validConfig().MySQL
	dsn, err := mysql.ParseDSN(cfg.GetDSN())
	require.NoError(t, err)
	assert.Equal(t, cfg.ConnectTimeout, dsn.Timeout)
	assert.Equal(t, cfg.ReadTimeout, dsn.ReadTimeout)
	assert.Equal(t, cfg.WriteTimeout, dsn.WriteTimeout)
}

func TestExplicitMySQLDSNReceivesRuntimeSafetyOptions(t *testing.T) {
	cfg := validConfig().MySQL
	cfg.DSN = "app:secret@tcp(mysql:3306)/app?parseTime=true"
	dsn, err := mysql.ParseDSN(cfg.GetDSN())
	require.NoError(t, err)
	assert.Equal(t, cfg.ConnectTimeout, dsn.Timeout)
	assert.Equal(t, cfg.ReadTimeout, dsn.ReadTimeout)
	assert.Equal(t, cfg.WriteTimeout, dsn.WriteTimeout)
}

func TestValidateMigrationAcceptsDedicatedDSNWithoutApplicationConfig(t *testing.T) {
	cfg := Config{Migration: MigrationConfig{
		Timeout: time.Minute,
		DSN:     "migrator:secret@tcp(mysql:3306)/followingfeed",
	}}

	require.NoError(t, cfg.validateMigration())
	assert.Equal(t, cfg.Migration.DSN, cfg.MigrationDSN())
}

func TestProductionRejectsExampleKeyAndInvalidOrigin(t *testing.T) {
	cfg := validConfig()
	cfg.Env = "production"
	cfg.JWT.AccessTokenKey = "local-development-access-token-key-change-before-production"
	require.ErrorContains(t, cfg.validate(), "示例 JWT")
	for _, origin := range []string{"invalid", "https://example.com/path", "https://user:pass@example.com"} {
		cfg = validConfig()
		cfg.CORS.AllowedOrigins = []string{origin}
		require.ErrorContains(t, cfg.validate(), "origin")
	}
}
func TestNewAuthAndRedisEnvConfiguration(t *testing.T) {
	t.Setenv("FOLLOWINGFEED_AUTH_SIGNUP_ENABLED", "false")
	t.Setenv("FOLLOWINGFEED_REDIS_PASSWORD", "test-only")
	t.Setenv("FOLLOWINGFEED_REDIS_TLS", "true")
	var cfg Config
	require.NoError(t, load(&cfg))
	assert.False(t, cfg.Auth.SignupEnabled)
	assert.True(t, cfg.Redis.TLS)
	assert.Equal(t, "test-only", cfg.Redis.Password)
}
