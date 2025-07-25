# Anime Tracker Bot

A Telegram bot for searching anime information using the Jikan API (MyAnimeList unofficial API).

## Features

- Search anime by name
- View anime details including score, episodes, year, genres, and synopsis
- Redis caching for improved performance
- Rate limiting to respect API constraints

## Setup

### Prerequisites

- Go 1.24.4 or higher
- Redis (optional, for caching)
- Telegram Bot Token from @BotFather

### Environment Variables

Create a `.env.local` file:

```
BOT_TOKEN=your_telegram_bot_token_here
PORT=8080
WEBHOOK_URL=https://your_domain.com/webhook
R_HOST=localhost
R_PORT=6379
R_PASS=
```

### Running Locally

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Start Redis (optional):
   ```bash
   docker-compose up redis -d
   ```
4. Run the bot:
   ```bash
   go run cmd/bot/main.go
   ```

### Setting up Webhook

Set your webhook URL with Telegram:
```bash
curl -X POST "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/setWebhook" \
     -d "url=https://your_domain.com/webhook"
```

## Commands (not yet updated to reflect rest of commands)

- `/start` - Show welcome message and available commands
- `/search <anime_name>` - Search for anime by name
- `/help` - Show help information

## Docker/Podman

Start Redis cache:
```bash
docker-compose up -d
OR
podman-compose up -d
```

## API

Uses Jikan API v4 (https://api.jikan.moe/v4) for anime data.

## License

MIT License
