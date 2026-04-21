package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultConfigPath returns ~/.config/ishtrak/config.toml
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ishtrak", "config.toml")
}

// DefaultLogPath returns ~/.config/ishtrak/ishtrak.log
func DefaultLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ishtrak", "ishtrak.log")
}

// Config is the top-level configuration structure.
type Config struct {
	ExtensionID string                     `toml:"extensionId"`
	Defaults    Defaults                   `toml:"defaults"`
	Platforms   map[string]PlatformConfig  `toml:"platforms"`
}

// Defaults holds project-wide default behaviour.
type Defaults struct {
	StoryPattern         string `toml:"storyPattern"`
	SkipIfNoStory        bool   `toml:"skipIfNoStory"`
	TaskTitleTemplate    string `toml:"taskTitleTemplate"`
	TaskDescTemplate     string `toml:"taskDescriptionTemplate"`
	MessagingTimeoutSecs int    `toml:"messagingTimeoutSecs"`
}

// PlatformConfig holds per-platform credentials and defaults.
type PlatformConfig struct {
	Token            string `toml:"token"`
	DefaultProjectID string `toml:"defaultProjectId"`
}

// MessagingTimeout returns the configured duration or 5s default.
func (c *Config) MessagingTimeout() time.Duration {
	if c.Defaults.MessagingTimeoutSecs <= 0 {
		return 5 * time.Second
	}
	return time.Duration(c.Defaults.MessagingTimeoutSecs) * time.Second
}

// Load reads and parses the config file at path.
func Load(path string) (*Config, error) {
	cfg := defaults()
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // return defaults if file doesn't exist yet
		}
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg to path, creating parent directories as needed.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// Skeleton returns an empty config with placeholder comments written as a
// raw string (TOML doesn't support comments via the encoder).
func Skeleton(extensionID string) string {
	return fmt.Sprintf(`# Ishtrak configuration
# Run 'ishtrak init' to regenerate this file.

extensionId = %q

[defaults]
storyPattern = "([A-Z]{2,10}-[0-9]{1,6})"
skipIfNoStory = true
taskTitleTemplate = "{storyId}: {commitSubject}"
taskDescriptionTemplate = "Commit: {commitHash}\nBranch: {branch}\n\n{commitBody}"
messagingTimeoutSecs = 5

# Add platforms below. Example:
# [platforms."tasks.acme.internal"]
# token = "pat_xxxxxxxxxxxxxxxx"
# defaultProjectId = "proj-abc123"
`, extensionID)
}

func defaults() *Config {
	return &Config{
		Defaults: Defaults{
			StoryPattern:         `([A-Z]{2,10}-[0-9]{1,6})`,
			SkipIfNoStory:        true,
			TaskTitleTemplate:    "{storyId}: {commitSubject}",
			TaskDescTemplate:     "Commit: {commitHash}\nBranch: {branch}\n\n{commitBody}",
			MessagingTimeoutSecs: 5,
		},
		Platforms: make(map[string]PlatformConfig),
	}
}
