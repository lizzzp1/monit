package monitor

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"monit/internal/ai"
	"monit/internal/config"
	"monit/internal/datadog"
	"monit/internal/github"
	"monit/internal/slack"
	"monit/internal/state"
)

type Monitor struct {
	cfg        *config.Config
	slack      *slack.Client
	ddClient   *datadog.Client
	aiClient   *ai.Client
	ghClient   *github.Client
	httpClient *http.Client
	state      *state.State
	errors     map[string]int
	requests   map[string]int
	dryRun     bool
	mu         sync.Mutex
}

const (
	alertCooldown = 10 * time.Minute
	pollInterval  = 30 * time.Second
)

func New(cfg *config.Config, slackClient *slack.Client, dryRun bool) (*Monitor, error) {
	ddClient := datadog.New(cfg.Datadog)
	aiClient := ai.New(cfg.OpenAI)
	ghClient := github.New(cfg.GitHub)

	state, err := state.Load("monit_state.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	return &Monitor{
		cfg:        cfg,
		slack:      slackClient,
		ddClient:   ddClient,
		aiClient:   aiClient,
		ghClient:   ghClient,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		state:      state,
		errors:     make(map[string]int),
		requests:   make(map[string]int),
		dryRun:     dryRun,
	}, nil
}

func (m *Monitor) Run() error {
	fmt.Println("Starting monitoring... (Press Ctrl+C to stop)")

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		m.checkEndpoints()
		m.checkAlerts()
		m.checkLogWatches()

		if err := m.state.Save(); err != nil {
			fmt.Printf("Warning: failed to save state: %v\n", err)
		}

		<-ticker.C
	}
}

func (m *Monitor) checkEndpoints() {
	var wg sync.WaitGroup

	for _, ep := range m.cfg.Endpoints {
		wg.Add(1)
		go func(endpoint config.Endpoint) {
			defer wg.Done()

			resp, err := m.httpClient.Get(endpoint.URL)
			if err != nil {
				m.mu.Lock()
				m.errors[endpoint.Name]++
				m.requests[endpoint.Name]++
				m.mu.Unlock()
				return
			}
			resp.Body.Close()

			m.mu.Lock()
			m.requests[endpoint.Name]++
			if resp.StatusCode >= 400 {
				m.errors[endpoint.Name]++
			}
			m.mu.Unlock()
		}(ep)
	}

	wg.Wait()
}

func (m *Monitor) checkAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ep := range m.cfg.Endpoints {
		requests := m.requests[ep.Name]
		errors := m.errors[ep.Name]

		if requests == 0 {
			continue
		}

		errorRate := float64(errors) / float64(requests) * 100

		if float64(errorRate) > float64(ep.ErrorThreshold) {
			if m.state.IsSnoozed(ep.Name) {
				fmt.Printf("Snoozed: %s\n", ep.Name)
				m.resetCounters(ep.Name)
				continue
			}

			if !m.state.ShouldAlert(ep.Name, alertCooldown) {
				fmt.Printf("Cooldown: %s (last alert too recent)\n", ep.Name)
				m.resetCounters(ep.Name)
				continue
			}

			fmt.Printf("Alert: %s error rate %.1f%% exceeds threshold %d%%\n",
				ep.Name, errorRate, ep.ErrorThreshold)

			m.state.SetAlertActive(ep.Name)

			aiSummary := m.getAIDiagnostics(ep.Name)

			prURL := ""
			if m.cfg.GitHub.APIToken != "" && m.cfg.GitHub.Owner != "" {
				prURL, _ = m.ghClient.CreateRevertPR(
					m.cfg.GitHub.Owner,
					m.cfg.GitHub.Repo,
					m.cfg.GitHub.BaseBranch,
					fmt.Sprintf("Auto-revert for %s alert", ep.Name),
				)
			}

			alert := slack.Alert{
				Endpoint:     ep.Name,
				ErrorRate:    math.Round(errorRate*10) / 10,
				Threshold:    ep.ErrorThreshold,
				AIDiagnostic: aiSummary,
				TriggerURL:   ep.CircleCITriggerURL,
				PRURL:        prURL,
			}

			if m.dryRun {
				fmt.Printf("[DRY-RUN] Would send Slack alert: %+v\n", alert)
			} else {
				if err := m.slack.SendAlert(alert); err != nil {
					fmt.Printf("Failed to send Slack alert: %v\n", err)
				}
			}

			m.resetCounters(ep.Name)
		} else {
			if m.state.IsAlertActive(ep.Name) {
				m.state.ResolveAlert(ep.Name)
				fmt.Printf("Resolved: %s\n", ep.Name)
			}
			m.resetCounters(ep.Name)
		}
	}
}

func (m *Monitor) resetCounters(endpoint string) {
	m.errors[endpoint] = 0
	m.requests[endpoint] = 0
}

func (m *Monitor) getAIDiagnostics(endpoint string) string {
	if m.cfg.OpenAI.APIKey == "" {
		return "Enable OpenAI in config for AI diagnostics"
	}

	logs, err := m.ddClient.GetRecentLogs(endpoint, 15*time.Minute)
	if err != nil {
		return fmt.Sprintf("Failed to fetch logs: %v", err)
	}

	if logs == "" {
		return "No recent errors found in Datadog"
	}

	analysis, err := m.aiClient.Analyze(endpoint, logs)
	if err != nil {
		return fmt.Sprintf("Recent logs:\n%s\n\nAI analysis failed: %v", logs, err)
	}

	return fmt.Sprintf("%s\n\n🤖 AI Analysis: %s", logs, analysis)
}

func (m *Monitor) checkLogWatches() {
	if len(m.cfg.LogWatches) == 0 {
		return
	}

	for _, lw := range m.cfg.LogWatches {
		logKey := "log:" + lw.Name

		if m.state.IsSnoozed(logKey) {
			continue
		}

		if !m.state.ShouldAlert(logKey, alertCooldown) {
			continue
		}

		count, err := m.ddClient.CountLogs(
			fmt.Sprintf("service:%s %s", lw.Service, lw.Pattern),
			5*time.Minute,
		)
		if err != nil {
			continue
		}

		if lw.Threshold > 0 && count >= lw.Threshold {
			fmt.Printf("Log Alert: %s matched %d times (threshold: %d)\n",
				lw.Name, count, lw.Threshold)

			m.state.SetAlertActive(logKey)

			logs, _ := m.ddClient.GetMatchingLogs(lw.Service, lw.Pattern, 15*time.Minute)

			var aiSummary string
			if logs != "" && m.cfg.OpenAI.APIKey != "" {
				aiSummary, _ = m.aiClient.Analyze(lw.Name, logs)
			} else if logs == "" {
				aiSummary = "No matching logs found"
			} else {
				aiSummary = logs
			}

			alert := slack.LogAlert{
				Name:         lw.Name,
				MatchCount:   count,
				Threshold:    lw.Threshold,
				Pattern:      lw.Pattern,
				AIDiagnostic: aiSummary,
			}

			if m.dryRun {
				fmt.Printf("[DRY-RUN] Would send Slack log alert: %+v\n", alert)
			} else {
				if err := m.slack.SendLogAlert(alert); err != nil {
					fmt.Printf("Failed to send Slack alert: %v\n", err)
				}
			}
		} else {
			if m.state.IsAlertActive(logKey) {
				m.state.ResolveAlert(logKey)
				fmt.Printf("Log Resolved: %s\n", lw.Name)
			}
		}
	}
}
