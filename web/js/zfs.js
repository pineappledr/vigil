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
        </svg>`,
        link: `<svg class="link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
            <polyline points="15 3 21 3 21 9"/>
            <line x1="10" y1="14" x2="21" y2="3"/>
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
                <span class="hint">ZFS pools will appear here when agents report them.</span>
            </div>
        `;
    },

    // ─── Pool Detail Modal ───────────────────────────────────────────────────
    async showPoolDetail(hostname, poolName) {
        this.showModal(`
            <div class="zfs-modal-header">
                <div class="zfs-modal-title">
                    <span class="pool-name">${poolName}</span>
                    <span class="pool-host">${hostname}</span>
                </div>
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

        this._currentHostname = hostname;

        const lastScrub = scrubHistory.length > 0 ? scrubHistory[0] : null;
        
        // Organize devices into vdevs and disks
        const { vdevs, disks } = this.organizeDevices(devices);
        const diskCount = disks.length;
        
        // Calculate topology
        const topology = this.calculateTopology(vdevs, disks, capacity);

        return `
            <div class="zfs-detail-tabs">
                <button class="zfs-tab active" onclick="ZFS.switchTab(this, 'overview')">Overview</button>
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'devices')">Devices (${diskCount})</button>
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'scrubs')">Scrub History</button>
            </div>

            <div id="zfs-tab-overview" class="zfs-tab-content active">
                ${this.renderOverviewTab(pool, capacity, state, topology, lastScrub)}
            </div>

            <div id="zfs-tab-devices" class="zfs-tab-content">
                ${this.renderDevicesTab(vdevs, disks, hostname)}
            </div>

            <div id="zfs-tab-scrubs" class="zfs-tab-content">
                ${this.renderScrubsTab(scrubHistory)}
            </div>
        `;
    },

    renderOverviewTab(pool, capacity, state, topology, lastScrub) {
        const poolName = pool.name || pool.pool_name || 'Unknown';
        
        return `
            <div class="zfs-overview-section">
                <div class="zfs-detail-grid">
                    <div class="zfs-detail-item">
                        <span class="label">Pool Name</span>
                        <span class="value">${poolName}</span>
                    </div>
                    <div class="zfs-detail-item">
                        <span class="label">Status</span>
                        <span class="value zfs-state-badge ${this.getStateClass(state)}">${state}</span>
                    </div>
                </div>
            </div>

            <div class="zfs-overview-section">
                <h4>Data Topology</h4>
                <div class="topology-display">${topology.description}</div>
            </div>

            <div class="zfs-overview-section">
                <h4>Capacity</h4>
                <div class="zfs-detail-grid">
                    <div class="zfs-detail-item">
                        <span class="label">Usable Capacity</span>
                        <span class="value">${capacity.total}</span>
                    </div>
                    <div class="zfs-detail-item">
                        <span class="label">Used</span>
                        <span class="value">${capacity.used} (${capacity.percent}%)</span>
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
                        <span class="value">${(pool.dedup_ratio || 1).toFixed(2)}x</span>
                    </div>
                </div>
            </div>
            
            <div class="zfs-overview-section">
                <h4>Last Scrub</h4>
                ${lastScrub ? `
                    <div class="zfs-detail-grid">
                        <div class="zfs-detail-item">
                            <span class="label">Date</span>
                            <span class="value">${this.formatFullDateTime(lastScrub.start_time)}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="label">Duration</span>
                            <span class="value">${this.formatDurationLong(lastScrub.duration_secs)}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="label">Data Examined</span>
                            <span class="value">${this.formatStorageSize(lastScrub.data_examined)}</span>
                        </div>
                        <div class="zfs-detail-item">
                            <span class="label">Errors Found</span>
                            <span class="value ${lastScrub.errors_found > 0 ? 'error-text' : ''}">${lastScrub.errors_found || 0}</span>
                        </div>
                    </div>
                ` : `<p class="zfs-no-data">No scrub history available</p>`}
            </div>

            <div class="zfs-overview-section">
                <h4>Pool Errors</h4>
                <div class="zfs-errors-row">
                    <div class="zfs-error-item">
                        <span class="error-count ${pool.read_errors > 0 ? 'has-errors' : ''}">${pool.read_errors || 0}</span>
                        <span class="error-label">Read</span>
                    </div>
                    <div class="zfs-error-item">
                        <span class="error-count ${pool.write_errors > 0 ? 'has-errors' : ''}">${pool.write_errors || 0}</span>
                        <span class="error-label">Write</span>
                    </div>
                    <div class="zfs-error-item">
                        <span class="error-count ${pool.checksum_errors > 0 ? 'has-errors' : ''}">${pool.checksum_errors || 0}</span>
                        <span class="error-label">Checksum</span>
                    </div>
                </div>
            </div>
        `;
    },

    organizeDevices(devices) {
        const vdevs = [];
        const disks = [];
        
        devices.forEach(dev => {
            const vdevType = (dev.vdev_type || 'disk').toLowerCase();
            if (vdevType === 'disk') {
                disks.push(dev);
            } else {
                vdevs.push(dev);
            }
        });
        
        return { vdevs, disks };
    },

    calculateTopology(vdevs, disks, capacity) {
        // Group disks by parent
        const disksByParent = {};
        disks.forEach(d => {
            const parent = d.vdev_parent || 'stripe';
            if (!disksByParent[parent]) disksByParent[parent] = [];
            disksByParent[parent].push(d);
        });

        // Count vdev types
        const vdevCounts = {};
        vdevs.forEach(v => {
            const type = (v.vdev_type || 'unknown').toUpperCase();
            vdevCounts[type] = (vdevCounts[type] || 0) + 1;
        });

        // Determine width (disks per vdev)
        let width = 0;
        Object.values(disksByParent).forEach(diskList => {
            if (diskList.length > width) width = diskList.length;
        });
        if (width === 0) width = disks.length || 1;

        // Build description
        let description = '';
        
        if (vdevCounts.MIRROR) {
            description = `${vdevCounts.MIRROR} x MIRROR | ${width} wide | ${capacity.total}`;
        } else if (vdevCounts.RAIDZ1 || vdevCounts.RAIDZ) {
            const count = vdevCounts.RAIDZ1 || vdevCounts.RAIDZ || 1;
            description = `${count} x RAIDZ1 | ${width} wide | ${capacity.total}`;
        } else if (vdevCounts.RAIDZ2) {
            description = `${vdevCounts.RAIDZ2} x RAIDZ2 | ${width} wide | ${capacity.total}`;
        } else if (vdevCounts.RAIDZ3) {
            description = `${vdevCounts.RAIDZ3} x RAIDZ3 | ${width} wide | ${capacity.total}`;
        } else if (disks.length > 0) {
            description = `${disks.length} x DISK (Stripe) | ${capacity.total}`;
        } else if (vdevs.length > 0) {
            const type = (vdevs[0].vdev_type || 'VDEV').toUpperCase();
            description = `${vdevs.length} x ${type} | ${capacity.total}`;
        } else {
            description = `Unknown topology | ${capacity.total}`;
        }

        return { description, vdevCounts, disksByParent, width };
    },

    renderDevicesTab(vdevs, disks, hostname) {
        if (disks.length === 0 && vdevs.length === 0) {
            return `<p class="zfs-no-data">No device information available</p>`;
        }

        // Group disks by their vdev parent
        const disksByParent = {};
        disks.forEach(disk => {
            const parent = disk.vdev_parent || 'root';
            if (!disksByParent[parent]) disksByParent[parent] = [];
            disksByParent[parent].push(disk);
        });

        let html = '<div class="zfs-devices-list">';

        // Render each vdev with its disks
        vdevs.forEach(vdev => {
            const vdevName = vdev.device_name || vdev.name || 'Unknown';
            const childDisks = disksByParent[vdevName] || [];
            html += this.renderVdevGroup(vdev, childDisks, hostname);
        });

        // Render orphan disks (no vdev parent) as stripe
        if (disksByParent['root'] && disksByParent['root'].length > 0) {
            html += `
                <div class="zfs-vdev-group">
                    <div class="zfs-vdev-header">
                        <span class="zfs-vdev-name">Data Disks</span>
                        <span class="zfs-vdev-type">STRIPE</span>
                    </div>
                    <div class="zfs-disk-list">
                        ${disksByParent['root'].map(disk => this.renderDiskRow(disk, hostname)).join('')}
                    </div>
                </div>
            `;
        }

        html += '</div>';
        return html;
    },

    renderVdevGroup(vdev, disks, hostname) {
        const vdevName = vdev.device_name || vdev.name || 'Unknown';
        const vdevType = (vdev.vdev_type || 'VDEV').toUpperCase();
        const vdevState = (vdev.state || 'ONLINE').toUpperCase();

        return `
            <div class="zfs-vdev-group">
                <div class="zfs-vdev-header">
                    <span class="zfs-vdev-name">${vdevName}</span>
                    <span class="zfs-vdev-type">${vdevType}</span>
                    <span class="zfs-vdev-state ${this.getStateClass(vdevState)}">${vdevState}</span>
                    <span class="zfs-vdev-errors">R: ${vdev.read_errors || 0} / W: ${vdev.write_errors || 0} / C: ${vdev.checksum_errors || 0}</span>
                </div>
                <div class="zfs-disk-list">
                    ${disks.length > 0 
                        ? disks.map(disk => this.renderDiskRow(disk, hostname)).join('')
                        : '<div class="zfs-no-disks">No disk details available - ensure agent reports child devices</div>'
                    }
                </div>
            </div>
        `;
    },

    renderDiskRow(disk, hostname) {
        const diskName = disk.device_name || disk.name || 'Unknown';
        const diskState = (disk.state || 'ONLINE').toUpperCase();
        const serial = disk.serial_number || '';
        const driveLink = serial ? this.findDriveBySerial(hostname, serial) : null;

        return `
            <div class="zfs-disk-row ${this.getStateClass(diskState)}">
                <div class="zfs-disk-main">
                    <span class="zfs-disk-indent">└─</span>
                    <span class="zfs-disk-name">${diskName}</span>
                    <span class="zfs-disk-state ${this.getStateClass(diskState)}">${diskState}</span>
                </div>
                <div class="zfs-disk-info">
                    ${serial ? `
                        <span class="zfs-disk-serial ${driveLink ? 'clickable' : ''}" 
                              ${driveLink ? `onclick="event.stopPropagation(); ZFS.navigateToDrive(${driveLink.serverIdx}, ${driveLink.driveIdx})" title="Click to view SMART data for ${serial}"` : ''}>
                            ${serial}
                            ${driveLink ? this.icons.link : ''}
                        </span>
                    ` : `<span class="zfs-disk-path">${disk.device_path || disk.path || 'No serial'}</span>`}
                    <span class="zfs-disk-errors">R:${disk.read_errors || 0} W:${disk.write_errors || 0} C:${disk.checksum_errors || 0}</span>
                </div>
            </div>
        `;
    },

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
                        <div class="zfs-scrub-main">
                            <span class="zfs-scrub-date">${this.formatFullDateTime(scrub.start_time)}</span>
                            <span class="zfs-scrub-type">${(scrub.scan_type || 'scrub').toUpperCase()}</span>
                        </div>
                        <div class="zfs-scrub-stats">
                            <span class="zfs-scrub-duration">${this.formatDurationLong(scrub.duration_secs)}</span>
                            <span class="zfs-scrub-errors ${scrub.errors_found > 0 ? 'has-errors' : ''}">
                                ${scrub.errors_found || 0} error${scrub.errors_found !== 1 ? 's' : ''}
                            </span>
                        </div>
                    </div>
                `).join('')}
            </div>
        `;
    },

    renderLoadingState() {
        return `<div class="loading-spinner"><div class="spinner"></div><span>Loading pool details...</span></div>`;
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
        const s = (state || '').toUpperCase();
        const stateMap = {
            'ONLINE': 'online',
            'DEGRADED': 'degraded',
            'FAULTED': 'faulted',
            'UNAVAIL': 'faulted',
            'OFFLINE': 'offline',
            'REMOVED': 'offline'
        };
        return stateMap[s] || 'unknown';
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
        const deviceCount = pool.device_count !== undefined ? pool.device_count : (pool.devices || []).length;
        let errors = (pool.read_errors || 0) + (pool.write_errors || 0) + (pool.checksum_errors || 0);
        (pool.devices || []).forEach(d => {
            errors += (d.read_errors || 0) + (d.write_errors || 0) + (d.checksum_errors || 0);
        });
        return { total: deviceCount, errors };
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
        return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[unitIndex]}`;
    },

    formatDurationLong(seconds) {
        if (!seconds) return 'Unknown';
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = Math.floor(seconds % 60);
        
        let parts = [];
        if (hours > 0) parts.push(`${hours} hour${hours !== 1 ? 's' : ''}`);
        if (minutes > 0) parts.push(`${minutes} minute${minutes !== 1 ? 's' : ''}`);
        if (secs > 0 && hours === 0) parts.push(`${secs} second${secs !== 1 ? 's' : ''}`);
        
        return parts.length > 0 ? parts.join(' ') : '0 seconds';
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
    },

    formatFullDateTime(dateStr) {
        if (!dateStr) return 'Unknown';
        try {
            const date = new Date(dateStr);
            if (isNaN(date.getTime())) return dateStr;
            // Format: YYYY-MM-DD HH:MM:SS
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            const hours = String(date.getHours()).padStart(2, '0');
            const mins = String(date.getMinutes()).padStart(2, '0');
            const secs = String(date.getSeconds()).padStart(2, '0');
            return `${year}-${month}-${day} ${hours}:${mins}:${secs}`;
        } catch {
            return dateStr;
        }
    }
};

function showZFSPools() {
    Navigation.showZFS();
}