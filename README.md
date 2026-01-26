# Vigil

> **Proactive, lightweight server monitoring.**

<p align="left">
  <img src="https://github.com/pineappledr/vigil/actions/workflows/ci.yml/badge.svg" alt="Build Status">
  <img src="https://img.shields.io/github/license/pineappledr/vigil" alt="License">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/SQLite-v1.44.0-003B57?logo=sqlite&logoColor=white" alt="SQLite Version">
</p>

<p align="right">
  <a href="https://github.com/pineappledr/vigil">
    <img src="https://img.shields.io/badge/GitHub-Repository-181717?logo=github&logoColor=white" alt="GitHub">
  </a>
</p>

**Vigil** is a next-generation monitoring system built for speed and simplicity. It provides instant visibility into your infrastructure with a modern web dashboard and predictive health analysis, ensuring you never miss a critical hardware failure.

Works on **any Linux system** (Ubuntu, Debian, Proxmox, Unraid, Fedora, etc.).

---

## 🚀 Features

- **🔥 Lightweight Agent:** Single Go binary with zero dependencies. Deploy it on any server in seconds.
- **🐳 Docker Server:** The central hub is containerized for easy deployment via Docker or Compose.
- **⚡ Fast Web Dashboard:** Modern HTML5/JS interface that loads instantly with real-time updates.
- **🔍 Deep Analysis:** View raw S.M.A.R.T. attributes, temperature history, and drive details.
- **🤖 Predictive Checks:** Advanced analysis to determine if a drive is failing or just aging.
- **📊 Continuous Monitoring:** Configurable reporting intervals with automatic reconnection.

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

## Deployment: Server

The central server runs in a container. It collects data from all your agents.

### Option A: Docker Run (Quick)

```bash
docker run -d \
  --name vigil-server \
  -p 9080:9080 \
  -v vigil_data:/data \
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil:latest
```

### Option B: Docker Compose

```yaml
services:
  vigil-server:
    container_name: vigil-server
    image: ghcr.io/pineappledr/vigil:latest
    restart: unless-stopped
    ports:
      - "9080:9080"
    environment:
      - PORT=9080
      - DB_PATH=/data/vigil.db
    volumes:
      - vigil_data:/data

volumes:
  vigil_data:
    name: vigil_data
```

---

## Deployment: Agent

The agent runs on your managed nodes (NAS, Servers, VMs). You can run it as a binary (recommended) or via Docker.

### Option A: Binary (One-Line Install)

Download and run the agent directly from GitHub.

```bash
# 1. Download and Install (Replace v1.0.0 with your latest version)
sudo curl -L https://github.com/pineappledr/vigil/releases/download/v1.0.4/vigil-agent-linux-amd64 \
  -o /usr/local/bin/vigil-agent

sudo chmod +x /usr/local/bin/vigil-agent

# 2. Run (Replace with your Server IP)
sudo vigil-agent --server http://YOUR_SERVER_IP:9080 --interval 60
```

### Option B: Systemd Service (Recommended for Production)

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
sudo systemctl status vigil-agent
```

### Option C: Docker Agent

> **Note:** The agent requires privileged access to read physical disk stats.

**Docker Run:**

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

**Docker Compose:**

```yaml
services:
  vigil-agent:
    container_name: vigil-agent
    image: ghcr.io/pineappledr/vigil-agent:latest
    restart: unless-stopped
    network_mode: host
    privileged: true
    command: ["--server", "http://YOUR_SERVER_IP:9080", "--interval", "60"]
    volumes:
      - /dev:/dev
```

---

## Configuration

### Agent Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | `http://localhost:9080` | Vigil server URL |
| `--interval` | `60` | Reporting interval in seconds (0 = single run) |
| `--hostname` | (auto-detected) | Override hostname |

### Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9080` | HTTP server port |
| `DB_PATH` | `vigil.db` | SQLite database path |

---

## Build from Source

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

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/api/report` | Receive agent reports |
| `GET` | `/api/history` | Get latest reports per host |
| `GET` | `/api/hosts` | List all known hosts |
| `DELETE` | `/api/hosts/{hostname}` | Remove a host and its data |

---

## License

MIT License - See [LICENSE](LICENSE) for details.