/**
 * Vigil Dashboard - ZFS Pools Module
 * Handles ZFS pool visualization, status cards, and detail modals
 */

const ZFS = {
    // ─── Icons ───────────────────────────────────────────────────────────────
    icons: {
        pool: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M4 6h16M4 12h16M4 18h16"/>
            <circle cx="7" cy="6" r="1" fill="currentColor"/>
            <circle cx="7" cy="12" r="1" fill="currentColor"/>
            <circle cx="7" cy="18" r="1" fill="currentColor"/>
        </svg>`,
        drive: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="4" width="20" height="16" rx="2"/>
            <circle cx="8" cy="12" r="2"/>
            <line x1="14" y1="9" x2="18" y2="9"/>
            <line x1="14" y1="12" x2="18" y2="12"/>
        </svg>`,
        scrub: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/>
            <polyline points="12 6 12 12 16 14"/>
        </svg>`,
        check: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
            <polyline points="22 4 12 14.01 9 11.01"/>
        </svg>`,
        warning: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
            <line x1="12" y1="9" x2="12" y2="13"/>
            <line x1="12" y1="17" x2="12.01" y2="17"/>
        </svg>`,
        error: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/>
            <line x1="15" y1="9" x2="9" y2="15"/>
            <line x1="9" y1="9" x2="15" y2="15"/>
        </svg>`,
        server: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="2" width="20" height="8" rx="2"/>
            <rect x="2" y="14" width="20" height="8" rx="2"/>
            <circle cx="6" cy="6" r="1"/>
            <circle cx="6" cy="18" r="1"/>
        </svg>`
    },

    // ─── Main Render ─────────────────────────────────────────────────────────
    render() {
        const container = document.getElementById('dashboard-view');
        if (!container) return;

        const stats = State.getZFSStats();
        const poolsByHost = State.getPoolsByHost();
        const hostnames = Object.keys(poolsByHost).sort();

        container.innerHTML = `
            ${this.renderSummaryCards(stats)}
            <div class="section-header">
                <h2>ZFS Pools</h2>
            </div>
            <div id="zfs-pools-container" class="zfs-pools-container">
                ${hostnames.length > 0 
                    ? hostnames.map(host => this.renderHostSection(host, poolsByHost[host])).join('')
                    : this.renderEmptyState()
                }
            </div>
        `;
    },

    // ─── Summary Cards ───────────────────────────────────────────────────────
    renderSummaryCards(stats) {
        return `
            <div class="summary-grid zfs-summary">
                <div class="summary-card">
                    <div class="icon accent">${this.icons.pool}</div>
                    <div class="value">${stats.totalPools}</div>
                    <div class="label">Total Pools</div>
                </div>
                <div class="summary-card ${stats.healthyPools === stats.totalPools ? 'healthy-glow' : ''}">
                    <div class="icon success">${this.icons.check}</div>
                    <div class="value">${stats.healthyPools}</div>
                    <div class="label">Online</div>
                </div>
                <div class="summary-card clickable ${stats.degradedPools > 0 ? 'warning-glow' : ''}" 
                     onclick="ZFS.filterByState('DEGRADED')"
                     style="${stats.degradedPools === 0 ? 'opacity: 0.5;' : ''}">
                    <div class="icon warning">${this.icons.warning}</div>
                    <div class="value">${stats.degradedPools}</div>
                    <div class="label">Degraded</div>
                </div>
                <div class="summary-card clickable ${stats.faultedPools > 0 ? 'danger-glow' : ''}"
                     onclick="ZFS.filterByState('FAULTED')"
                     style="${stats.faultedPools === 0 ? 'opacity: 0.5;' : ''}">
                    <div class="icon danger">${this.icons.error}</div>
                    <div class="value">${stats.faultedPools}</div>
                    <div class="label">Faulted</div>
                </div>
            </div>
        `;
    },

    // ─── Host Section ────────────────────────────────────────────────────────
    renderHostSection(hostname, pools) {
        return `
            <div class="zfs-host-section">
                <div class="zfs-host-header">
                    <div class="zfs-host-title">
                        ${this.icons.server}
                        <span>${hostname}</span>
                        <span class="zfs-host-count">${pools.length} pool${pools.length !== 1 ? 's' : ''}</span>
                    </div>
                </div>
                <div class="zfs-pool-grid">
                    ${pools.map(pool => this.renderPoolCard(pool, hostname)).join('')}
                </div>
            </div>
        `;
    },

    // ─── Pool Card ───────────────────────────────────────────────────────────
    renderPoolCard(pool, hostname) {
        const state = (pool.state || pool.health || 'UNKNOWN').toUpperCase();
        const stateClass = this.getStateClass(state);
        const capacity = this.parseCapacity(pool);
        const scrub = this.parseScrub(pool);
        const deviceStats = this.getDeviceStats(pool);

        return `
            <div class="zfs-pool-card ${stateClass}" onclick="ZFS.showPoolDetail('${hostname}', '${pool.name}')">
                <div class="zfs-pool-header">
                    <div class="zfs-pool-status">
                        <span class="zfs-status-dot ${stateClass}"></span>
                        <span class="zfs-pool-name">${pool.name}</span>
                    </div>
                    <span class="zfs-state-badge ${stateClass}">${state}</span>
                </div>
                
                <div class="zfs-pool-capacity">
                    <div class="zfs-capacity-info">
                        <span class="zfs-capacity-used">${capacity.used}</span>
                        <span class="zfs-capacity-sep">/</span>
                        <span class="zfs-capacity-total">${capacity.total}</span>
                        <span class="zfs-capacity-percent">(${capacity.percent}%)</span>
                    </div>
                    <div class="zfs-capacity-bar">
                        <div class="zfs-capacity-fill ${this.getCapacityClass(capacity.percent)}" 
                             style="width: ${Math.min(capacity.percent, 100)}%"></div>
                    </div>
                </div>

                <div class="zfs-pool-scrub">
                    ${this.icons.scrub}
                    <span class="zfs-scrub-info">
                        ${scrub.text}
                    </span>
                </div>

                <div class="zfs-pool-devices">
                    ${this.icons.drive}
                    <span class="zfs-device-info">
                        ${deviceStats.total} device${deviceStats.total !== 1 ? 's' : ''}
                        ${deviceStats.errors > 0 
                            ? `<span class="zfs-error-count">${deviceStats.errors} error${deviceStats.errors !== 1 ? 's' : ''}</span>` 
                            : '<span class="zfs-no-errors">0 errors</span>'
                        }
                    </span>
                </div>
            </div>
        `;
    },

    // ─── Empty State ─────────────────────────────────────────────────────────
    renderEmptyState() {
        return `
            <div class="empty-state zfs-empty">
                ${this.icons.pool}
                <p>No ZFS pools detected</p>
                <span class="hint">ZFS pools will appear here when agents report them</span>
            </div>
        `;
    },

    // ─── Pool Detail Modal ───────────────────────────────────────────────────
    async showPoolDetail(hostname, poolName) {
        // Show loading modal
        this.showModal(`
            <div class="zfs-modal-header">
                <h3>${poolName}</h3>
                <button class="modal-close" onclick="ZFS.closeModal()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="18" y1="6" x2="6" y2="18"/>
                        <line x1="6" y1="6" x2="18" y2="18"/>
                    </svg>
                </button>
            </div>
            <div class="zfs-modal-body">
                <div class="zfs-loading">Loading pool details...</div>
            </div>
        `);

        // Fetch detailed pool data
        const poolData = await Data.fetchZFSPoolDetail(hostname, poolName);
        
        if (!poolData) {
            this.updateModalBody('<div class="zfs-error">Failed to load pool details</div>');
            return;
        }

        this.updateModalBody(this.renderPoolDetailContent(poolData, hostname));
    },

    renderPoolDetailContent(data, hostname) {
        const pool = data.pool || data;
        const devices = data.devices || pool.devices || [];
        const scrubHistory = data.scrub_history || [];
        const state = (pool.state || pool.health || 'UNKNOWN').toUpperCase();
        const stateClass = this.getStateClass(state);
        const capacity = this.parseCapacity(pool);
        const scrub = this.parseScrub(pool);

        return `
            <div class="zfs-detail-tabs">
                <button class="zfs-tab active" onclick="ZFS.switchTab(this, 'overview')">Overview</button>
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'devices')">Devices (${devices.length})</button>
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'scrubs')">Scrub History</button>
            </div>

            <div id="zfs-tab-overview" class="zfs-tab-content active">
                <div class="zfs-detail-section">
                    <h4>Pool Status</h4>
                    <div class="zfs-detail-grid">
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">State</span>
                            <span class="zfs-state-badge ${stateClass}">${state}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Host</span>
                            <span class="zfs-detail-value">${hostname}</span>
                        </div>
                    </div>
                </div>

                <div class="zfs-detail-section">
                    <h4>Capacity</h4>
                    <div class="zfs-detail-grid">
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Total Size</span>
                            <span class="zfs-detail-value">${capacity.total}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Used</span>
                            <span class="zfs-detail-value">${capacity.used} (${capacity.percent}%)</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Free</span>
                            <span class="zfs-detail-value">${capacity.free}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Fragmentation</span>
                            <span class="zfs-detail-value">${pool.fragmentation || pool.frag || '0%'}</span>
                        </div>
                    </div>
                    <div class="zfs-capacity-bar large">
                        <div class="zfs-capacity-fill ${this.getCapacityClass(capacity.percent)}" 
                             style="width: ${Math.min(capacity.percent, 100)}%"></div>
                    </div>
                </div>

                <div class="zfs-detail-section">
                    <h4>Last Scrub</h4>
                    <div class="zfs-detail-grid">
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Status</span>
                            <span class="zfs-detail-value">${scrub.state || 'Unknown'}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Date</span>
                            <span class="zfs-detail-value">${scrub.date || 'Never'}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Duration</span>
                            <span class="zfs-detail-value">${scrub.duration || 'N/A'}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="zfs-detail-label">Errors</span>
                            <span class="zfs-detail-value ${scrub.errors > 0 ? 'error-text' : ''}">${scrub.errors ?? 'N/A'}</span>
                        </div>
                    </div>
                </div>
            </div>

            <div id="zfs-tab-devices" class="zfs-tab-content">
                ${this.renderDevicesTable(devices)}
            </div>

            <div id="zfs-tab-scrubs" class="zfs-tab-content">
                ${this.renderScrubHistory(scrubHistory)}
            </div>
        `;
    },

    renderDevicesTable(devices) {
        if (!devices || devices.length === 0) {
            return '<div class="zfs-no-data">No device information available</div>';
        }

        return `
            <div class="zfs-devices-table-wrapper">
                <table class="zfs-devices-table">
                    <thead>
                        <tr>
                            <th>Device</th>
                            <th>State</th>
                            <th>Read Errors</th>
                            <th>Write Errors</th>
                            <th>Checksum Errors</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${devices.map(device => this.renderDeviceRow(device)).join('')}
                    </tbody>
                </table>
            </div>
        `;
    },

    renderDeviceRow(device) {
        const state = (device.state || 'UNKNOWN').toUpperCase();
        const stateClass = this.getStateClass(state);
        const readErrors = device.read_errors || 0;
        const writeErrors = device.write_errors || 0;
        const checksumErrors = device.checksum_errors || 0;
        const hasErrors = readErrors > 0 || writeErrors > 0 || checksumErrors > 0;
        const deviceName = device.device_name || device.name || device.device || 'Unknown';
        const serial = device.serial ? ` (${device.serial})` : '';

        return `
            <tr class="${hasErrors ? 'has-errors' : ''}">
                <td class="device-name">
                    ${deviceName}${serial}
                    ${device.vdev ? `<span class="vdev-info">${device.vdev}</span>` : ''}
                </td>
                <td><span class="zfs-state-badge small ${stateClass}">${state}</span></td>
                <td class="${readErrors > 0 ? 'error-text' : ''}">${readErrors}</td>
                <td class="${writeErrors > 0 ? 'error-text' : ''}">${writeErrors}</td>
                <td class="${checksumErrors > 0 ? 'error-text' : ''}">${checksumErrors}</td>
            </tr>
        `;
    },

    renderScrubHistory(history) {
        if (!history || history.length === 0) {
            return '<div class="zfs-no-data">No scrub history available</div>';
        }

        return `
            <div class="zfs-scrub-history">
                ${history.map(scrub => `
                    <div class="zfs-scrub-item ${scrub.errors > 0 ? 'has-errors' : ''}">
                        <div class="zfs-scrub-date">
                            ${this.formatDate(scrub.end_time || scrub.start_time || scrub.date)}
                        </div>
                        <div class="zfs-scrub-details">
                            <span class="zfs-scrub-duration">${scrub.duration || 'Unknown duration'}</span>
                            <span class="zfs-scrub-errors ${scrub.errors > 0 ? 'error-text' : ''}">
                                ${scrub.errors || 0} error${scrub.errors !== 1 ? 's' : ''}
                            </span>
                            ${scrub.bytes_examined ? `<span class="zfs-scrub-bytes">${this.formatBytes(scrub.bytes_examined)} examined</span>` : ''}
                        </div>
                    </div>
                `).join('')}
            </div>
        `;
    },

    // ─── Modal Helpers ───────────────────────────────────────────────────────
    showModal(content) {
        let modal = document.getElementById('zfs-modal-overlay');
        if (!modal) {
            modal = document.createElement('div');
            modal.id = 'zfs-modal-overlay';
            modal.className = 'modal-overlay';
            modal.onclick = (e) => {
                if (e.target === modal) this.closeModal();
            };
            document.body.appendChild(modal);
        }

        modal.innerHTML = `<div class="modal zfs-modal">${content}</div>`;
        modal.classList.add('show');
    },

    updateModalBody(content) {
        const body = document.querySelector('#zfs-modal-overlay .zfs-modal-body');
        if (body) {
            body.innerHTML = content;
        }
    },

    closeModal() {
        const modal = document.getElementById('zfs-modal-overlay');
        if (modal) {
            modal.classList.remove('show');
            setTimeout(() => modal.remove(), 200);
        }
    },

    switchTab(button, tabName) {
        // Update tab buttons
        document.querySelectorAll('.zfs-tab').forEach(t => t.classList.remove('active'));
        button.classList.add('active');

        // Update tab content
        document.querySelectorAll('.zfs-tab-content').forEach(c => c.classList.remove('active'));
        document.getElementById(`zfs-tab-${tabName}`)?.classList.add('active');
    },

    // ─── Filter Helpers ──────────────────────────────────────────────────────
    filterByState(state) {
        // Could implement filtering, for now just scroll to first matching pool
        const pools = document.querySelectorAll(`.zfs-pool-card.${state.toLowerCase()}`);
        if (pools.length > 0) {
            pools[0].scrollIntoView({ behavior: 'smooth', block: 'center' });
            pools[0].classList.add('highlight');
            setTimeout(() => pools[0].classList.remove('highlight'), 2000);
        }
    },

    // ─── Utility Methods ─────────────────────────────────────────────────────
    getStateClass(state) {
        const stateMap = {
            'ONLINE': 'online',
            'DEGRADED': 'degraded',
            'FAULTED': 'faulted',
            'UNAVAIL': 'faulted',
            'OFFLINE': 'offline',
            'REMOVED': 'offline'
        };
        return stateMap[state] || 'unknown';
    },

    getCapacityClass(percent) {
        if (percent >= 90) return 'critical';
        if (percent >= 80) return 'warning';
        return 'healthy';
    },

    parseCapacity(pool) {
        // Handle different field names from backend
        const size = pool.size || pool.total_size || '0';
        const alloc = pool.alloc || pool.allocated || pool.used || '0';
        const free = pool.free || pool.available || '0';
        let percent = pool.capacity_percent || pool.capacity || 0;

        // If percent is a string with %, parse it
        if (typeof percent === 'string') {
            percent = parseInt(percent.replace('%', ''), 10) || 0;
        }

        return {
            total: this.formatStorageSize(size),
            used: this.formatStorageSize(alloc),
            free: this.formatStorageSize(free),
            percent: percent
        };
    },

    parseScrub(pool) {
        const scrub = pool.scrub || pool.scan || {};
        
        if (!scrub || Object.keys(scrub).length === 0) {
            return { text: 'No scrub data', state: null, date: null, duration: null, errors: null };
        }

        const state = scrub.state || scrub.status || 'unknown';
        const date = this.formatDate(scrub.end_time || scrub.date || scrub.timestamp);
        const duration = scrub.duration || null;
        const errors = scrub.errors ?? scrub.error_count ?? 0;

        let text = '';
        if (state === 'completed' || state === 'scrub repaired') {
            text = `${date} • ${errors} error${errors !== 1 ? 's' : ''}`;
            if (duration) text += ` • ${duration}`;
        } else if (state === 'in_progress' || state === 'scrubbing') {
            const progress = scrub.progress || scrub.percent || 0;
            text = `In progress (${progress}%)`;
        } else if (state === 'none' || state === 'never') {
            text = 'Never scrubbed';
        } else {
            text = state;
        }

        return { text, state, date, duration, errors };
    },

    getDeviceStats(pool) {
        const devices = pool.devices || [];
        let total = devices.length;
        let errors = 0;

        devices.forEach(d => {
            errors += (d.read_errors || 0) + (d.write_errors || 0) + (d.checksum_errors || 0);
        });

        return { total, errors };
    },

    formatStorageSize(size) {
        if (!size) return '0 B';
        if (typeof size === 'string') return size;
        
        const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        let value = parseFloat(size);
        let unitIndex = 0;

        while (value >= 1024 && unitIndex < units.length - 1) {
            value /= 1024;
            unitIndex++;
        }

        return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[unitIndex]}`;
    },

    formatBytes(bytes) {
        return this.formatStorageSize(bytes);
    },

    formatDate(dateStr) {
        if (!dateStr) return 'Unknown';
        
        try {
            const date = new Date(dateStr);
            if (isNaN(date.getTime())) return dateStr;
            
            const now = new Date();
            const diffDays = Math.floor((now - date) / (1000 * 60 * 60 * 24));
            
            if (diffDays === 0) return 'Today';
            if (diffDays === 1) return 'Yesterday';
            if (diffDays < 7) return `${diffDays} days ago`;
            
            return date.toLocaleDateString(undefined, { 
                month: 'short', 
                day: 'numeric',
                year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined
            });
        } catch {
            return dateStr;
        }
    }
};

// Global function for navigation
function showZFSPools() {
    Navigation.showZFS();
}