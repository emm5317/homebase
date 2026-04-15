# Homebase

A time-aware family dashboard for the household. Aggregates weather, calendar events, train schedules, chores, and grocery lists into a single, glanceable view that adapts to the time of day. Designed to run on a Raspberry Pi and display on a wall-mounted tablet.

## What it does

Homebase answers one question: **what does our household need to know right now?**

The dashboard reshapes itself throughout the day. Morning shows trains and weather. Midday shows the calendar and grocery list. Evening shows return trains and tomorrow's preview. It's a single, opinionated feed — not a grid of widgets.

Anyone in the household can add items by sending a plain email. Send a grocery item from your phone. Email a note to the babysitter. Forward a school newsletter and have it parsed into calendar entries. No app to install, no login to remember.

## Status

This repository contains the v1 scaffold:

- Card schema, provider interface, scheduler, and time-of-day context engine
- **Weather provider** (OpenWeatherMap)
- **Garbage/recycling schedule** with holiday shift logic
- **Metra provider** (GTFS-Realtime trip updates + service alerts)
- **Email ingestion** via Cloudflare Email Worker → `/api/ingest` webhook, with sender allowlist, plus-address routing, and keyword-based classification
- Preact + HTM frontend (no build step) with three CSS display modes
- SQLite (WAL mode) for persisting email-sourced cards
- systemd units, Cloudflare Tunnel config, Litestream backup config
- Raspberry Pi provisioning script and binary update script
- GitHub Actions workflow to cross-compile `linux/arm64` and `linux/amd64` releases

