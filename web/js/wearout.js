/**
 * Vigil Dashboard - Wearout Module
 * Drive health wearout visualization, trend charts, and factor breakdown
 */

const Wearout = {
    // Chart.js instance for trend chart
    trendChart: null,

    // ─── API Methods ──────────────────────────────────────────────────────────

    async fetchDriveWearout(hostname, serial) {
        try {
            const resp = await API.get(`/api/wearout/drive?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serial)}`);
            if (resp.ok) return await resp.json();
        } catch (e) { console.error('[Wearout] fetchDriveWearout error:', e); }
        return null;
    },

    async fetchAllWearout() {
        try {
            const resp = await API.get('/api/wearout/all');
            if (resp.ok) return await resp.json();
        } catch (e) { console.error('[Wearout] fetchAllWearout error:', e); }
        return null;
    },

    async fetchHistory(hostname, serial, days = 90) {
        try {
            const resp = await API.get(`/api/wearout/history?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serial)}&days=${days}`);
            if (resp.ok) return await resp.json();
        } catch (e) { console.error('[Wearout] fetchHistory error:', e); }
        return null;
    },

    async fetchTrend(hostname, serial) {
        try {
            const resp = await API.get(`/api/wearout/trend?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serial)}`);
            if (resp.ok) return await resp.json();
        } catch (e) { console.error('[Wearout] fetchTrend error:', e); }
        return null;
    },

    // ─── Status Helpers ───────────────────────────────────────────────────────

    getStatusClass(pct) {
        if (pct >= 80) return 'critical';
        if (pct >= 60) return 'warning';
        return 'healthy';
    },

    getStatusLabel(pct) {
        if (pct >= 80) return 'Critical';
        if (pct >= 60) return 'Warning';
        if (pct >= 30) return 'Fair';
        return 'Good';
    },

    // ─── Progress Bar (mini, for drive cards) ─────────────────────────────────

    miniProgressBar(pct) {
        if (pct == null) return '';
        const cls = this.getStatusClass(pct);
        const rounded = Math.round(pct * 10) / 10;
        return `
            <div class="wearout-mini" title="Drive Wearout: ${rounded}%">
                <div class="wearout-mini-bar">
                    <div class="wearout-mini-fill ${cls}" style="width:${Math.min(pct, 100)}%"></div>
                </div>
                <span class="wearout-mini-label ${cls}">${rounded}%</span>
            </div>
        `;
    },

    // ─── Wearout Tab Content (detail view) ────────────────────────────────────

    renderTabContent(snapshot, trendData, driveInfo) {
        if (!snapshot) {
            return `
                ${this.renderDriveSpecs(driveInfo)}
                <div class="smart-empty">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"/>
                        <line x1="12" y1="16" x2="12" y2="12"/>
                        <line x1="12" y1="8" x2="12.01" y2="8"/>
                    </svg>
                    <p>No wearout data available</p>
                    <span class="hint">Wearout data will appear after the first agent report is processed</span>
                </div>
            `;
        }

        const pct = snapshot.percentage ?? 0;
        const cls = this.getStatusClass(pct);
        const label = this.getStatusLabel(pct);
        const factors = this.parseFactors(snapshot.factors_json);

        let html = '';

        // Drive specifications
        html += this.renderDriveSpecs(driveInfo);

        // Main wearout gauge
        html += `
            <div class="wearout-gauge-panel">
                <div class="wearout-gauge-header">
                    <div class="wearout-gauge-icon ${cls}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
                        </svg>
                    </div>
                    <div class="wearout-gauge-title">
                        <h3>Drive Wearout</h3>
                        <span class="subtitle">${snapshot.drive_type || 'Unknown'} — ${label}</span>
                    </div>
                    <div class="wearout-pct ${cls}">${pct.toFixed(1)}%</div>
                </div>
                <div class="wearout-bar-container">
                    <div class="wearout-bar-track">
                        <div class="wearout-bar-fill ${cls}" style="width:${Math.min(pct, 100)}%"></div>
                    </div>
                    <div class="wearout-bar-labels">
                        <span>0%</span>
                        <span class="wearout-label-warning">60%</span>
                        <span class="wearout-label-critical">80%</span>
                        <span>100%</span>
                    </div>
                </div>
            </div>
        `;

        // Contributing factors breakdown
        if (factors.length > 0) {
            html += `
                <div class="wearout-factors-panel">
                    <div class="wearout-factors-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
                        </svg>
                        <span>Contributing Factors</span>
                    </div>
                    <div class="wearout-factors-list">
                        ${factors.map(f => this.renderFactor(f)).join('')}
                    </div>
                </div>
            `;
        }

        // Trend prediction
        html += this.renderTrendSection(trendData);

        // Trend chart canvas
        html += `
            <div class="wearout-chart-panel">
                <div class="wearout-chart-header">
                    <div class="chart-title">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/>
                        </svg>
                        <span>Wearout History</span>
                    </div>
                    <select class="wearout-period-select" onchange="Wearout.onPeriodChange(this.value)">
                        <option value="30">30 Days</option>
                        <option value="90" selected>90 Days</option>
                        <option value="180">180 Days</option>
                        <option value="365">1 Year</option>
                    </select>
                </div>
                <div class="wearout-chart-container">
                    <canvas id="wearout-trend-canvas"></canvas>
                </div>
            </div>
        `;

        return html;
    },

    renderFactor(factor) {
        const cls = this.getStatusClass(factor.percentage);
        return `
            <div class="wearout-factor-item">
                <div class="wearout-factor-info">
                    <span class="wearout-factor-name">${factor.name}</span>
                    <span class="wearout-factor-desc">${factor.description}</span>
                </div>
                <div class="wearout-factor-bar-wrap">
                    <div class="wearout-factor-bar">
                        <div class="wearout-factor-fill ${cls}" style="width:${Math.min(factor.percentage, 100)}%"></div>
                    </div>
                    <span class="wearout-factor-pct ${cls}">${factor.percentage.toFixed(1)}%</span>
                </div>
                <span class="wearout-factor-weight">Weight: ${(factor.weight * 100).toFixed(0)}%</span>
            </div>
        `;
    },

    renderTrendSection(trendData) {
        if (!trendData || !trendData.prediction) {
            return `
                <div class="wearout-trend-panel">
                    <div class="wearout-trend-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/>
                            <polyline points="17 6 23 6 23 12"/>
                        </svg>
                        <span>Trend Prediction</span>
                    </div>
                    <div class="wearout-trend-body">
                        <span class="wearout-trend-na">Insufficient data for trend prediction</span>
                    </div>
                </div>
            `;
        }

        const pred = trendData.prediction;
        const dailyRate = pred.daily_rate;
        const isImproving = dailyRate < 0;
        const rateClass = isImproving ? 'healthy' : this.getStatusClass(trendData.current_percentage);

        let remainingHtml = '';
        if (pred.months_remaining != null) {
            const months = pred.months_remaining;
            if (months < 1) {
                remainingHtml = `<span class="wearout-remaining critical">&lt; 1 month</span>`;
            } else if (months < 6) {
                remainingHtml = `<span class="wearout-remaining warning">${months.toFixed(1)} months</span>`;
            } else {
                remainingHtml = `<span class="wearout-remaining healthy">${months.toFixed(1)} months</span>`;
            }
        } else {
            remainingHtml = `<span class="wearout-remaining healthy">${isImproving ? 'Improving' : 'Stable'}</span>`;
        }

        const confidenceCls = pred.confidence === 'high' ? 'healthy' : pred.confidence === 'medium' ? 'warning' : 'critical';

        return `
            <div class="wearout-trend-panel">
                <div class="wearout-trend-header">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/>
                        <polyline points="17 6 23 6 23 12"/>
                    </svg>
                    <span>Trend Prediction</span>
                    <span class="wearout-confidence ${confidenceCls}">${pred.confidence}</span>
                </div>
                <div class="wearout-trend-stats">
                    <div class="wearout-trend-stat">
                        <span class="stat-label">Daily Rate</span>
                        <span class="stat-value ${rateClass}">${dailyRate >= 0 ? '+' : ''}${dailyRate.toFixed(4)}%</span>
                    </div>
                    <div class="wearout-trend-stat">
                        <span class="stat-label">Est. Remaining</span>
                        ${remainingHtml}
                    </div>
                    <div class="wearout-trend-stat">
                        <span class="stat-label">Data Points</span>
                        <span class="stat-value">${trendData.data_points}</span>
                    </div>
                </div>
            </div>
        `;
    },

    // ─── Drive Specifications Panel ──────────────────────────────────────────

    renderDriveSpecs(drive) {
        if (!drive) return '';

        const model = drive.model_name || 'Unknown';
        const firmware = drive.firmware_version || 'N/A';
        const serial = drive.serial_number || 'N/A';
        const capacity = typeof Utils !== 'undefined'
            ? Utils.formatSize(drive.user_capacity?.bytes)
            : (drive.user_capacity?.bytes ? `${(drive.user_capacity.bytes / 1e9).toFixed(0)} GB` : 'N/A');
        const driveType = typeof Utils !== 'undefined'
            ? Utils.getDriveType(drive)
            : (drive.device?.type || 'Unknown');
        const rpm = drive.rotation_rate ? `${drive.rotation_rate} RPM` : null;
        const poh = drive.power_on_time?.hours;
        const pohStr = poh != null
            ? (typeof Utils !== 'undefined' ? Utils.formatAge(poh) : `${poh}h`)
            : 'N/A';

        return `
            <div class="wearout-specs-panel">
                <div class="wearout-specs-header">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="2" y="4" width="20" height="16" rx="2"/>
                        <circle cx="8" cy="12" r="2"/>
                        <line x1="14" y1="9" x2="18" y2="9"/>
                        <line x1="14" y1="12" x2="18" y2="12"/>
                    </svg>
                    <span>Drive Specifications</span>
                </div>
                <div class="wearout-specs-grid">
                    ${this.specItem('Model', model)}
                    ${this.specItem('Serial', serial)}
                    ${this.specItem('Firmware', firmware)}
                    ${this.specItem('Capacity', capacity)}
                    ${this.specItem('Type', driveType)}
                    ${rpm ? this.specItem('Speed', rpm) : ''}
                    ${this.specItem('Power-On', pohStr)}
                    ${drive.temperature?.current != null ? this.specItem('Temp', `${drive.temperature.current}°C`) : ''}
                </div>
            </div>
        `;
    },

    specItem(label, value) {
        return `
            <div class="wearout-spec-item">
                <span class="wearout-spec-label">${label}</span>
                <span class="wearout-spec-value">${value}</span>
            </div>
        `;
    },

    // ─── Trend Chart ──────────────────────────────────────────────────────────

    renderTrendChart(historyData) {
        if (this.trendChart) {
            this.trendChart.destroy();
            this.trendChart = null;
        }

        const canvas = document.getElementById('wearout-trend-canvas');
        if (!canvas || !historyData?.history?.length) return;

        const history = historyData.history;
        const labels = history.map(h => {
            const d = new Date(h.timestamp);
            return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
        });
        const values = history.map(h => h.percentage);

        const ctx = canvas.getContext('2d');
        this.trendChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels,
                datasets: [{
                    label: 'Wearout %',
                    data: values,
                    borderColor: '#3b82f6',
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    fill: true,
                    tension: 0.3,
                    pointRadius: values.length > 60 ? 0 : 3,
                    pointHoverRadius: 5,
                    borderWidth: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: '#1a2234',
                        titleColor: '#f1f5f9',
                        bodyColor: '#94a3b8',
                        borderColor: 'rgba(148,163,184,0.2)',
                        borderWidth: 1,
                        callbacks: {
                            label: ctx => `Wearout: ${ctx.parsed.y.toFixed(1)}%`
                        }
                    }
                },
                scales: {
                    x: {
                        grid: { color: 'rgba(148,163,184,0.08)' },
                        ticks: { color: '#64748b', maxTicksLimit: 8, font: { family: "'JetBrains Mono'" } }
                    },
                    y: {
                        min: 0,
                        max: 100,
                        grid: { color: 'rgba(148,163,184,0.08)' },
                        ticks: {
                            color: '#64748b',
                            callback: v => v + '%',
                            font: { family: "'JetBrains Mono'" }
                        }
                    }
                }
            }
        });
    },

    // ─── Tab Integration ──────────────────────────────────────────────────────

    // Store current context for period changes
    _currentHostname: null,
    _currentSerial: null,

    async loadTab(hostname, serial) {
        this._currentHostname = hostname;
        this._currentSerial = serial;

        const container = document.getElementById('tab-wearout');
        if (!container) return;

        // Get drive info from SmartAttributes if available
        const driveInfo = (typeof SmartAttributes !== 'undefined') ? SmartAttributes.currentDrive : null;

        container.innerHTML = `
            <div class="smart-loading">
                <div class="smart-loading-spinner"></div>
                <span>Loading wearout data...</span>
            </div>
        `;

        const [snapshot, trendData] = await Promise.all([
            this.fetchDriveWearout(hostname, serial),
            this.fetchTrend(hostname, serial)
        ]);

        container.innerHTML = this.renderTabContent(snapshot, trendData, driveInfo);

        // Load chart data only if we have wearout data
        if (snapshot) {
            const historyData = await this.fetchHistory(hostname, serial, 90);
            this.renderTrendChart(historyData);
        }
    },

    async onPeriodChange(days) {
        if (!this._currentHostname || !this._currentSerial) return;
        const historyData = await this.fetchHistory(this._currentHostname, this._currentSerial, parseInt(days));
        this.renderTrendChart(historyData);
    },

    // ─── Utilities ────────────────────────────────────────────────────────────

    parseFactors(json) {
        if (!json) return [];
        try { return JSON.parse(json); } catch { return []; }
    }
};
