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

    serverDetail(server, serverIdx) {
        const serverList = document.getElementById('server-list');
        const summaryCards = document.getElementById('summary-cards');
        
        summaryCards.innerHTML = this.serverSummaryCards();
        
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

    driveDetails(drive, hostname) {
        const sidebar = document.getElementById('detail-sidebar');
        const container = document.getElementById('smart-view-container');
        const status = Utils.getHealthStatus(drive);
        const driveType = Utils.getDriveType(drive);
        
        sidebar.innerHTML = `
            <div class="drive-header">
                <div class="icon ${status}">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="2" y="4" width="20" height="16" rx="2"/>
                        <circle cx="8" cy="12" r="2"/>
                        <line x1="14" y1="9" x2="18" y2="9"/>
                        <line x1="14" y1="12" x2="18" y2="12"/>
                    </svg>
                </div>
                <h3>${Utils.getDriveName(drive)}</h3>
                <span class="serial">${drive.serial_number || 'N/A'}</span>
            </div>
            
            <div class="info-group">
                <div class="info-group-label">Drive Information</div>
                <div class="info-row">
                    <span class="label">Type</span>
                    <span class="value">${driveType}</span>
                </div>
                <div class="info-row">
                    <span class="label">Capacity</span>
                    <span class="value">${Utils.formatSize(drive.user_capacity?.bytes)}</span>
                </div>
                <div class="info-row">
                    <span class="label">Firmware</span>
                    <span class="value">${drive.firmware_version || 'N/A'}</span>
                </div>
            </div>
            
            <div class="info-group">
                <div class="info-group-label">Health Status</div>
                <div class="info-row">
                    <span class="label">S.M.A.R.T.</span>
                    <span class="value ${drive.smart_status?.passed ? 'success' : 'danger'}">
                        ${drive.smart_status?.passed ? 'PASSED' : 'FAILED'}
                    </span>
                </div>
                <div class="info-row">
                    <span class="label">Temperature</span>
                    <span class="value">${drive.temperature?.current ?? '--'}°C</span>
                </div>
                <div class="info-row">
                    <span class="label">Power On</span>
                    <span class="value">${Utils.formatAge(drive.power_on_time?.hours)}</span>
                </div>
            </div>
        `;
        
        // Use SmartAttributes module if available
        if (typeof SmartAttributes !== 'undefined') {
            SmartAttributes.init(hostname, drive.serial_number, drive);
        } else {
            // Fallback to basic table
            this.renderBasicSmartTable(drive, container);
        }
    },

    renderBasicSmartTable(drive, container) {
        const isNvme = Utils.getDriveType(drive) === 'NVMe';
        
        if (isNvme) {
            this.renderNvmeBasicInfo(drive, container);
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
        
        container.innerHTML = `
            <div class="table-container">
                <table class="smart-table">
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Attribute</th>
                            <th>Value</th>
                            <th>Worst</th>
                            <th>Thresh</th>
                            <th>Raw</th>
                            <th class="status-cell">Status</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${attrs.map(attr => `
                            <tr>
                                <td>${attr.id}</td>
                                <td>${attr.name || 'Unknown'}</td>
                                <td>${attr.value ?? '-'}</td>
                                <td>${attr.worst ?? '-'}</td>
                                <td>${attr.thresh ?? '-'}</td>
                                <td>${attr.raw?.value ?? '-'}</td>
                                <td class="status-cell">
                                    <span class="attr-pill ${attr.when_failed ? 'fail' : 'ok'}">
                                        ${attr.when_failed ? 'FAIL' : 'OK'}
                                    </span>
                                </td>
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            </div>
        `;
    },

    renderNvmeBasicInfo(drive, container) {
        const health = drive.nvme_smart_health_information_log || {};
        
        container.innerHTML = `
            <div class="table-container">
                <table class="smart-table">
                    <thead>
                        <tr>
                            <th>Metric</th>
                            <th>Value</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr>
                            <td>Available Spare</td>
                            <td>${health.available_spare ?? '--'}%</td>
                        </tr>
                        <tr>
                            <td>Available Spare Threshold</td>
                            <td>${health.available_spare_threshold ?? '--'}%</td>
                        </tr>
                        <tr>
                            <td>Percentage Used</td>
                            <td>${health.percentage_used ?? '--'}%</td>
                        </tr>
                        <tr>
                            <td>Power On Hours</td>
                            <td>${health.power_on_hours ?? '--'}</td>
                        </tr>
                        <tr>
                            <td>Power Cycles</td>
                            <td>${health.power_cycles ?? '--'}</td>
                        </tr>
                        <tr>
                            <td>Unsafe Shutdowns</td>
                            <td>${health.unsafe_shutdowns ?? '--'}</td>
                        </tr>
                        <tr>
                            <td>Media Errors</td>
                            <td>${health.media_errors ?? '--'}</td>
                        </tr>
                        <tr>
                            <td>Critical Warning</td>
                            <td>${health.critical_warning ?? '--'}</td>
                        </tr>
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
    }
};