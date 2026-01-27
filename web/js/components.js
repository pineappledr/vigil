/**
 * Vigil Dashboard - UI Components
 */

const Components = {
    icons: {
        server: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon server"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1"/><circle cx="6" cy="18" r="1"/></svg>`,
        nvme: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon nvme"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>`,
        ssd: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon ssd"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M6 8h4v8H6z"/><path d="M14 8h4"/><path d="M14 12h4"/><path d="M14 16h4"/></svg>`,
        hdd: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon hdd"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="3"/><line x1="14" y1="9" x2="18" y2="9"/><line x1="14" y1="12" x2="18" y2="12"/><line x1="14" y1="15" x2="18" y2="15"/></svg>`
    },

    summaryCard({ icon, iconClass, value, label, onClick, active = false, title = '' }) {
        const clickable = onClick ? 'clickable' : '';
        const activeClass = active ? 'active' : '';
        const onClickAttr = onClick ? `onclick="${onClick}"` : '';
        
        return `
            <div class="summary-card ${clickable} ${activeClass}" ${onClickAttr} title="${title}">
                <div class="icon ${iconClass}">${icon}</div>
                <div class="value">${value}</div>
                <div class="label">${label}</div>
            </div>
        `;
    },

    driveCard(drive, serverIdx) {
        const status = Utils.getHealthStatus(drive);
        const driveType = Utils.getDriveType(drive);
        const driveName = Utils.getDriveName(drive);
        const hostname = State.data[serverIdx]?.hostname || '';
        const serial = drive.serial_number || '';
        const alias = drive._alias || '';

        return `
            <div class="drive-card ${status}" onclick="Navigation.showDriveDetails(${serverIdx}, ${drive._idx})">
                <div class="drive-card-header">
                    <div class="drive-card-icon ${status}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="2" y="4" width="20" height="16" rx="2"/>
                            <circle cx="8" cy="12" r="2"/>
                            <line x1="14" y1="9" x2="18" y2="9"/>
                            <line x1="14" y1="12" x2="18" y2="12"/>
                        </svg>
                    </div>
                    <button class="alias-btn" onclick="event.stopPropagation(); Modals.showAlias('${hostname}', '${serial}', '${Utils.escapeHtml(alias)}', '${Utils.escapeHtml(driveName)}')" title="Set alias">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                        </svg>
                    </button>
                    <span class="status-badge ${drive.smart_status?.passed ? 'passed' : 'failed'}">
                        ${drive.smart_status?.passed ? 'Passed' : 'Failed'}
                    </span>
                </div>
                <div class="drive-card-body">
                    <div class="drive-card-model">${driveName}</div>
                    <div class="drive-card-serial">${serial || 'N/A'}</div>
                </div>
                <div class="drive-card-stats">
                    <div class="drive-card-stat">
                        <span class="stat-value">${Utils.formatSize(drive.user_capacity?.bytes)}</span>
                        <span class="stat-label">Capacity</span>
                    </div>
                    <div class="drive-card-stat">
                        <span class="stat-value">${drive.temperature?.current ?? '--'}°C</span>
                        <span class="stat-label">Temp</span>
                    </div>
                    <div class="drive-card-stat">
                        <span class="stat-value">${Utils.formatAge(drive.power_on_time?.hours)}</span>
                        <span class="stat-label">Age</span>
                    </div>
                </div>
                <div class="drive-card-footer">
                    <span class="drive-type-badge">${driveType}</span>
                </div>
            </div>
        `;
    },

    emptyState(type) {
        const states = {
            attention: {
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                    <polyline points="22 4 12 14.01 9 11.01"/>
                </svg>`,
                message: 'All drives are healthy!',
                hint: 'No drives currently need attention',
                className: 'success-state'
            },
            noDrives: {
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <rect x="2" y="4" width="20" height="16" rx="2"/>
                    <circle cx="8" cy="12" r="2"/>
                </svg>`,
                message: 'No drives found',
                hint: 'No drives match the current filter',
                className: ''
            },
            noServers: {
                icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <rect x="2" y="2" width="20" height="8" rx="2"/>
                    <rect x="2" y="14" width="20" height="8" rx="2"/>
                    <circle cx="6" cy="6" r="1"/>
                    <circle cx="6" cy="18" r="1"/>
                </svg>`,
                message: 'Waiting for agents to connect...',
                hint: 'Run vigil-agent on your servers to begin monitoring',
                className: ''
            }
        };

        const state = states[type] || states.noDrives;
        return `
            <div class="empty-state ${state.className}">
                ${state.icon}
                <p>${state.message}</p>
                <span class="hint">${state.hint}</span>
            </div>
        `;
    },

    driveSection(title, icon, drives, serverIdx) {
        if (drives.length === 0) return '';
        
        return `
            <div class="drive-section">
                <div class="drive-section-header">
                    <div class="drive-section-title">
                        ${icon}
                        <span>${title}</span>
                        <span class="drive-section-count">${drives.length}</span>
                    </div>
                </div>
                <div class="drive-grid">
                    ${drives.map(d => this.driveCard(d, serverIdx)).join('')}
                </div>
            </div>
        `;
    },

    serverSection(server, serverIdx, drives) {
        const driveCount = drives.length;
        const countText = `${driveCount} drive${driveCount !== 1 ? 's' : ''}`;

        return `
            <div class="drive-section">
                <div class="drive-section-header clickable" onclick="Navigation.showServer(${serverIdx})">
                    <div class="drive-section-title">
                        ${this.icons.server}
                        <span>${server.hostname}</span>
                        <span class="drive-section-count">${countText}</span>
                    </div>
                    <div class="drive-section-meta">
                        <span class="timestamp">${Utils.formatTime(server.timestamp)}</span>
                    </div>
                    <div class="drive-section-arrow">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="9 18 15 12 9 6"/>
                        </svg>
                    </div>
                </div>
                ${driveCount > 0 
                    ? `<div class="drive-grid">${drives.map(d => this.driveCard(d, serverIdx)).join('')}</div>`
                    : '<div class="drive-grid-empty"><p>No drives detected</p></div>'
                }
            </div>
        `;
    }
};
