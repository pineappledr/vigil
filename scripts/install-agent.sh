#!/bin/bash
set -e

# Vigil Agent Installer
# Usage: curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- -s <server> -t <token> -n <name> [-z] [-v version]

INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/vigil-agent.service"
REPO="https://github.com/pineappledr/vigil/releases"
INTERVAL=60

# ─── Parse arguments ─────────────────────────────────────────────────────────
usage() {
    echo "Vigil Agent Installer"
    echo ""
    echo "Usage: install-agent.sh -s <server_url> -t <token> -n <hostname> [-z] [-v version]"
    echo ""
    echo "  -s  Server URL (e.g. http://YOUR_SERVER_IP:9080)"
    echo "  -t  Registration token"
    echo "  -n  Agent hostname/name"
    echo "  -z  Enable ZFS monitoring (installs ZFS dependencies)"
    echo "  -v  Agent version to install (e.g. v2.4.0). Defaults to latest."
    echo "  -h  Show this help"
    exit 1
}

SERVER=""
TOKEN=""
NAME=""
ZFS=false
VERSION=""

while getopts "s:t:n:v:zh" opt; do
    case $opt in
        s) SERVER="$OPTARG" ;;
        t) TOKEN="$OPTARG" ;;
        n) NAME="$OPTARG" ;;
        z) ZFS=true ;;
        v) VERSION="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done

if [ -z "$SERVER" ] || [ -z "$TOKEN" ] || [ -z "$NAME" ]; then
    echo "Error: -s, -t, and -n are all required."
    echo ""
    usage
fi

# Build download URL based on version
if [ -n "$VERSION" ]; then
    BINARY_URL="$REPO/download/$VERSION/vigil-agent-linux-amd64"
else
    BINARY_URL="$REPO/latest/download/vigil-agent-linux-amd64"
fi

# ─── Detect distro and install dependencies ──────────────────────────────────
install_deps() {
    echo "→ Installing dependencies..."
    if command -v apt-get &>/dev/null; then
        sudo apt-get update -qq
        local pkgs="smartmontools nvme-cli"
        if [ "$ZFS" = true ]; then pkgs="$pkgs zfsutils-linux"; fi
        sudo apt-get install -y -qq $pkgs
    elif command -v dnf &>/dev/null; then
        local pkgs="smartmontools nvme-cli"
        if [ "$ZFS" = true ]; then pkgs="$pkgs zfs"; fi
        sudo dnf install -y -q $pkgs
    elif command -v pacman &>/dev/null; then
        local pkgs="smartmontools nvme-cli"
        if [ "$ZFS" = true ]; then pkgs="$pkgs zfs-utils"; fi
        sudo pacman -S --noconfirm --needed $pkgs
    else
        echo "⚠ Could not detect package manager. Please install dependencies manually."
    fi
}

# ─── Download and install binary ─────────────────────────────────────────────
install_binary() {
    echo "→ Downloading vigil-agent${VERSION:+ $VERSION}..."
    curl -sL "$BINARY_URL" -o /tmp/vigil-agent
    chmod +x /tmp/vigil-agent
    sudo mv /tmp/vigil-agent "$INSTALL_DIR/vigil-agent"
    echo "✓ Installed to $INSTALL_DIR/vigil-agent"
}

# ─── Register agent ─────────────────────────────────────────────────────────
register_agent() {
    echo "→ Registering agent with $SERVER..."
    # Use timeout to limit the registration to 5 seconds after drive detection
    # The agent should only register and exit, not run continuously
    timeout 5s sudo "$INSTALL_DIR/vigil-agent" --register --server "$SERVER" --token "$TOKEN" --hostname "$NAME" 2>&1 | while IFS= read -r line; do
        echo "$line"
        # Exit after successful report
        if echo "$line" | grep -q "Report sent"; then
            sleep 0.5  # Give it a moment to finish
            pkill -f "vigil-agent --register" 2>/dev/null || true
            break
        fi
    done
    echo "✓ Agent registered"
}

# ─── Create systemd service ─────────────────────────────────────────────────
create_service() {
    echo "→ Creating systemd service..."
    sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=Vigil Monitoring Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/vigil-agent --server $SERVER --hostname $NAME --interval $INTERVAL
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable --now vigil-agent
    echo "✓ Service enabled and started"
}

# ─── Main ────────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════╗"
echo "║       Vigil Agent Installer          ║"
echo "╚══════════════════════════════════════╝"
echo ""
echo "  Server:   $SERVER"
echo "  Name:     $NAME"
echo "  Version:  ${VERSION:-latest}"
echo "  ZFS:      $ZFS"
echo ""

install_deps
install_binary
register_agent
create_service

echo ""
echo "✅ Vigil agent is running!"
echo "   Check status: sudo systemctl status vigil-agent"
echo "   View logs:    sudo journalctl -u vigil-agent -f"
echo ""
