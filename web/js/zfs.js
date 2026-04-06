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
        const container = document.getElementById('zfs-view');
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
                        <span>${Utils.escapeHtml(hostname)}</span>
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
        
        // Use device_count from API, or calculate from unique disks
        let deviceCount = pool.device_count;
        if (deviceCount === undefined || deviceCount === 0) {
            const devices = pool.devices || [];
            const { uniqueDisks } = this.deduplicateDevices(devices);
            deviceCount = uniqueDisks.length;
        }
        
        let errors = (pool.read_errors || 0) + (pool.write_errors || 0) + (pool.checksum_errors || 0);

        return `
            <div class="zfs-pool-card ${stateClass}" onclick="ZFS.showPoolDetail('${Utils.escapeJSString(hostname)}', '${Utils.escapeJSString(poolName)}')">
                <div class="zfs-pool-header">
                    <div class="zfs-pool-status">
                        <span class="zfs-status-dot ${stateClass}"></span>
                        <span class="zfs-pool-name">${Utils.escapeHtml(poolName)}</span>
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

                <div class="zfs-pool-stats">
                    <span class="zfs-frag-info" title="Fragmentation">Frag: ${pool.fragmentation || 0}%</span>
                    ${(pool.compress_ratio || 1) > 1.00 ? `<span class="zfs-compress-info" title="Compression Ratio">Comp: ${(pool.compress_ratio).toFixed(2)}x</span>` : ''}
                    ${(pool.dedup_ratio || 1) > 1.00 ? `<span class="zfs-dedup-info" title="Dedup Ratio">Dedup: ${(pool.dedup_ratio).toFixed(2)}x</span>` : ''}
                </div>

                ${scrub.active ? this.renderScanProgressBar(pool) : `
                <div class="zfs-pool-scrub scrub-${scrub.staleness}">
                    ${this.icons.scrub}
                    <span class="zfs-scrub-info">${scrub.text}</span>
                </div>`}

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
        const datasets = detail.datasets || [];
        const scrubHistory = detail.scrub_history || [];
        const poolName = pool.name || pool.pool_name || 'Unknown';
        const state = (pool.status || pool.state || pool.health || 'UNKNOWN').toUpperCase();
        const capacity = this.parseCapacity(pool);
        const daysSinceScrub = detail.days_since_last_scrub;

        this._currentHostname = hostname;

        const lastScrub = scrubHistory.length > 0 ? scrubHistory[0] : null;
        
        // Deduplicate devices
        const { vdevs, uniqueDisks } = this.deduplicateDevices(devices);
        const diskCount = uniqueDisks.length;
        const topology = this.calculateTopology(vdevs, uniqueDisks, capacity);

        console.log('Devices from API:', devices.length);
        console.log('After dedup - vdevs:', vdevs.length, 'disks:', diskCount);

        return `
            <div class="zfs-detail-tabs">
                <button class="zfs-tab active" onclick="ZFS.switchTab(this, 'overview')">Overview</button>
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'devices')">Devices (${diskCount})</button>
                ${datasets.length > 0 ? `<button class="zfs-tab" onclick="ZFS.switchTab(this, 'datasets')">Datasets (${datasets.length})</button>` : ''}
                <button class="zfs-tab" onclick="ZFS.switchTab(this, 'scrubs')">Scrub History</button>
            </div>

            <div id="zfs-tab-overview" class="zfs-tab-content active">
                ${this.renderOverviewTab(pool, capacity, state, topology, lastScrub, daysSinceScrub)}
            </div>

            <div id="zfs-tab-devices" class="zfs-tab-content">
                ${this.renderDevicesTab(vdevs, uniqueDisks, hostname)}
            </div>

            ${datasets.length > 0 ? `
            <div id="zfs-tab-datasets" class="zfs-tab-content">
                ${this.renderDatasetsTab(datasets)}
            </div>
            ` : ''}

            <div id="zfs-tab-scrubs" class="zfs-tab-content">
                ${this.renderScrubsTab(scrubHistory)}
            </div>
        `;
    },

    /**
     * Deduplicate devices - FIXED VERSION
     * Handles:
     * 1. GUID + device name duplicates (same vdev_parent:vdev_index)
     * 2. Partition duplicates (mmcblk0p3 appears twice = 1 disk)
     * 3. Stripe pools (no vdev_parent)
     */
    deduplicateDevices(devices) {
        const vdevs = [];
        const disksByPosition = new Map(); // "parent:index" -> best device
        const disksByBaseName = new Map(); // "mmcblk0" -> best device (for stripes)
        
        console.log('=== Deduplication Start ===');
        console.log('Total devices:', devices.length);
        
        devices.forEach((dev, idx) => {
            const vdevType = (dev.vdev_type || 'disk').toLowerCase();
            const name = dev.device_name || dev.name || '';
            const serial = dev.serial_number || '';
            const parent = dev.vdev_parent || '';
            const vdevIndex = dev.vdev_index;
            
            console.log(`Device ${idx}: name="${name}", type="${vdevType}", serial="${serial}", parent="${parent}", index=${vdevIndex}`);
            
            // Separate vdevs (mirror, raidz) from disks
            if (vdevType !== 'disk') {
                vdevs.push(dev);
                console.log(`  -> Added to vdevs`);
                return;
            }
            
            // For disks with vdev_parent (mirror/raidz), use position key
            if (parent && vdevIndex !== undefined) {
                const posKey = `${parent}:${vdevIndex}`;
                
                if (!disksByPosition.has(posKey)) {
                    disksByPosition.set(posKey, dev);
                    console.log(`  -> First at position ${posKey}, added`);
                } else {
                    // Duplicate position - keep the one with serial (or better name)
                    const existing = disksByPosition.get(posKey);
                    if (this.isBetterDevice(dev, existing)) {
                        disksByPosition.set(posKey, dev);
                        console.log(`  -> Replaced at position ${posKey}`);
                    } else {
                        console.log(`  -> Keeping existing at ${posKey}`);
                    }
                }
            } else {
                // Stripe pool (no vdev_parent) - dedupe by base device name
                const baseName = this.getBaseDeviceName(name);
                
                if (!disksByBaseName.has(baseName)) {
                    disksByBaseName.set(baseName, dev);
                    console.log(`  -> First with base name ${baseName}, added`);
                } else {
                    const existing = disksByBaseName.get(baseName);
                    if (this.isBetterDevice(dev, existing)) {
                        disksByBaseName.set(baseName, dev);
                        console.log(`  -> Replaced at base name ${baseName}`);
                    } else {
                        console.log(`  -> Keeping existing at ${baseName}`);
                    }
                }
            }
        });
        
        // Combine unique disks from both maps
        const uniqueDisks = [
            ...Array.from(disksByPosition.values()),
            ...Array.from(disksByBaseName.values())
        ];
        
        console.log('=== Deduplication End ===');
        console.log('Vdevs:', vdevs.length);
        console.log('Unique disks:', uniqueDisks.length);
        
        return { vdevs, uniqueDisks };
    },

    // Check if newDev is better than existing (has serial, or better name)
    isBetterDevice(newDev, existing) {
        const newSerial = newDev.serial_number || '';
        const existingSerial = existing.serial_number || '';
        const newName = newDev.device_name || newDev.name || '';
        const existingName = existing.device_name || existing.name || '';
        
        // Prefer device WITH serial
        if (!existingSerial && newSerial) return true;
        if (existingSerial && !newSerial) return false;
        
        // Both have serial or both don't - prefer non-GUID name
        const existingIsGuid = this.looksLikeGUID(existingName);
        const newIsGuid = this.looksLikeGUID(newName);
        
        if (existingIsGuid && !newIsGuid) return true;
        return false;
    },

    // Extract base device name: mmcblk0p3 -> mmcblk0, sda2 -> sda, nvme0n1p1 -> nvme0n1
    getBaseDeviceName(name) {
        if (!name) return 'unknown';
        
        // mmcblk0p3 -> mmcblk0
        const mmcMatch = name.match(/^(mmcblk\d+)/);
        if (mmcMatch) return mmcMatch[1];
        
        // nvme0n1p1 -> nvme0n1
        const nvmeMatch = name.match(/^(nvme\d+n\d+)/);
        if (nvmeMatch) return nvmeMatch[1];
        
        // sda2 -> sda, hdb1 -> hdb
        const sdMatch = name.match(/^([shv]d[a-z]+)/);
        if (sdMatch) return sdMatch[1];
        
        // da0p2 -> da0 (FreeBSD)
        const daMatch = name.match(/^(da\d+|ada\d+)/);
        if (daMatch) return daMatch[1];
        
        return name;
    },

    looksLikeGUID(name) {
        if (!name) return false;
        if (name.length >= 20 && name.includes('-')) {
            const hexChars = (name.match(/[0-9a-fA-F]/g) || []).length;
            return hexChars >= 20;
        }
        return false;
    },

    renderOverviewTab(pool, capacity, state, topology, lastScrub, daysSinceScrub) {
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
                    <span class="zfs-detail-label">Compression Ratio</span>
                    <span class="zfs-detail-value">${(pool.compress_ratio || 1).toFixed(2)}x</span>
                </div>
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Dedup Ratio</span>
                    <span class="zfs-detail-value">${(pool.dedup_ratio || 1).toFixed(2)}x</span>
                </div>
            </div>
            
            ${(pool.scan_state === 'scanning' || pool.scan_state === 'in_progress') ? `
            <div class="zfs-overview-section">
                <h4>Active Scan</h4>
                ${this.renderScanProgressBar(pool)}
                <div class="zfs-detail-row" style="margin-top: 8px">
                    <span class="zfs-detail-label">Type</span>
                    <span class="zfs-detail-value">${(pool.scan_function || 'scrub').charAt(0).toUpperCase() + (pool.scan_function || 'scrub').slice(1)}</span>
                </div>
                ${pool.scan_errors > 0 ? `
                <div class="zfs-detail-row">
                    <span class="zfs-detail-label">Errors Found</span>
                    <span class="zfs-detail-value error-text">${pool.scan_errors}</span>
                </div>` : ''}
            </div>
            ` : ''}

            <div class="zfs-overview-section">
                <h4>Last Scan</h4>
                ${lastScrub ? `
                    <div class="zfs-detail-row">
                        <span class="zfs-detail-label">Last Scan Date</span>
                        <span class="zfs-detail-value">${this.formatScrubDate(lastScrub.start_time)}</span>
                    </div>
                    ${daysSinceScrub !== undefined && daysSinceScrub >= 0 ? `
                    <div class="zfs-detail-row">
                        <span class="zfs-detail-label">Days Since Scrub</span>
                        <span class="zfs-detail-value scrub-staleness-${this.getScrubStaleness(daysSinceScrub)}">${daysSinceScrub} day${daysSinceScrub !== 1 ? 's' : ''}</span>
                    </div>
                    ` : ''}
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
                    <span class="zfs-vdev-errors ${(vdev.read_errors || 0) + (vdev.write_errors || 0) + (vdev.checksum_errors || 0) > 0 ? 'has-errors' : ''}">R:${vdev.read_errors || 0} W:${vdev.write_errors || 0} C:${vdev.checksum_errors || 0}</span>
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
        
        const displayName = this.getDisplayName(diskName);
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
                    <span class="zfs-disk-errors ${(disk.read_errors || 0) + (disk.write_errors || 0) + (disk.checksum_errors || 0) > 0 ? 'has-errors' : ''}">R:${disk.read_errors || 0} W:${disk.write_errors || 0} C:${disk.checksum_errors || 0}</span>
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
                            <span class="zfs-scrub-date">${this.formatScrubDate(scrub.start_time || scrub.end_time || scrub.created_at)}</span>
                            <span class="zfs-scrub-type">${(scrub.scan_type || 'scrub').charAt(0).toUpperCase() + (scrub.scan_type || 'scrub').slice(1)}</span>
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

    renderDatasetsTab(datasets) {
        if (!datasets || datasets.length === 0) {
            return `<p class="zfs-no-data">No dataset information available</p>`;
        }

        // Sort by used_bytes descending
        const sorted = [...datasets].sort((a, b) => (b.used_bytes || 0) - (a.used_bytes || 0));
        const maxUsed = sorted[0]?.used_bytes || 1;

        return `
            <div class="zfs-datasets-list">
                <div class="zfs-datasets-header">
                    <span class="zfs-ds-col-name">Dataset</span>
                    <span class="zfs-ds-col-used">Used</span>
                    <span class="zfs-ds-col-avail">Available</span>
                    <span class="zfs-ds-col-refer">Referenced</span>
                    <span class="zfs-ds-col-compress">Compress</span>
                    <span class="zfs-ds-col-mount">Mountpoint</span>
                </div>
                ${sorted.map(ds => {
                    const barPct = maxUsed > 0 ? Math.max(1, (ds.used_bytes / maxUsed) * 100) : 0;
                    const quotaPct = ds.quota_bytes > 0 ? Math.round(ds.used_bytes / ds.quota_bytes * 100) : 0;
                    const quotaClass = quotaPct >= 90 ? 'critical' : quotaPct >= 75 ? 'warning' : '';
                    return `
                    <div class="zfs-dataset-row ${quotaClass}">
                        <span class="zfs-ds-col-name" title="${ds.dataset_name}">
                            ${this.shortenDatasetName(ds.dataset_name)}
                            ${ds.quota_bytes > 0 ? `<span class="zfs-ds-quota">${quotaPct}% of quota</span>` : ''}
                        </span>
                        <span class="zfs-ds-col-used">
                            <span class="zfs-ds-bar-bg"><span class="zfs-ds-bar-fill" style="width:${barPct}%"></span></span>
                            ${this.formatStorageSize(ds.used_bytes)}
                        </span>
                        <span class="zfs-ds-col-avail">${this.formatStorageSize(ds.available_bytes)}</span>
                        <span class="zfs-ds-col-refer">${this.formatStorageSize(ds.referenced_bytes)}</span>
                        <span class="zfs-ds-col-compress">${(ds.compress_ratio || 1).toFixed(2)}x</span>
                        <span class="zfs-ds-col-mount" title="${ds.mountpoint || '-'}">${ds.mountpoint || '-'}</span>
                    </div>`;
                }).join('')}
            </div>
        `;
    },

    shortenDatasetName(name) {
        if (!name) return 'Unknown';
        // Show last two segments for readability (e.g. "pool/parent/child" → "parent/child")
        const parts = name.split('/');
        if (parts.length <= 2) return name;
        return '…/' + parts.slice(-2).join('/');
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
        const daysSince = pool.days_since_last_scrub;

        if (!scanFunction || scanFunction === 'none') {
            return { text: 'No scrub data', state: null, staleness: 'none', active: false };
        }

        let text = '';
        let staleness = 'fresh'; // green
        let active = false;
        if (scanState === 'finished' || scanState === 'completed') {
            if (daysSince !== undefined && daysSince >= 0) {
                text = daysSince === 0 ? 'Scrubbed today' : `Last scrub: ${daysSince}d ago`;
                staleness = this.getScrubStaleness(daysSince);
            } else {
                const date = this.formatScrubDate(lastScanTime);
                text = date !== 'Unknown' ? `Last: ${date}` : 'Completed';
            }
        } else if (scanState === 'scanning' || scanState === 'in_progress') {
            text = `In progress (${Math.round(scanProgress)}%)`;
            staleness = 'active';
            active = true;
        } else if (scanState === 'canceled') {
            text = 'Scrub canceled';
            staleness = 'stale';
        } else {
            text = scanState || 'No scrub data';
            staleness = 'none';
        }

        return { text, state: scanState, staleness, active };
    },

    getScrubStaleness(days) {
        if (days <= 7) return 'fresh';    // green
        if (days <= 14) return 'aging';   // yellow
        return 'overdue';                  // red
    },

    renderScanProgressBar(pool) {
        const scanFunc = pool.scan_function || 'scrub';
        const isResilver = scanFunc === 'resilver';
        const pct = Math.round(pool.scan_progress || 0);
        const speed = pool.scan_speed || 0;
        const eta = pool.scan_time_remaining || 0;
        const colorClass = isResilver ? 'resilver' : 'scrub';
        const label = isResilver ? 'Resilver' : 'Scrub';

        let etaText = '';
        if (eta > 0) {
            const h = Math.floor(eta / 3600);
            const m = Math.floor((eta % 3600) / 60);
            etaText = h > 0 ? `${h}h ${m}m left` : `${m}m left`;
        }

        let speedText = '';
        if (speed > 0) {
            speedText = this.formatStorageSize(speed) + '/s';
        }

        return `
            <div class="zfs-scan-progress scan-${colorClass}">
                <div class="zfs-scan-header">
                    <span class="zfs-scan-label">${label}: ${pct}%</span>
                    <span class="zfs-scan-eta">${[speedText, etaText].filter(Boolean).join(' · ')}</span>
                </div>
                <div class="zfs-scan-bar">
                    <div class="zfs-scan-fill scan-${colorClass}" style="width: ${Math.min(pct, 100)}%"></div>
                </div>
            </div>
        `;
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