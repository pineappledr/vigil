# Vigil

> **Proactive, lightweight server monitoring.**

![Build Status](https://github.com/pineappledr/vigil/actions/workflows/build.yml/badge.svg)
![License](https://img.shields.io/github/license/pineappledr/vigil)
![Go Version](https://img.shields.io/github/go-mod/go-version/pineappledr/vigil)

**Vigil** is a next-generation monitoring system built for speed and simplicity. It provides instant visibility into your infrastructure with a mobile-first design and predictive health analysis, ensuring you never miss a critical hardware failure.

---

## 🚀 Features

- **🔥 Single Binary Architecture:** No complex databases or multi-container setups. Just one file.
- **📱 Mobile-First:** Native iOS & Android app (Flutter) for monitoring on the go.
- **🔍 Predictive Health Check:** Advanced analysis to determine if a drive is *actually* dying or just old.
- **⚡ Real-time S.M.A.R.T. Tracking:** Monitors temperature, reallocated sectors, and power-on hours.
- **🔔 Push Notifications:** Get alerted instantly on your phone when a drive fails.

---

## 🛠️ Architecture

Vigil follows a clean **Hub & Spoke** model:

1.  **Vigil Agent (Go):** A lightweight binary that runs on your servers (Proxmox, Ubuntu, Unraid). It wraps `smartctl` to read raw disk health.
2.  **Vigil Server (Go):** The central hub that receives data, stores it in SQLite, and serves the API.
3.  **Vigil UI (Flutter):** A beautiful, responsive interface that runs as a Web App *and* a Native Mobile App.

---

## 📦 Installation

### 1. The Agent (Proxmox / Linux)
The agent is a single static binary. You can download it from the [Releases](https://github.com/pineappledr/vigil/releases) page.

**One-Liner Install (Coming Soon):**
```bash
curl -sL [https://vigil.sh/install-agent](https://vigil.sh/install-agent) | sudo bash