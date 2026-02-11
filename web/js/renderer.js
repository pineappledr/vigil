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
     * Ensure dashboard structure exists (ZFS.render destroys it)
     */
    ensureDashboardStructure() {
        const container = document.getElementById('dashboard-view');
        if (!container) return false;
        
        // Check if structure exists
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
        
        summaryCards.innerHTML = this.serverSummaryCards();
        
        if (!servers || servers.length === 0) {
            serverList.innerHTML = Components.emptyState('noServers');
            return;
        }
        
        const sortedServers = State.getSortedData();
        
        serverList.innerHTML = sortedServers.map((server) => {
            const actualIdx = State.data.findIndex(s => s.hostname === server.hostname);
            const drives = (server.details?.drives || []).map((d, i) => ({...d, _idx: i}));
            return Components.serverSection(server, actualIdx, drives);
        }).join('');
    },

    serverSummaryCards() {
        const stats = State.getStats();
        const zfsStats = State.getZFSStats();
        const showZFS = zfsStats.totalPools > 0;
        
        return `
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/></svg>`,
                iconClass: 'blue',
                value: stats.totalServers,
                label: 'Servers',
                onClick: null
            })}
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="2"/></svg>`,
                iconClass: 'purple',
                value: stats.totalDrives,
                label: 'Total Drives',
                onClick: "Navigation.showFilter('all')",
                active: State.activeFilter === 'all',
                title: 'Click to view all drives'
            })}
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`,
                iconClass: 'green',
                value: stats.healthyDrives,
                label: 'Healthy',
                onClick: "Navigation.showFilter('healthy')",
                active: State.activeFilter === 'healthy',
                title: 'Click to view healthy drives'
            })}
            ${Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
                iconClass: 'red',
                value: stats.attentionDrives,
                label: 'Need Attention',
                onClick: stats.attentionDrives > 0 ? "Navigation.showFilter('attention')" : null,
                active: State.activeFilter === 'attention',
                title: stats.attentionDrives > 0 ? 'Click to view drives needing attention' : 'All drives healthy'
            })}
            ${showZFS ? Components.summaryCard({
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 6h16M4 12h16M4 18h16"/><circle cx="7" cy="6" r="1" fill="currentColor"/><circle cx="7" cy="12" r="1" fill="currentColor"/><circle cx="7" cy="18" r="1" fill="currentColor"/></svg>`,
                iconClass: zfsStats.attentionPools > 0 ? 'red' : 'cyan',
                value: `${zfsStats.healthyPools}/${zfsStats.totalPools}`,
                label: 'ZFS Pools',
                onClick: "Navigation.showZFS()",
                active: State.activeView === 'zfs',
                title: zfsStats.attentionPools > 0 
                    ? `${zfsStats.attentionPools} pool(s) need attention` 
                    : 'Click to view ZFS pools'
            }) : ''}
        `;
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
        
        const renderFilteredSection = (title, icon, drives) => {
            if (drives.length === 0) return '';
            return `
                <div class="drive-type-section">
                    <div class="drive-type-header">
                        ${icon}
                        <span>${title}</span>
                        <span class="drive-type-count">${drives.length}</span>
                    </div>
                    <div class="drives-grid">
                        ${drives.map(d => Components.driveCard(d, d._serverIdx)).join('')}
                    </div>
                </div>
            `;
        };
        
        serverList.innerHTML = `
            <div class="filtered-drives-view">
                ${renderFilteredSection('NVMe Drives', Components.icons.nvme, nvme)}
                ${renderFilteredSection('Solid State Drives', Components.icons.ssd, ssd)}
                ${renderFilteredSection('Hard Disk Drives', Components.icons.hdd, hdd)}
            </div>
        `;
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
                                <div class="settings-item-desc">Currently signed in as <strong>${State.currentUser || 'admin'}</strong></div>
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
                            <span id="settings-version">${document.getElementById('app-version')?.textContent || 'v...'}</span>
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