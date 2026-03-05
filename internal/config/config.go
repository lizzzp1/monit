package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Endpoints  []Endpoint `yaml:"endpoints"`
	LogWatches []LogWatch `yaml:"log_watches"`
	Slack      Slack      `yaml:"slack"`
	Datadog    Datadog    `yaml:"datadog"`
	CircleCI   CircleCI   `yaml:"circleci"`
	OpenAI     OpenAI     `yaml:"openai"`
	GitHub     GitHub     `yaml:"github"`
}

type Endpoint struct {
	Name               string `yaml:"name"`
	URL                string `yaml:"url"`
	Service            string `yaml:"service"`
	ErrorThreshold     int    `yaml:"error_threshold"`
	PollInterval       int    `yaml:"poll_interval"`
	CircleCITriggerURL string `yaml:"circleci_trigger_url"`
}

type LogWatch struct {
	Name      string `yaml:"name"`
	Query     string `yaml:"query"`
	Service   string `yaml:"service"`
	Pattern   string `yaml:"pattern"`
	Threshold int    `yaml:"threshold"`
}

type Slack struct {
	BotToken string `yaml:"bot_token"`
	Channel  string `yaml:"channel"`
}

type Datadog struct {
	APIKey string `yaml:"api_key"`
	AppKey string `yaml:"app_key"`
}

type CircleCI struct {
	APIToken string `yaml:"api_token"`
}

type OpenAI struct {
	APIKey string `yaml:"api_key"`
}

type GitHub struct {
	APIToken   string `yaml:"api_token"`
	Owner      string `yaml:"owner"`
	Repo       string `yaml:"repo"`
	BaseBranch string `yaml:"base_branch"`
}

func Load() (*Config, error) {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Endpoints == nil {
		cfg.Endpoints = []Endpoint{}
	}

	if cfg.LogWatches == nil {
		cfg.LogWatches = []LogWatch{}
	}

	cfg.loadEnvVars()

	return &cfg, nil
}

func (c *Config) loadEnvVars() {
	if c.Slack.BotToken == "" {
		c.Slack.BotToken = os.Getenv("SLACK_BOT_TOKEN")
	}
	if c.Slack.Channel == "" {
		c.Slack.Channel = os.Getenv("SLACK_CHANNEL")
	}

	if c.Datadog.APIKey == "" {
		c.Datadog.APIKey = os.Getenv("DATADOG_API_KEY")
	}
	if c.Datadog.AppKey == "" {
		c.Datadog.AppKey = os.Getenv("DATADOG_APP_KEY")
	}

	if c.CircleCI.APIToken == "" {
		c.CircleCI.APIToken = os.Getenv("CIRCLECI_API_TOKEN")
	}

	if c.OpenAI.APIKey == "" {
		c.OpenAI.APIKey = os.Getenv("OPENAI_API_KEY")
	}

	if c.GitHub.APIToken == "" {
		c.GitHub.APIToken = os.Getenv("GITHUB_API_TOKEN")
	}
}

func defaultConfig() *Config {
	return &Config{
		Endpoints:  []Endpoint{},
		LogWatches: []LogWatch{},
		Slack:      Slack{},
		Datadog:    Datadog{},
		CircleCI:   CircleCI{},
		OpenAI:     OpenAI{},
		GitHub:     GitHub{},
	}
}

func (c *Config) Services() []string {
	seen := make(map[string]bool)
	var services []string

	for _, ep := range c.Endpoints {
		if ep.Service != "" && !seen[ep.Service] {
			services = append(services, ep.Service)
			seen[ep.Service] = true
		}
	}

	for _, lw := range c.LogWatches {
		if lw.Service != "" && !seen[lw.Service] {
			services = append(services, lw.Service)
			seen[lw.Service] = true
		}
	}

	return services
}
