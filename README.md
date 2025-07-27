# GoELF - European League Football Backend

A Golang backend application for an HTMX webapp that displays European League Football data. The application fetches data from external APIs and caches it in a local SQLite database for efficiency.

## Features

- **HTMX Integration**: Modern web interface using HTMX for dynamic content updates
- **API Endpoints**: RESTful API hosted on `/api` path
- **Data Caching**: SQLite database for efficient data storage and retrieval
- **Automatic Updates**: Background job fetches new data every 5 minutes
- **Live Scores**: Real-time scoreboard display
- **Match Schedule**: Upcoming matches information

## API Endpoints

- `GET /api/schedule` - Get upcoming matches
- `GET /api/scoreboard` - Get live scores
- `GET /api/refresh` - Manually trigger data refresh

## External Data Sources

The application fetches data from:
- `https://europeanleague.football/api/schedule`
- `https://europeanleague.football/api/scoreboard`

## Prerequisites

- Go 1.21 or higher
- SQLite3

## Installation

### Option 1: Local Development

1. Clone the repository:
```bash
git clone <repository-url>
cd goelf
```

2. Install dependencies:
```bash
go mod tidy
```

3. Run the application:
```bash
go run main.go
```

The server will start on `http://localhost:8080`

### Option 2: Docker Deployment

#### Using Docker Compose (Recommended)
```bash
# Pull and run the latest image
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the application
docker-compose down
```

#### Using Docker directly
```bash
# Pull the image
docker pull ghcr.io/floholz/goelf:latest

# Run the container
docker run -d \
  --name goelf \
  -p 8080:8080 \
  -v $(pwd)/database:/app/database \
  --restart unless-stopped \
  ghcr.io/floholz/goelf:latest
```

### Production Deployment

The application is automatically built and pushed to GitHub Container Registry (ghcr.io) on every push to main/master branch and on version tags.

**Available tags:**
- `ghcr.io/floholz/goelf:latest` - Latest stable version
- `ghcr.io/floholz/goelf:v1.0.0` - Specific version tags
- `ghcr.io/floholz/goelf:main` - Latest from main branch

## Project Structure

```
goelf/
├── main.go              # Main application file
├── go.mod               # Go module file
├── go.sum               # Go dependencies checksum
├── README.md            # This file
├── Dockerfile           # Docker container definition
├── docker-compose.yml   # Docker Compose configuration
├── .dockerignore        # Docker build exclusions
├── .github/workflows/   # GitHub Actions workflows
│   └── docker.yml       # Docker build and push workflow
├── templates/           # HTML templates
│   ├── index.html       # Main page template
│   ├── scoreboard.html  # Scoreboard display template
│   ├── schedule.html    # Schedule display template
│   └── refresh.html     # Refresh status template
└── database/
    └── elf25.db         # SQLite database (created automatically)
```

## Database Schema

### Schedule Table
- `id` (INTEGER PRIMARY KEY)
- `home_team` (TEXT)
- `away_team` (TEXT)
- `date` (TEXT)
- `time` (TEXT)
- `competition` (TEXT)
- `created_at` (DATETIME)

### Scoreboard Table
- `id` (INTEGER PRIMARY KEY)
- `home_team` (TEXT)
- `away_team` (TEXT)
- `home_score` (INTEGER)
- `away_score` (INTEGER)
- `status` (TEXT)
- `competition` (TEXT)
- `created_at` (DATETIME)

## Usage

1. Open your browser and navigate to `http://localhost:8080`
2. The application will automatically load live scores
3. Use the tabs to switch between "Live Scores" and "Upcoming Matches"
4. Click "Refresh Data" to manually update the data
5. Data automatically refreshes every 5 minutes

## Development

### Adding New API Endpoints

1. Add the route in `main.go` within the API group
2. Create corresponding handler functions
3. Add HTML templates if needed for HTMX responses

### Modifying Data Fetching

1. Update the `fetchSchedule()` and `fetchScoreboard()` functions
2. Modify the data structures if the API response format changes
3. Update database schema if needed

## Dependencies

- `github.com/gin-gonic/gin` - Web framework
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/robfig/cron/v3` - Cron job scheduler

## License

This project is open source and available under the MIT License. 