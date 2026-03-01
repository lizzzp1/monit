package state

import (
	"encoding/json"
	"os"
	"time"
)

type State struct {
	Alerts   map[string]AlertState `json:"alerts"`
	Snoozes  map[string]time.Time  `json:"snoozes"`
	Filepath string                `json:"-"`
}

type AlertState struct {
	Active     bool      `json:"active"`
	LastAlert  time.Time `json:"last_alert"`
	AlertCount int       `json:"alert_count"`
	ResolvedAt time.Time `json:"resolved_at"`
}

func Load(filepath string) (*State, error) {
	state := &State{
		Alerts:   make(map[string]AlertState),
		Snoozes:  make(map[string]time.Time),
		Filepath: filepath,
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}

	state.cleanupExpired()
	state.Filepath = filepath
	return state, nil
}

func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Filepath, data, 0644)
}

func (s *State) cleanupExpired() {
	now := time.Now()
	for endpoint, snoozedUntil := range s.Snoozes {
		if now.After(snoozedUntil) {
			delete(s.Snoozes, endpoint)
		}
	}
}

func (s *State) IsSnoozed(endpoint string) bool {
	s.cleanupExpired()
	if until, ok := s.Snoozes[endpoint]; ok {
		return time.Now().Before(until)
	}
	return false
}

func (s *State) Snooze(endpoint string, duration time.Duration) {
	s.Snoozes[endpoint] = time.Now().Add(duration)
}

func (s *State) IsAlertActive(endpoint string) bool {
	if alert, ok := s.Alerts[endpoint]; ok {
		return alert.Active
	}
	return false
}

func (s *State) SetAlertActive(endpoint string) {
	alert := s.Alerts[endpoint]
	alert.Active = true
	alert.LastAlert = time.Now()
	alert.AlertCount++
	s.Alerts[endpoint] = alert
}

func (s *State) ResolveAlert(endpoint string) {
	if alert, ok := s.Alerts[endpoint]; ok {
		alert.Active = false
		alert.ResolvedAt = time.Now()
		s.Alerts[endpoint] = alert
	}
}

func (s *State) ShouldAlert(endpoint string, cooldown time.Duration) bool {
	if !s.IsAlertActive(endpoint) {
		return true
	}

	alert := s.Alerts[endpoint]
	return time.Since(alert.LastAlert) > cooldown
}
