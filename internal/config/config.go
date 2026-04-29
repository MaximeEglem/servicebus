package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	CurrentContext string             `yaml:"current-context"`
	Contexts       map[string]Context `yaml:"contexts"`
	Settings       Settings           `yaml:"settings"`
}

type Context struct {
	ConnectionString      string    `yaml:"connection-string,omitempty"`
	Namespace             string    `yaml:"namespace,omitempty"`
	UseDefaultCredentials bool      `yaml:"use-default-credentials,omitempty"`
	Favorites             Favorites `yaml:"favorites,omitempty"`
}

// Favorites are bubbled to the top of every list/sort output and surfaced
// in `sb overview`. Subscriptions are stored as "topic/subscription".
type Favorites struct {
	Queues        []string `yaml:"queues,omitempty"`
	Topics        []string `yaml:"topics,omitempty"`
	Subscriptions []string `yaml:"subscriptions,omitempty"`
}

func (f Favorites) HasQueue(name string) bool { return contains(f.Queues, name) }
func (f Favorites) HasTopic(name string) bool { return contains(f.Topics, name) }
func (f Favorites) HasSubscription(topic, name string) bool {
	return contains(f.Subscriptions, topic+"/"+name)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

type Settings struct {
	MaxItems            int        `yaml:"max-items"`
	WorkingDirectory    string     `yaml:"working-directory"`
	CacheExpirationDays int        `yaml:"cache-expiration-days"`
	Thresholds          Thresholds `yaml:"thresholds"`
}

type Thresholds struct {
	SizePercentage int `yaml:"size-percentage"`
	LastAccessDays int `yaml:"last-access-days"`
}

func defaults() *Config {
	return &Config{
		Contexts: map[string]Context{},
		Settings: Settings{
			MaxItems:            100,
			WorkingDirectory:    "~/.sb/exports",
			CacheExpirationDays: 30,
			Thresholds:          Thresholds{SizePercentage: 70, LastAccessDays: 30},
		},
	}
}

func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if os.IsNotExist(err) {
		return defaults(), nil
	}
	if err != nil {
		return nil, err
	}
	cfg := defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.Contexts == nil {
		cfg.Contexts = map[string]Context{}
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0600)
}
