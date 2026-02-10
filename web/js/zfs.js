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
                ${stats.totalPools > 0 ? `<span class="section-count">${stats.totalPools} pool${stats.totalPools !== 1 ? 's' : ''}</span>` : ''}
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
        // Get pool name - handle both 'name' and 'pool_name' from API
        const poolName = pool.name || pool.pool_name || 'Unknown Pool';
        const state = (pool.status || pool.state || pool.health || 'UNKNOWN').toUpperCase();
        const stateClass = this.getStateClass(state);
        const capacity = this.parseCapacity(pool);
        const scrub = this.parseScrub(pool);
        const deviceStats = this.getDeviceStats(pool);

        return `
            <div class="zfs-pool-card ${stateClass}" onclick="ZFS.showPoolDetail('${hostname}', '${poolName}')">
                <div class="zfs-pool-header">
                    <div class="zfs-pool-status">
                        <span class="zfs-status-dot ${stateClass}"></span>
                        <span class="zfs-pool-name">${poolName}</span>
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
                    <span class="zfs-scrub-info">${scrub.text}</span>
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
                <span class="hint">ZFS pools will appear here when agents report them.<br>
                Make sure ZFS is installed and the Vigil agent has ZFS detection enabled.</span>
            </div>
        `;
    },

    renderLoadingState() {
        return `
            <div class="zfs-loading-state">
                <div class="zfs-loading-spinner"></div>
                <p>Loading ZFS pools...</p>
            </div>
        `;
    },

    // ─── Pool Detail Modal ───────────────────────────────────────────────────
    async showPoolDetail(hostname, poolName) {
        this.showModal(`
            <div class="zfs-modal-header">
                <h3>${poolName}</h3>
                <span class="zfs-modal-host">${hostname}</span>
                <button class="modal-close" onclick="ZFS.closeModal()">×</button>
            </div>
            <div class="zfs-modal-body">
                ${this.renderLoadingState()}
            </div>
        `);

        try {
            const detail = await Data.fetchZFSPoolDetail(hostname, poolName);
            if (detail) {
                this.updateModalBody(this.renderPoolDetail(detail, hostname));
            } else {
                this.updateModalBody(`<p class="error-text">Failed to load pool details</p>`);
            }
        } catch (err) {
            console.error('Failed to load pool detail:', err);
            this.updateModalBody(`<p class="error-text">Error: ${err.message}</p>`);
        }
    },

    renderPoolDetail(detail, hostname) {
        const pool = detail.pool || detail;
        const devices = detail.devices || pool.devices || [];
        const scrubHistory = detail.scrub_history || [];
        const poolName = pool.name || pool.pool_name || 'Unknown';
        const state = (pool.status || pool.state || pool.health || 'UNKNOWN').toUpperCase();
        const capacity = this.parseCapacity(pool);

        // Store hostname for device click handlers
        this._currentHostname = hostname;

        return `
            <div class="zfs-detail-tabs">
                <button class="zfs-tab active" onclick="ZFS.switchTab(this, 'overview')">Overview</button>
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'devices')">Devices (${devices.length})</button>
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'scrubs')">Scrub History</button>
            </div>

            <div id="zfs-tab-overview" class="zfs-tab-content active">
                ${this.renderOverviewTab(pool, capacity, state)}
            </div>

            <div id="zfs-tab-devices" class="zfs-tab-content">
                ${this.renderDevicesTab(devices, hostname)}
            </div>

            <div id="zfs-tab-scrubs" class="zfs-tab-content">
                ${this.renderScrubsTab(scrubHistory)}
            </div>
        `;
    },

    renderOverviewTab(pool, capacity, state) {
        const poolName = pool.name || pool.pool_name || 'Unknown';
        return `
            <div class="zfs-detail-grid">
                <div class="zfs-detail-item">
                    <span class="label">Pool Name</span>
                    <span class="value">${poolName}</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Status</span>
                    <span class="value zfs-state-badge ${this.getStateClass(state)}">${state}</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Capacity</span>
                    <span class="value">${capacity.used} / ${capacity.total} (${capacity.percent}%)</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Free Space</span>
                    <span class="value">${capacity.free}</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Fragmentation</span>
                    <span class="value">${pool.fragmentation || 0}%</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Dedup Ratio</span>
                    <span class="value">${pool.dedup_ratio || 1.00}x</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Read Errors</span>
                    <span class="value ${pool.read_errors > 0 ? 'error-text' : ''}">${pool.read_errors || 0}</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Write Errors</span>
                    <span class="value ${pool.write_errors > 0 ? 'error-text' : ''}">${pool.write_errors || 0}</span>
                </div>
                <div class="zfs-detail-item">
                    <span class="label">Checksum Errors</span>
                    <span class="value ${pool.checksum_errors > 0 ? 'error-text' : ''}">${pool.checksum_errors || 0}</span>
                </div>
            </div>
        `;
    },

    renderDevicesTab(devices, hostname) {
        if (!devices || devices.length === 0) {
            return `<p class="zfs-no-data">No device information available</p>`;
        }

        return `
            <div class="zfs-devices-list">
                ${devices.map(dev => {
                    const serial = dev.serial_number || '';
                    const hasSerial = serial && serial.length > 0;
                    const driveLink = hasSerial ? this.findDriveBySerial(hostname, serial) : null;
                    
                    return `
                    <div class="zfs-device-item ${this.getStateClass(dev.state || 'ONLINE')}">
                        <div class="zfs-device-header">
                            <span class="zfs-device-name">${dev.device_name || dev.name || 'Unknown'}</span>
                            <span class="zfs-device-type">${dev.vdev_type || 'disk'}</span>
                            <span class="zfs-device-state ${this.getStateClass(dev.state || 'ONLINE')}">${dev.state || 'ONLINE'}</span>
                        </div>
                        <div class="zfs-device-details">
                            ${dev.device_path || dev.path ? `<span class="zfs-device-path">${dev.device_path || dev.path}</span>` : ''}
                            ${hasSerial ? `
                                <span class="zfs-device-serial ${driveLink ? 'clickable' : ''}" 
                                      ${driveLink ? `onclick="ZFS.navigateToDrive(${driveLink.serverIdx}, ${driveLink.driveIdx})" title="Click to view drive details"` : ''}>
                                    S/N: ${serial}
                                    ${driveLink ? '<svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>' : ''}
                                </span>
                            ` : ''}
                            <span class="zfs-device-errors">
                                R: ${dev.read_errors || 0} / W: ${dev.write_errors || 0} / C: ${dev.checksum_errors || 0}
                            </span>
                        </div>
                    </div>
                `}).join('')}
            </div>
        `;
    },

    /**
     * Find a drive in State.data by hostname and serial number
     * @returns {Object|null} { serverIdx, driveIdx } or null
     */
    findDriveBySerial(hostname, serial) {
        if (!serial || !hostname) return null;
        
        for (let serverIdx = 0; serverIdx < State.data.length; serverIdx++) {
            const server = State.data[serverIdx];
            if (server.hostname !== hostname) continue;
            
            const drives = server.details?.drives || [];
            for (let driveIdx = 0; driveIdx < drives.length; driveIdx++) {
                if (drives[driveIdx].serial_number === serial) {
                    return { serverIdx, driveIdx };
                }
            }
        }
        return null;
    },

    /**
     * Navigate to drive details from ZFS pool view
     */
    navigateToDrive(serverIdx, driveIdx) {
        this.closeModal();
        Navigation.showDriveDetails(serverIdx, driveIdx);
    },

    renderScrubsTab(history) {
        if (!history || history.length === 0) {
            return `<p class="zfs-no-data">No scrub history available</p>`;
        }

        return `
            <div class="zfs-scrub-history">
                ${history.map(scrub => `
                    <div class="zfs-scrub-item ${scrub.errors_found > 0 ? 'has-errors' : ''}">
                        <div class="zfs-scrub-date">
                            ${this.formatDate(scrub.end_time || scrub.start_time)}
                        </div>
                        <div class="zfs-scrub-details">
                            <span class="zfs-scrub-duration">${this.formatDuration(scrub.duration_secs) || 'Unknown'}</span>
                            <span class="zfs-scrub-errors ${scrub.errors_found > 0 ? 'error-text' : ''}">
                                ${scrub.errors_found || 0} error${scrub.errors_found !== 1 ? 's' : ''}
                            </span>
                            ${scrub.data_examined ? `<span class="zfs-scrub-bytes">${this.formatBytes(scrub.data_examined)} examined</span>` : ''}
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
        if (body) body.innerHTML = content;
    },

    closeModal() {
        const modal = document.getElementById('zfs-modal-overlay');
        if (modal) {
            modal.classList.remove('show');
            setTimeout(() => modal.remove(), 200);
        }
    },

    switchTab(button, tabName) {
        document.querySelectorAll('.zfs-tab').forEach(t => t.classList.remove('active'));
        button.classList.add('active');
        document.querySelectorAll('.zfs-tab-content').forEach(c => c.classList.remove('active'));
        document.getElementById(`zfs-tab-${tabName}`)?.classList.add('active');
    },

    filterByState(state) {
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
        const size = pool.size_bytes || pool.size || pool.total_size || 0;
        const alloc = pool.allocated_bytes || pool.alloc || pool.allocated || pool.used || 0;
        const free = pool.free_bytes || pool.free || pool.available || 0;
        let percent = pool.capacity_pct || pool.capacity_percent || pool.capacity || 0;
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
        const scanFunction = pool.scan_function || '';
        const scanState = pool.scan_state || '';
        const scanProgress = pool.scan_progress || 0;
        const lastScanTime = pool.last_scan_time || '';
        
        if (!scanFunction || scanFunction === 'none') {
            return { text: 'No scrub data', state: null };
        }

        let text = '';
        if (scanState === 'finished' || scanState === 'completed') {
            const date = this.formatDate(lastScanTime);
            text = date !== 'Unknown' ? `Last: ${date}` : 'Completed';
        } else if (scanState === 'scanning' || scanState === 'in_progress') {
            text = `In progress (${Math.round(scanProgress)}%)`;
        } else if (scanState === 'canceled') {
            text = 'Scrub canceled';
        } else {
            text = scanState || 'No scrub data';
        }

        return { text, state: scanState };
    },

    getDeviceStats(pool) {
        const devices = pool.devices || [];
        let errors = (pool.read_errors || 0) + (pool.write_errors || 0) + (pool.checksum_errors || 0);
        devices.forEach(d => {
            errors += (d.read_errors || 0) + (d.write_errors || 0) + (d.checksum_errors || 0);
        });
        return { total: devices.length, errors };
    },

    formatStorageSize(size) {
        if (!size) return '0';
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

    formatDuration(seconds) {
        if (!seconds) return null;
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
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

function showZFSPools() {
    Navigation.showZFS();
}