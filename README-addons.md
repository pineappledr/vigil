# Vigil Add-ons

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-linux%2Famd64%20%7C%20linux%2Farm64-lightgrey)](https://github.com/pineappledr/vigil-addons)
[![Vigil](https://img.shields.io/badge/Requires-Vigil%20v3.0%2B-8b5cf6)](https://github.com/pineappledr/vigil)

The official add-on ecosystem for [Vigil](https://github.com/pineappledr/vigil) v3.0+. Each add-on is a standalone, multi-architecture background daemon that registers with the Vigil server, streams real-time telemetry over WebSockets, and renders a dynamic UI in the Vigil dashboard — no frontend code required.

---

## Architecture Overview

Vigil add-ons follow a fully decoupled architecture. The server and add-on communicate exclusively through a WebSocket channel and a structured JSON manifest that declaratively describes the UI.

```
┌────────────────────┐          WebSocket           ┌────────────────────┐
│                    │  ◄──── telemetry frames ────  │                    │
│   Vigil Server     │         (progress, logs,      │   Add-on Daemon    │
│   (Go + SQLite)    │          notifications,       │   (standalone      │
│                    │          heartbeats)           │    binary/container)│
│  ┌──────────────┐  │                               │                    │
│  │ Dashboard UI │  │  ───── SSE (real-time) ─────► │  Runs on any host  │
│  │ (dynamic     │  │                               │  amd64 or ARM      │
│  │  rendering)  │  │                               └────────────────────┘
│  └──────────────┘  │
└────────────────────┘
```

**How it works:**

1. The admin registers an add-on from the Vigil dashboard (Add-ons → Add Add-on), which generates a registration token and add-on ID.
2. The add-on daemon starts, connects to the Vigil server via WebSocket at `/api/addons/ws?addon_id=<ID>`, and begins streaming telemetry frames.
3. The Vigil server routes each frame to the dashboard via Server-Sent Events and, for notification frames, to the configured alert channels (email, Discord, Telegram, etc.) through the event bus.
4. The add-on's JSON manifest declares its UI — forms, progress bars, charts, log viewers, and SMART tables — which the dashboard renders dynamically. No frontend modifications are needed.

---

## Available Add-ons

### Disk Burn-in & Pre-clear Tool

A production-grade drive qualification daemon for new or repurposed drives. Designed for homelabs, NAS builders, and data hoarders who want to stress-test drives before trusting them with data.

**Features:**

- **Precise drive targeting** — Select drives by serial number, model, or path. No ambiguity, no accidents.
- **Automated partitioning** — Handles `fdisk` (MBR) and `gdisk` (GPT) workflows automatically based on drive capacity.
- **Variable ext4 formatting** — Configurable reserved space (`-m 0` for data drives, `-m 2` for OS drives) to match your use case.
- **Multi-phase burn-in** — Sequential write → read-back → SMART comparison pipeline with per-phase progress tracking.
- **Strict security gates** — Every destructive operation (partitioning, formatting, writing) requires explicit confirmation through a two-step security gate in the UI. No drive is touched without deliberate user action.
- **Real-time telemetry** — Live progress bars with ETA and throughput, streaming log viewer, and phase-by-phase status updates pushed to the Vigil dashboard over WebSocket.
- **SMART delta tracking** — Captures SMART attributes before and after the burn-in cycle, highlighting any attribute changes (reallocated sectors, pending sectors, temperature deltas).
- **Notification integration** — Emits structured events (`job_started`, `phase_complete`, `burnin_passed`, `job_failed`) that route through Vigil's notification system to your configured channels.

---

## Deployment Guide

### Option A: Docker Compose (Recommended)

Create a `docker-compose.yml` on the host where the add-on should run:

```yaml
services:
  vigil-burnin:
    image: ghcr.io/pineappledr/vigil-burnin:latest
    container_name: vigil-burnin
    restart: unless-stopped
    privileged: true                    # Required for direct disk access
    environment:
      VIGIL_SERVER_URL: "http://vigil-server:7000"
      VIGIL_ADDON_ID: "1"              # From Vigil dashboard
      VIGIL_ADDON_TOKEN: "your-token"  # From registration step
    volumes:
      - /dev:/dev                      # Pass through block devices
    network_mode: host                 # Or use bridge with proper port mapping
```

```bash
docker compose up -d
```

The add-on will connect to the Vigil server, register its heartbeat, and appear as "Online" in the Add-ons tab.

### Option B: Pre-compiled Binary

Retrieve the latest release for your architecture from the [Releases](https://github.com/pineappledr/vigil-addons/releases) page.

```bash
# Install the binary
tar -xzf vigil-burnin-linux-amd64.tar.gz
sudo mv vigil-burnin /usr/local/bin/

# Run with environment variables
VIGIL_SERVER_URL="http://vigil-server:7000" \
VIGIL_ADDON_ID="1" \
VIGIL_ADDON_TOKEN="your-token" \
vigil-burnin
```

For persistent operation, create a systemd unit:

```ini
# /etc/systemd/system/vigil-burnin.service
[Unit]
Description=Vigil Burn-in Add-on
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vigil-burnin
Environment=VIGIL_SERVER_URL=http://vigil-server:7000
Environment=VIGIL_ADDON_ID=1
Environment=VIGIL_ADDON_TOKEN=your-token
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now vigil-burnin
```

---

## Contributing / Developer Guide

Building a new Vigil add-on requires three things: a JSON manifest, a WebSocket client, and multi-architecture build support.

### 1. Provide a JSON UI Manifest

Every add-on must include a JSON manifest that declares its name, version, and UI layout. The Vigil dashboard renders this manifest dynamically — you never write frontend code.

```json
{
  "name": "my-addon",
  "version": "1.0.0",
  "description": "What this add-on does",
  "pages": [
    {
      "id": "main",
      "title": "Dashboard",
      "components": [
        { "type": "progress", "id": "job-progress", "title": "Job Progress" },
        { "type": "log-viewer", "id": "logs", "title": "Live Logs" },
        {
          "type": "form",
          "id": "controls",
          "title": "Controls",
          "config": {
            "fields": [
              { "name": "target", "label": "Target", "type": "select", "required": true },
              { "name": "confirm", "label": "I understand this is destructive", "type": "checkbox", "security_gate": true }
            ],
            "action": "start_job"
          }
        }
      ]
    }
  ]
}
```

**Supported component types:**

| Type | Purpose |
|------|---------|
| `form` | Input forms with validation, field dependencies, live calculations, and security gates |
| `progress` | Job progress bars with phase tracking, ETA, and throughput |
| `chart` | Chart.js-based data visualization |
| `smart-table` | SMART attribute tables with delta highlighting |
| `log-viewer` | Real-time streaming log viewer with level filtering |

**Limits:** Max 20 pages, 50 components total, 100 form fields per form, 256 KiB manifest size.

### 2. Handle WebSocket Communication

Connect to the Vigil server at:

```
ws://<vigil-server>/api/addons/ws?addon_id=<ID>
```

Send JSON-encoded telemetry frames:

```json
{"type": "heartbeat", "payload": null}
```

```json
{"type": "progress", "payload": {"job_id": "abc", "phase": "writing", "percent": 42.5, "message": "Writing to /dev/sda"}}
```

```json
{"type": "log", "payload": {"level": "info", "message": "Scan complete", "source": "scanner"}}
```

```json
{"type": "notification", "payload": {"event_type": "job_complete", "severity": "info", "message": "Burn-in passed for WD Red 8TB"}}
```

**Protocol requirements:**

- Send a `heartbeat` frame at least every 60 seconds. The server expects activity within 90 seconds before marking the connection as dead.
- The server sends WebSocket pings every 30 seconds; respond to pongs automatically (most WebSocket libraries handle this).
- Maximum frame size: 64 KB.
- After 3 missed heartbeat intervals (configurable, default 1 minute each), the add-on status transitions to `degraded` and emits an `addon_degraded` event.

### 3. Support Multi-Architecture Builds

All official add-ons must produce binaries and container images for both `linux/amd64` and `linux/arm64`. Use a multi-stage Dockerfile with `buildx`:

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /addon ./cmd/addon

FROM alpine:3.21
COPY --from=builder /addon /usr/local/bin/addon
ENTRYPOINT ["addon"]
```

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/pineappledr/vigil-my-addon:latest --push .
```

### Development Workflow

1. Clone the repository and create a new directory under `addons/`:
   ```bash
   git clone https://github.com/pineappledr/vigil-addons.git
   cd vigil-addons
   mkdir addons/my-addon
   ```

2. Implement your add-on daemon with the WebSocket protocol above.

3. Place your JSON manifest at `addons/my-addon/manifest.json`.

4. Run a local Vigil server, register your add-on from the dashboard, and test the full flow.

5. Open a pull request with your add-on, tests, and a brief description of what it does.

---

## WebSocket Frame Reference

| Frame Type | Direction | Purpose |
|------------|-----------|---------|
| `heartbeat` | Add-on → Server | Keep-alive signal, resets heartbeat timer |
| `progress` | Add-on → Server | Job progress update (percent, phase, ETA, throughput) |
| `log` | Add-on → Server | Log message with level (`info`, `warn`, `error`) and optional source |
| `notification` | Add-on → Server | Structured event that triggers notification dispatch |
| Ping/Pong | Server → Add-on | WebSocket-level keep-alive (30s interval) |

### Event Types for Notifications

| Event Type | Severity | Description |
|------------|----------|-------------|
| `job_started` | info | A new job has begun |
| `phase_complete` | info | A job phase finished successfully |
| `burnin_passed` | info | Full burn-in cycle completed with no errors |
| `job_complete` | info | Job finished (generic) |
| `job_failed` | critical | Job encountered an unrecoverable error |
| `addon_degraded` | warning | Server-generated: add-on missed heartbeats |
| `addon_online` | info | Server-generated: add-on recovered from degraded state |

---

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
