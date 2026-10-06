package serverconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config is loaded from worldfiles/server.yaml.
// See that file for an example of this configuration.
type Config struct {
	ContentPath string `yaml:"contentPath"`
	Log         struct {
		File  string
		Level string
	}
	Telnet struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	// TLS is a second port beside telnet's, same host, speaking TLS. Off when
	// port is 0. cert and key are PEM files, re-read when they change on disk,
	// so a renewal needs no restart.
	TLS struct {
		Port int    `yaml:"port"`
		Cert string `yaml:"cert"`
		Key  string `yaml:"key"`
	} `yaml:"tls"`

	// Health is an HTTP port answering /healthz: 200 while the game loop is
	// ticking, 503 once it has stopped. Off when port is 0. It says nothing
	// a player needs, so bind it where only the box can reach it.
	Health struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"health"`

	Mongo MongoConfig `yaml:"mongo"`
}

// MongoConfig says where characters are persisted. An empty Uri means the
// in-memory store, which is a real choice: it is what you want for a throwaway
// server, and it is what you get if you haven't started docker-compose yet.
// Nothing falls back the other way -- a Uri that is set and wrong fails
// startup, because silently forgetting every character is worse than not
// starting.
type MongoConfig struct {
	Uri      string `yaml:"uri"`
	Database string `yaml:"database"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read server config: %w", err)
	}
	var cfg Config
	// parse the configuration file
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// The uri carries the database password, so a deployment keeps it out
	// of the file (which is checked in) and passes it in the environment --
	// a compose .env, or a kubernetes Secret.
	if uri := os.Getenv("WATCHMUD_MONGO_URI"); uri != "" {
		cfg.Mongo.Uri = uri
	}
	// verify contents
	if len(cfg.Telnet.Host) == 0 || cfg.Telnet.Port == 0 {
		return nil, fmt.Errorf("telnet host and port must be configured")
	}
	if cfg.TLS.Port != 0 && (cfg.TLS.Cert == "" || cfg.TLS.Key == "") {
		return nil, fmt.Errorf("tls.port is set, so tls.cert and tls.key must be too")
	}
	if cfg.Health.Port != 0 && cfg.Health.Host == "" {
		cfg.Health.Host = "127.0.0.1"
	}
	return &cfg, nil
}
