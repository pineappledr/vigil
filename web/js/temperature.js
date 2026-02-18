/**
 * Vigil - Temperature Dashboard Module
 * Integrates with existing Navigation, State, and Renderer patterns
 */

const Temperature = {
    // State
    selectedDrives: [],
    currentPeriod: '24h',
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
        snowflake: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="2" x2="12" y2="22"/><path d="M20 16l-4-4 4-4"/><path d="M4 8l4 4-4 4"/><path d="M16 4l-4 4-4-4"/><path d="M8 20l4-4 4 4"/></svg>`,
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
        return `
            <div class="temp-dashboard">
                <div class="temp-header">
                    <h1 class="temp-title">${this.icons.thermometer} Temperature Monitor</h1>
                    <div class="temp-controls">
                        <select id="temp-period" class="temp-select" onchange="Temperature.changePeriod(this.value)">
                            <option value="24h" ${this.currentPeriod === '24h' ? 'selected' : ''}>Last 24 Hours</option>
                            <option value="7d" ${this.currentPeriod === '7d' ? 'selected' : ''}>Last 7 Days</option>
                            <option value="30d" ${this.currentPeriod === '30d' ? 'selected' : ''}>Last 30 Days</option>
                        </select>
                        <button class="btn btn-secondary" onclick="Temperature.refresh()">
                            ${this.icons.refresh} Refresh
                        </button>
                    </div>
                </div>

                <!-- Summary Cards -->
                <div class="temp-summary-grid" id="temp-summary-cards">
                    <div class="loading-placeholder">Loading temperature data...</div>
                </div>

                <!-- Main Content Grid -->
                <div class="temp-content-grid">
                    <!-- Temperature Chart -->
                    <div class="temp-chart-section">
                        <div class="temp-section-header">
                            <h2>${this.icons.trend} Temperature Trends</h2>
                            <button class="btn btn-sm" onclick="Temperature.showComparison()" id="compare-btn" disabled>
                                ${this.icons.compare} Compare (<span id="compare-count">0</span>)
                            </button>
                        </div>
                        <div class="temp-chart-container" id="temp-chart-container">
                            <canvas id="temp-chart"></canvas>
                        </div>
                    </div>

                    <!-- Alerts Section -->
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

                <!-- Drives Grid -->
                <div class="temp-drives-section">
                    <div class="temp-section-header">
                        <h2>${this.icons.thermometer} All Drives</h2>
                        <div class="drive-filter-tabs">
                            <button class="filter-tab active" data-filter="all" onclick="Temperature.filterDrives('all')">All</button>
                            <button class="filter-tab" data-filter="normal" onclick="Temperature.filterDrives('normal')">Normal</button>
                            <button class="filter-tab" data-filter="warning" onclick="Temperature.filterDrives('warning')">Warning</button>
                            <button class="filter-tab" data-filter="critical" onclick="Temperature.filterDrives('critical')">Critical</button>
                        </div>
                    </div>
                    <div id="temp-drives-grid" class="temp-drives-grid">
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
            this.renderDrivesGrid(dashboard);
            this.renderChart(dashboard);
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
        const drives = [];
        let totalTemp = 0;
        let minTemp = 999;
        let maxTemp = -999;
        let hottest = null;
        let coolest = null;

        State.data.forEach(server => {
            const serverDrives = server.details?.drives || [];
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
                        last_updated: server.last_seen
                    };
                    drives.push(driveData);

                    totalTemp += temp;
                    if (temp < minTemp) { minTemp = temp; coolest = driveData; }
                    if (temp > maxTemp) { maxTemp = temp; hottest = driveData; }
                }
            });
        });

        const byStatus = { normal: [], warning: [], critical: [] };
        drives.forEach(d => byStatus[d.status].push(d));

        return {
            total_drives: drives.length,
            drives_normal: byStatus.normal.length,
            drives_warning: byStatus.warning.length,
            drives_critical: byStatus.critical.length,
            avg_temperature: drives.length > 0 ? Math.round(totalTemp / drives.length * 10) / 10 : null,
            min_temperature: minTemp < 999 ? minTemp : null,
            max_temperature: maxTemp > -999 ? maxTemp : null,
            hottest_drive: hottest,
            coolest_drive: coolest,
            thresholds: thresholds,
            drives_by_status: byStatus
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
     * Render drives grid
     */
    renderDrivesGrid(data) {
        const container = document.getElementById('temp-drives-grid');
        if (!container) return;

        const allDrives = [
            ...(data.drives_by_status?.critical || []),
            ...(data.drives_by_status?.warning || []),
            ...(data.drives_by_status?.normal || [])
        ];

        if (allDrives.length === 0) {
            container.innerHTML = `
                <div class="empty-drives">
                    ${this.icons.thermometer}
                    <p>No temperature data available</p>
                    <span>Temperature data will appear after collection</span>
                </div>
            `;
            return;
        }

        container.innerHTML = allDrives.map(drive => this.driveCard(drive)).join('');
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
                        <div class="drive-host">${drive.hostname}</div>
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

        // Generate sample time labels
        const labels = [];
        const now = new Date();
        for (let i = 23; i >= 0; i--) {
            const time = new Date(now - i * 3600000);
            labels.push(time.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
        }

        // Use hottest drive temperature as current reference
        const currentTemp = dashboard.max_temperature || dashboard.avg_temperature || 35;
        const data = labels.map((_, i) => {
            // Simulate slight variations around current temp
            const variation = Math.sin(i / 4) * 3 + (Math.random() - 0.5) * 2;
            return Math.round((currentTemp + variation) * 10) / 10;
        });

        const ctx = canvas.getContext('2d');
        this.chartInstance = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Temperature (°C)',
                    data: data,
                    borderColor: '#3b82f6',
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    fill: true,
                    tension: 0.4,
                    pointRadius: 0,
                    pointHoverRadius: 5
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    intersect: false,
                    mode: 'index'
                },
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: 'rgba(15, 23, 42, 0.9)',
                        titleColor: '#fff',
                        bodyColor: '#94a3b8',
                        borderColor: '#334155',
                        borderWidth: 1,
                        callbacks: {
                            label: ctx => `${ctx.parsed.y}°C`
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
     * Toggle drive selection for comparison
     */
    toggleDriveSelection(hostname, serial, checked) {
        if (checked) {
            if (this.selectedDrives.length >= 5) {
                alert('Maximum 5 drives can be compared');
                return;
            }
            this.selectedDrives.push({ hostname, serial });
        } else {
            this.selectedDrives = this.selectedDrives.filter(
                d => !(d.hostname === hostname && d.serial === serial)
            );
        }

        // Update compare button
        const btn = document.getElementById('compare-btn');
        const count = document.getElementById('compare-count');
        if (btn) btn.disabled = this.selectedDrives.length < 2;
        if (count) count.textContent = this.selectedDrives.length;
    },

    /**
     * Show drive comparison
     */
    showComparison() {
        if (this.selectedDrives.length < 2) {
            alert('Select at least 2 drives to compare');
            return;
        }
        // TODO: Implement comparison modal
        console.log('[Temperature] Compare drives:', this.selectedDrives);
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

// Add to Navigation
Navigation.showTemperature = function() {
    console.log('[Nav] === showTemperature START ===');
    
    State.activeView = 'temperature';
    State.activeServerIndex = null;
    State.activeServerHostname = null;
    State.activeFilter = null;

    document.getElementById('dashboard-view')?.classList.remove('hidden');
    document.getElementById('details-view')?.classList.add('hidden');
    document.getElementById('settings-view')?.classList.add('hidden');

    document.getElementById('page-title').textContent = 'Temperature Monitor';
    document.getElementById('breadcrumbs')?.classList.add('hidden');

    // Update nav
    Navigation._clearNavSelection();
    document.getElementById('nav-temperature')?.classList.add('active');

    // Render
    Temperature.render();

    console.log('[Nav] === showTemperature END ===');
};

// Cleanup when leaving temperature view
const originalShowDashboard = Navigation.showDashboard;
Navigation.showDashboard = function() {
    Temperature.stopAutoRefresh();
    originalShowDashboard.call(Navigation);
};