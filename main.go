package main

import (
	"flag"
	"fmt"
	"os"

	"monit/internal/config"
	"monit/internal/monitor"
	"monit/internal/slack"
)

func main() {
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	testCmd := flag.NewFlagSet("test", flag.ExitOnError)

	dryRun := runCmd.Bool("dry-run", false, "Simulate alerts without sending to Slack")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd.Parse(os.Args[2:])
		run(*dryRun)
	case "status":
		if err := statusCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		status()
	case "test":
		if err := testCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		testSlack()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: monit <command>

Commands:
  run     Start the monitoring loop
  status  Show endpoint status
  test    Send a test Slack message

Options for 'run':
  --dry-run    Simulate alerts without sending to Slack

Examples:
  monit run
  monit run --dry-run
  monit status
  monit test`)
}

func run(dryRun bool) error {
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
}

func status() error {
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
}

func testSlack() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Slack.BotToken == "" || cfg.Slack.Channel == "" {
		return fmt.Errorf("Slack bot_token and channel must be configured")
	}

	slackClient := slack.New(cfg.Slack)
	return slackClient.SendMessage("Test message from monit")
}
