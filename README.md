# Anime Tracker Bot

A Telegram bot for searching anime information using the Jikan API (MyAnimeList unofficial API).

## Prerequisites

- Go
- PostgreSQL
- Redis
- Your Telegram Bot Token from @BotFather

You can change the setup to your own taste (change db, cache handling, etc...)

## Local Setup

### 1. Clone and Install Dependencies

```bash
git clone <repository_url>
cd sletish
go mod download
```

### 2. Environment Configuration

Create a `.env` file, update it to match whatever your personal environment is

### 3. Database Setup

Start PostgreSQL and Redis using Docker/Podman/Your custom choice:

```bash
docker-compose up -d
```

Run database migrations:

```bash
# Install migrate tool (if not installed)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path migrations -database "postgres://app_user:your_app_password@localhost:5432/sletish?sslmode=disable" up
```

### 4. Run the Bot

```bash
go run cmd/bot/main.go
```

### 5. Set Webhook (for production)

```bash
curl -X POST "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/setWebhook" \
     -d "url=https://your_domain.com/webhook"
```

## Available Commands

- `/start` - Welcome message and bot introduction
- `/search <anime_name>` - Search for anime by name
- `/profile` - View your user profile information
- `/help` - Show available commands


## API

Uses Jikan API v4 (https://api.jikan.moe/v4) for anime data.

## License

MIT License
