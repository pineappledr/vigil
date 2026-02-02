/**
 * Vigil Dashboard - View Renderer
 */

const Renderer = {
    dashboard(servers) {
        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');
        
        summaryCards.innerHTML = this.serverSummaryCards();
        
        if (servers.length === 0) {
            serverList.innerHTML = Components.emptyState('noServers');
            return;
        }
        
        serverList.innerHTML = servers.map((server, idx) => {
            const drives = (server.details?.drives || []).map((d, i) => ({...d, _idx: i}));
            return Components.serverSection(server, idx, drives);
        }).join('');
    },

    serverSummaryCards() {
        const stats = State.getStats();
        
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
        `;
    },

    serverDetailSummaryCards(server) {
        const drives = server.details?.drives || [];
        const totalDrives = drives.length;
        
        // Calculate total capacity
        const totalBytes = drives.reduce((sum, d) => sum + (d.user_capacity?.bytes || 0), 0);
        const totalCapacity = Utils.formatSize(totalBytes);
        
        // Calculate average temperature
        const temps = drives.map(d => d.temperature?.current).filter(t => t != null);
        const avgTemp = temps.length > 0 ? Math.round(temps.reduce((a, b) => a + b, 0) / temps.length) : null;
        
        // Count healthy drives
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
        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');
        
        // Use server-specific summary cards
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
        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');
        
        summaryCards.innerHTML = this.serverSummaryCards();
        
        const matchingDrives = [];
        State.data.forEach((server, serverIdx) => {
            (server.details?.drives || []).forEach((drive, driveIdx) => {
                if (filterFn(drive)) {
                    matchingDrives.push({
                        ...drive,
                        _idx: driveIdx,
                        _serverIdx: serverIdx,
                        _hostname: server.hostname
                    });
                }
            });
        });
        
        if (matchingDrives.length === 0) {
            serverList.innerHTML = Components.emptyState(filterType);
            return;
        }
        
        const byServer = {};
        matchingDrives.forEach(drive => {
            const key = drive._serverIdx;
            if (!byServer[key]) byServer[key] = [];
            byServer[key].push(drive);
        });
        
        serverList.innerHTML = Object.entries(byServer).map(([serverIdx, drives]) => {
            const server = State.data[serverIdx];
            return Components.serverSection(server, parseInt(serverIdx), drives);
        }).join('');
    },

    driveDetails(serverIdx, driveIdx) {
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) return;

        const status = Utils.getHealthStatus(drive);
        const sidebar = document.getElementById('detail-sidebar');
        const hostname = server.hostname;

        // Detect drive type properly including NVMe
        const isNvme = drive.device?.type?.toLowerCase() === 'nvme' || 
                       drive.device?.protocol === 'NVMe' ||
                       !!drive.nvme_smart_health_information_log;
        
        let rotationType, rotationDetail;
        if (isNvme) {
            rotationType = 'NVMe';
            rotationDetail = 'NVMe SSD';
        } else if (drive.rotation_rate === 0) {
            rotationType = 'SSD';
            rotationDetail = 'Solid State Drive';
        } else if (drive.rotation_rate) {
            rotationType = 'HDD';
            rotationDetail = `${drive.rotation_rate} RPM`;
        } else {
            rotationType = 'Unknown';
            rotationDetail = 'Not reported';
        }

        sidebar.innerHTML = `
            <div class="drive-header">
                <div class="icon ${status}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="2"/></svg></div>
                <h3>${Utils.getDriveName(drive)}</h3>
                <span class="serial">${drive.serial_number || 'N/A'}</span>
            </div>
            <div class="info-group">
                <div class="info-group-label">Device Information</div>
                ${this.infoRow('Capacity', Utils.formatSize(drive.user_capacity?.bytes))}
                ${this.infoRow('Firmware', drive.firmware_version || 'N/A')}
                ${this.infoRow('Drive Type', rotationType)}
                ${this.infoRow('Rotation Rate', rotationDetail)}
                ${this.infoRow('Interface', drive.device?.protocol || 'ATA')}
            </div>
            <div class="info-group">
                <div class="info-group-label">Health Status</div>
                ${this.infoRow('SMART Status', drive.smart_status?.passed === true ? 'PASSED' : drive.smart_status?.passed === false ? 'FAILED' : 'Unknown', drive.smart_status?.passed === true ? 'success' : drive.smart_status?.passed === false ? 'danger' : '')}
                ${this.infoRow('Temperature', drive.temperature?.current != null ? `${drive.temperature.current}°C` : 'N/A', drive.temperature?.current > 50 ? 'warning' : '')}
                ${this.infoRow('Powered On', Utils.formatAge(drive.power_on_time?.hours))}
                ${this.infoRow('Power Cycles', drive.power_cycle_count ?? 'N/A')}
            </div>
        `;

        // Use SmartAttributes module if available, otherwise render basic view
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