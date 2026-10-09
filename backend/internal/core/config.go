package core

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	System   SystemConfig   `yaml:"system"`
	LXC      LXCConfig      `yaml:"lxc"`
	Traffic  TrafficConfig  `yaml:"traffic"`
	Task     TaskConfig     `yaml:"task"`
	Admin    AdminConfig    `yaml:"admin"`
	Plugins  PluginsConfig  `yaml:"plugins"`
}

type SystemConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Security SecurityConfig `yaml:"security"`
	Logger   LoggerConfig   `yaml:"logger"`
	Database DatabaseConfig `yaml:"database"`
}

type ServerConfig struct {
	Host string    `yaml:"host"`
	Port int       `yaml:"port"`
	Mode string    `yaml:"mode"`
	TLS  TLSConfig `yaml:"tls"`
	// PublicURL 对外访问地址（如 https://panel.example.com），用于生成激活/重置邮件链接；
	// 留空时根据请求头（X-Forwarded-Proto/Host）推断。
	PublicURL string `yaml:"public_url"`
	// TrustedProxies 信任的反向代理地址/网段，用于正确获取客户端 IP（登录限流依赖它）；
	// 留空默认信任本机与内网地址。
	TrustedProxies []string `yaml:"trusted_proxies"`
	// SecureCookie 会话 Cookie 的 Secure 标志：auto（默认，按请求是否 HTTPS 判断）/ true / false
	SecureCookie string `yaml:"secure_cookie"`
}

type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	AutoGenerate bool   `yaml:"auto_generate"`
}

type SecurityConfig struct {
	APIHash       string `yaml:"api_hash"`
	EnableCaptcha bool   `yaml:"enable_captcha"`
}

type LoggerConfig struct {
	Level    string `yaml:"level"`
	Colorful bool   `yaml:"colorful"`
}

type LXCConfig struct {
	Socket         string `yaml:"socket"`
	Timeout        int    `yaml:"timeout"`
	DefaultStorage string `yaml:"default_storage"`
}

type TrafficConfig struct {
	Enabled   bool `yaml:"enabled"`
	Interval  int  `yaml:"interval"`
	BatchSize int  `yaml:"batch_size"`
}

type TaskConfig struct {
	Enabled         bool        `yaml:"enabled"`
	Backend         string      `yaml:"backend"`
	Workers         int         `yaml:"workers"`
	QueueSize       int         `yaml:"queue_size"`
	Timeout         int         `yaml:"timeout"`
	AutoCleanupDays int         `yaml:"auto_cleanup_days"`
	Redis           RedisConfig `yaml:"redis"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	MySQL    MySQLConfig    `yaml:"mysql"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"sslmode"`
}

type PluginsConfig struct {
	Nginx PluginConfig `yaml:"nginx"`
}

type PluginConfig struct {
	Enabled bool `yaml:"enabled"`
}

type AdminConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	SessionSecret string `yaml:"session_secret"`
}

var GlobalConfig *Config

func LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	GlobalConfig = &Config{}
	return yaml.Unmarshal(data, GlobalConfig)
}


// IsPlaceholder 判断配置值是否为空或仍是模板占位符（如 __API_HASH__、CHANGE_ME），
// 这类值不能当作真实密钥/密码使用。
func IsPlaceholder(v string) bool {
	s := strings.TrimSpace(v)
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "__") && strings.HasSuffix(s, "__") {
		return true
	}
	switch strings.ToLower(s) {
	case "change_me", "changeme", "change-me", "your_api_hash", "your_password":
		return true
	}
	return false
}