Planned for v1 completion: Skylight (calendar/chores/lists) provider. See [Roadmap](#roadmap).

## Architecture

```
                 emails to home@yourdomain.com
                            |
                Cloudflare Email Worker (parses MIME)
                            |
                   Cloudflare Tunnel (HTTPS)
                            |
   +--- Raspberry Pi 5 ---------------------------+
   |                                              |
   |  homebase (Go binary)                        |
   |   - GET  /          (embedded Preact SPA)    |
   |   - GET  /api/cards (JSON, time-filtered)    |
   |   - POST /api/ingest (email webhook)         |
   |   - GET  /api/health                         |
   |   - Pollers: per-provider goroutines         |
   |   - SQLite (WAL) for email cards             |
   |                                              |
   |  cloudflared (Cloudflare Tunnel)             |
   |  litestream  (continuous DB backup → R2)     |
   |                                              |
   +----------------------------------------------+
                            |
                    LAN (homebase.local)
                            |
                +-----------+-----------+
                |           |           |
            Tablet       Phone       E-ink
```

All three Pi processes run as systemd services with auto-restart. The dashboard is reachable on the LAN via mDNS; only `/api/ingest` is exposed externally through Cloudflare Tunnel.

## Project structure

```
homebase/
  cmd/homebase/        Entry point (main.go)
  internal/
    cards/             Uniform card schema
    config/            Loads .env + config.yaml
    providers/         provider.go, weather.go, garbage.go
    ingest/            Email webhook handler, parsers, classifier
    engine/            Time-of-day filtering and priority sorting
    scheduler/         Goroutine tickers per provider
    store/             In-memory cache + SQLite persistence
  static/              Embedded Preact + HTM frontend
  worker/              Cloudflare Email Worker (deployed separately)
  deploy/              systemd units, cloudflared/litestream configs
  scripts/             provision.sh, update.sh, vendor.sh
```

## Quick start (development)

Requirements: Go 1.25 or newer (the Go toolchain mechanism will auto-download the required version if needed).

```bash
git clone https://github.com/emm5317/homebase.git
cd homebase
cp config.example.yaml config.yaml

# Add a free OpenWeatherMap API key and any random ingest token
cat > .env <<EOF
OWM_API_KEY=your_openweathermap_key
INGEST_BEARER_TOKEN=any_random_string
EOF

go run ./cmd/homebase
```

Open `http://localhost:8080` to see the dashboard. Try `?mode=mobile` or `?mode=eink` for alternate display modes.

## Deployment

Targets a Raspberry Pi 5 (4 GB or 8 GB) running Raspberry Pi OS 64-bit.

1. **Build the binary** — push a `v*` tag to trigger the GitHub Actions workflow, which produces `homebase-linux-arm64` and `homebase-linux-amd64` binaries as a GitHub Release.

2. **Provision the Pi** — clone this repo onto a fresh Pi and run:

   ```bash
   ./scripts/provision.sh
   ```

   This installs `cloudflared` and `litestream`, creates the service user, sets up systemd units, configures tmpfs for SD-card longevity, and downloads the latest binary.

3. **Configure** — edit `/opt/homebase/config.yaml` (location, garbage schedule, allowed email senders) and `/opt/homebase/.env` (API keys).

4. **Set up Cloudflare Tunnel** for the email webhook:

   ```bash
   cloudflared tunnel create homebase
   sudo cp deploy/cloudflared.yml /etc/cloudflared/config.yml
   ```

5. **Set up Litestream** to back up SQLite to Cloudflare R2 (free tier):

   ```bash
   sudo cp deploy/litestream.yml /etc/litestream.yml
   ```

6. **Start everything**:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now homebase cloudflared litestream
   ```

7. Open the dashboard at `http://homebase.local:8080` from any device on your LAN.

To update later, run `./scripts/update.sh` on the Pi.

## Metra GTFS-RT setup

The Metra provider polls the public GTFS-Realtime feeds every 60 seconds for
upcoming departures at the configured station and any active alerts on the
configured route.

1. Request an API token for your email address at
   [metra.com/metra-gtfs-api](https://metra.com/metra-gtfs-api).
2. Add it to `.env` as `METRA_API_KEY` (the provider authenticates via
   `Authorization: Bearer <token>`).
3. In `config.yaml`, set `metra.route`, `metra.station`, and
   `metra.destination` to the GTFS `route_id` / `stop_id` values that match
   your commute.

Feed URLs default to the public endpoints but can be overridden under
`metra:` in `config.yaml` (`trip_updates_url`, `alerts_url`).

| Endpoint | Purpose |
|---|---|
| `https://gtfspublic.metrarr.com/gtfs/public/tripupdates` | Per-trip delay/ETA predictions |
| `https://gtfspublic.metrarr.com/gtfs/public/alerts` | Service alerts by route/stop |
| `https://gtfspublic.metrarr.com/gtfs/public/positions` | Live vehicle positions (unused) |
| `https://schedules.metrarail.com/gtfs/schedule.zip` | Static GTFS schedule (for looking up route / stop IDs) |

## Email ingestion setup

1. Enable Email Routing for your domain in the Cloudflare dashboard.
2. Deploy the worker:

   ```bash
   cd worker
   npx wrangler deploy
   npx wrangler secret put INGEST_SECRET
   ```

3. Add an email route mapping `home@yourdomain.com` → the deployed worker.

Once configured, any allowlisted sender can email items to the dashboard:

| Address | Behavior |
|---|---|
| `home+grocery@yourdomain.com` | Each line/comma-separated item becomes a grocery list item |
| `home+chore@yourdomain.com` | Subject becomes a chore |
| `home+note@yourdomain.com` | Becomes an info card |
| `home+reminder@yourdomain.com` | Becomes a reminder card |
| `home+fridge@yourdomain.com` | Each item becomes a fridge card; optional `Nd` suffix sets TTL (default 3 days), color warns as expiry nears |
| `home@yourdomain.com` (no tag) | Auto-classified by keyword matching |

## Configuration

`config.yaml` (non-secret, deployed to `/opt/homebase/config.yaml`):

```yaml
location:
  lat: 41.8009
  lon: -87.9331
  timezone: America/Chicago

garbage:
  pickup_day: thursday
  recycling_weeks: even
  yard_waste: april-november
  holidays: ["2026-07-04", "2026-12-25"]

allowed_senders:
  "eric@example.com": Eric
  "kayla@example.com": Kayla
```

`.env` (secrets, not in version control):

```
OWM_API_KEY=...
METRA_API_KEY=...
SKYLIGHT_EMAIL=...
SKYLIGHT_PASSWORD=...
SKYLIGHT_FRAME_ID=...
INGEST_BEARER_TOKEN=...
```

## Roadmap

**v1 (in progress)**

- [x] Provider interface, scheduler, context engine
- [x] Weather provider
- [x] Garbage schedule provider (with holiday shift logic)
- [x] Email ingestion with keyword classification
- [x] Preact frontend with display/mobile/eink modes
- [x] Pi deployment scripts and systemd units
- [x] Metra GTFS-RT provider
- [x] Reminders (birthdays / anniversaries)
- [x] Meals rotation
- [x] Quiet hours
- [ ] Skylight calendar/chores/lists provider

**v2**

- [ ] LLM classification fallback for ambiguous emails
- [ ] Email confirmation replies
- [ ] Skylight write-back (push email items to the physical frame)
- [ ] iCal feed integration (school district, etc.)
- [ ] Per-person card profiles

## Family touches

Three quality-of-life features make Homebase feel like it knows your household:

**Reminders** — add birthdays, anniversaries, or one-off events to `reminders:` in `config.yaml` using `MM-DD` (annual) or `YYYY-MM-DD` (one-off); a card appears up to 7 days before and escalates in color and priority as the date approaches (see `config.example.yaml` for examples).

**Meals rotation** — set a flat rotation list under `meals:` with a `start_date` anchor; the dashboard shows "Tonight: X" from 3–7 PM and "Tomorrow: X" from 7–10 PM so nobody has to ask what's for dinner (see `config.example.yaml` for an example week).

**Quiet hours** — set `quiet_hours: "21:00-06:00"` (or any `HH:MM-HH:MM` range, including cross-midnight) to suppress non-urgent cards overnight; only `alert_level: severe` cards and `priority <= 1` cards render during the window.

## License

[MIT](LICENSE)
