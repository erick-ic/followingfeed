package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Config 汇总 API 服务及其辅助命令共享的全部配置。
type Config struct {
	Env           string              `mapstructure:"env"           yaml:"env"`           // 标识当前运行环境，例如 development 或 production。
	Server        ServerConfig        `mapstructure:"server"        yaml:"server"`        // HTTP 服务配置。
	MySQL         MySQLConfig         `mapstructure:"mysql"         yaml:"mysql"`         // 数据库连接和连接池配置。
	Redis         RedisConfig         `mapstructure:"redis"         yaml:"redis"`         // 缓存服务配置。
	RateLimit     RateLimitConfig     `mapstructure:"rate_limit"    yaml:"rate_limit"`    // 基于 Redis 的客户端 IP 限流配置。
	JWT           JWTConfig           `mapstructure:"jwt"           yaml:"jwt"`           // 令牌签名配置。
	CORS          CORSConfig          `mapstructure:"cors"          yaml:"cors"`          // 跨域访问策略。
	Observability ObservabilityConfig `mapstructure:"observability" yaml:"observability"` // 监控指标服务配置。
	Swagger       SwaggerConfig       `mapstructure:"swagger"       yaml:"swagger"`       // 交互式 API 文档配置。
	Migration     MigrationConfig     `mapstructure:"migration"     yaml:"migration"`     // 数据库迁移配置。
}

// Load 加载并校验 API 服务运行所需的完整配置。
func Load() (Config, error) {
	var cfg Config
	if err := load(&cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// load 合并默认值、YAML 和环境变量并解码到 target，不执行使用场景相关的校验。
func load(target any) error {
	// .env 仅用于补充本地环境变量；文件不存在时继续使用系统环境变量和 YAML。
	if err := gotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("加载 .env 文件失败：%w", err)
	}

	// 使用独立实例，避免 Viper 的包级全局状态污染其他命令或测试。
	v := viper.New()
	v.SetConfigType("yaml")
	// 将 server.read_timeout 映射为 FOLLOWINGFEED_SERVER_READ_TIMEOUT 等环境变量。
	v.SetEnvPrefix("FOLLOWINGFEED")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 部署环境可指定配置文件；本地开发默认使用 dev.yaml。
	configFile := os.Getenv("FOLLOWINGFEED_CONFIG_FILE")
	if configFile == "" {
		configFile = "./config/dev.yaml"
	}
	v.SetConfigFile(configFile)

	setDefaults(v)

	// 配置文件不存在时沿用环境变量和默认值；其他读取或解析错误直接返回。
	if err := v.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取配置文件失败：%w", err)
	}

	if err := v.Unmarshal(target); err != nil {
		return fmt.Errorf("解析配置内容失败：%w", err)
	}

	return nil
}

// setDefaults 同时声明可通过环境变量覆盖的配置项。
// 优先级为：环境变量 > YAML > 默认值。
func setDefaults(v *viper.Viper) {
	v.SetDefault("env", "development")
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.read_header_timeout", "5s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.shutdown_timeout", "15s")
	v.SetDefault("server.trusted_proxies", []string{})
	v.SetDefault("mysql.host", "")
	v.SetDefault("mysql.port", "3306")
	v.SetDefault("mysql.user", "")
	v.SetDefault("mysql.password", "")
	v.SetDefault("mysql.database", "")
	v.SetDefault("mysql.dsn", "")
	v.SetDefault("mysql.max_open_conns", 25)
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.conn_max_lifetime", "30m")
	v.SetDefault("mysql.connect_timeout", "3s")
	v.SetDefault("mysql.read_timeout", "5s")
	v.SetDefault("mysql.write_timeout", "5s")
	v.SetDefault("mysql.query_timeout", "3s")
	v.SetDefault("redis.addr", "")
	v.SetDefault("rate_limit.window", "1s")
	v.SetDefault("rate_limit.threshold", 100)
	v.SetDefault("jwt.access_token_key", "")
	v.SetDefault("jwt.refresh_token_key", "")
	v.SetDefault("cors.allowed_origins", []string{})
	v.SetDefault("observability.metrics_addr", ":8081")
	v.SetDefault("swagger.enabled", false)
	v.SetDefault("migration.timeout", "30m")
	v.SetDefault("migration.lock_timeout", 30)
	v.SetDefault("migration.dsn", "")
}

