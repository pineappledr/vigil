# Vigil 👁️

> **Proactive, lightweight server monitoring.**

![Build Status](https://github.com/pineappledr/vigil/actions/workflows/build.yml/badge.svg)
![License](https://img.shields.io/github/license/pineappledr/vigil)
![Go Version](https://img.shields.io/github/go-mod/go-version/pineappledr/vigil)

**Vigil** is a next-generation monitoring system built for speed and simplicity. It provides instant visibility into your infrastructure with a mobile-first design and predictive health analysis, ensuring you never miss a critical hardware failure.

Works on **any Linux system** (Ubuntu, Debian, Proxmox, Unraid, Fedora, etc.).

---

## 🚀 Features

- **🔥 Single Binary Architecture:** No complex databases or multi-container setups. Just one file.
- **📱 Mobile-First:** Native iOS & Android app (Flutter) for monitoring on the go.
- **🔍 Predictive Health Check:** Advanced analysis to determine if a drive is *actually* dying or just old.
- **⚡ Real-time S.M.A.R.T. Tracking:** Monitors temperature, reallocated sectors, and power-on hours.
- **🔔 Push Notifications:** Get alerted instantly on your phone when a drive fails.

---

## 📋 Requirements

Vigil is lightweight, but it relies on standard system tools to talk to your hardware.

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