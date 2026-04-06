/**
 * Vigil Dashboard - View Renderer
 * FIXED: Restores dashboard structure after ZFS view
 */

const Renderer = {
    // Template for the dashboard structure
    dashboardTemplate: `
        <div class="summary-grid" id="summary-cards"></div>
        <div class="section-header"><h2>Servers</h2></div>
        <div id="server-list" class="server-grid"></div>
    `,

    /**
     * Ensure dashboard structure exists inside dashboard-view.
     * Each non-dashboard view now has its own container, so
     * dashboard-view is never overwritten by other views.
     */
    ensureDashboardStructure() {
        const container = document.getElementById('dashboard-view');
        if (!container) return false;

        const summaryCards = document.getElementById('summary-cards');
        const serverList = document.getElementById('server-list');

        if (!summaryCards || !serverList) {
            console.log('[Renderer] Restoring dashboard structure');
            container.innerHTML = this.dashboardTemplate;
        }
        return true;
    },

    dashboard(servers) {
        console.log('[Renderer] dashboard() called with', servers?.length || 0, 'servers');
        
        // CRITICAL: Restore structure if destroyed by ZFS.render()
        this.ensureDashboardStructure();
        
        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');
        
        if (!serverList || !summaryCards) {
            console.error('[Renderer] Dashboard elements not found!');
            return;
        }
        
        this._reconcileSummaryCards(summaryCards);

        if (!servers || servers.length === 0) {
            let emptyType = 'loading';
            if (State.initialFetchDone) {
                emptyType = State.historyError ? 'fetchError' : 'noServers';
            }
            serverList.innerHTML = Components.emptyState(emptyType);
            return;
        }

        const sortedServers = State.getSortedData();

        const htmls = [];
        const keys = [];
        sortedServers.forEach((server) => {
            const actualIdx = State.data.findIndex(s => s.hostname === server.hostname);
            const drives = (server.details?.drives || []).map((d, i) => ({...d, _idx: i}));
            keys.push(server.hostname);
            htmls.push(Components.serverSection(server, actualIdx, drives));
        });
        Utils.reconcileChildren(serverList, htmls, keys);
    },

    _healthScoreCard() {
        const hs = State.healthScore;
        if (!hs) return '';
        let iconClass = 'green';
        if (hs.score < 40) iconClass = 'red';
        else if (hs.score < 75) iconClass = 'yellow';
        return Components.summaryCard({
            icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>`,
            iconClass, key: 'health-score',
            value: hs.score,
            label: hs.grade,
            onClick: "Navigation.showHealthScore()",
            active: State.activeFilter === 'health',
            title: 'Click to view health score breakdown'
        });
    },

    _reconcileSummaryCards(container) {
        const stats = State.getStats();
        const zfsStats = State.getZFSStats();
        const showZFS = zfsStats.totalPools > 0;
        const htmls = [];
        const keys = [];

        const add = (key, html) => { if (html) { keys.push(key); htmls.push(html); } };

        add('servers', Components.summaryCard({
            icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/></svg>`,
            iconClass: 'blue', key: 'servers',
            value: stats.totalServers,
            label: 'Servers',
            onClick: "Navigation.showDashboard()",
            active: !State.activeFilter && State.activeView === 'drives' && State.activeServerIndex === null,
            title: 'Click to view dashboard'
        }));
        add('drives', Components.summaryCard({
            icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="2"/></svg>`,
            iconClass: 'purple', key: 'drives',
            value: stats.totalDrives,
            label: `<span class="card-type-breakdown">${stats.nvmeCount} NVMe · ${stats.ssdCount} SSD · ${stats.hddCount} HDD</span>`,
            onClick: "Navigation.showFilter('all')",
            active: State.activeFilter === 'all',
            title: 'Click to view all drives'
        }));
        add('healthy', Components.summaryCard({
            icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`,
            iconClass: 'green', key: 'healthy',
            value: stats.healthyDrives,
            label: 'Healthy',
            onClick: "Navigation.showFilter('healthy')",
            active: State.activeFilter === 'healthy',
            title: 'Click to view healthy drives'
        }));
        add('health-score', this._healthScoreCard());
        if (stats.warningDrives > 0) {
            add('warning', Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
                iconClass: 'yellow', key: 'warning',
                value: stats.warningDrives,
                label: 'Warning',
                onClick: "Navigation.showFilter('warning')",
                active: State.activeFilter === 'warning',
                title: 'Click to view drives with warnings'
            }));
        }
        if (stats.criticalDrives > 0) {
            add('critical', Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`,
                iconClass: 'red', key: 'critical',
                value: stats.criticalDrives,
                label: 'Critical',
                onClick: "Navigation.showFilter('critical')",
                active: State.activeFilter === 'critical',
                title: 'Click to view failing drives'
            }));
        }
        if (showZFS) {
            add('zfs', Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 6h16M4 12h16M4 18h16"/><circle cx="7" cy="6" r="1" fill="currentColor"/><circle cx="7" cy="12" r="1" fill="currentColor"/><circle cx="7" cy="18" r="1" fill="currentColor"/></svg>`,
                iconClass: zfsStats.attentionPools > 0 ? 'red' : 'cyan', key: 'zfs',
                value: `${zfsStats.healthyPools}/${zfsStats.totalPools}`,
                label: 'ZFS Pools',
                onClick: "Navigation.showZFS()",
                active: State.activeView === 'zfs',
                title: zfsStats.attentionPools > 0
                    ? `${zfsStats.attentionPools} pool(s) need attention`
                    : 'Click to view ZFS pools'
            }));
        }

        Utils.reconcileChildren(container, htmls, keys);
    },

    // String version for one-shot renders (filtered views, health breakdown)
    serverSummaryCards() {
        const container = document.createElement('div');
        this._reconcileSummaryCards(container);
        return container.innerHTML;
    },

    serverDetailSummaryCards(server) {
        const drives = server.details?.drives || [];
        const totalDrives = drives.length;
        const totalBytes = drives.reduce((sum, d) => sum + (d.user_capacity?.bytes || 0), 0);
        const totalCapacity = Utils.formatSize(totalBytes);
        const temps = drives.map(d => d.temperature?.current).filter(t => t != null);
        const avgTemp = temps.length > 0 ? Math.round(temps.reduce((a, b) => a + b, 0) / temps.length) : null;
        const healthyDrives = drives.filter(d => Utils.getHealthStatus(d) === 'healthy').length;
        
        return `
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="2"/></svg>`,
                iconClass: 'blue',
                value: totalDrives,
                label: 'Total Drives',
                onClick: null
            })}
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>`,
                iconClass: 'purple',
                value: totalCapacity,
                label: 'Total Capacity',
                onClick: null
            })}
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 14.76V3.5a2.5 2.5 0 0 0-5 0v11.26a4.5 4.5 0 1 0 5 0z"/></svg>`,
                iconClass: 'yellow',
                value: avgTemp != null ? `${avgTemp}°C` : 'N/A',
                label: 'Avg Temperature',
                onClick: null
            })}
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`,
                iconClass: 'green',
                value: `${healthyDrives}/${totalDrives}`,
                label: 'Healthy',
                onClick: null
            })}
        `;
    },

    serverDetail(server, serverIdx) {
        // Restore structure first
        this.ensureDashboardStructure();
        
        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');
        
        if (!serverList || !summaryCards) return;
        
        summaryCards.innerHTML = this.serverDetailSummaryCards(server);
        
        const drives = (server.details?.drives || []).map((d, i) => ({...d, _idx: i}));
        
        if (drives.length === 0) {
            serverList.innerHTML = Components.emptyState('noDrives');
            return;
        }
        
        const nvme = drives.filter(d => Utils.getDriveType(d) === 'NVMe');
        const ssd = drives.filter(d => Utils.getDriveType(d) === 'SSD');
        const hdd = drives.filter(d => !['NVMe', 'SSD'].includes(Utils.getDriveType(d)));
        
        serverList.innerHTML = `
            <div class="server-detail-view">
                ${Components.driveSection('NVMe Drives', Components.icons.nvme, nvme, serverIdx)}
                ${Components.driveSection('Solid State Drives', Components.icons.ssd, ssd, serverIdx)}
                ${Components.driveSection('Hard Disk Drives', Components.icons.hdd, hdd, serverIdx)}
            </div>
        `;
    },

    healthBreakdown() {
        this.ensureDashboardStructure();
        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');
        if (!serverList || !summaryCards) return;

        summaryCards.innerHTML = this.serverSummaryCards();

        const hs = State.healthScore;
        if (!hs) {
            serverList.innerHTML = '<p style="color:var(--text-muted);padding:20px">Health score not available.</p>';
            return;
        }

        const gradeColor = (score) => {
            if (score >= 90) return 'var(--success)';
            if (score >= 75) return Utils.getCSSVar('--success');
            if (score >= 60) return 'var(--warning)';
            if (score >= 40) return '#f97316';
            return 'var(--danger)';
        };

        const componentCard = (label, comp, icon) => {
            const ded = Math.round(comp.deduction);
            return `
                <div class="health-component-card">
                    <div class="health-component-header">
                        <span class="health-component-icon">${icon}</span>
                        <span class="health-component-label">${label}</span>
                    </div>
                    <div class="health-component-deduction" style="color:${ded > 0 ? 'var(--danger)' : 'var(--success)'}">
                        ${ded > 0 ? '−' + ded : '0'}
                    </div>
                    <div class="health-component-details">${Utils.escapeHtml(comp.details)}</div>
                </div>`;
        };

        // Build drive list for drives with issues
        const issuesDrives = [];
        State.data.forEach((server, serverIdx) => {
            (server.details?.drives || []).forEach((drive, driveIdx) => {
                if (Utils.getHealthStatus(drive) !== 'healthy') {
                    issuesDrives.push({ ...drive, _serverIdx: serverIdx, _driveIdx: driveIdx, _hostname: server.hostname, _idx: driveIdx });
                }
            });
        });

        const drivesHtml = issuesDrives.length > 0
            ? `<div class="drive-table-wrapper">
                    <table class="drive-table">
                        <thead>
                            <tr>
                                <th>Status</th>
                                <th>Name</th>
                                <th>Serial</th>
                                <th>Host</th>
                                <th>Type</th>
                                <th>Temp</th>
                                <th>Age</th>
                                <th>SMART</th>
                                <th></th>
                            </tr>
                        </thead>
                        <tbody>
                            ${issuesDrives.map(d => this._driveTableRow(d)).join('')}
                        </tbody>
                    </table>
                </div>`
            : '<p style="color:var(--text-muted)">All drives are healthy.</p>';

        serverList.innerHTML = `
            <div class="health-breakdown-view">
                <div class="health-score-banner" style="border-color:${gradeColor(hs.score)}">
                    <div class="health-score-big" style="color:${gradeColor(hs.score)}">${hs.score}</div>
                    <div class="health-score-info">
                        <div class="health-score-grade" style="color:${gradeColor(hs.score)}">${Utils.escapeHtml(hs.grade)}</div>
                        <div class="health-score-sub">out of 100</div>
                    </div>
                </div>
                <h3 class="health-section-title">Score Components</h3>
                <div class="health-components-grid">
                    ${componentCard('SMART', hs.components.smart, '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="2"/></svg>')}
                    ${componentCard('Wearout', hs.components.wearout, '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>')}
                    ${componentCard('ZFS', hs.components.zfs, '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M4 6h16M4 12h16M4 18h16"/></svg>')}
                </div>
                <h3 class="health-section-title">Drives Needing Attention</h3>
                ${drivesHtml}
            </div>
        `;
    },

    filteredDrives(filterFn, filterType) {
        // Restore structure first
        this.ensureDashboardStructure();

        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');

        if (!serverList || !summaryCards) return;

        summaryCards.innerHTML = this.serverSummaryCards();

        const matchingDrives = [];
        const sortedServers = State.getSortedData();

        sortedServers.forEach((server) => {
            const actualIdx = State.data.findIndex(s => s.hostname === server.hostname);
            (server.details?.drives || []).forEach((drive, driveIdx) => {
                if (filterFn(drive)) {
                    matchingDrives.push({
                        ...drive,
                        _serverIdx: actualIdx,
                        _driveIdx: driveIdx,
                        _hostname: server.hostname,
                        _idx: driveIdx
                    });
                }
            });
        });

        if (matchingDrives.length === 0) {
            serverList.innerHTML = Components.emptyState(filterType === 'attention' ? 'attention' : 'noDrives');
            return;
        }

        const nvme = matchingDrives.filter(d => Utils.getDriveType(d) === 'NVMe');
        const ssd = matchingDrives.filter(d => Utils.getDriveType(d) === 'SSD');
        const hdd = matchingDrives.filter(d => !['NVMe', 'SSD'].includes(Utils.getDriveType(d)));

        const renderTableSection = (title, icon, drives) => {
            if (drives.length === 0) return '';
            return `
                <div class="drive-table-section">
                    <div class="drive-table-header">
                        ${icon}
                        <span>${title}</span>
                        <span class="drive-table-count">${drives.length}</span>
                    </div>
                    <div class="drive-table-wrapper">
                        <table class="drive-table">
                            <thead>
                                <tr>
                                    <th>Status</th>
                                    <th>Name</th>
                                    <th>Serial</th>
                                    <th>Host</th>
                                    <th>Capacity</th>
                                    <th>Temp</th>
                                    <th>Age</th>
                                    <th>Wearout</th>
                                    <th>SMART</th>
                                    <th></th>
                                </tr>
                            </thead>
                            <tbody>
                                ${drives.map(d => this._driveTableRow(d)).join('')}
                            </tbody>
                        </table>
                    </div>
                </div>
            `;
        };

        serverList.innerHTML = `
            <div class="filtered-drives-view">
                ${renderTableSection('NVMe Drives', Components.icons.nvme, nvme)}
                ${renderTableSection('Solid State Drives', Components.icons.ssd, ssd)}
                ${renderTableSection('Hard Disk Drives', Components.icons.hdd, hdd)}
            </div>
        `;
    },

    _driveTableRow(drive) {
        const status = Utils.getHealthStatus(drive);
        const driveName = Utils.getDriveName(drive);
        const serial = drive.serial_number || 'N/A';
        const hostname = drive._hostname || '';
        const wearoutData = State.getWearoutForDrive(hostname, serial);
        const wearoutRaw = wearoutData ? wearoutData.percentage : null;
        const wearoutPct = wearoutRaw !== null ? Math.round(wearoutRaw * 10) / 10 : null;
        const smartPassed = drive.smart_status?.passed;
        const alias = drive._alias || '';
        const displayName = alias || driveName;

        return `
            <tr class="drive-table-row ${status}" onclick="Navigation.showDriveDetails(${drive._serverIdx}, ${drive._driveIdx})">
                <td><span class="drive-status-dot ${status}"></span></td>
                <td class="drive-table-name" title="${Utils.escapeHtml(driveName)}">${Utils.escapeHtml(displayName)}</td>
                <td class="drive-table-serial">${Utils.escapeHtml(serial)}</td>
                <td class="drive-table-host">${Utils.escapeHtml(hostname)}</td>
                <td>${Utils.formatSize(drive.user_capacity?.bytes)}</td>
                <td>${drive.temperature?.current ?? '--'}°C</td>
                <td>${Utils.formatAge(drive.power_on_time?.hours)}</td>
                <td class="drive-table-wearout">${wearoutPct !== null ? this._wearoutBar(wearoutPct) : '--'}</td>
                <td><span class="smart-badge ${smartPassed ? 'passed' : 'failed'}">${smartPassed ? 'OK' : 'FAIL'}</span></td>
                <td class="drive-table-actions">
                    <button class="alias-btn-sm" onclick="event.stopPropagation(); Modals.showAlias('${Utils.escapeJSString(hostname)}', '${Utils.escapeJSString(serial)}', '${Utils.escapeJSString(alias)}', '${Utils.escapeJSString(driveName)}')" title="Set alias">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                    </button>
                </td>
            </tr>
        `;
    },

    _wearoutBar(pct) {
        const remaining = Math.max(0, Math.min(100, 100 - pct));
        let barClass = 'wearout-good';
        if (pct > 80) barClass = 'wearout-critical';
        else if (pct > 50) barClass = 'wearout-warning';
        return `
            <div class="wearout-bar-container">
                <span class="wearout-tooltip">${pct}% used · ${remaining.toFixed(1)}% remaining</span>
                <div class="wearout-bar">
                    <div class="wearout-bar-fill ${barClass}" style="width:${Math.min(pct, 100)}%"></div>
                </div>
            </div>`;
    },

    driveDetails(serverIdx, driveIdx) {
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) return;
        
        const hostname = server.hostname;
        const sidebar = document.getElementById('detail-sidebar');
        
        sidebar.innerHTML = `
            <div class="detail-header">
                ${Components.icons[Utils.getDriveType(drive).toLowerCase()] || Components.icons.hdd}
                <div class="drive-detail-title">
                    <h3>${Utils.getDriveName(drive)}</h3>
                    <span class="drive-serial">${drive.serial_number || 'Unknown'}</span>
                </div>
            </div>
            <div class="info-group">
                ${this.infoRow('Model', drive.model_name || 'Unknown')}
                ${this.infoRow('Serial', drive.serial_number || 'Unknown')}
                ${this.infoRow('Firmware', drive.firmware_version || 'Unknown')}
                ${this.infoRow('Capacity', Utils.formatSize(drive.user_capacity?.bytes))}
                ${this.infoRow('Type', Utils.getDriveType(drive))}
                ${drive.rotation_rate ? this.infoRow('RPM', drive.rotation_rate) : ''}
                ${this.infoRow('Temperature', drive.temperature?.current != null ? `${drive.temperature.current}°C` : 'N/A')}
                ${this.infoRow('SMART Status', drive.smart_status?.passed ? 'Passed' : 'Failed', drive.smart_status?.passed ? 'healthy' : 'critical')}
                ${this.infoRow('Powered On', Utils.formatAge(drive.power_on_time?.hours))}
                ${this.infoRow('Power Cycles', drive.power_cycle_count ?? 'N/A')}
            </div>
        `;

        if (typeof SmartAttributes !== 'undefined') {
            SmartAttributes.init(hostname, drive.serial_number, drive);
        } else {
            this.renderBasicSmartView(drive);
        }
    },

    renderBasicSmartView(drive) {
        const container = document.getElementById('smart-view-container');
        if (!container) return;

        const isNvme = Utils.getDriveType(drive) === 'NVMe';
        
        if (isNvme) {
            const health = drive.nvme_smart_health_information_log || {};
            container.innerHTML = `
                <div class="table-container">
                    <table class="smart-table">
                        <thead><tr><th>Metric</th><th>Value</th></tr></thead>
                        <tbody>
                            <tr><td>Available Spare</td><td>${health.available_spare ?? '--'}%</td></tr>
                            <tr><td>Percentage Used</td><td>${health.percentage_used ?? '--'}%</td></tr>
                            <tr><td>Temperature</td><td>${health.temperature ?? '--'}°C</td></tr>
                            <tr><td>Power On Hours</td><td>${health.power_on_hours ?? '--'}</td></tr>
                            <tr><td>Power Cycles</td><td>${health.power_cycles ?? '--'}</td></tr>
                            <tr><td>Unsafe Shutdowns</td><td>${health.unsafe_shutdowns ?? '--'}</td></tr>
                            <tr><td>Media Errors</td><td>${health.media_errors ?? '--'}</td></tr>
                        </tbody>
                    </table>
                </div>
            `;
            return;
        }
        
        const attrs = drive.ata_smart_attributes?.table || [];
        
        if (attrs.length === 0) {
            container.innerHTML = `
                <div class="nvme-notice">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"/>
                        <line x1="12" y1="16" x2="12" y2="12"/>
                        <line x1="12" y1="8" x2="12.01" y2="8"/>
                    </svg>
                    <p>No S.M.A.R.T. attributes available</p>
                </div>
            `;
            return;
        }
        
        const criticalIds = [5, 187, 197, 198];
        container.innerHTML = `
            <div class="table-container">
                <table class="smart-table">
                    <thead>
                        <tr>
                            <th class="status-cell">Status</th>
                            <th>ID</th>
                            <th>Attribute</th>
                            <th>Value</th>
                            <th>Worst</th>
                            <th>Thresh</th>
                            <th>Raw</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${attrs.map(a => {
                            const fail = (criticalIds.includes(a.id) && a.raw?.value > 0) || (a.thresh > 0 && a.value <= a.thresh);
                            return `<tr>
                                <td class="status-cell"><span class="attr-pill ${fail ? 'fail' : 'ok'}">${fail ? 'FAIL' : 'OK'}</span></td>
                                <td>${a.id}</td>
                                <td style="font-family:var(--font-sans)">${a.name}</td>
                                <td>${a.value}</td>
                                <td>${a.worst ?? '-'}</td>
                                <td>${a.thresh}</td>
                                <td>${a.raw?.value ?? '-'}</td>
                            </tr>`;
                        }).join('')}
                    </tbody>
                </table>
            </div>
        `;
    },

    settingsPage() {
        return `
            <button class="btn-back" onclick="Navigation.showDashboard()">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="19" y1="12" x2="5" y2="12"/>
                    <polyline points="12 19 5 12 12 5"/>
                </svg>
                Back to Dashboard
            </button>
            <div class="settings-container">
                <div class="settings-section">
                    <div class="settings-section-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                            <circle cx="12" cy="7" r="4"/>
                        </svg>
                        <h3>Account</h3>
                    </div>
                    <div class="settings-card">
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Username</div>
                                <div class="settings-item-desc">Currently signed in as <strong>${Utils.escapeHtml(State.currentUser || 'admin')}</strong></div>
                            </div>
                            <button class="btn btn-secondary" onclick="Modals.showChangeUsername()">Change</button>
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Password</div>
                                <div class="settings-item-desc">Change your account password</div>
                            </div>
                            <button class="btn btn-secondary" onclick="Modals.showChangePassword()">Change</button>
                        </div>
                    </div>
                </div>

                <div class="settings-section">
                    <div class="settings-section-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="23 4 23 10 17 10"/>
                            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                        </svg>
                        <h3>Updates</h3>
                    </div>
                    <div class="settings-card">
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Software Updates</div>
                                <div class="settings-item-desc">Check for new versions of Vigil</div>
                            </div>
                            <button id="check-updates-btn" class="check-updates-btn" onclick="Version.manualCheck()">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                    <polyline points="23 4 23 10 17 10"/>
                                    <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                                </svg>
                                Check for Updates
                            </button>
                        </div>
                        <div id="update-check-status"></div>
                    </div>
                </div>
                
                <div class="settings-section">
                    <div class="settings-section-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="3 6 5 6 21 6"/>
                            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/>
                            <line x1="10" y1="11" x2="10" y2="17"/>
                            <line x1="14" y1="11" x2="14" y2="17"/>
                        </svg>
                        <h3>Data Retention</h3>
                    </div>
                    <div class="settings-card" id="retention-settings">
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Notification History</div>
                                <div class="settings-item-desc">Days to keep notification history</div>
                            </div>
                            <input type="number" class="settings-input" id="retention-notification-days" min="1" max="3650" value="90"
                                onchange="Settings.saveRetention('notification_history_days', this.value)">
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">SMART Data</div>
                                <div class="settings-item-desc">Days to keep SMART attribute and temperature history</div>
                            </div>
                            <input type="number" class="settings-input" id="retention-smart-days" min="1" max="3650" value="90"
                                onchange="Settings.saveRetention('smart_data_days', this.value)">
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Host History Limit</div>
                                <div class="settings-item-desc">Maximum report history entries per host</div>
                            </div>
                            <input type="number" class="settings-input" id="retention-host-limit" min="10" max="1000" value="50"
                                onchange="Settings.saveRetention('host_history_limit', this.value)">
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Notification Display Limit</div>
                                <div class="settings-item-desc">Default number of notification records to show</div>
                            </div>
                            <input type="number" class="settings-input" id="retention-notify-limit" min="10" max="500" value="50"
                                onchange="Settings.saveRetention('notification_display_limit', this.value)">
                        </div>
                    </div>
                </div>

                <div class="settings-section">
                    <div class="settings-section-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                            <polyline points="7 10 12 15 17 10"/>
                            <line x1="12" y1="15" x2="12" y2="3"/>
                        </svg>
                        <h3>Database Backup</h3>
                    </div>
                    <div class="settings-card" id="backup-settings">
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Automatic Backups</div>
                                <div class="settings-item-desc">Enable scheduled database backups</div>
                            </div>
                            <input type="checkbox" class="settings-toggle" id="backup-enabled" checked
                                onchange="Settings.saveBackupSetting('enabled', this.checked ? 'true' : 'false')">
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Backup Interval</div>
                                <div class="settings-item-desc">Hours between automatic backups</div>
                            </div>
                            <input type="number" class="settings-input" id="backup-interval" min="1" max="168" value="24"
                                onchange="Settings.saveBackupSetting('interval_hours', this.value)">
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Max Backups</div>
                                <div class="settings-item-desc">Maximum number of backup files to retain</div>
                            </div>
                            <input type="number" class="settings-input" id="backup-max" min="1" max="100" value="7"
                                onchange="Settings.saveBackupSetting('max_backups', this.value)">
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Manual Backup</div>
                                <div class="settings-item-desc">Create a backup now</div>
                            </div>
                            <div class="backup-actions">
                                <button class="btn btn-secondary" id="backup-now-btn" onclick="Settings.triggerBackup()">Backup Now</button>
                                <button class="btn btn-secondary" onclick="Settings.restoreFromFile()">Restore from File</button>
                            </div>
                        </div>
                        <div id="backup-progress" class="backup-progress" style="display:none">
                            <div class="backup-progress-bar"><div class="backup-progress-fill"></div></div>
                            <span class="backup-progress-text">Creating backup…</span>
                        </div>
                        <div id="backup-list"></div>
                    </div>
                </div>

                <div class="settings-section">
                    <div class="settings-section-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
                            <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
                            <line x1="6" y1="6" x2="6.01" y2="6"/>
                            <line x1="6" y1="18" x2="6.01" y2="18"/>
                        </svg>
                        <h3>Drive Groups</h3>
                    </div>
                    <div class="settings-card" id="drive-groups-settings">
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-desc">Organize drives into groups with per-group notification rules</div>
                            </div>
                        </div>
                        <div id="drive-groups-list"></div>
                        <div id="drive-groups-create"></div>
                    </div>
                </div>

                <div class="settings-section">
                    <div class="settings-section-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="3" y="3" width="18" height="18" rx="2"/>
                            <path d="M3 9h18"/>
                            <path d="M9 21V9"/>
                        </svg>
                        <h3>System Stats</h3>
                    </div>
                    <div class="settings-card" id="system-stats">
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-desc">Loading stats...</div>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="settings-section">
                    <div class="settings-section-header">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="10"/>
                            <line x1="12" y1="16" x2="12" y2="12"/>
                            <line x1="12" y1="8" x2="12.01" y2="8"/>
                        </svg>
                        <h3>About</h3>
                    </div>
                    <div class="settings-card">
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Vigil</div>
                                <div class="settings-item-desc">Server infrastructure monitoring</div>
                            </div>
                            <span id="settings-version">${Utils.escapeHtml(document.getElementById('app-version')?.textContent || 'v...')}</span>
                        </div>
                        <div class="settings-item">
                            <div class="settings-item-info">
                                <div class="settings-item-title">Documentation</div>
                                <div class="settings-item-desc">
                                    <a href="https://github.com/pineappledr/vigil" target="_blank" rel="noopener">GitHub Repository</a>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    },

    infoRow(label, value, cls = '') {
        return `<div class="info-row"><span class="label">${label}</span><span class="value ${cls}">${value}</span></div>`;
    }
};