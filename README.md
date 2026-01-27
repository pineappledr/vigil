# Vigil

> **Proactive, lightweight server & drive monitoring with SMART health analysis.**

<p align="left">
  <img src="https://github.com/pineappledr/vigil/actions/workflows/ci.yml/badge.svg" alt="Build Status">
  <img src="https://img.shields.io/github/license/pineappledr/vigil" alt="License">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/SQLite-v1.44.0-003B57?logo=sqlite&logoColor=white" alt="SQLite Version">
</p>

**Vigil** is a next-generation monitoring system built for speed and simplicity. It provides instant visibility into your infrastructure with a modern web dashboard and predictive health analysis, ensuring you never miss a critical hardware failure.

Works on **any Linux system** (Ubuntu, Debian, Proxmox, Unraid, Fedora, etc.) including systems with **LSI/Broadcom HBA controllers**.

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

### Settings
Manage your password and account settings.

---

## 📋 Requirements

**Essential:**
- **Linux OS:** (64-bit recommended)
- **Root/Sudo Access:** Required for the Agent to read physical disk health.
- **smartmontools:** The core engine for reading HDD/SSD health data.

**Install Requirements:**

```bash
# Ubuntu / Debian / Proxmox
sudo apt update && sudo apt install smartmontools nvme-cli -y
```

```bash
# Fedora / CentOS / RHEL
sudo dnf install smartmontools nvme-cli
```

```bash
# Arch Linux
sudo pacman -S smartmontools nvme-cli
```

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

On each server you want to monitor:

```bash
# Download agent
sudo curl -L https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 \
  -o /usr/local/bin/vigil-agent
sudo chmod +x /usr/local/bin/vigil-agent

# Run agent
sudo vigil-agent --server http://YOUR_SERVER_IP:9080 --interval 60
```

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
# Create service file
sudo tee /etc/systemd/system/vigil-agent.service > /dev/null <<EOF
[Unit]
Description=Vigil Monitoring Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vigil-agent --server http://YOUR_SERVER_IP:9080 --interval 60
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable vigil-agent
sudo systemctl start vigil-agent
```

### Agent: Docker

```bash
docker run -d \
  --name vigil-agent \
  --net=host \
  --privileged \
  -v /dev:/dev \
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil-agent:latest \
  --server http://YOUR_SERVER_IP:9080 \
  --interval 60
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

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | `http://localhost:9080` | Vigil server URL |
| `--interval` | `60` | Reporting interval in seconds (0 = single run) |
| `--hostname` | (auto-detected) | Override hostname |
| `--version` | - | Show version |

---

## 🏷️ Drive Aliases

You can set custom names for your drives to make them easier to identify:

1. Hover over any drive card
2. Click the **edit icon** (pencil) in the top-right corner
3. Enter a friendly name like "Plex Media", "VM Storage", or "Backup Drive"
4. Click **Save**

Aliases are stored in the database and persist across reboots.

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
| `POST` | `/api/report` | Receive agent reports |

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

## 🐛 Troubleshooting

### Agent not detecting drives

1. Ensure `smartmontools` is installed
2. Run `smartctl --scan` to see detected drives
3. Check if drives need special device type: `smartctl -a -d sat /dev/sdX`

### "Unknown Drive" showing instead of model name

This can happen with drives behind HBA controllers. The latest agent version automatically handles this, but the drive may be reporting limited info. Setting an alias can help identify the drive.

### Authentication issues

- Check logs for generated password: `docker logs vigil-server | grep password`
- Reset by deleting the database: `docker volume rm vigil_data`

---

## 📄 License

MIT License - See [LICENSE](LICENSE) for details.

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/pineappledr">PineappleDR</a>
</p>
