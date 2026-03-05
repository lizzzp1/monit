package main

import (
	"fmt"
	"os"

	"monit/internal/config"
	"monit/internal/monitor"
	"monit/internal/slack"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "monit",
		Short: "A monitoring tool for endpoints and logs",
	}

	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(testCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the monitoring loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if len(cfg.Endpoints) == 0 {
				fmt.Println("No endpoints configured. Add endpoints to config.yaml")
				return nil
			}

			slackClient := slack.New(cfg.Slack)
			mon, err := monitor.New(cfg, slackClient, dryRun)
			if err != nil {
				return err
			}

			fmt.Println("Starting monitoring... (Press Ctrl+C to stop)")
			return mon.Run()
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate alerts without sending to Slack")

	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show endpoint status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			fmt.Println("Configured endpoints:")
			for _, ep := range cfg.Endpoints {
				fmt.Printf("  - %s: %s (threshold: %d%%)\n", ep.Name, ep.URL, ep.ErrorThreshold)
			}

			if len(cfg.Endpoints) == 0 {
				fmt.Println("  (none)")
			}

			fmt.Println("\nConfigured log watches:")
			for _, lw := range cfg.LogWatches {
				fmt.Printf("  - %s: service=%s pattern=%q threshold=%d\n", lw.Name, lw.Service, lw.Pattern, lw.Threshold)
			}

			if len(cfg.LogWatches) == 0 {
				fmt.Println("  (none)")
			}

			return nil
		},
	}
}

func testCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Send a test Slack message",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if cfg.Slack.BotToken == "" || cfg.Slack.Channel == "" {
				return fmt.Errorf("Slack bot_token and channel must be configured")
			}

			slackClient := slack.New(cfg.Slack)
			return slackClient.SendMessage("Test message from monit")
		},
	}
}
