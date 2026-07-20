// Copyright 2026 [Copyright Holder]
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: [YOUR_NAME]

package config

import (
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	HTTPS    HTTPSConfig    `mapstructure:"https"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Security SecurityConfig `mapstructure:"security"`
}

type ServerConfig struct {
	Listen     string `mapstructure:"listen"`
	ListenHTTP string `mapstructure:"listen_http"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type HTTPSConfig struct {
	Enabled  bool     `mapstructure:"enabled"`
	Domains  []string `mapstructure:"domains"`
	Email    string   `mapstructure:"email"`
	CacheDir string   `mapstructure:"cache_dir"`
}

type AuthConfig struct {
	Basic        BasicAuthConfig `mapstructure:"basic"`
	BearerTokens []string        `mapstructure:"bearer_tokens"`
}

type BasicAuthConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type SecurityConfig struct {
	RateLimit   int        `mapstructure:"rate_limit"`
	MaxBodySize int64      `mapstructure:"max_body_size"`
	CORS        CORSConfig `mapstructure:"cors"`
}

type CORSConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// Manager coordinates concurrency-safe configuration access and reload operations
type Manager struct {
	mu  sync.RWMutex
	cfg *Config
}

func NewManager(cfg *Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) Update(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
}

func Load(configPath string) (*Config, error) {
	v := viper.New()
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// Support environment variables prefixes
	v.SetEnvPrefix("TF_HTTP_BACKEND")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind default values
	v.SetDefault("server.listen", ":443")
	v.SetDefault("server.listen_http", ":80")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "terraform")
	v.SetDefault("database.password", "password")
	v.SetDefault("database.database", "tfstate")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 25)
	v.SetDefault("database.conn_max_lifetime", 15*time.Minute)
	v.SetDefault("https.enabled", false)
	v.SetDefault("https.cache_dir", "/var/lib/tf-http-backend/certs")
	v.SetDefault("logging.level", "info")
	v.SetDefault("security.rate_limit", 100)
	v.SetDefault("security.max_body_size", int64(52428800)) // 50MB

	if err := v.ReadInConfig(); err != nil {
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
