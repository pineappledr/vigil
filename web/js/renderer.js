/**
 * Vigil Dashboard - Rendering Functions
 */

const Renderer = {
    icons: {
        server: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1"/><circle cx="6" cy="18" r="1"/></svg>`,
        drive: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="2"/></svg>`,
        check: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`,
        warning: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
        capacity: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>`,
        temp: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 14.76V3.5a2.5 2.5 0 0 0-5 0v11.26a4.5 4.5 0 1 0 5 0z"/></svg>`
    },

    summaryCards(filterType = null) {
        const stats = State.getStats();
        const container = document.getElementById('summary-cards');

        const cards = [
            {
                icon: this.icons.server, iconClass: 'blue',
                value: stats.totalServers, label: 'Servers',
                onClick: 'Navigation.resetDashboard()', active: !filterType,
                title: 'View all servers'
            },
            {
                icon: this.icons.drive, iconClass: 'blue',
                value: stats.totalDrives, label: 'Total Drives',
                onClick: 'Navigation.showAllDrives()', active: filterType === 'all',
                title: 'View all drives'
            },
            {
                icon: this.icons.check, iconClass: 'green',
                value: stats.healthyDrives, label: 'Healthy',
                onClick: 'Navigation.showHealthyDrives()', active: filterType === 'healthy',
                title: 'View healthy drives'
            },
            {
                icon: this.icons.warning,
                iconClass: stats.attentionDrives > 0 ? 'red' : 'green',
                value: stats.attentionDrives, label: 'Needs Attention',
                onClick: stats.attentionDrives > 0 ? 'Navigation.showNeedsAttention()' : null,
                active: filterType === 'attention',
                title: stats.attentionDrives > 0 ? 'View drives needing attention' : 'All drives healthy'
            }
        ];

        container.innerHTML = cards.map(c => Components.summaryCard(c)).join('');
    },

    dashboard(servers) {
        const container = document.getElementById('server-list');
        this.summaryCards();

        if (!servers?.length) {
            container.innerHTML = Components.emptyState('noServers');
            return;
        }

        const sections = servers.map(server => {
            const serverIdx = State.data.findIndex(s => s.hostname === server.hostname);
            const drives = server.details?.drives || [];
            const indexed = drives.map((d, i) => ({ ...d, _idx: i }));
            return Components.serverSection(server, serverIdx, indexed);
        }).join('');

        container.innerHTML = `<div class="server-detail-view">${sections}</div>`;
        container.style.display = 'block';
    },

    filteredDrives(filterFn, filterType) {
        const container = document.getElementById('server-list');
        this.summaryCards(filterType);

        let html = '';
        State.data.forEach((server, serverIdx) => {
            const drives = server.details?.drives || [];
            const filtered = drives.map((d, i) => ({ ...d, _idx: i })).filter(filterFn);
            if (!filtered.length) return;

            html += `
                <div class="drive-section">
                    <div class="drive-section-header clickable" onclick="Navigation.showServer(${serverIdx})">
                        <div class="drive-section-title">
                            ${Components.icons.server}
                            <span>${server.hostname}</span>
                            <span class="drive-section-count">${filtered.length} drive${filtered.length !== 1 ? 's' : ''}</span>
                        </div>
                        <div class="drive-section-arrow">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
                        </div>
                    </div>
                    <div class="drive-grid">${filtered.map(d => Components.driveCard(d, serverIdx)).join('')}</div>
                </div>`;
        });

        container.innerHTML = html 
            ? `<div class="server-detail-view">${html}</div>` 
            : Components.emptyState(filterType === 'attention' ? 'attention' : 'noDrives');
        container.style.display = 'block';
    },

    serverDetail(server, serverIdx) {
        const container = document.getElementById('server-list');
        const drives = server.details?.drives || [];
        const nvme = [], ssd = [], hdd = [];

        drives.forEach((d, i) => {
            const drive = { ...d, _idx: i };
            const type = d.device?.type?.toLowerCase() || '';
            if (type === 'nvme' || d.device?.protocol === 'NVMe') nvme.push(drive);
            else if (d.rotation_rate === 0) ssd.push(drive);
            else hdd.push(drive);
        });

        this.serverSummaryCards(drives);

        container.innerHTML = `<div class="server-detail-view">
            ${Components.driveSection('NVMe Drives', Components.icons.nvme, nvme, serverIdx)}
            ${Components.driveSection('Solid State Drives', Components.icons.ssd, ssd, serverIdx)}
            ${Components.driveSection('Hard Disk Drives', Components.icons.hdd, hdd, serverIdx)}
        </div>`;
        container.style.display = 'block';
    },

    serverSummaryCards(drives) {
        const container = document.getElementById('summary-cards');
        let healthy = 0, totalCap = 0, totalTemp = 0, tempCount = 0;

        drives.forEach(d => {
            if (Utils.getHealthStatus(d) === 'healthy') healthy++;
            if (d.user_capacity?.bytes) totalCap += d.user_capacity.bytes;
            if (d.temperature?.current) { totalTemp += d.temperature.current; tempCount++; }
        });

        const avgTemp = tempCount ? Math.round(totalTemp / tempCount) : 0;
        const tempClass = avgTemp > 50 ? 'red' : avgTemp > 40 ? 'yellow' : 'green';
        const healthClass = healthy === drives.length ? 'green' : healthy > 0 ? 'yellow' : 'red';

        container.innerHTML = `
            ${Components.summaryCard({ icon: this.icons.drive, iconClass: 'blue', value: drives.length, label: 'Total Drives' })}
            ${Components.summaryCard({ icon: this.icons.capacity, iconClass: 'purple', value: Utils.formatSize(totalCap), label: 'Total Capacity' })}
            ${Components.summaryCard({ icon: this.icons.temp, iconClass: tempClass, value: `${avgTemp}°C`, label: 'Avg Temperature' })}
            ${Components.summaryCard({ icon: this.icons.check, iconClass: healthClass, value: `${healthy}/${drives.length}`, label: 'Healthy' })}
        `;
    },

    driveDetails(serverIdx, driveIdx) {
        const drive = State.data[serverIdx]?.details?.drives?.[driveIdx];
        if (!drive) return;

        const status = Utils.getHealthStatus(drive);
        const sidebar = document.getElementById('detail-sidebar');
        const table = document.getElementById('detail-table');

        const rotationType = drive.rotation_rate === 0 ? 'SSD' : drive.rotation_rate ? 'HDD' : 'Unknown';
        const rotationDetail = drive.rotation_rate === 0 ? 'Solid State Drive' : drive.rotation_rate ? `${drive.rotation_rate} RPM` : 'Not reported';

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
                ${this.infoRow('SMART Status', drive.smart_status?.passed ? 'PASSED' : 'FAILED', drive.smart_status?.passed ? 'success' : 'danger')}
                ${this.infoRow('Temperature', `${drive.temperature?.current ?? 'N/A'}°C`, drive.temperature?.current > 50 ? 'warning' : '')}
                ${this.infoRow('Powered On', Utils.formatAge(drive.power_on_time?.hours))}
                ${this.infoRow('Power Cycles', drive.power_cycle_count ?? 'N/A')}
            </div>
        `;

        const attrs = drive.ata_smart_attributes?.table || [];
        if (!attrs.length) {
            table.innerHTML = `<div class="nvme-notice"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg><p>No standard ATA SMART attributes available</p><span>NVMe drives use different health reporting</span></div>`;
        } else {
            const criticalIds = [5, 187, 197, 198];
            table.innerHTML = `<thead><tr><th class="status-cell">Status</th><th>ID</th><th>Attribute</th><th>Value</th><th>Worst</th><th>Thresh</th><th>Raw</th></tr></thead><tbody>${attrs.map(a => {
                const fail = (criticalIds.includes(a.id) && a.raw?.value > 0) || (a.thresh > 0 && a.value <= a.thresh);
                return `<tr><td class="status-cell"><span class="attr-pill ${fail ? 'fail' : 'ok'}">${fail ? 'FAIL' : 'OK'}</span></td><td>${a.id}</td><td style="font-family:var(--font-sans)">${a.name}</td><td>${a.value}</td><td>${a.worst ?? '-'}</td><td>${a.thresh}</td><td>${a.raw?.value ?? '-'}</td></tr>`;
            }).join('')}</tbody>`;
        }
    },

    infoRow(label, value, cls = '') {
        return `<div class="info-row"><span class="label">${label}</span><span class="value ${cls}">${value}</span></div>`;
    },

    settings() {
        let el = document.getElementById('settings-view');
        if (!el) {
            el = document.createElement('div');
            el.id = 'settings-view';
            el.className = 'view settings-view';
            document.querySelector('.main-content').appendChild(el);
        }

        el.innerHTML = `<div class="settings-container">
            <div class="settings-section">
                <div class="settings-section-header"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg><h3>Security</h3></div>
                <div class="settings-card"><div class="settings-item"><div class="settings-item-info"><div class="settings-item-title">Change Password</div><div class="settings-item-desc">Update your account password</div></div><button class="btn btn-secondary" onclick="Modals.showChangePassword()">Change</button></div></div>
            </div>
            <div class="settings-section">
                <div class="settings-section-header"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg><h3>Account</h3></div>
                <div class="settings-card">
                    <div class="settings-item"><div class="settings-item-info"><div class="settings-item-title">Username</div><div class="settings-item-desc">${State.currentUser}</div></div><button class="btn btn-secondary" onclick="Modals.showChangeUsername()">Change</button></div>
                    <div class="settings-item"><div class="settings-item-info"><div class="settings-item-title">Sign Out</div><div class="settings-item-desc">Log out of your account</div></div><button class="btn btn-danger" onclick="Auth.logout()">Sign Out</button></div>
                </div>
            </div>
            <div class="settings-section">
                <div class="settings-section-header"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg><h3>About</h3></div>
                <div class="settings-card">
                    <div class="settings-item"><div class="settings-item-info"><div class="settings-item-title">Version</div><div class="settings-item-desc" id="settings-version">Loading...</div></div></div>
                    <div class="settings-item"><div class="settings-item-info"><div class="settings-item-title">GitHub</div><div class="settings-item-desc"><a href="https://github.com/pineappledr/vigil" target="_blank">github.com/pineappledr/vigil</a></div></div></div>
                </div>
            </div>
        </div>`;
        el.classList.remove('hidden');
        API.getVersion().then(r => r.json()).then(d => document.getElementById('settings-version').textContent = d.version || 'Unknown');
    }
};
