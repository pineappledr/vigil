# Vigil Deployment & Release Guide

## Table of Contents
1. [Server Deployment](#server-deployment)
2. [Agent Deployment](#agent-deployment)
3. [Creating Releases](#creating-releases)
4. [Semantic Versioning](#semantic-versioning)

---

## Server Deployment

The Vigil server collects data from all your agents and provides the web dashboard.

### Option 1: Docker (Recommended)

```bash
# Create a directory for Vigil
mkdir -p /opt/vigil
cd /opt/vigil

# Create docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  server:
    container_name: vigil-server
    image: ghcr.io/pineappledr/vigil:latest
    ports:
      - "8090:8090"
    volumes:
      - ./data:/data
    restart: unless-stopped
    environment:
      - PORT=8090
      - DB_PATH=/data/vigil.db

EOF

# Start the server
docker compose up -d

# Check logs
docker logs -f vigil-server
```

### Option 2: Binary

```bash
# Download the server binary
curl -L https://github.com/pineappledr/vigil/releases/latest/download/vigil-server-linux-amd64 \
  -o /usr/local/bin/vigil-server
chmod +x /usr/local/bin/vigil-server

# Create data directory
mkdir -p /var/lib/vigil

# Create systemd service
cat > /etc/systemd/system/vigil-server.service << 'EOF'
[Unit]
Description=Vigil Monitoring Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/vigil
Environment=PORT=8090
Environment=DB_PATH=/var/lib/vigil/vigil.db
ExecStart=/usr/local/bin/vigil-server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable vigil-server
systemctl start vigil-server

# Check status
systemctl status vigil-server
```

### Verify Server is Running

Open your browser and go to: `http://YOUR_SERVER_IP:8090`

You should see the Vigil dashboard (empty until agents connect).

---

## Agent Deployment

Deploy agents on each server you want to monitor.

### Prerequisites

Install smartmontools on each server:

```bash
# Ubuntu/Debian/Proxmox
sudo apt update && sudo apt install -y smartmontools

# Fedora/CentOS/RHEL
sudo dnf install -y smartmontools

# Arch Linux
sudo pacman -S smartmontools
```

### Option 1: Binary with Systemd (Recommended)

```bash
# Download the agent
sudo curl -L https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 \
  -o /usr/local/bin/vigil-agent
sudo chmod +x /usr/local/bin/vigil-agent

# Create systemd service (replace YOUR_SERVER_IP)
sudo cat > /etc/systemd/system/vigil-agent.service << 'EOF'
[Unit]
Description=Vigil Monitoring Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vigil-agent --server http://YOUR_SERVER_IP:8090 --interval 60
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable vigil-agent
sudo systemctl start vigil-agent

# Check status
sudo systemctl status vigil-agent
```

### Option 2: Docker Agent

```bash
docker run -d \
  --name vigil-agent \
  --net=host \
  --privileged \
  -v /dev:/dev:ro \
  --restart unless-stopped \
  ghcr.io/pineappledr/vigil-agent:latest \
  --server http://YOUR_SERVER_IP:8090 \
  --interval 60
```

### Option 3: One-time Manual Run (Testing)

```bash
sudo vigil-agent --server http://YOUR_SERVER_IP:8090 --interval 0
```

### Agent Configuration Options

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | `http://localhost:8090` | Vigil server URL |
| `--interval` | `60` | Reporting interval in seconds (0 = run once) |
| `--hostname` | (auto) | Override the hostname |
| `--version` | - | Show version and exit |

---

## Creating Releases

The GitHub Actions pipeline automatically creates releases when you push a version tag.

### How to Create a Release

```bash
# 1. Make sure all your changes are committed
git add .
git commit -m "Your commit message"
git push origin main

# 2. Create a version tag
git tag v1.0.0

# 3. Push the tag to GitHub
git push origin v1.0.0
```

That's it! GitHub Actions will automatically:
- Run tests
- Build binaries for AMD64 and ARM64
- Build and push Docker images
- Create a GitHub Release with:
  - Release notes (auto-generated from commits)
  - Downloadable binaries
  - SHA256 checksums
  - Installation instructions

### Viewing Your Release

After pushing a tag, go to:
`https://github.com/pineappledr/vigil/releases`

You'll see your new release with all the artifacts.

---

## Semantic Versioning

Vigil uses [Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`

### Version Format: `vX.Y.Z`

| Part | When to Increment | Example |
|------|-------------------|---------|
| **MAJOR** (X) | Breaking changes, incompatible API changes | `v1.0.0` → `v2.0.0` |
| **MINOR** (Y) | New features, backwards compatible | `v1.0.0` → `v1.1.0` |
| **PATCH** (Z) | Bug fixes, small improvements | `v1.0.0` → `v1.0.1` |

### Examples

#### Patch Release (Bug Fixes)
```bash
# Current: v1.0.0
# Fixed a UI bug, no new features
git tag v1.0.1
git push origin v1.0.1
```

#### Minor Release (New Features)
```bash
# Current: v1.0.1
# Added email notifications feature
git tag v1.1.0
git push origin v1.1.0
```

#### Major Release (Breaking Changes)
```bash
# Current: v1.1.0
# Changed API format, agents need update
git tag v2.0.0
git push origin v2.0.0
```

### Pre-release Versions

For testing before official release:

```bash
# Alpha (early testing)
git tag v1.1.0-alpha.1
git push origin v1.1.0-alpha.1

# Beta (feature complete, testing)
git tag v1.1.0-beta.1
git push origin v1.1.0-beta.1

# Release Candidate (final testing)
git tag v1.1.0-rc.1
git push origin v1.1.0-rc.1
```

Pre-release tags are automatically marked as "Pre-release" on GitHub.

### Release Checklist

Before creating a release:

1. ✅ Update version in `web/index.html` (sidebar footer)
2. ✅ Update version in `cmd/agent/main.go` and `cmd/server/main.go`
3. ✅ Test locally
4. ✅ Commit all changes
5. ✅ Push to main
6. ✅ Create and push tag

### Deleting a Tag (if you made a mistake)

```bash
# Delete local tag
git tag -d v1.0.0

# Delete remote tag
git push origin --delete v1.0.0
```

---

## Quick Reference

### Deploy Server
```bash
docker run -d --name vigil-server -p 8090:8090 -v vigil_data:/data --restart unless-stopped ghcr.io/pineappledr/vigil:latest
```

### Deploy Agent
```bash
sudo curl -L https://github.com/pineappledr/vigil/releases/latest/download/vigil-agent-linux-amd64 -o /usr/local/bin/vigil-agent && sudo chmod +x /usr/local/bin/vigil-agent
sudo vigil-agent --server http://SERVER_IP:8090 --interval 60
```

### Create Release
```bash
git tag v1.0.0 && git push origin v1.0.0
```

---

## Troubleshooting

### Agent can't connect to server
```bash
# Test connectivity
curl http://YOUR_SERVER_IP:8090/health

# Check firewall
sudo ufw allow 8090/tcp  # Ubuntu
sudo firewall-cmd --add-port=8090/tcp --permanent  # RHEL/Fedora
```

### Agent shows no drives
```bash
# Check if smartctl works
sudo smartctl --scan

# Check permissions (agent needs root)
sudo vigil-agent --server http://localhost:8090 --interval 0
```

### Check logs
```bash
# Server (Docker)
docker logs vigil-server

# Server (Systemd)
journalctl -u vigil-server -f

# Agent (Systemd)
journalctl -u vigil-agent -f
```