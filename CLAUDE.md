# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

webcal_sync is a Go application that synchronizes webcal/iCal feeds to Google Calendar. It fetches events from iCal URLs, compares them with existing Google Calendar events, and creates/updates/deletes events as needed to keep them in sync.

## Architecture

### Core Components

- **main.go**: Entry point and sync orchestration
  - `parseICal()`: Fetches and parses iCal feeds from URLs using the gocal library
  - `diffEvents()`: Compares upstream iCal events with existing Google Calendar events to determine creates, updates, and deletes
  - `iCalToGEvent()`: Converts iCal events to Google Calendar event format
  - Only processes future events (past events are ignored)

- **gcal/** package: Google Calendar API wrapper
  - `gcal.go`: CRUD operations for calendar events (Create, Update, Delete, GetEventsForAttribute)
  - `token.go`: OAuth2 authentication flow and token management
  - Uses Google Calendar API v3 with primary calendar
  - Events are tagged with extended properties to track their source URL

- **config.go**: Configuration management
  - Reads from `config.yml` (YAML format)
  - Each config entry represents one iCal feed to sync
  - Config fields: `url` (iCal feed), `color_id` (calendar color), `id_format` (event ID strategy), `reminder` (minutes before event)

- **helpers.go**: Utility functions
  - `unescapeICalText()`: Unescapes iCalendar TEXT escape sequences (`\n`, `\N`) per RFC5545, since gocal doesn't handle newline unescaping

### Event Identification

Events are identified using one of two strategies (configured via `id_format`):
- Empty string (default): Use the iCal event's UID directly
- `"url"`: Generate SHA256 hash of the event's URL field

Events are also tagged with a SHA256 hash of their source iCal feed URL in extended properties for filtering.

### Event Comparison Logic

The `diffEvents()` function implements a three-way diff:
1. **Create**: Events in iCal feed but not in Google Calendar
2. **Update**: Events in both, but with differences in: summary, start/end times, description, location, color, status, or reminders
3. **Delete**: Events in Google Calendar (tagged with this feed's URL) but not in iCal feed

The comparison handles both all-day and timed events, with special logic for date-only vs datetime fields.

### Data Flow

```
config.yml → parseICal(URL) → gocal.Event[]
                                     ↓
                           diffEvents() compares with
                                     ↓
                   gcal.GetEventsForAttribute() → calendar.Event[]
                                     ↓
                          create/update/delete operations
                                     ↓
                            Google Calendar (primary)
```

## Development Commands

### Build
```bash
go build *.go
```
Produces an executable binary in the current directory.

### Run
```bash
go run *.go
```
Requires `config.yml` and `credentials.json` in the working directory. On first run, will prompt for OAuth authorization and save `token.json`.

### Lint
```bash
golangci-lint run
```
CI uses golangci-lint-action@v3.

### Dependencies
Dependencies are vendored. To update:
```bash
go mod tidy
go mod vendor
```

### Docker

Build multi-architecture Docker images:
```bash
docker buildx build --platform linux/amd64,linux/arm64 -t webcal_sync:latest .
```

Run with Docker:
```bash
docker run -v $(pwd)/config:/app/config webcal_sync:latest
```

The Docker image expects config files in `/app/config` directory.

## Configuration Requirements

The application expects these files in the working directory:

1. **config.yml**: Array of sync configurations
   ```yaml
   - url: "https://example.com/calendar.ics"
     color_id: "1"
     id_format: "url"  # or omit for default UID-based
     reminder: 30      # minutes before event, or omit for no reminder
   ```

2. **credentials.json**: Google OAuth2 client credentials
   - Obtain from Google Cloud Console
   - Requires Calendar API enabled
   - Scopes: `calendar.events` and `calendar.readonly`

3. **token.json**: OAuth2 token (auto-generated on first run)

## CI/CD

### GitHub Actions

- **build.yml**: Builds the Go binary on every push
- **lint.yml**: Runs golangci-lint on pushes to main and PRs
- **release.yml**: Creates release binaries for linux/darwin with amd64/arm64 architectures
- **docker.yml**: Builds and pushes multi-arch Docker images
  - Triggers on push to main (tags as `main` and `latest`)
  - Triggers on release creation (tags with version number, e.g., `v1.0.0`)
  - Builds for linux/amd64 and linux/arm64
  - Requires `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets

## Kubernetes Deployment

### Helm Chart

A Helm chart is available at `helm/webcal-sync/` for deploying as a Kubernetes CronJob.

Key features:
- Runs on a configurable cron schedule (default: hourly)
- Uses PersistentVolume for storing config files
- Includes an OAuth setup Job for initial authentication
- Configurable resources, scheduling, and node placement

Installation:
```bash
helm install webcal-sync ./helm/webcal-sync
```

See `helm/webcal-sync/README.md` for detailed deployment instructions.

### OAuth Setup in Kubernetes

The Helm chart includes an interactive Job for OAuth setup:

1. Enable the OAuth setup job:
   ```bash
   helm upgrade webcal-sync ./helm/webcal-sync --set oauthSetup.enabled=true --reuse-values
   ```

2. Attach to the pod to complete OAuth flow:
   ```bash
   kubectl attach -it $(kubectl get pods -l app.kubernetes.io/component=oauth-setup -o jsonpath='{.items[0].metadata.name}')
   ```

3. Follow the OAuth URL, authorize, and paste the code

4. Disable the setup job after completion:
   ```bash
   helm upgrade webcal-sync ./helm/webcal-sync --set oauthSetup.enabled=false --reuse-values
   ```

## Important Implementation Details

- The application uses a regex workaround (`dateFormatFix`) to handle malformed DTSTAMP fields in some iCal feeds
- All string fields from iCal events are trimmed of whitespace using reflection
- Event times are compared after truncating to the second (ignoring sub-second precision)
- All-day events are handled specially: end times are truncated to day boundaries and use the `Date` field instead of `DateTime`
- Events are only deleted if they haven't started yet (checked via start time)
- The application operates on the user's primary Google Calendar
- Duplicate event IDs within a single sync run are detected and skipped
