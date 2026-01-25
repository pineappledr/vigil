# Vigil

> **Proactive, lightweight server monitoring.**

![Build Status](https://github.com/pineappledr/vigil/actions/workflows/build.yml/badge.svg)
![License](https://img.shields.io/github/license/pineappledr/vigil)
![Go Version](https://img.shields.io/github/go-mod/go-version/pineappledr/vigil)
![SQLite Version](https://img.shields.io/badge/SQLite-v1.44.3-003B57?logo=sqlite&logoColor=white)

**Vigil** is a next-generation monitoring system built for speed and simplicity. It provides instant visibility into your infrastructure with a modern web interface and predictive health analysis, ensuring you never miss a critical hardware failure.

Works on **any Linux system** (Ubuntu, Debian, Proxmox, Unraid, Fedora, etc.).

---

## 🚀 Features

- **Lightweight Agent:** Single Go binary with zero dependencies. Deploy it on any server in seconds.
- **Docker Server:** The central hub is containerized for easy deployment via Docker or Compose.
- **Responsive Web Dashboard:** Beautiful Flutter-based web interface that works perfectly on Desktop and Mobile browsers.
- **Predictive Health Check:** Advanced analysis to determine if a drive is *actually* dying or just old.
- **Telegram Alerts:** Get instant notifications via a Telegram Bot when a drive fails.

---

## 📋 Requirements

Vigil is lightweight, but the **Agent** relies on standard system tools to talk to your hardware.

**Essential:**
- **Linux OS:** (64-bit recommended)
- **Root/Sudo Access:** Required to read physical disk health.
- **smartmontools:** The core engine for reading HDD, SSD, and NVMe health data.

**Recommended:**
- **nvme-cli:** Provides enhanced detail for NVMe drives.

**Install Requirements:**
```bash
# Ubuntu / Debian / Proxmox
sudo apt update && sudo apt install smartmontools nvme-cli -y

# Fedora / CentOS / RHEL
sudo dnf install smartmontools nvme-cli

# Arch Linux
sudo pacman -S smartmontools nvme-cli

---

## Deployment: Server
The central server runs in a container. It collects data from all your agents.

Option A: Docker Run (Quick)

```bash
docker run -d \
  --name vigil-server \
  -p 8090:8090 \
  -v vigil_data:/data \
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil:latest

Option B: Docker Compose

````
services:
  server:
    container_name: vigil-server
    image: ghcr.io/pineappledr/vigil:latest
    ports:
      - "8090:8090"
    volumes:
      - vigil_data:/data
    restart: unless-stopped
    environment:
      - PORT=8090
      - DB_PATH=/data/vigil.db

volumes:
  vigil_data:

---

## Deployment: Agent
The agent runs on your managed nodes (NAS, Servers, VMs). You can run it as a binary (recommended) or via Docker.

Option A: Binary (One-Line Install)

Download and run the agent directly from GitHub.

````bash
# 1. Download and Install (Replace v1.0.0 with your latest version)
sudo curl -L [https://github.com/pineappledr/vigil/releases/download/v1.0.0/vigil-agent-linux-amd64](https://github.com/pineappledr/vigil/releases/download/v1.0.0/vigil-agent-linux-amd64) -o /usr/local/bin/vigil-agent
sudo chmod +x /usr/local/bin/vigil-agent

# 2. Run (Replace with your Server IP)
sudo vigil-agent --server http://YOUR_SERVER_IP:8090

Option B: Docker Agent

Note: The agent requires privileged access to read physical disk stats.

Docker Run:

````bash

docker run -d \
  --name vigil-agent \
  --net=host \
  --privileged \
  -v /dev:/dev \
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil-agent:latest \
  --server http://YOUR_SERVER_IP:8090

Docker Compose:

````
services:
  agent:
    image: ghcr.io/pineappledr/vigil-agent:latest
    container_name: vigil-agent
    network_mode: host
    privileged: true
    volumes:
      - /dev:/dev
    restart: unless-stopped
    command: ["--server", "http://YOUR_SERVER_IP:8090"]
````

---
## Build from Source (Advanced)

```bash

# 1. Build the binary for Linux (amd64) on your local machine
GOOS=linux GOARCH=amd64 go build -o vigil-agent ./cmd/agent

# 2. Upload it to your server
scp vigil-agent user@YOUR_SERVER_IP:~