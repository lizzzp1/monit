package slack

import (
	"fmt"

	"github.com/slack-go/slack"
	"monit/internal/config"
)

type Client struct {
	api     *slack.Client
	channel string
}

func New(cfg config.Slack) *Client {
	api := slack.New(cfg.BotToken)
	return &Client{
		api:     api,
		channel: cfg.Channel,
	}
}

type Alert struct {
	Endpoint     string
	ErrorRate    float64
	Threshold    int
	AIDiagnostic string
	TriggerURL   string
	PRURL        string
}

type LogAlert struct {
	Name         string
	MatchCount   int
	Threshold    int
	Pattern      string
	AIDiagnostic string
}

func (c *Client) SendAlert(alert Alert) error {
	attachments := []slack.Attachment{
		{
			Color: "danger",
			Title: fmt.Sprintf("🔴 Alert: %s error rate spiked", alert.Endpoint),
			Text:  fmt.Sprintf("Error Rate: %.1f%% (threshold: %d%%)", alert.ErrorRate, alert.Threshold),
			Fields: []slack.AttachmentField{
				{
					Title: "AI Analysis",
					Value: alert.AIDiagnostic,
				},
			},
		},
	}

	attachments[0].Actions = []slack.AttachmentAction{
		{
			Name:  "rollback",
			Text:  "Rollback",
			Type:  "button",
			Style: "danger",
			URL:   alert.TriggerURL,
		},
	}

	if alert.PRURL != "" {
		attachments[0].Actions = append(attachments[0].Actions,
			slack.AttachmentAction{
				Name: "revert_pr",
				Text: "Create Revert PR",
				Type: "button",
				URL:  alert.PRURL,
			},
		)
	}

	attachments[0].Actions = append(attachments[0].Actions,
		slack.AttachmentAction{
			Name: "snooze",
			Text: "Snooze 30m",
			Type: "button",
		},
	)

	_, _, err := c.api.PostMessage(
		c.channel,
		slack.MsgOptionAttachments(attachments...),
	)
	return err
}

func (c *Client) SendLogAlert(alert LogAlert) error {
	attachments := []slack.Attachment{
		{
			Color: "warning",
			Title: fmt.Sprintf("🔍 Log Alert: %s", alert.Name),
			Text:  fmt.Sprintf("Matched %d times (threshold: %d)\nPattern: %s", alert.MatchCount, alert.Threshold, alert.Pattern),
			Fields: []slack.AttachmentField{
				{
					Title: "AI Analysis",
					Value: alert.AIDiagnostic,
				},
			},
		},
	}

	attachments[0].Actions = []slack.AttachmentAction{
		{
			Name: "snooze",
			Text: "Snooze 30m",
			Type: "button",
		},
	}

	_, _, err := c.api.PostMessage(
		c.channel,
		slack.MsgOptionAttachments(attachments...),
	)
	return err
}

func (c *Client) SendMessage(msg string) error {
	_, _, err := c.api.PostMessage(c.channel, slack.MsgOptionText(msg, false))
	return err
}