// validate 校验 API 服务启动所需的完整配置。
// 只在启动时调用一次，复制成本可以忽略，值接收者表达只读语义。
func (cfg Config) validate() error {
	if cfg.Server.Addr == "" {
		return errors.New("服务监听地址不能为空")
	}
	if cfg.Server.ReadTimeout <= 0 || cfg.Server.ReadHeaderTimeout <= 0 ||
		cfg.Server.WriteTimeout <= 0 || cfg.Server.IdleTimeout <= 0 || cfg.Server.ShutdownTimeout <= 0 {
		return errors.New("服务超时时间必须大于 0")
	}
	for _, proxy := range cfg.Server.TrustedProxies {
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("可信代理地址无效：%s", proxy)
			}
		}
	}
	if cfg.MySQL.DSN == "" && (cfg.MySQL.Host == "" || cfg.MySQL.Port == "" ||
		cfg.MySQL.User == "" || cfg.MySQL.Database == "") {
		return errors.New("MySQL 配置不完整")
	}
	if cfg.MySQL.MaxOpenConns <= 0 || cfg.MySQL.MaxIdleConns < 0 ||
		cfg.MySQL.MaxIdleConns > cfg.MySQL.MaxOpenConns || cfg.MySQL.ConnMaxLifetime <= 0 ||
		cfg.MySQL.ConnectTimeout <= 0 || cfg.MySQL.ReadTimeout <= 0 ||
		cfg.MySQL.WriteTimeout <= 0 || cfg.MySQL.QueryTimeout <= 0 {
		return errors.New("MySQL 连接池配置无效")
	}
	if cfg.MySQL.DSN != "" {
		if _, err := mysqlDriver.ParseDSN(cfg.MySQL.DSN); err != nil {
			return fmt.Errorf("MySQL DSN 无效：%w", err)
		}
	}
	if cfg.Redis.Addr == "" {
		return errors.New("Redis 地址不能为空")
	}
	if cfg.RateLimit.Window <= 0 || cfg.RateLimit.Threshold <= 0 {
		return errors.New("限流窗口和阈值必须大于 0")
	}
	if len(cfg.JWT.AccessTokenKey) < 32 || len(cfg.JWT.RefreshTokenKey) < 32 {
		return errors.New("JWT 签名密钥长度均不得少于 32 个字符")
	}
	if cfg.JWT.AccessTokenKey == cfg.JWT.RefreshTokenKey {
		return errors.New("访问令牌和刷新令牌不得使用相同签名密钥")
	}
	if cfg.Env == "production" && len(cfg.CORS.AllowedOrigins) == 0 {
		return errors.New("生产环境的 CORS 允许来源不能为空")
	}
	for _, origin := range cfg.CORS.AllowedOrigins {
		if origin == "*" {
			return errors.New("携带认证 Cookie 时 CORS 不允许使用通配来源")
		}
	}
	if cfg.Observability.MetricsAddr == "" {
		return errors.New("监控指标监听地址不能为空")
	}

	return nil
}

