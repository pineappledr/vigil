/**
 * Vigil Dashboard - ZFS Pools Module
 * Handles ZFS pool visualization, status cards, and detail modals
 */

const ZFS = {
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

    renderPoolCard(pool, hostname) {
        const poolName = pool.name || pool.pool_name || 'Unknown Pool';
        const state = (pool.status || pool.state || pool.health || 'UNKNOWN').toUpperCase();
        const stateClass = this.getStateClass(state);
        const capacity = this.parseCapacity(pool);
        const scrub = this.parseScrub(pool);
        
        // Use device_count from API if available, otherwise calculate from devices
        let deviceCount = pool.device_count;
        if (deviceCount === undefined || deviceCount === 0) {
            // Calculate from devices array if available
            const devices = pool.devices || [];
            const uniqueSerials = new Set();
            devices.forEach(d => {
                if (d.vdev_type === 'disk' && d.serial_number) {
                    uniqueSerials.add(d.serial_number);
                }
            });
            deviceCount = uniqueSerials.size || devices.filter(d => d.vdev_type === 'disk').length;
        }
        
        let errors = (pool.read_errors || 0) + (pool.write_errors || 0) + (pool.checksum_errors || 0);

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
                        ${deviceCount} device${deviceCount !== 1 ? 's' : ''}
                        ${errors > 0 
                            ? `<span class="zfs-error-count">${errors} error${errors !== 1 ? 's' : ''}</span>` 
                            : '<span class="zfs-no-errors">0 errors</span>'
                        }
                    </span>
                </div>
            </div>
        `;
    },

    renderEmptyState() {
        return `
            <div class="empty-state zfs-empty">
                ${this.icons.pool}
                <p>No ZFS pools detected</p>
                <span class="hint">ZFS pools will appear here when agents report them.</span>
            </div>
        `;
    },

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
        
        // Deduplicate devices
        const { vdevs, uniqueDisks } = this.deduplicateDevices(devices);
        const diskCount = uniqueDisks.length;
        const topology = this.calculateTopology(vdevs, uniqueDisks, capacity);

        console.log('Devices from API:', devices.length);
        console.log('After dedup - vdevs:', vdevs.length, 'disks:', uniqueDisks.length);

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
                ${this.renderDevicesTab(vdevs, uniqueDisks, hostname)}
            </div>

            <div id="zfs-tab-scrubs" class="zfs-tab-content">
                ${this.renderScrubsTab(scrubHistory)}
            </div>
        `;
    },

    /**
     * Deduplicate devices - KEY FUNCTION
     * Problem: Database has both GUID entries AND device name entries with same serial
     * Solution: Keep only one entry per serial number, preferring readable names
     */
    deduplicateDevices(devices) {
        const vdevs = [];
        const diskMap = new Map(); // serial -> best disk entry
        const noSerialDisks = [];
        
        console.log('=== Deduplication Start ===');
        console.log('Total devices:', devices.length);
        
        devices.forEach((dev, idx) => {
            const vdevType = (dev.vdev_type || 'disk').toLowerCase();
            const name = dev.device_name || dev.name || '';
            const serial = dev.serial_number || '';
            
            console.log(`Device ${idx}: name="${name}", type="${vdevType}", serial="${serial}"`);
            
            // Separate vdevs (mirror, raidz) from disks
            if (vdevType !== 'disk') {
                vdevs.push(dev);
                console.log(`  -> Added to vdevs`);
                return;
            }
            
            // Handle disks
            if (!serial) {
                // No serial - add to noSerial list
                noSerialDisks.push(dev);
                console.log(`  -> No serial, added to noSerial list`);
                return;
            }
            
            // Has serial - check for duplicates
            if (!diskMap.has(serial)) {
                diskMap.set(serial, dev);
                console.log(`  -> First with serial ${serial}, added`);
            } else {
                // Duplicate serial - keep the one with better name
                const existing = diskMap.get(serial);
                const existingName = existing.device_name || existing.name || '';
                
                const existingIsGuid = this.looksLikeGUID(existingName);
                const currentIsGuid = this.looksLikeGUID(name);
                
                console.log(`  -> Duplicate serial. Existing="${existingName}" (GUID:${existingIsGuid}), Current="${name}" (GUID:${currentIsGuid})`);
                
                if (existingIsGuid && !currentIsGuid) {
                    // Current has better name
                    diskMap.set(serial, dev);
                    console.log(`  -> Replaced with current (better name)`);
                } else if (!existingIsGuid && currentIsGuid) {
                    // Existing has better name, keep it
                    console.log(`  -> Keeping existing (better name)`);
                } else if (name.length < existingName.length) {
                    // Prefer shorter name
                    diskMap.set(serial, dev);
                    console.log(`  -> Replaced with current (shorter name)`);
                } else {
                    console.log(`  -> Keeping existing`);
                }
            }
        });
        
        // Combine unique disks
        const uniqueDisks = Array.from(diskMap.values());
        
        // Add no-serial disks (but avoid duplicates by name)
        const seenNames = new Set(uniqueDisks.map(d => d.device_name || d.name));
        noSerialDisks.forEach(disk => {
            const name = disk.device_name || disk.name;
            if (!seenNames.has(name)) {
                uniqueDisks.push(disk);
                seenNames.add(name);
            }
        });
        
        console.log('=== Deduplication End ===');
        console.log('Vdevs:', vdevs.length);
        console.log('Unique disks:', uniqueDisks.length);
        
        return { vdevs, uniqueDisks };
    },

    looksLikeGUID(name) {
        if (!name) return false;
        // GUIDs: contain dashes and are 32+ chars (like 79b85ece-9d17-4299-8dc9-97ba419af8ae)
        if (name.length >= 20 && name.includes('-')) {
            // Count hex chars
            const hexChars = (name.match(/[0-9a-fA-F]/g) || []).length;
            return hexChars >= 20;
        }
        return false;
    },

    renderOverviewTab(pool, capacity, state, topology, lastScrub) {
        const poolName = pool.name || pool.pool_name || 'Unknown';
        
        return `
            <div class="zfs-overview-section">
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Pool Name</span>
                    <span class="zfs-detail-value">${poolName}</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Status</span>
                    <span class="zfs-detail-value">
                        <span class="zfs-state-badge ${this.getStateClass(state)}">${state}</span>
                    </span>
                </div>
            </div>

            <div class="zfs-overview-section">
                <h4>Data Topology</h4>
                <div class="topology-display">${topology.description}</div>
            </div>

            <div class="zfs-overview-section">
                <h4>Capacity</h4>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Usable Capacity</span>
                    <span class="zfs-detail-value">${capacity.total}</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Used</span>
                    <span class="zfs-detail-value">${capacity.used} (${capacity.percent}%)</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Free Space</span>
                    <span class="zfs-detail-value">${capacity.free}</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Fragmentation</span>
                    <span class="zfs-detail-value">${pool.fragmentation || 0}%</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Dedup Ratio</span>
                    <span class="zfs-detail-value">${(pool.dedup_ratio || 1).toFixed(2)}x</span>
                </div>
            </div>
            
            <div class="zfs-overview-section">
                <h4>Last Scan</h4>
                ${lastScrub ? `
                    <div class="zfs-detail-row">
                        <span class="zfs-detail-label">Last Scan Date</span>
                        <span class="zfs-detail-value">${this.formatScrubDate(lastScrub.start_time)}</span>
                    </div>
                    <div class="zfs-detail-row">
                        <span class="zfs-detail-label">Last Scan Duration</span>
                        <span class="zfs-detail-value">${this.formatDurationLong(lastScrub.duration_secs)}</span>
                    </div>
                    <div class="zfs-detail-row">
                        <span class="zfs-detail-label">Last Scan Errors</span>
                        <span class="zfs-detail-value ${lastScrub.errors_found > 0 ? 'error-text' : ''}">${lastScrub.errors_found || 0}</span>
                    </div>
                ` : `
                    <div class="zfs-detail-row">
                        <span class="zfs-detail-label">Last Scan</span>
                        <span class="zfs-detail-value">No scrub history available</span>
                    </div>
                `}
            </div>

            <div class="zfs-overview-section">
                <h4>Pool Errors</h4>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Read Errors</span>
                    <span class="zfs-detail-value ${pool.read_errors > 0 ? 'error-text' : ''}">${pool.read_errors || 0}</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Write Errors</span>
                    <span class="zfs-detail-value ${pool.write_errors > 0 ? 'error-text' : ''}">${pool.write_errors || 0}</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Checksum Errors</span>
                    <span class="zfs-detail-value ${pool.checksum_errors > 0 ? 'error-text' : ''}">${pool.checksum_errors || 0}</span>
                </div>
            </div>
        `;
    },

    calculateTopology(vdevs, disks, capacity) {
        const vdevCounts = {};
        vdevs.forEach(v => {
            const type = (v.vdev_type || 'unknown').toUpperCase();
            vdevCounts[type] = (vdevCounts[type] || 0) + 1;
        });

        const width = vdevs.length > 0 ? Math.ceil(disks.length / vdevs.length) : disks.length;

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
        } else {
            description = `Unknown topology | ${capacity.total}`;
        }

        return { description, vdevCounts, width };
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

        // Render vdevs with their child disks
        vdevs.forEach(vdev => {
            const vdevName = vdev.device_name || vdev.name || 'Unknown';
            const childDisks = disksByParent[vdevName] || [];
            html += this.renderVdevGroup(vdev, childDisks, hostname);
        });

        // Render orphan disks (stripe)
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
                    <span class="zfs-vdev-errors">R:${vdev.read_errors || 0} W:${vdev.write_errors || 0} C:${vdev.checksum_errors || 0}</span>
                </div>
                <div class="zfs-disk-list">
                    ${disks.length > 0 
                        ? disks.map(disk => this.renderDiskRow(disk, hostname)).join('')
                        : '<div class="zfs-no-disks">No disk details available</div>'
                    }
                </div>
            </div>
        `;
    },

    renderDiskRow(disk, hostname) {
        const diskName = disk.device_name || disk.name || 'Unknown';
        const diskState = (disk.state || 'ONLINE').toUpperCase();
        const serial = disk.serial_number || '';
        
        // Get display name
        const displayName = this.getDisplayName(diskName);
        
        // Find drive by serial
        const driveLink = serial ? this.findDriveBySerial(hostname, serial) : null;

        return `
            <div class="zfs-disk-row ${this.getStateClass(diskState)}">
                <div class="zfs-disk-main">
                    <span class="zfs-disk-indent">└─</span>
                    <span class="zfs-disk-name">${displayName}</span>
                    <span class="zfs-disk-state ${this.getStateClass(diskState)}">${diskState}</span>
                </div>
                <div class="zfs-disk-info">
                    ${serial ? `
                        <span class="zfs-disk-serial ${driveLink ? 'clickable' : ''}" 
                              ${driveLink ? `onclick="event.stopPropagation(); ZFS.navigateToDrive(${driveLink.serverIdx}, ${driveLink.driveIdx})" title="View SMART data"` : ''}>
                            ${serial}
                            ${driveLink ? this.icons.link : ''}
                        </span>
                    ` : ''}
                    <span class="zfs-disk-errors">R:${disk.read_errors || 0} W:${disk.write_errors || 0} C:${disk.checksum_errors || 0}</span>
                </div>
            </div>
        `;
    },

    getDisplayName(name) {
        if (!name) return 'Unknown';
        if (this.looksLikeGUID(name)) {
            return name.substring(0, 8) + '...';
        }
        if (name.startsWith('/dev/')) {
            return name.replace('/dev/', '');
        }
        return name;
    },

    findDriveBySerial(hostname, serial) {
        if (!serial || !hostname) return null;
        
        for (let serverIdx = 0; serverIdx < State.data.length; serverIdx++) {
            const server = State.data[serverIdx];
            if (server.hostname !== hostname) continue;
            
            const drives = server.details?.drives || [];
            for (let driveIdx = 0; driveIdx < drives.length; driveIdx++) {
                const driveSerial = drives[driveIdx].serial_number;
                if (driveSerial && driveSerial === serial) {
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
                            <span class="zfs-scrub-date">${this.formatScrubDate(scrub.start_time)}</span>
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

    getStateClass(state) {
        const s = (state || '').toUpperCase();
        return { 'ONLINE': 'online', 'DEGRADED': 'degraded', 'FAULTED': 'faulted', 'UNAVAIL': 'faulted', 'OFFLINE': 'offline', 'REMOVED': 'offline' }[s] || 'unknown';
    },

    getCapacityClass(percent) {
        if (percent >= 90) return 'critical';
        if (percent >= 80) return 'warning';
        return 'healthy';
    },

    parseCapacity(pool) {
        const size = pool.size_bytes || pool.size || 0;
        const alloc = pool.allocated_bytes || pool.alloc || pool.allocated || 0;
        const free = pool.free_bytes || pool.free || 0;
        let percent = pool.capacity_pct || pool.capacity_percent || 0;
        if (typeof percent === 'string') percent = parseInt(percent.replace('%', ''), 10) || 0;
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
            const date = this.formatScrubDate(lastScanTime);
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

    formatStorageSize(size) {
        if (!size) return '0 B';
        if (typeof size === 'string') return size;
        const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
        let value = parseFloat(size);
        let unitIndex = 0;
        while (value >= 1024 && unitIndex < units.length - 1) {
            value /= 1024;
            unitIndex++;
        }
        return `${value.toFixed(2)} ${units[unitIndex]}`;
    },

    formatDurationLong(seconds) {
        if (!seconds || seconds <= 0) return 'Unknown';
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = Math.floor(seconds % 60);
        
        let parts = [];
        if (hours > 0) parts.push(`${hours} hour${hours !== 1 ? 's' : ''}`);
        if (minutes > 0) parts.push(`${minutes} minute${minutes !== 1 ? 's' : ''}`);
        if (secs > 0) parts.push(`${secs} second${secs !== 1 ? 's' : ''}`);
        
        return parts.length > 0 ? parts.join(' ') : '0 seconds';
    },

    formatScrubDate(dateStr) {
        if (!dateStr) return 'Unknown';
        try {
            const date = new Date(dateStr);
            if (isNaN(date.getTime())) return dateStr;
            if (date.getFullYear() < 2000) return 'Unknown';
            
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            const hours = String(date.getHours()).padStart(2, '0');
            const mins = String(date.getMinutes()).padStart(2, '0');
            const secs = String(date.getSeconds()).padStart(2, '0');
            
            return `${year}-${month}-${day} ${hours}:${mins}:${secs}`;
        } catch {
            return dateStr || 'Unknown';
        }
    }
};

function showZFSPools() {
    Navigation.showZFS();
}