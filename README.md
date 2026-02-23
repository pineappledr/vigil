<div align="center">

  <h1>
    <img src="web/img/Vigil.png" alt="Vigil Logo" width="120" style="vertical-align: middle; margin-right: 5px;">
    <span style="vertical-align: middle;">Vigil</span>
  </h1>

  **Proactive, lightweight server & drive monitoring with S.M.A.R.T. health analysis and ZFS pool management.**
  
  <p>
    <img src="https://github.com/pineappledr/vigil/actions/workflows/ci.yml/badge.svg" alt="Build Status">
    <img src="https://img.shields.io/github/license/pineappledr/vigil" alt="License">
    <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go Version">
    <img src="https://img.shields.io/badge/SQLite-v1.44.0-003B57?logo=sqlite&logoColor=white" alt="SQLite Version">
  </p>

</div>

> **⚠️ BREAKING CHANGE in v2.4.0:** This release introduces **Ed25519 key-based mutual authentication** between the server and agents. All existing agents **must be re-registered** using a one-time registration token. Agents running older versions will be rejected by the server. See [Agent Authentication](#-agent-authentication) for the new setup workflow.

**Vigil** is a next-generation monitoring system built for speed and simplicity. It provides instant visibility into your infrastructure with a modern web dashboard, predictive health analysis, and comprehensive ZFS pool monitoring, ensuring you never miss a critical hardware failure.

Works on **any Linux system** (Ubuntu, Debian, Proxmox, TrueNAS, Unraid, Fedora, etc.) including systems with **LSI/Broadcom HBA controllers**.

---

## ✨ Features

- **🔥 Lightweight Agent:** Single Go binary with zero dependencies. Deploy it on any server in seconds.
- **🐳 Docker Server:** The central hub is containerized for easy deployment via Docker or Compose.
- **⚡ Fast Web Dashboard:** Modern HTML5/JS interface that loads instantly with real-time updates.
- **🔍 Deep Analysis:** View raw S.M.A.R.T. attributes, temperature history, and drive details.
- **🤖 Predictive Checks:** Advanced analysis to determine if a drive is failing or just aging.
- **📊 Continuous Monitoring:** Configurable reporting intervals with automatic reconnection.
- **🔐 Authentication:** Built-in login system with secure sessions.
- **🏷️ Drive Aliases:** Set custom names for your drives (e.g., "Plex Media", "Backup Drive").
- **🔧 HBA Support:** Automatic detection for SATA drives behind SAS HBA controllers (LSI SAS3224, etc.).
- **🗄️ ZFS Pool Monitoring:** Full ZFS support with pool health, device hierarchy, scrub history, and SMART integration.

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
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil:latest
```

### 2. Access the Dashboard

Open `http://YOUR_SERVER_IP:9080` in your browser.

**Default login:**
- Username: `admin`
- Password: Check server logs or set via `ADMIN_PASS` environment variable

> 💡 To find the generated password in the logs, run: `docker logs vigil-server 2>&1 | grep "Generated admin password"`
> 
> On first login with a generated password, you'll be prompted to change it.

### 3. Deploy Agents

Generate a registration token from the web UI (**Agents → Add Agent**), then on each server:

```bash
curl -sL https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 -o /tmp/vigil-agent && chmod +x /tmp/vigil-agent && sudo mv /tmp/vigil-agent /usr/local/bin/ && sudo vigil-agent --server http://YOUR_SERVER_IP:9080 --register --token YOUR_TOKEN
```

> The agent auto-registers on first run when `TOKEN` is provided, then continues reporting normally on subsequent starts.

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
    volumes:
      - vigil_data:/data

volumes:
  vigil_data:
    name: vigil_data
```

### Agent: Systemd Service (Recommended)

```bash
# Install dependencies (Debian/Ubuntu)
sudo apt update && sudo apt install -y smartmontools nvme-cli zfsutils-linux

# Download and install vigil-agent
curl -sL https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 -o /tmp/vigil-agent \
  && chmod +x /tmp/vigil-agent \
  && sudo mv /tmp/vigil-agent /usr/local/bin/

# Register agent with server
sudo vigil-agent --register --server http://YOUR_SERVER_IP:9080 --token YOUR_REGISTRATION_TOKEN --hostname my-server

# Create systemd service
sudo tee /etc/systemd/system/vigil-agent.service > /dev/null <<EOF
[Unit]
Description=Vigil Monitoring Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vigil-agent --server http://YOUR_SERVER_IP:9080 --hostname my-server --interval 60
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start the service
sudo systemctl daemon-reload && sudo systemctl enable --now vigil-agent
```

### Agent: Docker (Standard Linux)

The agent auto-registers on first boot when `TOKEN` is set, then ignores it on subsequent restarts.

```bash
docker run -d \
  --name vigil-agent \
  --restart unless-stopped \
  --network host \
  --privileged \
  -e SERVER=http://YOUR_SERVER_IP:9080 \
  -e TOKEN=YOUR_REGISTRATION_TOKEN \
  -v /dev:/dev:ro \
  -v /sys:/sys:ro \
  -v /proc:/proc:ro \
  -v /dev/zfs:/dev/zfs \
  -v vigil_agent_data:/var/lib/vigil-agent \
  ghcr.io/pineappledr/vigil-agent:latest
```

### Agent: Docker (TrueNAS)

For TrueNAS SCALE/CORE, use the Debian-based agent with host ZFS tools:

```bash
docker run -d \
  --name vigil-agent \
  --restart unless-stopped \
  --network host \
  --pid host \
  --privileged \
  -e SERVER=http://YOUR_SERVER_IP:9080 \
  -e TOKEN=YOUR_REGISTRATION_TOKEN \
  -v /dev:/dev:ro \
  -v /sys:/sys:ro \
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
2. **Admin creates a registration token** from the web UI (**Agents → Add Agent**). Tokens are single-use and expire after 24 hours.
3. **Agent registers** using `--register --token <TOKEN>`. During registration, the agent generates its own Ed25519 key pair and a unique machine fingerprint, then exchanges public keys with the server.
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

1. If `ADMIN_PASS` is not set, a random password is generated and logged:
   ```
   🔑 Generated admin password: a1b2c3d4e5f6
   ✓ Created admin user: admin
   ```

2. Login at `http://YOUR_SERVER_IP:9080/login.html`

3. You'll be prompted to change your password on first login

### Disable Authentication

For internal networks or testing, you can disable authentication:

```bash
docker run -e AUTH_ENABLED=false ghcr.io/pineappledr/vigil:latest
```

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

Dev branch builds are automatically compiled and available as artifacts in GitHub Actions. This is useful for testing new features before they're released.

### Download Dev Agent Binary

1. Go to [GitHub Actions](https://github.com/pineappledr/vigil/actions)
2. Click on the latest workflow run for your branch (e.g., `develop`)
3. Scroll down to **Artifacts**
4. Download `vigil-agent-dev-{branch}-{commit}`

### Use Dev Docker Images

```bash
# Pull dev branch images
docker pull ghcr.io/pineappledr/vigil:dev-develop
docker pull ghcr.io/pineappledr/vigil-agent:dev-develop

# Or for feature branches (slashes replaced with dashes)
docker pull ghcr.io/pineappledr/vigil-agent:dev-feature-new-feature
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

This means the agent is not registered or its session has expired:
1. Check if the agent has been registered: look for `auth.json` in the agent's data directory
2. Re-register the agent: `sudo vigil-agent --server http://YOUR_SERVER_IP:9080 --register --token <NEW_TOKEN>`
3. If upgrading from v2.3.x, all agents must be re-registered (see [Upgrading from v2.3.x](#upgrading-from-v23x))

### Agent rejected with 403 Forbidden (fingerprint mismatch)

The agent's machine fingerprint has changed (e.g., hardware change, VM migration):
1. Delete the old agent from the web UI (**Agents** page)
2. On the agent machine, remove the old state: `sudo rm -rf /var/lib/vigil-agent/`
3. Re-register with a new token

### Authentication issues

- Check logs for generated password: `docker logs vigil-server | grep password`
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