// ServerConfig 定义 HTTP 服务的监听地址和生命周期超时。
type ServerConfig struct {
	Addr              string        `mapstructure:"addr"                yaml:"addr"`                // HTTP 服务监听地址，格式为 host:port；省略 host 时监听所有网卡。
	ReadTimeout       time.Duration `mapstructure:"read_timeout"        yaml:"read_timeout"`        // 读取完整请求的最长时间。
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout" yaml:"read_header_timeout"` // 读取请求头的最长时间。
	WriteTimeout      time.Duration `mapstructure:"write_timeout"       yaml:"write_timeout"`       // 写入响应的最长时间。
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"        yaml:"idle_timeout"`        // 空闲 Keep-Alive 连接的保留时间。
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"    yaml:"shutdown_timeout"`    // 服务优雅关闭的等待时间。
	TrustedProxies    []string      `mapstructure:"trusted_proxies"     yaml:"trusted_proxies"`     // 允许提供客户端转发 IP 的反向代理 IP 或 CIDR；留空表示不信任代理请求头。
}

// MySQLConfig 定义数据库连接信息和连接池参数。
type MySQLConfig struct {
	Host            string        `mapstructure:"host"              yaml:"host"`              // MySQL 服务的主机名或 IP 地址。
	Port            string        `mapstructure:"port"              yaml:"port"`              // MySQL 服务端口。
	User            string        `mapstructure:"user"              yaml:"user"`              // 应用使用的数据库账号。
	Password        string        `mapstructure:"password"          yaml:"password"`          // 数据库账号密码。
	Database        string        `mapstructure:"database"          yaml:"database"`          // 默认连接的数据库名称。
	DSN             string        `mapstructure:"dsn"               yaml:"dsn"`               // 可选的完整连接字符串；设置后优先于分项连接配置。
	MaxOpenConns    int           `mapstructure:"max_open_conns"    yaml:"max_open_conns"`    // 连接池允许同时打开的最大连接数。
	MaxIdleConns    int           `mapstructure:"max_idle_conns"    yaml:"max_idle_conns"`    // 连接池保留的最大空闲连接数。
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"` // 单个数据库连接可复用的最长时间。
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"   yaml:"connect_timeout"`   // 建立 MySQL 网络连接的最长时间。
	ReadTimeout     time.Duration `mapstructure:"read_timeout"      yaml:"read_timeout"`      // 驱动从 MySQL 读取单次网络响应的最长时间。
	WriteTimeout    time.Duration `mapstructure:"write_timeout"     yaml:"write_timeout"`     // 驱动向 MySQL 写入单次网络请求的最长时间。
	QueryTimeout    time.Duration `mapstructure:"query_timeout"     yaml:"query_timeout"`     // 每条 GORM 业务 SQL 的最长执行时间。
}

// GetDSN 优先使用完整 DSN，否则根据分项配置生成连接字符串。
func (mc MySQLConfig) GetDSN() string {
	if mc.DSN != "" {
		dsn, err := mysqlDriver.ParseDSN(mc.DSN)
		if err != nil {
			// validate/validateMigration 会返回不包含密码的配置错误；
			// 这里保留原值，便于单独使用 GetDSN 的调用方获得驱动错误。
			return mc.DSN
		}
		applyMySQLRuntimeOptions(dsn, mc)
		return dsn.FormatDSN()
	}
	dsn := mysqlDriver.NewConfig()
	dsn.User = mc.User
	dsn.Passwd = mc.Password
	dsn.Net = "tcp"
	dsn.Addr = net.JoinHostPort(mc.Host, mc.Port)
	dsn.DBName = mc.Database
	dsn.ParseTime = true
	dsn.Loc = time.Local
	dsn.Params = map[string]string{"charset": "utf8mb4"}
	applyMySQLRuntimeOptions(dsn, mc)
	return dsn.FormatDSN()
}

func applyMySQLRuntimeOptions(dsn *mysqlDriver.Config, cfg MySQLConfig) {
	dsn.Timeout = cfg.ConnectTimeout
	dsn.ReadTimeout = cfg.ReadTimeout
	dsn.WriteTimeout = cfg.WriteTimeout
}

// RedisConfig 定义 Redis 服务地址。
type RedisConfig struct {
	Addr string `mapstructure:"addr" yaml:"addr"` // Redis 服务地址，格式为 host:port。
}

