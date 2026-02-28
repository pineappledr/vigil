<div align="center">

  <h1>
    <img src="web/img/Vigil.png" alt="Vigil Logo" width="120" style="vertical-align: middle; margin-right: 5px;">
    <span style="vertical-align: middle;">Vigil</span>
  </h1>

  **Proactive, lightweight server & drive monitoring with S.M.A.R.T. health analysis, ZFS pool management, extensible add-ons, and multi-channel notifications.**
  
  <p>
    <img src="https://github.com/pineappledr/vigil/actions/workflows/ci.yml/badge.svg" alt="Build Status">
    <img src="https://img.shields.io/github/license/pineappledr/vigil" alt="License">
    <img src="https://img.shields.io/badge/Go-1.25.7-00ADD8?logo=go&logoColor=white" alt="Go Version">
    <img src="https://img.shields.io/badge/SQLite-v1.44.0-003B57?logo=sqlite&logoColor=white" alt="SQLite Version">
  </p>

</div>

> **⚠️ BREAKING CHANGE in v2.4.0:** This release introduces **Ed25519 key-based mutual authentication** between the server and agents. All existing agents **must be re-registered** using a one-time registration token. Agents running older versions will be rejected by the server. See [Agent Authentication](#-agent-authentication) for the new setup workflow.

> **🆕 v3.0:** Adds the **Add-on ecosystem** (WebSocket-connected daemons with manifest-driven UI), **multi-channel notifications** (Telegram, Discord, Slack, Email, Pushover, Gotify, webhooks) with a guided provider wizard, and **add-on registration tokens**. See [Notifications](#-notifications), [Add-ons](#-add-ons), and the companion [vigil-addons](https://github.com/pineappledr/vigil-addons) repository.

**Vigil** is a next-generation monitoring system built for speed and simplicity. It provides instant visibility into your infrastructure with a modern web dashboard, predictive health analysis, comprehensive ZFS pool monitoring, extensible add-ons, and multi-channel notifications — ensuring you never miss a critical hardware failure.

Works on **any Linux system** (Ubuntu, Debian, Proxmox, TrueNAS, Unraid, Fedora, etc.) including systems with **LSI/Broadcom HBA controllers**.

---

## ✨ Features

- **🔥 Lightweight Agent:** Single Go binary with zero dependencies. Deploy it on any server in seconds.
- **🐳 Docker Server:** The central hub is containerized for easy deployment via Docker or Compose.
- **⚡ Fast Web Dashboard:** Modern HTML5/JS interface that loads instantly with real-time updates.
- **🔍 Deep Analysis:** View raw S.M.A.R.T. attributes, temperature history, and drive details.
- **🤖 Predictive Checks:** Advanced analysis to determine if a drive is failing or just aging.
- **📊 Continuous Monitoring:** Configurable reporting intervals with automatic reconnection.
- **🔐 Authentication:** Built-in login system with bcrypt password hashing, secure cookies, and rate limiting.
- **🏷️ Drive Aliases:** Set custom names for your drives (e.g., "Plex Media", "Backup Drive").
- **🔧 HBA Support:** Automatic detection for SATA drives behind SAS HBA controllers (LSI SAS3224, etc.).
- **🗄️ ZFS Pool Monitoring:** Full ZFS support with pool health, device hierarchy, scrub history, and SMART integration.
- **🧩 Extensible Add-ons:** Third-party daemons register via API, stream telemetry over WebSocket, and render UI from a JSON manifest — no frontend code required.
- **📣 Multi-Channel Notifications:** Guided provider wizard for Telegram, Discord, Slack, Email, Pushover, Gotify, and generic webhooks. Event routing, quiet hours, and digest batching included.

---

## 🗄️ ZFS Monitoring Features

Vigil provides comprehensive ZFS pool monitoring:

- **Pool Overview:** Health status, capacity, fragmentation, and dedup ratio
- **Data Topology:** Visual display of pool configuration (MIRROR, RAIDZ1/2/3, Stripe)
- **Device Hierarchy:** View vdevs and their member disks with proper parent-child relationships
- **Scrub History:** Track scrub dates, durations, and errors over time
- **SMART Integration:** Click any drive serial to view its detailed SMART data
- **Error Tracking:** Read, write, and checksum errors at pool and device level
- **TrueNAS Compatible:** Full support for TrueNAS SCALE and CORE with GUID resolution

---

## 📸 Screenshots

<img width="461" height="537" alt="Login" src="https://github.com/user-attachments/assets/ce1b6b4b-27e6-4b10-81f0-181596af931d" />
<img width="1349" height="1030" alt="pwd_change" src="https://github.com/user-attachments/assets/6854f192-a66c-4124-bfe3-cd5ec19a949b" />
<img width="1345" height="1024" alt="screen1" src="https://github.com/user-attachments/assets/a476366b-6738-4065-b764-9bf63c56bd0a" />
<img width="1342" height="1023" alt="Screen2" src="https://github.com/user-attachments/assets/caa910a7-cbc2-4b83-b361-f424986804a9" />
<img width="1341" height="1025" alt="screen3" src="https://github.com/user-attachments/assets/ae84ee07-7a1c-4000-b53c-40a6dcfcb89e" />
<img width="1342" height="1014" alt="screen4" src="https://github.com/user-attachments/assets/bc5b9147-1fdd-428c-88de-18f34ee4d079" />

### Dashboard
The main dashboard shows all servers with their drives in a clean card grid layout.

### Drive Details
Click any drive to see detailed S.M.A.R.T. attributes, temperature, power-on hours, and health status.

### ZFS Pools
View all ZFS pools with health status, capacity, and scrub information. Click to see device hierarchy and history.

### Settings
Manage your password and account settings.

---

## 📋 Requirements

### Essential
- **Linux OS:** (64-bit recommended)
- **Root/Sudo Access:** Required for the Agent to read physical disk health and ZFS data.
- **smartmontools:** The core engine for reading HDD/SSD health data.

### Optional (for ZFS monitoring)
- **zfsutils-linux** (Linux) or **zfs** (FreeBSD/TrueNAS): Required for ZFS pool monitoring.
- **nvme-cli:** For enhanced NVMe drive support.

### Install Requirements

```bash
# Ubuntu / Debian / Proxmox
sudo apt update && sudo apt install -y smartmontools nvme-cli zfsutils-linux
```

```bash
# Fedora / CentOS / RHEL
sudo dnf install -y https://zfsonlinux.org/fedora/zfs-release-latest.noarch.rpm
sudo dnf install -y smartmontools nvme-cli zfs
```

```bash
# Arch Linux
sudo pacman -S smartmontools nvme-cli
sudo yay -S zfs-dkms
```

```bash
# TrueNAS SCALE / CORE
# ZFS tools are pre-installed, just ensure smartmontools is available
sudo apt install -y smartmontools  # SCALE
pkg install smartmontools          # CORE
```

**Optional: Arch Linux** using the archzfs Repository

Follow the instructions on the [archzfs website](https://github.com/archzfs/archzfs) to add their GPG key and repository URL.

Once added, you can then run:

```bash
sudo pacman -S zfs-linux 
```
or the version matching your kernel.

---

## 🚀 Quick Start

### 1. Deploy the Server

```bash
docker run -d \
  --name vigil-server \
  -p 9080:9080 \
  -v vigil_data:/data \
  -e ADMIN_PASS=your-secure-password \
  -e TZ=${TZ:-UTC} \
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil:latest
```

### 2. Access the Dashboard

Open `http://YOUR_SERVER_IP:9080` in your browser.

**Default login:**
- Username: `admin`
- Password: Check server logs or set via `ADMIN_PASS` environment variable

> 💡 To find the generated password, run: `docker logs vigil-server 2>&1 | grep "Admin password"`
> 
> On first login with a generated password, you'll be prompted to change it.

### 3. Deploy Agents

Generate a registration token from the web UI (**Agents → Add Agent**), then on each server:

```bash
curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- \
  -s "http://YOUR_SERVER_IP:9080" \
  -t "YOUR_REGISTRATION_TOKEN" \
  -n "my-server"
```

> This installs dependencies, downloads the agent, registers with the server, and creates a systemd service — all in one command. Add `-z` for ZFS support or `-v "v2.4.0"` for a specific version.

---

## 📦 Deployment Options

### Server: Docker Compose (Recommended)

```yaml
services:
  vigil-server:
    image: ghcr.io/pineappledr/vigil:latest
    container_name: vigil-server
    restart: unless-stopped
    ports:
      - "9080:9080"
    environment:
      - PORT=9080
      - DB_PATH=/data/vigil.db
      - AUTH_ENABLED=true
      - ADMIN_USER=admin
      - ADMIN_PASS=your-secure-password
      - TZ=${TZ:-UTC}
    volumes:
      - vigil_data:/data

volumes:
  vigil_data:
    name: vigil_data
```

### Agent: Systemd Service (Recommended)

```bash
curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- \
  -s "http://YOUR_SERVER_IP:9080" \
  -t "YOUR_REGISTRATION_TOKEN" \
  -n "my-server"
```

This one-liner downloads the install script, which automatically:
- Detects your distro and installs dependencies (smartmontools, nvme-cli)
- Downloads the latest vigil-agent binary
- Registers with the server
- Creates and enables a systemd service

**Install Script Flags:**

| Flag | Description |
|------|-------------|
| `-s` | Server URL (required) |
| `-t` | Registration token (required) |
| `-n` | Agent hostname/name (required) |
| `-z` | Enable ZFS monitoring (installs ZFS packages) |
| `-v` | Version or release tag (e.g. `v2.4.0`, `dev-release-v2.4.0`). Defaults to latest. |

**Examples:**

```bash
# Install with ZFS support
curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- \
  -s "http://YOUR_SERVER_IP:9080" -t "YOUR_TOKEN" -n "nas-01" -z

# Install a specific version
curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- \
  -s "http://YOUR_SERVER_IP:9080" -t "YOUR_TOKEN" -n "web-01" -v "v2.4.0"

# Install from a dev branch
curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- \
  -s "http://YOUR_SERVER_IP:9080" -t "YOUR_TOKEN" -n "test-01" -v "dev-release-v2.4.0"
```

### Agent: Docker (Standard Linux)

The agent auto-registers on first boot when `TOKEN` is set, then ignores it on subsequent restarts. If the container is recreated, the agent automatically reconnects using its stored credentials — no new token is needed.

```bash
docker run -d \
  --name vigil-agent \
  --restart unless-stopped \
  --network host \
  --privileged \
  -e SERVER=http://YOUR_SERVER_IP:9080 \
  -e TOKEN=YOUR_REGISTRATION_TOKEN \
  -e TZ=${TZ:-UTC} \
  -v /dev:/dev:ro \
  -v vigil_agent_data:/var/lib/vigil-agent \
  ghcr.io/pineappledr/vigil-agent:latest
```

> **Important:** The `vigil_agent_data` volume is required to persist agent credentials across container restarts and recreations.

> **ZFS Monitoring:** Add `-v /sys:/sys:ro -v /proc:/proc:ro -v /dev/zfs:/dev/zfs` if your host uses ZFS.

### Agent: Docker (TrueNAS)

For TrueNAS SCALE/CORE, use the Debian-based agent with host ZFS tools.

**docker-compose.yml:**

```yaml
services:
  vigil-agent:
    image: ghcr.io/pineappledr/vigil-agent:debian
    container_name: vigil-agent
    restart: unless-stopped
    network_mode: host
    pid: host
    privileged: true
    environment:
      SERVER: http://YOUR_SERVER_IP:9080
      TOKEN: YOUR_REGISTRATION_TOKEN
      HOSTNAME: my-truenas       # Optional: custom display name
      TZ: ${TZ:-UTC}
    volumes:
      - /dev:/dev:ro
      - /dev/zfs:/dev/zfs
      - /sbin/zpool:/sbin/zpool:ro
      - /sbin/zfs:/sbin/zfs:ro
      - /lib:/lib:ro
      - /lib64:/lib64:ro
      - /usr/lib:/usr/lib:ro
      - vigil_agent_data:/var/lib/vigil-agent
    deploy:
      resources:
        limits:
          cpus: '0.50'
          memory: 512M
        reservations:
          cpus: '0.10'
          memory: 128M

volumes:
  vigil_agent_data:
    name: vigil_agent_data
```

Or with `docker run`:

```bash
docker run -d \
  --name vigil-agent \
  --restart unless-stopped \
  --network host \
  --pid host \
  --privileged \
  -e SERVER=http://YOUR_SERVER_IP:9080 \
  -e TOKEN=YOUR_REGISTRATION_TOKEN \
  -e TZ=${TZ:-UTC} \
  -v /dev:/dev:ro \
  -v /dev/zfs:/dev/zfs \
  -v /sbin/zpool:/sbin/zpool:ro \
  -v /sbin/zfs:/sbin/zfs:ro \
  -v /lib:/lib:ro \
  -v /lib64:/lib64:ro \
  -v /usr/lib:/usr/lib:ro \
  -v vigil_agent_data:/var/lib/vigil-agent \
  ghcr.io/pineappledr/vigil-agent:debian
```

---

## 🔄 Upgrading the Agent

When a new version of Vigil is released, follow these steps to upgrade your agents.

### Upgrade Binary Agent (Systemd)

```bash
# 1. Stop the agent service
sudo systemctl stop vigil-agent

# 2. Backup the current binary (optional)
sudo cp /usr/local/bin/vigil-agent /usr/local/bin/vigil-agent.bak

# 3. Download the new version
# For latest release:
sudo curl -L https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 \
  -o /usr/local/bin/vigil-agent

# Or for a specific version (e.g., v1.2.0):
sudo curl -L https://github.com/pineappledr/vigil/releases/download/v1.2.0/vigil-agent-linux-amd64 \
  -o /usr/local/bin/vigil-agent

# 4. Make it executable
sudo chmod +x /usr/local/bin/vigil-agent

# 5. Verify the new version
vigil-agent --version

# 6. Start the agent service
sudo systemctl start vigil-agent

# 7. Check status
sudo systemctl status vigil-agent
```

### Upgrade Docker Agent

```bash
# 1. Pull the latest image
docker pull ghcr.io/pineappledr/vigil-agent:latest

# 2. Stop and remove the old container
docker stop vigil-agent
docker rm vigil-agent

# 3. Start with the new image (keep agent data volume for auth keys)
docker run -d \
  --name vigil-agent \
  --net=host \
  --privileged \
  -e SERVER=http://YOUR_SERVER_IP:9080 \
  -e TZ=${TZ:-UTC} \
  -v /dev:/dev \
  -v /sys:/sys:ro \
  -v /dev/zfs:/dev/zfs \
  -v vigil_agent_data:/var/lib/vigil-agent \
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil-agent:latest
```

> **Note:** If upgrading from v2.3.x to v2.4.0+, add `-e TOKEN=YOUR_TOKEN` on first run to auto-register. See [Upgrading from v2.3.x](#upgrading-from-v23x).

### Upgrade Script (Automated)

For convenience, you can use this one-liner to upgrade the binary agent:

```bash
# One-liner upgrade (stops, downloads, restarts)
sudo systemctl stop vigil-agent && \
sudo curl -L https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 \
  -o /usr/local/bin/vigil-agent && \
sudo chmod +x /usr/local/bin/vigil-agent && \
sudo systemctl start vigil-agent && \
echo "✅ Agent upgraded to $(vigil-agent --version)"
```

### Batch Upgrade (Multiple Servers)

If you have multiple servers running the agent, you can use SSH to upgrade them all:

```bash
# Create a list of your servers
SERVERS="server1.local server2.local server3.local"

# Upgrade each server
for server in $SERVERS; do
  echo "Upgrading $server..."
  ssh root@$server 'systemctl stop vigil-agent && \
    curl -sL https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 \
    -o /usr/local/bin/vigil-agent && \
    chmod +x /usr/local/bin/vigil-agent && \
    systemctl start vigil-agent'
  echo "✅ $server upgraded"
done
```

### Rollback (If Needed)

If you encounter issues with a new version:

```bash
# Rollback to backup
sudo systemctl stop vigil-agent
sudo mv /usr/local/bin/vigil-agent.bak /usr/local/bin/vigil-agent
sudo systemctl start vigil-agent
```

Or download a specific older version:

```bash
sudo systemctl stop vigil-agent
sudo curl -L https://github.com/pineappledr/vigil/releases/download/v1.0.0/vigil-agent-linux-amd64 \
  -o /usr/local/bin/vigil-agent
sudo chmod +x /usr/local/bin/vigil-agent
sudo systemctl start vigil-agent
```

---

## ⚙️ Configuration

### Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9080` | HTTP server port |
| `DB_PATH` | `vigil.db` | SQLite database path |
| `AUTH_ENABLED` | `true` | Enable/disable authentication |
| `ADMIN_USER` | `admin` | Default admin username |
| `ADMIN_PASS` | (generated) | Admin password (random if not set) |
| `TZ` | `UTC` | Timezone for timestamps (e.g., `America/New_York`) |

### Agent Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--server` | `SERVER` | `http://localhost:9080` | Vigil server URL |
| `--interval` | - | `60` | Reporting interval in seconds (0 = single run) |
| `--hostname` | `HOSTNAME` | (auto-detected) | Override hostname |
| `--data-dir` | - | `/var/lib/vigil-agent` | Directory for agent keys and auth state |
| `--register` | - | - | Run one-time registration, then exit |
| `--token` | `TOKEN` | - | Registration token (auto-enables `--register` if set) |
| `--version` | - | - | Show version |
| - | `TZ` | `UTC` | Timezone (should match server for consistent timestamps) |

> Environment variables override flags. When `TOKEN` is set, the agent auto-registers on first boot and skips registration on subsequent starts — ideal for Docker deployments.

---

## 🏷️ Drive Aliases

You can set custom names for your drives to make them easier to identify:

1. Hover over any drive card
2. Click the **edit icon** (pencil) in the top-right corner
3. Enter a friendly name like "Plex Media", "VM Storage", or "Backup Drive"
4. Click **Save**

Aliases are stored in the database and persist across reboots.

---

## 🔒 Agent Authentication

Starting with **v2.4.0**, Vigil uses **Ed25519 key-based mutual authentication** between the server and agents. This ensures that only authorized agents can submit reports.

### How It Works

1. **Server generates an Ed25519 key pair** on first startup (stored in the data directory alongside the database).
2. **Admin creates a registration token** from the web UI (**Agents → Add Agent**). Tokens are single-use. Expiration is optional — tokens can be set to never expire or expire after a configurable duration.
3. **Agent registers** using `--register --token <TOKEN>`. During registration, the agent generates its own Ed25519 key pair and a unique machine fingerprint, then exchanges public keys with the server. If an agent with the same fingerprint and public key reconnects, it automatically re-authenticates without consuming a new token.
4. **Agent authenticates** on each run by signing a challenge with its private key. The server verifies the signature and issues a 1-hour session token.
5. **Session auto-refreshes** — the agent proactively re-authenticates when the session has less than 5 minutes remaining.

### Registering an Agent

```bash
# From the web UI: Agents → Add Agent → copy the token
# On the target server:
sudo vigil-agent --server http://YOUR_SERVER_IP:9080 --register --token <TOKEN>

# After successful registration, run normally:
sudo vigil-agent --server http://YOUR_SERVER_IP:9080 --interval 60
```

### Upgrading from v2.3.x

> **⚠️ Breaking Change:** Agents running v2.3.x or earlier will be rejected by a v2.4.0+ server. You must:
> 1. Update the server to v2.4.0
> 2. Download the new agent binary on each server
> 3. Generate a registration token from the web UI
> 4. Register each agent with `--register --token <TOKEN>`
> 5. Restart the agent service

### Security Details

- **Key storage:** Server keys in `vigil.key`/`vigil.pub`, agent key in `agent.key` (0600 permissions)
- **Machine fingerprint:** Derived from `/etc/machine-id`, MAC address, or random (persisted to file)
- **Fingerprint mismatch:** If an agent presents a known public key but a different fingerprint, the report is **rejected** and an alert is logged
- **Session TTL:** 1 hour with automatic refresh

---

## 🔐 Authentication

### First Login

When you first start Vigil with authentication enabled:

1. If `ADMIN_PASS` is not set, a random password is generated and printed to stderr on first startup:
   ```
   🔑 Generated admin password — check server logs only on first run
     Admin password: a1b2c3d4e5f6
   ✓ Created admin user: admin
   ```

2. Login at `http://YOUR_SERVER_IP:9080/login.html`

3. You'll be prompted to change your password on first login

### Disable Authentication

For internal networks or testing, you can disable authentication:

```bash
docker run -e AUTH_ENABLED=false ghcr.io/pineappledr/vigil:latest
```

### Security Features

| Layer | Protection |
|-------|-----------|
| **Passwords** | bcrypt hashing (cost 10) |
| **Sessions** | Cryptographically random tokens, 7-day TTL, hourly cleanup |
| **Cookies** | `HttpOnly`, `Secure` (HTTPS), `SameSite=Lax` |
| **Rate Limiting** | Per-IP token bucket — 5 req/min on login, 10 req/min on agent auth |
| **XSS** | All user-controlled data escaped in HTML and JavaScript contexts |
| **CORS** | Origin reflection with `Vary: Origin` (no wildcard) |
| **CI/CD** | govulncheck, gosec, and Trivy scans gate every build |

---

## 🔧 HBA Controller Support

Vigil automatically handles drives behind SAS HBA controllers (like LSI SAS3224, Broadcom, etc.):

- Automatically tries multiple device types (`sat`, `scsi`, `auto`)
- No manual configuration required
- Works with SATA drives connected to SAS backplanes

---

## 📡 API Endpoints

### Public Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/version` | Get server version |
| `GET` | `/api/auth/status` | Check authentication status |
| `POST` | `/api/auth/login` | Login |
| `POST` | `/api/auth/logout` | Logout |
| `POST` | `/api/report` | Receive agent reports (requires agent session) |
| `GET` | `/api/v1/server/pubkey` | Get server's Ed25519 public key |
| `POST` | `/api/v1/agents/register` | Register agent with token |
| `POST` | `/api/v1/agents/auth` | Authenticate agent (Ed25519 signature) |

### Protected Endpoints (Require Authentication)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/history` | Get latest reports per host |
| `GET` | `/api/hosts` | List all known hosts |
| `DELETE` | `/api/hosts/{hostname}` | Remove a host and its data |
| `GET` | `/api/hosts/{hostname}/history` | Get host history |
| `GET` | `/api/aliases` | Get all drive aliases |
| `POST` | `/api/aliases` | Set a drive alias |
| `DELETE` | `/api/aliases/{id}` | Delete an alias |
| `GET` | `/api/users/me` | Get current user |
| `POST` | `/api/users/password` | Change password |
| `POST` | `/api/users/username` | Change username |

### Agent Management Endpoints (Require Authentication)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/agents` | List all registered agents |
| `DELETE` | `/api/v1/agents/{id}` | Delete an agent |
| `POST` | `/api/v1/tokens` | Create a registration token |
| `GET` | `/api/v1/tokens` | List registration tokens |
| `DELETE` | `/api/v1/tokens/{id}` | Delete a registration token |

### ZFS Endpoints (Require Authentication)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/zfs/pools` | Get all ZFS pools |
| `GET` | `/api/zfs/pools?hostname=X` | Get pools for specific host |
| `GET` | `/api/zfs/pools/{hostname}/{poolname}` | Get pool details with devices |
| `GET` | `/api/zfs/pools/{hostname}/{poolname}/devices` | Get pool devices |
| `GET` | `/api/zfs/pools/{hostname}/{poolname}/scrubs` | Get scrub history |
| `GET` | `/api/zfs/summary` | Get ZFS summary stats |
| `GET` | `/api/zfs/health` | Get pools needing attention |
| `GET` | `/api/zfs/drive/{hostname}/{serial}` | Cross-reference drive with ZFS |
| `DELETE` | `/api/zfs/pools/{hostname}/{poolname}` | Remove pool from database |

---

## 📣 Notifications

Vigil v3.0 introduces a multi-channel notification system powered by [Shoutrrr](https://containrrr.dev/shoutrrr/). A guided provider wizard lets you configure each channel through dedicated form fields — no need to construct raw URLs.

### Supported Providers

| Provider | Fields |
|----------|--------|
| **Telegram** | Bot Token, Chat ID, Message Thread ID, Send Silently, Parse Mode |
| **Discord** | Webhook URL, Bot Display Name, Avatar URL |
| **Slack** | Incoming Webhook URL, Bot Username, Icon Emoji, Channel |
| **Email (SMTP)** | Host, Port, Security (None / STARTTLS / SSL), Username, Password, From, To, Subject |
| **Pushover** | User Key, App Token, Device, Title, Priority, Sound |
| **Gotify** | Server URL, App Token, Priority |
| **Generic Webhook** | Webhook URL |

### Features

- **Provider Wizard** — Select a provider from the dropdown and fill in the dedicated fields. Vigil builds and validates the Shoutrrr URL automatically.
- **Test Before Save** — Send a test notification directly from the setup modal to verify your credentials before committing.
- **Event Rules** — Choose which event types (drive failure, ZFS errors, add-on notifications, etc.) each service should receive.
- **Quiet Hours** — Suppress non-critical alerts during configurable time windows.
- **Digest Batching** — Aggregate frequent events into periodic summaries instead of individual messages.
- **Secret Masking** — Password and token fields are masked in API responses. Editing a service preserves secrets unless you explicitly change them.

---

## 🧩 Add-ons

Vigil supports **add-ons** — external programs that extend the server with custom functionality. Add-ons register themselves via the API, stream real-time telemetry over WebSocket, and render their UI from a declarative JSON manifest.

### How Add-ons Work

```
┌──────────────┐   1. Register     ┌──────────────┐   SSE Stream   ┌──────────────┐
│   Add-on     │ ───(POST /api)──► │ Vigil Server │ ─────────────► │   Browser    │
│  (external)  │                   │              │                │  (Dashboard) │
│              │ ◄──(WebSocket)──► │  /api/addons │                │  Add-ons Tab │
│              │   2. Telemetry    │     /ws      │                │              │
└──────────────┘                   └──────────────┘                └──────────────┘
```

1. **Register** — The add-on POSTs a JSON manifest describing its name, version, and UI pages
2. **Connect** — The add-on opens a WebSocket to stream telemetry (progress, logs, notifications)
3. **Render** — The Vigil dashboard reads the manifest and renders the add-on's UI automatically
4. **Act** — Users trigger actions from the UI; the server validates and signs commands for the agent

### Creating an Add-on

#### Step 1: Define a Manifest

The manifest is a JSON document that describes your add-on's UI. It contains pages, and each page contains components.

```json
{
  "name": "my-addon",
  "version": "1.0.0",
  "description": "Example add-on for Vigil",
  "author": "Your Name",
  "pages": [
    {
      "id": "config",
      "title": "Configuration",
      "components": [
        {
          "type": "form",
          "id": "settings-form",
          "title": "Settings",
          "config": {
            "action": "apply_settings",
            "fields": [
              { "name": "target_pool", "label": "Target Pool", "type": "select", "required": true, "options": [{"label": "Pool A", "value": "pool-a"}] },
              { "name": "passes", "label": "Number of Passes", "type": "number", "required": true },
              { "name": "dry_run", "label": "Dry Run", "type": "checkbox" }
            ]
          }
        }
      ]
    },
    {
      "id": "status",
      "title": "Status",
      "components": [
        { "type": "progress", "id": "job-progress", "title": "Job Progress" },
        { "type": "log-viewer", "id": "job-logs", "title": "Logs" },
        { "type": "chart", "id": "speed-chart", "title": "Transfer Speed", "config": { "y_label": "MB/s" } }
      ]
    }
  ]
}
```

**Available component types:**

| Type | Description |
|------|-------------|
| `form` | Input form with text, number, select, checkbox, range fields. Supports `visible_when`, `depends_on`, `live_calculation`, and `security_gate` |
| `progress` | Per-job progress cards with phase tracking, speed, and ETA |
| `chart` | Chart.js time-series with optional dual Y-axes |
| `smart-table` | SMART attribute table with delta highlighting |
| `log-viewer` | Auto-tailing log terminal with severity filtering |

#### Step 2: Register the Add-on

```bash
curl -X POST http://YOUR_SERVER:9080/api/addons \
  -H "Content-Type: application/json" \
  -H "Cookie: session=YOUR_SESSION_TOKEN" \
  -d '{
    "manifest": {
      "name": "my-addon",
      "version": "1.0.0",
      "description": "Example add-on",
      "pages": [ ... ]
    }
  }'
```

The server validates the manifest structure and returns the addon record with an `id`.

#### Step 3: Connect via WebSocket

After registration, open a WebSocket connection to stream telemetry:

```
ws://YOUR_SERVER:9080/api/addons/ws?addon_id=<ID>
```

Send JSON frames to report status:

```json
// Heartbeat (keeps addon "online")
{"type": "heartbeat"}

// Progress update
{"type": "progress", "payload": {
  "job_id": "job-001",
  "phase": "scanning",
  "percent": 45.5,
  "message": "Scanning drive 3 of 8",
  "bytes_done": 5368709120,
  "bytes_total": 11811160064
}}

// Log entry
{"type": "log", "payload": {
  "level": "info",
  "message": "Starting pass 2",
  "source": "worker-1"
}}

// Notification (published to the event bus → Shoutrrr)
{"type": "notification", "payload": {
  "event_type": "job_complete",
  "severity": "info",
  "message": "All drives passed burn-in test",
  "metadata": {"drives": "8", "duration": "24h"}
}}
```

All telemetry is bridged to SSE so the browser receives live updates automatically.

#### Step 4: View in the Dashboard

Navigate to **Extensions → Add-ons** in the sidebar. Your addon appears in the list. Click it to open its manifest-driven UI with live telemetry.

### Add-on Lifecycle

| Status | Meaning |
|--------|---------|
| **Online** | WebSocket connected and heartbeats received |
| **Degraded** | Missed 3 consecutive heartbeat intervals |
| **Offline** | WebSocket disconnected |

The server runs a heartbeat monitor that automatically transitions addons between these states and publishes events to the notification bus.

### Form Features

Forms support advanced behaviors defined in the manifest:

- **`visible_when`** — Conditionally show fields: `"visible_when": "mode = advanced"` (supports `=`, `!=`, `>`, `<`, `>=`, `<=`)
- **`depends_on`** — Cascading dropdowns: when a parent field changes, child options are fetched from `GET /api/addons/{id}/options?field={name}&parent_value={val}`
- **`live_calculation`** — Safe arithmetic expressions: `"live_calculation": "capacity * (percent / 100)"` — evaluated in the browser with a sandboxed parser
- **`security_gate`** — Destructive actions require a 3-step confirmation: Password → Review & type CONFIRM → Execute

### API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/addons` | Register/update add-on (upsert by name) |
| `GET` | `/api/addons` | List all registered add-ons |
| `GET` | `/api/addons/{id}` | Get add-on details + manifest |
| `DELETE` | `/api/addons/{id}` | Deregister add-on |
| `PUT` | `/api/addons/{id}/enabled` | Enable/disable add-on |
| `GET` | `/api/addons/{id}/telemetry` | SSE stream (browser) |
| `GET` | `/api/addons/ws?addon_id=X` | WebSocket (add-on process) |
| `POST` | `/api/addons/register` | Register add-on from UI wizard |
| `POST` | `/api/addons/tokens` | Create add-on registration token |
| `GET` | `/api/addons/tokens` | List add-on registration tokens |
| `DELETE` | `/api/addons/tokens/{id}` | Delete add-on registration token |

### Notification Endpoints (Require Authentication)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/notifications/providers` | Get provider field schemas (for wizard) |
| `GET` | `/api/notifications/services` | List notification services |
| `GET` | `/api/notifications/services/{id}` | Get service details (secrets masked) |
| `POST` | `/api/notifications/services` | Create notification service |
| `PUT` | `/api/notifications/services/{id}` | Update notification service |
| `DELETE` | `/api/notifications/services/{id}` | Delete notification service |
| `PUT` | `/api/notifications/services/{id}/rules` | Update event routing rules |
| `PUT` | `/api/notifications/services/{id}/quiet-hours` | Configure quiet hours |
| `PUT` | `/api/notifications/services/{id}/digest` | Configure digest batching |
| `POST` | `/api/notifications/test` | Fire a test notification |
| `POST` | `/api/notifications/test-url` | Test a Shoutrrr URL or provider fields |
| `GET` | `/api/notifications/history` | Get notification dispatch history |

---

## 🔨 Build from Source

```bash
# Clone the repository
git clone https://github.com/pineappledr/vigil.git
cd vigil

# Build the server
go build -o vigil-server ./cmd/server

# Build the agent
go build -o vigil-agent ./cmd/agent

# Cross-compile for Linux (from macOS/Windows)
GOOS=linux GOARCH=amd64 go build -o vigil-agent-linux-amd64 ./cmd/agent
GOOS=linux GOARCH=arm64 go build -o vigil-agent-linux-arm64 ./cmd/agent
```

---

## 🛠️ Development Builds

Dev branch builds are automatically compiled and published as GitHub prereleases. Each branch gets its own release tag (`dev-{branch-name}`), so you can install dev binaries the same way as stable releases.

### Install Dev Agent Binary

Use the install script with the `-v` flag pointing to your branch's dev release tag:

```bash
# Install from a dev branch (e.g., release-v2.4.0)
curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- \
  -s "http://YOUR_SERVER_IP:9080" \
  -t "YOUR_TOKEN" \
  -n "test-server" \
  -v "dev-release-v2.4.0"
```

Or download manually:

```bash
# Download dev binary directly
curl -L https://github.com/pineappledr/vigil/releases/download/dev-release-v2.4.0/vigil-agent-linux-amd64 \
  -o vigil-agent
chmod +x vigil-agent
```

> Dev releases are updated on every push to the branch. Find available dev releases on the [Releases page](https://github.com/pineappledr/vigil/releases).

### Use Dev Docker Images

```bash
# Pull dev branch images
docker pull ghcr.io/pineappledr/vigil:dev-develop
docker pull ghcr.io/pineappledr/vigil-agent:dev-develop

# Or for feature branches (slashes replaced with dashes)
docker pull ghcr.io/pineappledr/vigil-agent:dev-release-v2.4.0
```

---

## 🐛 Troubleshooting

### Agent not detecting drives

1. Ensure `smartmontools` is installed
2. Run `smartctl --scan` to see detected drives
3. Check if drives need special device type: `smartctl -a -d sat /dev/sdX`

### "Unknown Drive" showing instead of model name

This can happen with drives behind HBA controllers. The latest agent version automatically handles this, but the drive may be reporting limited info. Setting an alias can help identify the drive.

### ZFS pools not showing

1. Ensure ZFS tools are installed (`zpool` command available)
2. Check agent logs for ZFS detection: `journalctl -u vigil-agent | grep -i zfs`
3. For TrueNAS Docker deployments, ensure host ZFS binaries are mounted (see TrueNAS docker-compose)
4. Verify ZFS is detected: `sudo zpool list`

### ZFS showing GUIDs instead of device names

On TrueNAS, ZFS uses disk GUIDs by default. The agent attempts to resolve these to device names. If GUIDs still appear:
1. Update to the latest agent version
2. The frontend will shorten long GUIDs for display
3. Serial numbers are used for SMART data correlation

### Agent rejected with 401 Unauthorized

A 401 error means the agent cannot authenticate with the server. This is usually caused by an expired session token or a missing registration. When this happens, the agent stops sending reports and the server will show the system as **"Not Reporting"** — even though the server itself is running fine.

**How agent authentication works:**

1. You generate a **registration token** from the Vigil web UI (**Agents → Add System**)
2. The agent uses this one-time token to register with the server (`--register --token <TOKEN>`)
3. After registration, the agent stores its credentials in `/var/lib/vigil-agent/auth.json`
4. The agent automatically authenticates using Ed25519 keys and receives a **session token** (valid for 1 hour)
5. Session tokens are renewed automatically — no manual intervention needed after initial registration

**Troubleshooting steps:**

1. **Check if the agent is registered** — look for the credentials file:
   ```bash
   sudo ls -la /var/lib/vigil-agent/auth.json
   ```
   If this file doesn't exist, the agent was never registered or its state was cleared.

2. **Check agent logs** for authentication errors:
   ```bash
   sudo journalctl -u vigil-agent --since "10 minutes ago"
   ```
   Look for messages like `401`, `Unauthorized`, or `authentication failed`.

3. **Generate a new token and re-register** if the agent is not registered:
   - Go to the Vigil web UI → **Agents** → **Add System** → copy the registration command
   - On the agent machine, run:
     ```bash
     sudo vigil-agent --server http://YOUR_SERVER_IP:9080 --register --token <NEW_TOKEN>
     ```

4. **Restart the agent** after re-registering:
   ```bash
   sudo systemctl restart vigil-agent
   ```

5. If upgrading from **v2.3.x**, all agents must be re-registered because v2.4.0+ switched to Ed25519 key-based authentication (see [Upgrading from v2.3.x](#upgrading-from-v23x))

> **Note:** Registration tokens are single-use. If a token was configured with an expiration, it may have expired — generate a new one from the web UI. If the agent was previously registered and is simply reconnecting (same fingerprint and key), it will automatically re-authenticate without needing a new token.

### Agent rejected with 403 Forbidden (fingerprint mismatch)

The agent's machine fingerprint has changed (e.g., hardware change, VM migration). The fingerprint is generated from the machine's unique hardware identifiers and is used as a security measure to prevent unauthorized agents from impersonating registered systems.

1. Delete the old agent from the web UI (**Agents** page)
2. On the agent machine, remove the old state:
   ```bash
   sudo rm -rf /var/lib/vigil-agent/
   ```
3. Generate a new registration token from the web UI and re-register the agent

### Drive added, removed, or replaced

Vigil automatically detects drive changes. Each time the agent reports (every 60 seconds by default), it scans for all currently connected drives using `smartctl --scan`.

- **Drive removed:** The drive disappears from the dashboard on the next report cycle. Historical data for that drive is preserved in the database.
- **Drive added:** New drives appear automatically on the next report cycle.
- **Drive replaced:** The old drive disappears and the new one appears — Vigil tracks drives by serial number, so the replacement is treated as a new drive.

No manual action is needed. If you want to clean up old aliases for removed drives, you can do so from the drive detail view.

### Authentication issues

- Check logs for generated password: `docker logs vigil-server 2>&1 | grep "Admin password"`
- Reset by deleting the database: `docker volume rm vigil_data`

### Agent version mismatch

Check your agent version:
```bash
vigil-agent --version
```

Compare with the latest release on [GitHub Releases](https://github.com/pineappledr/vigil/releases).

---

## 📄 License

MIT License - See [LICENSE](LICENSE) for details.

---

> **Note:** This code has been created with the help of AI. Every change has been tested extensively before merging to main.