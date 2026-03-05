package main

import (
	"fmt"
	"os"
	"time"

	"monit/internal/config"
	"monit/internal/datadog"
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
	rootCmd.AddCommand(checkDeployCmd())

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

func checkDeployCmd() *cobra.Command {
	var duration int

	cmd := &cobra.Command{
		Use:   "check-deploy [service]",
		Short: "Check if recent deploy has errors",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			dd := datadog.New(cfg.Datadog)

			commit, err := getCurrentCommit()
			if err != nil {
				return fmt.Errorf("not a git repo: %w", err)
			}
			fmt.Printf("Checking deploy: %s\n", commit[:7])

			services := cfg.Services()
			if len(args) > 0 {
				services = []string{args[0]}
			}

			if len(services) == 0 {
				return fmt.Errorf("no services configured. Add services to config.yaml or pass as argument")
			}

			d := time.Duration(duration) * time.Minute
			hasErrors := false

			for _, svc := range services {
				count, err := dd.CountLogs("service:"+svc+" status:error", d)
				if err != nil {
					return err
				}

				if count > 0 {
					fmt.Printf("  ❌ %s: %d errors\n", svc, count)
					hasErrors = true
				} else {
					fmt.Printf("  ✅ %s: no errors\n", svc)
				}
			}

			if hasErrors {
				fmt.Println("\n❌ Deploy has errors - check Datadog")
				return fmt.Errorf("errors found")
			}

			fmt.Println("\n✅ Deploy looks good!")
			return nil
		},
	}

	cmd.Flags().IntVar(&duration, "duration", 10, "Minutes back to check")

	return cmd
}

func getCurrentCommit() (string, error) {
	ref, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return "", err
	}

	var refPath string
	fmt.Sscanf(string(ref), "ref: %s", &refPath)

	commit, err := os.ReadFile(".git/" + refPath)
	if err != nil {
		return "", err
	}

	return string(commit)[:40], nil
}