// RateLimitConfig 定义按客户端 IP 计算的分布式滑动窗口限流参数。
type RateLimitConfig struct {
	Window    time.Duration `mapstructure:"window"    yaml:"window"`    // 统计窗口长度。
	Threshold int           `mapstructure:"threshold" yaml:"threshold"` // 单个窗口内允许的最大请求数。
}

// JWTConfig 定义访问令牌和刷新令牌的签名密钥。
type JWTConfig struct {
	AccessTokenKey  string `mapstructure:"access_token_key"  yaml:"access_token_key"`  // 用于签名和验证访问令牌。
	RefreshTokenKey string `mapstructure:"refresh_token_key" yaml:"refresh_token_key"` // 用于签名和验证刷新令牌。
}

// CORSConfig 定义允许跨域访问 API 的来源。
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins" yaml:"allowed_origins"` // 允许跨域访问 API 的来源；生产环境不得为空。
}

// ObservabilityConfig 定义独立的监控指标监听地址。
type ObservabilityConfig struct {
	MetricsAddr string `mapstructure:"metrics_addr" yaml:"metrics_addr"` // Prometheus 指标服务的监听地址。
}

// SwaggerConfig 只控制交互式 Swagger UI；OpenAPI YAML 始终可用。
type SwaggerConfig struct {
	Enabled bool `mapstructure:"enabled" yaml:"enabled"` // 是否启用交互式 Swagger UI。
}

// MigrationConfig 定义迁移总超时、数据库锁等待时间和专用 DSN。
type MigrationConfig struct {
	Timeout     time.Duration `mapstructure:"timeout"      yaml:"timeout"`      // 整个数据库迁移过程的最长时间。
	LockTimeout int           `mapstructure:"lock_timeout" yaml:"lock_timeout"` // 等待数据库迁移锁的最长秒数。
	DSN         string        `mapstructure:"dsn"          yaml:"dsn"`          // 迁移专用连接字符串；留空时复用应用的 MySQL 配置。
}

// migrationSettings 限定迁移命令的解码范围，避免依赖 API 服务专用配置。
type migrationSettings struct {
	MySQL     MySQLConfig     `mapstructure:"mysql"`     // 未配置迁移专用 DSN 时使用的数据库连接信息。
	Migration MigrationConfig `mapstructure:"migration"` // 迁移命令的超时、锁和专用连接配置。
}

// LoadMigration 只校验迁移命令需要的数据库和超时配置，避免迁移任务依赖 Redis、JWT 或 CORS。
// API 服务和迁移命令是两个独立程序、两个独立进程，它们不会共享内存中的配置。
func LoadMigration() (Config, error) {
	var settings migrationSettings
	if err := load(&settings); err != nil {
		return Config{}, err
	}

	cfg := Config{
		MySQL:     settings.MySQL,
		Migration: settings.Migration,
	}
	if err := cfg.validateMigration(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// validateMigration 只校验执行数据库迁移所必需的配置。
func (cfg Config) validateMigration() error {
	if cfg.Migration.Timeout <= 0 || cfg.Migration.LockTimeout < 0 {
		return errors.New("迁移超时配置无效")
	}
	if cfg.Migration.DSN == "" && cfg.MySQL.DSN == "" &&
		(cfg.MySQL.Host == "" || cfg.MySQL.Port == "" || cfg.MySQL.User == "" || cfg.MySQL.Database == "") {
		return errors.New("迁移所需的 MySQL 配置不完整")
	}
	if cfg.Migration.DSN != "" {
		if _, err := mysqlDriver.ParseDSN(cfg.Migration.DSN); err != nil {
			return fmt.Errorf("迁移 MySQL DSN 无效：%w", err)
		}
	}
	return nil
}

// MigrationDSN 优先使用迁移专用 DSN，否则复用应用的 MySQL 配置。
func (cfg Config) MigrationDSN() string {
	if cfg.Migration.DSN != "" {
		return cfg.Migration.DSN
	}
	return cfg.MySQL.GetDSN()
}
