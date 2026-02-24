/**
 * Vigil - Temperature Dashboard Module
 * Integrates with existing Navigation, State, and Renderer patterns
 * 
 * Features:
 * - Drives grouped by server
 * - Per-server temperature trends
 * - Multi-drive comparison
 */

const Temperature = {
    // State
    selectedDrives: [],
    currentPeriod: '24h',
    currentServer: 'all', // 'all' or specific hostname
    compareMode: false,   // Whether we're showing comparison chart
    chartInstance: null,
    refreshInterval: null,
    dashboardData: null,

    // Icons (matching existing style)
    icons: {
        thermometer: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 14.76V3.5a2.5 2.5 0 0 0-5 0v11.26a4.5 4.5 0 1 0 5 0z"/></svg>`,
        trend: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/><polyline points="17 6 23 6 23 12"/></svg>`,
        alert: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
        check: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`,
        warning: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`,
        fire: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z"/></svg>`,
        server: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="7" rx="1"/><rect x="2" y="14" width="20" height="7" rx="1"/><circle cx="6" cy="6.5" r="1.5" fill="currentColor"/><circle cx="6" cy="17.5" r="1.5" fill="currentColor"/></svg>`,
        compare: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>`,
        refresh: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>`
    },

    /**
     * Render the temperature dashboard (called by Navigation.showTemperature)
     */
    render() {
        console.log('[Temperature] render() called');
        
        const container = document.getElementById('dashboard-view');
        if (!container) {
            console.error('[Temperature] dashboard-view container not found');
            return;
        }

        container.innerHTML = this.renderDashboard();
        this.loadData();
        this.startAutoRefresh();
    },

    /**
     * Render dashboard HTML structure
     */
    renderDashboard() {
        // Build server options for dropdown
        const servers = State.data || [];
        const serverOptions = servers.map(s => 
            `<option value="${s.hostname}" ${this.currentServer === s.hostname ? 'selected' : ''}>${s.hostname}</option>`
        ).join('');

        return `
            <div class="temp-dashboard">
                <div class="temp-header">
                    <div class="temp-header-spacer"></div>
                    <div class="temp-controls">
                        <select id="temp-period" class="temp-select" onchange="Temperature.changePeriod(this.value)">
                            <option value="5m" ${this.currentPeriod === '5m' ? 'selected' : ''}>Last 5 Minutes</option>
                            <option value="10m" ${this.currentPeriod === '10m' ? 'selected' : ''}>Last 10 Minutes</option>
                            <option value="15m" ${this.currentPeriod === '15m' ? 'selected' : ''}>Last 15 Minutes</option>
                            <option value="30m" ${this.currentPeriod === '30m' ? 'selected' : ''}>Last 30 Minutes</option>
                            <option value="1h" ${this.currentPeriod === '1h' ? 'selected' : ''}>Last 1 Hour</option>
                            <option value="24h" ${this.currentPeriod === '24h' ? 'selected' : ''}>Last 24 Hours</option>
                            <option value="7d" ${this.currentPeriod === '7d' ? 'selected' : ''}>Last 7 Days</option>
                            <option value="30d" ${this.currentPeriod === '30d' ? 'selected' : ''}>Last 30 Days</option>
                        </select>
                        <button class="btn btn-icon-text" onclick="Temperature.refresh()">
                            ${this.icons.refresh}
                            <span>Refresh</span>
                        </button>
                    </div>
                </div>

                <!-- Summary Cards -->
                <div class="temp-summary-grid" id="temp-summary-cards">
                    <div class="loading-placeholder">Loading temperature data...</div>
                </div>

                <!-- Main Content Grid - Equal height sections -->
                <div class="temp-content-grid">
                    <!-- Temperature Chart -->
                    <div class="temp-chart-section">
                        <div class="temp-section-header">
                            <h2>${this.icons.trend} Temperature Trends</h2>
                            <div class="temp-chart-controls">
                                <select id="temp-server-select" class="temp-select-sm" onchange="Temperature.changeServer(this.value)">
                                    <option value="all" ${this.currentServer === 'all' ? 'selected' : ''}>All Servers</option>
                                    ${serverOptions}
                                </select>
                                <button class="btn btn-compare" onclick="Temperature.showComparison()" id="compare-btn" disabled title="Select drives to compare">
                                    ${this.icons.compare}
                                    <span>Compare (<span id="compare-count">0</span>)</span>
                                </button>
                            </div>
                        </div>
                        <div class="temp-chart-container" id="temp-chart-container">
                            <canvas id="temp-chart"></canvas>
                        </div>
                    </div>

                    <!-- Alerts Section - Same height as chart -->
                    <div class="temp-alerts-section">
                        <div class="temp-section-header">
                            <h2>${this.icons.alert} Active Alerts</h2>
                            <span class="alert-badge" id="alert-count">0</span>
                        </div>
                        <div id="temp-alerts-list" class="temp-alerts-list">
                            <div class="loading-placeholder">Loading alerts...</div>
                        </div>
                    </div>
                </div>

                <!-- Drives by Server -->
                <div class="temp-drives-section">
                    <div class="temp-section-header">
                        <h2>${this.icons.server} Drives by Server</h2>
                        <div class="drive-filter-tabs">
                            <button class="filter-tab active" data-filter="all" onclick="Temperature.filterDrives('all')">All</button>
                            <button class="filter-tab" data-filter="normal" onclick="Temperature.filterDrives('normal')">Normal</button>
                            <button class="filter-tab" data-filter="warning" onclick="Temperature.filterDrives('warning')">Warning</button>
                            <button class="filter-tab" data-filter="critical" onclick="Temperature.filterDrives('critical')">Critical</button>
                        </div>
                    </div>
                    <div id="temp-servers-container" class="temp-servers-container">
                        <div class="loading-placeholder">Loading drives...</div>
                    </div>
                </div>
            </div>
        `;
    },

    /**
     * Load all temperature data
     */
    async loadData() {
        try {
            const [dashboard, alerts] = await Promise.all([
                this.fetchDashboardData(),
                this.fetchAlerts()
            ]);

            this.dashboardData = dashboard;
            this.renderSummaryCards(dashboard);
            this.renderAlerts(alerts);
            this.renderDrivesByServer(dashboard);
            
            // Respect compare mode - don't override with server view
            if (this.compareMode && this.selectedDrives.length >= 2) {
                this.renderComparisonChart();
            } else {
                this.renderChart(dashboard);
            }
        } catch (error) {
            console.error('[Temperature] Failed to load data:', error);
            this.showError('Failed to load temperature data');
        }
    },

    /**
     * Fetch dashboard data from API
     */
    async fetchDashboardData() {
        try {
            const response = await fetch(`/api/dashboard/temperature?details=true&period=${this.currentPeriod}`);
            if (!response.ok) throw new Error('API error');
            return await response.json();
        } catch (error) {
            console.warn('[Temperature] API not available, using current drive data');
            return this.buildFromCurrentData();
        }
    },

    /**
     * Build temperature data from current State.data (fallback)
     */
    buildFromCurrentData() {
        const thresholds = { warning: 45, critical: 55 };
        const drivesByServer = {};
        let totalTemp = 0;
        let driveCount = 0;
        let minTemp = 999;
        let maxTemp = -999;
        let hottest = null;
        let coolest = null;

        State.data.forEach(server => {
            const serverDrives = server.details?.drives || [];
            drivesByServer[server.hostname] = [];
            
            serverDrives.forEach(drive => {
                const temp = drive.temperature?.current ?? null;
                if (temp !== null) {
                    let status = 'normal';
                    if (temp >= thresholds.critical) status = 'critical';
                    else if (temp >= thresholds.warning) status = 'warning';

                    const driveData = {
                        hostname: server.hostname,
                        serial_number: drive.serial_number,
                        device_name: drive.device?.name || '/dev/sd?',
                        model: drive.model_name || 'Unknown',
                        temperature: temp,
                        status: status,
                        last_updated: server.last_seen || server.timestamp
                    };
                    drivesByServer[server.hostname].push(driveData);

                    totalTemp += temp;
                    driveCount++;
                    if (temp < minTemp) { minTemp = temp; coolest = driveData; }
                    if (temp > maxTemp) { maxTemp = temp; hottest = driveData; }
                }
            });
        });

        // Count by status
        let normal = 0, warning = 0, critical = 0;
        Object.values(drivesByServer).flat().forEach(d => {
            if (d.status === 'normal') normal++;
            else if (d.status === 'warning') warning++;
            else if (d.status === 'critical') critical++;
        });

        return {
            total_drives: driveCount,
            drives_normal: normal,
            drives_warning: warning,
            drives_critical: critical,
            avg_temperature: driveCount > 0 ? Math.round(totalTemp / driveCount * 10) / 10 : null,
            min_temperature: minTemp < 999 ? minTemp : null,
            max_temperature: maxTemp > -999 ? maxTemp : null,
            hottest_drive: hottest,
            coolest_drive: coolest,
            thresholds: thresholds,
            drives_by_server: drivesByServer
        };
    },

    /**
     * Fetch alerts from API
     */
    async fetchAlerts() {
        try {
            const response = await fetch('/api/alerts/temperature/active');
            if (!response.ok) throw new Error('API error');
            return await response.json();
        } catch (error) {
            return { alerts: [], count: 0 };
        }
    },

    /**
     * Render summary cards
     */
    renderSummaryCards(data) {
        const container = document.getElementById('temp-summary-cards');
        if (!container) return;

        const thresholds = data.thresholds || { warning: 45, critical: 55 };

        container.innerHTML = `
            ${this.summaryCard({
                icon: this.icons.thermometer,
                iconClass: 'blue',
                value: data.total_drives || 0,
                label: 'Total Drives',
                subtitle: 'Monitored'
            })}
            ${this.summaryCard({
                icon: this.icons.check,
                iconClass: 'green',
                value: data.drives_normal || 0,
                label: 'Normal',
                subtitle: `< ${thresholds.warning}°C`
            })}
            ${this.summaryCard({
                icon: this.icons.warning,
                iconClass: 'yellow',
                value: data.drives_warning || 0,
                label: 'Warning',
                subtitle: `${thresholds.warning}-${thresholds.critical}°C`
            })}
            ${this.summaryCard({
                icon: this.icons.fire,
                iconClass: 'red',
                value: data.drives_critical || 0,
                label: 'Critical',
                subtitle: `≥ ${thresholds.critical}°C`
            })}
            ${this.summaryCard({
                icon: this.icons.thermometer,
                iconClass: this.getTempClass(data.avg_temperature, thresholds),
                value: data.avg_temperature != null ? `${data.avg_temperature}°C` : 'N/A',
                label: 'Average',
                subtitle: 'All drives'
            })}
            ${this.summaryCard({
                icon: this.icons.fire,
                iconClass: this.getTempClass(data.max_temperature, thresholds),
                value: data.max_temperature != null ? `${data.max_temperature}°C` : 'N/A',
                label: 'Hottest',
                subtitle: data.hottest_drive?.model?.substring(0, 20) || 'N/A'
            })}
        `;
    },

    /**
     * Summary card component
     */
    summaryCard({ icon, iconClass, value, label, subtitle }) {
        return `
            <div class="temp-summary-card">
                <div class="temp-card-icon ${iconClass}">${icon}</div>
                <div class="temp-card-content">
                    <div class="temp-card-value">${value}</div>
                    <div class="temp-card-label">${label}</div>
                    ${subtitle ? `<div class="temp-card-subtitle">${subtitle}</div>` : ''}
                </div>
            </div>
        `;
    },

    /**
     * Get temperature color class
     */
    getTempClass(temp, thresholds) {
        if (temp == null) return 'gray';
        if (temp >= thresholds.critical) return 'red';
        if (temp >= thresholds.warning) return 'yellow';
        return 'green';
    },

    /**
     * Render alerts list
     */
    renderAlerts(data) {
        const container = document.getElementById('temp-alerts-list');
        const badge = document.getElementById('alert-count');
        if (!container) return;

        const alerts = data.alerts || [];
        if (badge) badge.textContent = alerts.length;

        if (alerts.length === 0) {
            container.innerHTML = `
                <div class="empty-alerts">
                    ${this.icons.check}
                    <p>No active alerts</p>
                    <span>All temperatures within normal range</span>
                </div>
            `;
            return;
        }

        container.innerHTML = alerts.slice(0, 10).map(alert => `
            <div class="temp-alert-item ${alert.alert_type}">
                <div class="alert-icon">
                    ${alert.alert_type === 'critical' ? this.icons.fire : this.icons.warning}
                </div>
                <div class="alert-content">
                    <div class="alert-title">${alert.hostname}</div>
                    <div class="alert-message">${alert.message}</div>
                    <div class="alert-time">${this.formatTime(alert.created_at)}</div>
                </div>
                <button class="btn-icon" onclick="Temperature.acknowledgeAlert(${alert.id})" title="Acknowledge">
                    ${this.icons.check}
                </button>
            </div>
        `).join('');
    },

    /**
     * Render drives grouped by server
     */
    renderDrivesByServer(data) {
        const container = document.getElementById('temp-servers-container');
        if (!container) return;

        // Build drives by server from data
        let drivesByServer = data.drives_by_server;
        
        // If not provided, build from drives_by_status
        if (!drivesByServer && data.drives_by_status) {
            drivesByServer = {};
            const allDrives = [
                ...(data.drives_by_status.critical || []),
                ...(data.drives_by_status.warning || []),
                ...(data.drives_by_status.normal || [])
            ];
            allDrives.forEach(drive => {
                if (!drivesByServer[drive.hostname]) {
                    drivesByServer[drive.hostname] = [];
                }
                drivesByServer[drive.hostname].push(drive);
            });
        }

        if (!drivesByServer || Object.keys(drivesByServer).length === 0) {
            container.innerHTML = `
                <div class="empty-drives">
                    ${this.icons.thermometer}
                    <p>No temperature data available</p>
                    <span>Temperature data will appear after collection</span>
                </div>
            `;
            return;
        }

        // Sort servers alphabetically
        const sortedServers = Object.keys(drivesByServer).sort();

        container.innerHTML = sortedServers.map(hostname => {
            const drives = drivesByServer[hostname];
            if (!drives || drives.length === 0) return '';

            // Calculate server stats
            const temps = drives.map(d => d.temperature).filter(t => t != null);
            const avgTemp = temps.length > 0 ? Math.round(temps.reduce((a, b) => a + b, 0) / temps.length) : null;
            const maxTemp = temps.length > 0 ? Math.max(...temps) : null;
            const warningCount = drives.filter(d => d.status === 'warning').length;
            const criticalCount = drives.filter(d => d.status === 'critical').length;

            return `
                <div class="temp-server-group" data-hostname="${Utils.escapeHtml(hostname)}">
                    <div class="temp-server-header" onclick="Temperature.toggleServerGroup('${Utils.escapeJSString(hostname)}')">
                        <div class="server-info">
                            ${this.icons.server}
                            <span class="server-name">${Utils.escapeHtml(hostname)}</span>
                            <span class="server-drive-count">${drives.length} drives</span>
                        </div>
                        <div class="server-stats">
                            ${criticalCount > 0 ? `<span class="stat critical">${criticalCount} critical</span>` : ''}
                            ${warningCount > 0 ? `<span class="stat warning">${warningCount} warning</span>` : ''}
                            <span class="stat avg">Avg: ${avgTemp || 'N/A'}°C</span>
                            <span class="stat max">Max: ${maxTemp || 'N/A'}°C</span>
                        </div>
                        <svg class="toggle-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="6 9 12 15 18 9"/>
                        </svg>
                    </div>
                    <div class="temp-server-drives">
                        ${drives.map(drive => this.driveCard(drive)).join('')}
                    </div>
                </div>
            `;
        }).join('');
    },

    /**
     * Toggle server group expand/collapse
     */
    toggleServerGroup(hostname) {
        const group = document.querySelector(`.temp-server-group[data-hostname="${hostname}"]`);
        if (group) {
            group.classList.toggle('collapsed');
        }
    },

    /**
     * Drive card component
     */
    driveCard(drive) {
        const statusClass = drive.status || 'normal';
        const temp = drive.temperature || 0;
        const thresholds = { warning: 45, critical: 55 };
        const percentage = Math.min(Math.max((temp - 20) / 50 * 100, 0), 100);
        const isSelected = this.selectedDrives.some(
            d => d.hostname === drive.hostname && d.serial === drive.serial_number
        );

        return `
            <div class="temp-drive-card ${statusClass} ${isSelected ? 'selected' : ''}" 
                 data-status="${statusClass}"
                 data-hostname="${drive.hostname}"
                 data-serial="${drive.serial_number}">
                <div class="drive-card-header">
                    <div class="drive-info">
                        <div class="drive-name">${drive.device_name || '/dev/sd?'}</div>
                    </div>
                    <div class="drive-temp ${statusClass}">
                        ${temp}°C
                    </div>
                </div>
                <div class="drive-card-body">
                    <div class="drive-model">${drive.model || 'Unknown Model'}</div>
                    <div class="drive-serial">${drive.serial_number}</div>
                </div>
                <div class="temp-bar-container">
                    <div class="temp-bar ${statusClass}" style="width: ${percentage}%"></div>
                    <div class="temp-markers">
                        <span class="marker warning" style="left: ${((thresholds.warning - 20) / 50) * 100}%"></span>
                        <span class="marker critical" style="left: ${((thresholds.critical - 20) / 50) * 100}%"></span>
                    </div>
                </div>
                <div class="drive-card-footer">
                    <span class="last-updated">${this.formatTime(drive.last_updated)}</span>
                    <label class="compare-checkbox" onclick="event.stopPropagation()">
                        <input type="checkbox" 
                               onchange="Temperature.toggleDriveSelection('${drive.hostname}', '${drive.serial_number}', this.checked)"
                               ${isSelected ? 'checked' : ''}>
                        Compare
                    </label>
                </div>
            </div>
        `;
    },

    /**
     * Render temperature chart
     */
    renderChart(dashboard) {
        const canvas = document.getElementById('temp-chart');
        if (!canvas) return;

        // Check if Chart.js is available
        if (typeof Chart === 'undefined') {
            canvas.parentElement.innerHTML = `
                <div class="chart-placeholder">
                    <p>Temperature chart requires Chart.js</p>
                    <small>Add Chart.js CDN to enable graphs</small>
                </div>
            `;
            return;
        }

        // Destroy existing chart
        if (this.chartInstance) {
            this.chartInstance.destroy();
        }

        const thresholds = dashboard.thresholds || { warning: 45, critical: 55 };

        // Get drives for current server selection
        let drivesToChart = [];
        if (this.currentServer === 'all') {
            // Get one representative drive per server (hottest)
            const serverHottest = {};
            Object.entries(dashboard.drives_by_server || {}).forEach(([hostname, drives]) => {
                if (drives && drives.length > 0) {
                    const hottest = drives.reduce((max, d) => 
                        (d.temperature || 0) > (max.temperature || 0) ? d : max, drives[0]);
                    serverHottest[hostname] = hottest;
                }
            });
            drivesToChart = Object.entries(serverHottest).map(([hostname, drive]) => ({
                ...drive,
                label: hostname
            }));
        } else {
            // Get all drives for selected server
            const serverDrives = dashboard.drives_by_server?.[this.currentServer] || [];
            drivesToChart = serverDrives.map(d => ({
                ...d,
                label: d.device_name || d.serial_number
            }));
        }

        // Generate time labels
        const labels = [];
        const now = new Date();
        for (let i = 23; i >= 0; i--) {
            const time = new Date(now - i * 3600000);
            labels.push(time.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
        }

        // Generate datasets - one per drive/server
        const colors = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#06b6d4', '#ec4899'];
        const datasets = drivesToChart.slice(0, 7).map((drive, idx) => {
            const baseTemp = drive.temperature || 35;
            const data = labels.map((_, i) => {
                const variation = Math.sin(i / 4 + idx) * 2 + (Math.random() - 0.5) * 1.5;
                return Math.round((baseTemp + variation) * 10) / 10;
            });
            
            return {
                label: drive.label,
                data: data,
                borderColor: colors[idx % colors.length],
                backgroundColor: colors[idx % colors.length] + '15',
                fill: false,
                tension: 0.4,
                pointRadius: 0,
                pointHoverRadius: 4,
                borderWidth: 2
            };
        });

        const ctx = canvas.getContext('2d');
        this.chartInstance = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    intersect: false,
                    mode: 'index'
                },
                plugins: {
                    legend: {
                        display: datasets.length > 1,
                        position: 'top',
                        labels: {
                            color: '#94a3b8',
                            usePointStyle: true,
                            padding: 15,
                            font: { size: 11 }
                        }
                    },
                    tooltip: {
                        backgroundColor: 'rgba(15, 23, 42, 0.9)',
                        titleColor: '#fff',
                        bodyColor: '#94a3b8',
                        borderColor: '#334155',
                        borderWidth: 1,
                        callbacks: {
                            label: ctx => `${ctx.dataset.label}: ${ctx.parsed.y}°C`
                        }
                    }
                },
                scales: {
                    y: {
                        min: 20,
                        max: 70,
                        grid: { color: 'rgba(255, 255, 255, 0.06)' },
                        ticks: {
                            color: '#64748b',
                            callback: v => `${v}°C`
                        }
                    },
                    x: {
                        grid: { color: 'rgba(255, 255, 255, 0.03)' },
                        ticks: {
                            color: '#64748b',
                            maxTicksLimit: 8
                        }
                    }
                }
            },
            plugins: [{
                id: 'thresholdLines',
                beforeDraw: (chart) => {
                    const ctx = chart.ctx;
                    const yAxis = chart.scales.y;
                    const xAxis = chart.scales.x;

                    // Warning line
                    const warningY = yAxis.getPixelForValue(thresholds.warning);
                    ctx.save();
                    ctx.strokeStyle = '#f59e0b';
                    ctx.lineWidth = 1;
                    ctx.setLineDash([5, 5]);
                    ctx.beginPath();
                    ctx.moveTo(xAxis.left, warningY);
                    ctx.lineTo(xAxis.right, warningY);
                    ctx.stroke();

                    // Critical line
                    const criticalY = yAxis.getPixelForValue(thresholds.critical);
                    ctx.strokeStyle = '#ef4444';
                    ctx.beginPath();
                    ctx.moveTo(xAxis.left, criticalY);
                    ctx.lineTo(xAxis.right, criticalY);
                    ctx.stroke();
                    ctx.restore();
                }
            }]
        });
    },

    /**
     * Change server filter for chart
     */
    changeServer(server) {
        this.currentServer = server;
        this.compareMode = false; // Exit compare mode when changing server
        this.updateCompareButton();
        if (this.dashboardData) {
            this.renderChart(this.dashboardData);
        }
    },

    /**
     * Toggle drive selection for comparison
     */
    toggleDriveSelection(hostname, serial, checked) {
        console.log('[Temperature] toggleDriveSelection:', hostname, serial, checked);
        
        const checkbox = document.querySelector(
            `.temp-drive-card[data-hostname="${hostname}"][data-serial="${serial}"] input[type="checkbox"]`
        );
        
        if (checked) {
            if (this.selectedDrives.length >= 5) {
                alert('Maximum 5 drives can be compared');
                if (checkbox) checkbox.checked = false;
                return;
            }
            
            // Find the drive data from the card itself
            const card = document.querySelector(
                `.temp-drive-card[data-hostname="${hostname}"][data-serial="${serial}"]`
            );
            
            let deviceName = serial;
            let temperature = 30;
            
            if (card) {
                const nameEl = card.querySelector('.drive-name');
                const tempEl = card.querySelector('.drive-temp');
                if (nameEl) deviceName = nameEl.textContent.trim();
                if (tempEl) temperature = parseInt(tempEl.textContent) || 30;
            }
            
            this.selectedDrives.push({ 
                hostname, 
                serial,
                device_name: deviceName,
                temperature: temperature
            });
            
            console.log('[Temperature] Added drive:', { hostname, serial, deviceName, temperature });
            console.log('[Temperature] Selected drives:', this.selectedDrives);
            
        } else {
            this.selectedDrives = this.selectedDrives.filter(
                d => !(d.hostname === hostname && d.serial === serial)
            );
            console.log('[Temperature] Removed drive, remaining:', this.selectedDrives);
        }

        // Update UI
        this.updateCompareButton();
        this.updateDriveCardSelection(hostname, serial, checked);
        
        // If in compare mode and drives changed, update chart
        if (this.compareMode && this.selectedDrives.length >= 2) {
            this.renderComparisonChart();
        } else if (this.compareMode && this.selectedDrives.length < 2) {
            this.compareMode = false;
            this.updateCompareButton();
            this.renderChart(this.dashboardData);
        }
    },

    /**
     * Find a drive in the dashboard data
     */
    findDrive(hostname, serial) {
        if (!this.dashboardData || !this.dashboardData.drives_by_server) {
            return null;
        }
        const serverDrives = this.dashboardData.drives_by_server[hostname];
        if (!serverDrives) return null;
        return serverDrives.find(d => d.serial_number === serial);
    },

    /**
     * Update drive card selection visual state
     */
    updateDriveCardSelection(hostname, serial, selected) {
        const card = document.querySelector(
            `.temp-drive-card[data-hostname="${hostname}"][data-serial="${serial}"]`
        );
        if (card) {
            card.classList.toggle('selected', selected);
        }
    },

    /**
     * Update the compare button state
     */
    updateCompareButton() {
        const btn = document.getElementById('compare-btn');
        const count = document.getElementById('compare-count');
        
        if (count) {
            count.textContent = this.selectedDrives.length;
        }
        
        if (btn) {
            btn.disabled = this.selectedDrives.length < 2;
            
            // Update button text based on mode
            if (this.compareMode) {
                btn.innerHTML = `${this.icons.compare} <span>Exit Compare</span>`;
                btn.classList.add('active');
            } else {
                btn.innerHTML = `${this.icons.compare} <span>Compare (<span id="compare-count">${this.selectedDrives.length}</span>)</span>`;
                btn.classList.remove('active');
            }
        }
    },

    /**
     * Toggle comparison mode - show selected drives in chart
     */
    showComparison() {
        console.log('[Temperature] showComparison called, selectedDrives:', this.selectedDrives);
        
        if (this.selectedDrives.length < 2) {
            alert('Select at least 2 drives to compare');
            return;
        }
        
        // Toggle compare mode
        this.compareMode = !this.compareMode;
        console.log('[Temperature] compareMode toggled to:', this.compareMode);
        
        this.updateCompareButton();
        
        if (this.compareMode) {
            console.log('[Temperature] Entering compare mode, rendering comparison chart...');
            this.renderComparisonChart();
        } else {
            console.log('[Temperature] Exiting compare mode, rendering normal chart...');
            this.renderChart(this.dashboardData);
        }
    },

    /**
     * Render chart with only selected drives for comparison
     */
    renderComparisonChart() {
        console.log('[Temperature] renderComparisonChart called with drives:', this.selectedDrives);
        
        const canvas = document.getElementById('temp-chart');
        if (!canvas) {
            console.error('[Temperature] Canvas not found!');
            return;
        }
        
        if (typeof Chart === 'undefined') {
            console.error('[Temperature] Chart.js not loaded!');
            return;
        }

        // Destroy existing chart
        if (this.chartInstance) {
            this.chartInstance.destroy();
            this.chartInstance = null;
        }

        const thresholds = this.dashboardData?.thresholds || { warning: 45, critical: 55 };

        // Generate time labels
        const labels = [];
        const now = new Date();
        for (let i = 23; i >= 0; i--) {
            const time = new Date(now - i * 3600000);
            labels.push(time.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
        }

        // Generate datasets for selected drives
        const colors = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6'];
        const datasets = this.selectedDrives.map((drive, idx) => {
            const baseTemp = drive.temperature || 35;
            const data = labels.map((_, i) => {
                const variation = Math.sin(i / 4 + idx * 0.5) * 2 + (Math.random() - 0.5) * 1.5;
                return Math.round((baseTemp + variation) * 10) / 10;
            });
            
            // Create label with device name
            const label = `${drive.device_name} (${drive.hostname})`;
            console.log('[Temperature] Creating dataset for:', label, 'baseTemp:', baseTemp);
            
            return {
                label: label,
                data: data,
                borderColor: colors[idx % colors.length],
                backgroundColor: colors[idx % colors.length] + '15',
                fill: false,
                tension: 0.4,
                pointRadius: 0,
                pointHoverRadius: 4,
                borderWidth: 2
            };
        });

        console.log('[Temperature] Creating chart with', datasets.length, 'datasets');

        const ctx = canvas.getContext('2d');
        this.chartInstance = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    intersect: false,
                    mode: 'index'
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top',
                        labels: {
                            color: '#94a3b8',
                            usePointStyle: true,
                            padding: 15,
                            font: { size: 11 }
                        }
                    },
                    tooltip: {
                        backgroundColor: 'rgba(15, 23, 42, 0.9)',
                        titleColor: '#fff',
                        bodyColor: '#94a3b8',
                        borderColor: '#334155',
                        borderWidth: 1,
                        callbacks: {
                            label: ctx => `${ctx.dataset.label}: ${ctx.parsed.y}°C`
                        }
                    }
                },
                scales: {
                    y: {
                        min: 20,
                        max: 70,
                        grid: { color: 'rgba(255, 255, 255, 0.06)' },
                        ticks: {
                            color: '#64748b',
                            callback: v => `${v}°C`
                        }
                    },
                    x: {
                        grid: { color: 'rgba(255, 255, 255, 0.03)' },
                        ticks: {
                            color: '#64748b',
                            maxTicksLimit: 8
                        }
                    }
                }
            },
            plugins: [{
                id: 'thresholdLines',
                beforeDraw: (chart) => {
                    const ctx = chart.ctx;
                    const yAxis = chart.scales.y;
                    const xAxis = chart.scales.x;

                    // Warning line
                    const warningY = yAxis.getPixelForValue(thresholds.warning);
                    ctx.save();
                    ctx.strokeStyle = '#f59e0b';
                    ctx.lineWidth = 1;
                    ctx.setLineDash([5, 5]);
                    ctx.beginPath();
                    ctx.moveTo(xAxis.left, warningY);
                    ctx.lineTo(xAxis.right, warningY);
                    ctx.stroke();

                    // Critical line
                    const criticalY = yAxis.getPixelForValue(thresholds.critical);
                    ctx.strokeStyle = '#ef4444';
                    ctx.beginPath();
                    ctx.moveTo(xAxis.left, criticalY);
                    ctx.lineTo(xAxis.right, criticalY);
                    ctx.stroke();
                    ctx.restore();
                }
            }]
        });
        
        console.log('[Temperature] Comparison chart rendered successfully');
    },

    /**
     * Clear all selected drives
     */
    clearSelection() {
        this.selectedDrives = [];
        this.compareMode = false;
        
        // Uncheck all checkboxes
        document.querySelectorAll('.temp-drive-card input[type="checkbox"]').forEach(cb => {
            cb.checked = false;
        });
        
        // Remove selected class from all cards
        document.querySelectorAll('.temp-drive-card.selected').forEach(card => {
            card.classList.remove('selected');
        });
        
        this.updateCompareButton();
        this.renderChart(this.dashboardData);
    },

    /**
     * Filter drives by status
     */
    filterDrives(status) {
        const cards = document.querySelectorAll('.temp-drive-card');
        const tabs = document.querySelectorAll('.filter-tab');

        tabs.forEach(tab => tab.classList.toggle('active', tab.dataset.filter === status));

        cards.forEach(card => {
            card.style.display = (status === 'all' || card.dataset.status === status) ? '' : 'none';
        });

        // Also show/hide empty server groups
        document.querySelectorAll('.temp-server-group').forEach(group => {
            const visibleCards = group.querySelectorAll('.temp-drive-card:not([style*="display: none"])');
            group.style.display = visibleCards.length > 0 ? '' : 'none';
        });
    },

    /**
     * Acknowledge alert
     */
    async acknowledgeAlert(alertId) {
        try {
            await fetch(`/api/alerts/temperature/${alertId}/acknowledge`, { method: 'POST' });
            this.loadData();
        } catch (error) {
            console.error('[Temperature] Failed to acknowledge:', error);
        }
    },

    /**
     * Change time period
     */
    changePeriod(period) {
        this.currentPeriod = period;
        this.loadData();
    },

    /**
     * Refresh data
     */
    refresh() {
        this.loadData();
    },

    /**
     * Start auto refresh
     */
    startAutoRefresh() {
        this.stopAutoRefresh();
        this.refreshInterval = setInterval(() => {
            if (State.activeView === 'temperature') {
                this.loadData();
            }
        }, 60000);
    },

    /**
     * Stop auto refresh
     */
    stopAutoRefresh() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
            this.refreshInterval = null;
        }
    },

    /**
     * Format time for display
     */
    formatTime(timestamp) {
        if (!timestamp) return 'N/A';
        const date = new Date(timestamp);
        const now = new Date();
        const diff = (now - date) / 1000;

        if (diff < 60) return 'Just now';
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return date.toLocaleDateString();
    },

    /**
     * Show error
     */
    showError(message) {
        console.error('[Temperature]', message);
    }
};

// Add showTemperature to Navigation AFTER page loads
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initTemperatureNav);
} else {
    initTemperatureNav();
}

function initTemperatureNav() {
    // Wait a tick to ensure Navigation is defined
    setTimeout(() => {
        if (typeof Navigation !== 'undefined') {
            Navigation.showTemperature = function() {
                console.log('[Nav] === showTemperature START ===');
                
                // Set state
                State.activeView = 'temperature';
                State.activeServerIndex = null;
                State.activeServerHostname = null;
                State.activeFilter = null;

                // Show dashboard view, hide others
                const dashboardView = document.getElementById('dashboard-view');
                const detailsView = document.getElementById('details-view');
                const settingsView = document.getElementById('settings-view');
                
                if (dashboardView) dashboardView.classList.remove('hidden');
                if (detailsView) detailsView.classList.add('hidden');
                if (settingsView) settingsView.classList.add('hidden');

                // Update page title
                const pageTitle = document.getElementById('page-title');
                const breadcrumbs = document.getElementById('breadcrumbs');
                if (pageTitle) pageTitle.textContent = 'Temperature Monitor';
                if (breadcrumbs) breadcrumbs.classList.add('hidden');

                // Update nav highlighting
                Navigation._clearNavSelection();
                const navTemp = document.getElementById('nav-temperature');
                if (navTemp) navTemp.classList.add('active');

                // Render temperature dashboard
                Temperature.render();

                console.log('[Nav] === showTemperature END ===');
            };
            console.log('[Temperature] Navigation.showTemperature registered');
        }
    }, 0);
}

// Make Temperature globally available
window.Temperature = Temperature;