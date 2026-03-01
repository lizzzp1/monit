# monit

A little CLI tool that watches your services and bugs you on Slack when things go sideways.

## What it does

- Polls your endpoints every 30s
- Alerts when error rate spikes above your threshold
- Watches Datadog logs for specific patterns (useful for migrations)
- Grabs recent logs from Datadog and asks AI what's wrong
- Sends you a Slack message with a rollback button and (optionally) a revert PR link

## Quick start

```bash
# copy the example config and fill in your keys
cp config.example.yaml config.yaml

# see what endpoints are configured
./monit status

# test your Slack connection
./monit test

# start watching (dry-run first to see what happens)
./monit run --dry-run

# actually start monitoring
./monit run
```

## Config

Everything lives in `config.yaml`. You can also use environment variables - these take precedence over config file values:

```yaml
endpoints:
  - name: api-service
    url: https://api.example.com/health
    error_threshold: 5

slack:
  bot_token: "xoxb-..."    # or set SLACK_BOT_TOKEN
  channel: "#alerts"       # or set SLACK_CHANNEL (use U123456 for direct message)

datadog:
  api_key: "..."            # or set DATADOG_API_KEY
  app_key: "..."            # or set DATADOG_APP_KEY

circleci:
  api_token: "..."          # or set CIRCLECI_API_TOKEN

openai:
  api_key: "..."            # or set OPENAI_API_KEY

github:
  api_token: "ghp_..."      # or set GITHUB_API_TOKEN
  owner: "your-org"
  repo: "your-repo"
  base_branch: "main"
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SLACK_BOT_TOKEN` | Slack bot token (starts with `xoxb-`) |
| `SLACK_CHANNEL` | Channel name (`#alerts`) or user ID (`U123456`) for DM |
| `DATADOG_API_KEY` | Datadog API key |
| `DATADOG_APP_KEY` | Datadog application key |
| `CIRCLECI_API_TOKEN` | CircleCI API token |
| `OPENAI_API_KEY` | OpenAI API key |
| `GITHUB_API_TOKEN` | GitHub personal access token |

Example:
```bash
export SLACK_BOT_TOKEN="xoxb-xxx"
export SLACK_CHANNEL="U123456789"  # direct message to you
export DATADOG_API_KEY="xxx"
export OPENAI_API_KEY="sk-xxx"
./monit run
```

## How alerts work

1. Monit polls your endpoints
2. If error rate > threshold, it checks if you're snoozed or in cooldown
3. If not, it fetches logs from Datadog, sends them to OpenAI for analysis
4. Posts to Slack with what's wrong and a button to rollback

State is saved to `monit_state.json` so it remembers if you're snoozed across restarts.

## Buttons

- **Rollback** - hits your CircleCI trigger URL
- **Revert PR** - creates a GitHub PR reverting the last change
- **Snooze 30m** - shuts up for 30 minutes
