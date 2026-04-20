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
        </svg>`,
        chevron: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 12 15 18 9"/>
        </svg>`
    },

    // Cache of data fetched from the global endpoints. Reset on each render()
    // so switching views always gets fresh data.
    _datasets: [],
    _devices: [],
    _scrubs: [],
    _scrubLimit: 10,

    // Collapsed state survives re-renders (auto-refresh) so the user's
    // expand/collapse choice isn't reset every polling cycle.
    // Default: Datasets collapsed, everything else open.
    _collapsedSections: {
        'zfs-section-datasets': true,
    },

    // Per-dataset collapse state. Keys are `${hostname}|${dataset_name}` so
    // a dataset of the same name on two hosts is tracked independently.
    _collapsedDatasets: new Set(),

    _isCollapsed(id) {
        return this._collapsedSections[id] === true;
    },

    _datasetKey(hostname, name) {
        return `${hostname || ''}|${name || ''}`;
    },

    async render() {
        const container = document.getElementById('zfs-view');
        if (!container) return;

        const stats = State.getZFSStats();
        const pools = (State.zfsPools || []).filter(Boolean);

        // Preserve scroll position across refreshes — without this the
        // browser loses its anchor when we swap innerHTML and jumps to top.
        const scrollY = window.scrollY;

        // First render paints the shell; subsequent renders only update the
        // parts that changed so scroll position and collapse state survive.
        let tablesHost = document.getElementById('zfs-tables-container');
        if (!tablesHost) {
            container.innerHTML = `
                ${this.renderSummaryCards(stats)}
                <div id="zfs-tables-container" class="zfs-tables-container">
                    ${pools.length === 0 ? this.renderEmptyState() : this.renderTablesLoading()}
                </div>
            `;
            tablesHost = document.getElementById('zfs-tables-container');
        } else {
            // Refresh the summary cards in place without touching the tables.
            const summary = container.querySelector('.zfs-summary');
            if (summary) summary.outerHTML = this.renderSummaryCards(stats);
        }

        if (pools.length === 0) {
            if (tablesHost) tablesHost.innerHTML = this.renderEmptyState();
            return;
        }

        // Fan out the three global fetches in parallel. Failures are non-fatal:
        // a missing dataset/device/scrub table simply renders as an empty row.
        const [dsResp, devResp, scrubResp] = await Promise.allSettled([
            API.getZFSDatasets(),
            API.getZFSAllDevices(),
            API.getZFSAllScrubs(this._scrubLimit),
        ]);

        this._datasets = await this._readJSON(dsResp);
        this._devices  = await this._readJSON(devResp);
        this._scrubs   = await this._readJSON(scrubResp);

        tablesHost = document.getElementById('zfs-tables-container');
        if (!tablesHost) return;
        tablesHost.innerHTML = [
            this.renderPoolsTable(pools),
            this.renderDatasetsTable(this._datasets),
            this.renderDevicesTable(this._devices),
            this.renderScrubsTable(this._scrubs),
        ].join('');

        // Restore scroll — even the in-place summary replacement can shift
        // layout by a pixel or two depending on stat counts.
        if (window.scrollY !== scrollY) window.scrollTo(0, scrollY);
    },

    async _readJSON(settled) {
        if (settled.status !== 'fulfilled') return [];
        const resp = settled.value;
        if (!resp || !resp.ok) return [];
        try {
            const data = await resp.json();
            return Array.isArray(data) ? data : [];
        } catch {
            return [];
        }
    },

    renderTablesLoading() {
        return `<div class="loading-spinner"><div class="spinner"></div>Loading ZFS tables…</div>`;
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

    // ── Grouped tables ────────────────────────────────────────────────────
    //
    // All four tables share the `.drive-table-*` CSS from the Servers page so
    // the look matches exactly. Each table renders nothing when its data set
    // is empty — keeps the page tidy for small deployments.

    _tableSection(title, icon, count, headerCells, bodyRows, tableClass = '', opts = {}) {
        const {
            sectionId = '',
            collapsible = false,
            collapsed = false,
            extraHeader = '',
        } = opts;

        const idAttr = sectionId ? ` id="${sectionId}"` : '';
        const collapsedCls = collapsible && collapsed ? ' is-collapsed' : '';
        const headerCls = collapsible ? ' drive-table-header-collapsible' : '';
        const headerClick = collapsible && sectionId
            ? ` onclick="ZFS.toggleSection('${sectionId}')"`
            : '';
        const toggleBtn = collapsible ? `
            <button class="zfs-section-toggle" tabindex="-1"
                    aria-label="${collapsed ? 'Expand' : 'Collapse'}">
                ${this.icons.chevron}
            </button>` : '';

        return `
            <div class="drive-table-section${collapsedCls}"${idAttr}>
                <div class="drive-table-header${headerCls}"${headerClick}>
                    ${icon}
                    <span>${title}</span>
                    <span class="drive-table-count">${count}</span>
                    ${extraHeader}
                    ${toggleBtn}
                </div>
                <div class="drive-table-wrapper">
                    <table class="drive-table ${tableClass}">
                        <thead><tr>${headerCells}</tr></thead>
                        <tbody>${bodyRows}</tbody>
                    </table>
                </div>
            </div>
        `;
    },

    toggleSection(id) {
        const section = document.getElementById(id);
        if (!section) return;
        const collapsed = section.classList.toggle('is-collapsed');
        this._collapsedSections[id] = collapsed;
        const btn = section.querySelector('.zfs-section-toggle');
        if (btn) btn.setAttribute('aria-label', collapsed ? 'Expand' : 'Collapse');
    },

    async changeScrubLimit(raw) {
        const limit = Math.max(1, parseInt(raw, 10) || 10);
        this._scrubLimit = limit;

        const section = document.getElementById('zfs-section-scrubs');
        if (section) section.classList.add('is-loading');

        const resp = await API.getZFSAllScrubs(limit);
        const data = resp && resp.ok ? await resp.json().catch(() => []) : [];
        this._scrubs = Array.isArray(data) ? data : [];

        if (section) {
            section.outerHTML = this.renderScrubsTable(this._scrubs);
        }
    },

    renderPoolsTable(pools) {
        if (pools.length === 0) return '';

        const headers = ['Status', 'Name', 'Host', 'Capacity', 'Used', 'Scrub', 'Disks', 'Errors', 'Frag'];
        const rows = pools.map(p => this._poolRow(p)).join('');
        return this._tableSection('Pools', this.icons.pool, pools.length,
            headers.map(h => `<th>${h}</th>`).join(''),
            rows, 'zfs-pools-table');
    },

    _poolRow(pool) {
        const poolName = pool.name || pool.pool_name || 'Unknown';
        const hostname = pool.hostname || '';
        const state = (pool.status || pool.state || pool.health || 'UNKNOWN').toUpperCase();
        const stateClass = this.getStateClass(state);
        const capacity = this.parseCapacity(pool);
        const scrub = this.parseScrub(pool);

        let deviceCount = pool.device_count;
        if (!deviceCount) {
            const { uniqueDisks } = this.deduplicateDevices(pool.devices || []);
            deviceCount = uniqueDisks.length;
        }

        const errors = (pool.read_errors || 0) + (pool.write_errors || 0) + (pool.checksum_errors || 0);
        const frag = pool.fragmentation || 0;
        const pct = capacity.percent;

        return `
            <tr class="drive-table-row ${stateClass}" onclick="ZFS.showPoolDetail('${Utils.escapeJSString(hostname)}', '${Utils.escapeJSString(poolName)}')">
                <td><span class="drive-status-dot ${stateClass}"></span></td>
                <td class="drive-table-name">${Utils.escapeHtml(poolName)}</td>
                <td class="drive-table-host">${Utils.escapeHtml(hostname)}</td>
                <td>
                    <div class="zfs-inline-bar">
                        <div class="zfs-inline-bar-fill ${this.getCapacityClass(pct)}" style="width:${Math.min(pct, 100)}%"></div>
                        <span class="zfs-inline-bar-label">${pct}%</span>
                    </div>
                </td>
                <td>${capacity.used} / ${capacity.total}</td>
                <td class="zfs-scrub-cell scrub-${scrub.staleness}">${Utils.escapeHtml(scrub.text)}</td>
                <td>${deviceCount}</td>
                <td class="${errors > 0 ? 'zfs-cell-error' : ''}">${errors}</td>
                <td>${frag}%</td>
            </tr>
        `;
    },

    renderDatasetsTable(datasets) {
        const headers = ['Name', 'Host', 'Pool', 'Used', 'Available', 'Referenced', 'Compression', 'Mountpoint'];

        if (datasets.length === 0) {
            const emptyRow = `
                <tr class="drive-table-row">
                    <td colspan="${headers.length}" class="zfs-table-empty">
                        No datasets reported yet — agents will populate this list on their next check-in.
                    </td>
                </tr>`;
            return this._tableSection('Datasets', this.icons.drive, 0,
                headers.map(h => `<th>${h}</th>`).join(''),
                emptyRow, 'zfs-datasets-table',
                { sectionId: 'zfs-section-datasets', collapsible: true, collapsed: this._isCollapsed('zfs-section-datasets') });
        }

        // Sort so that within a pool the root dataset comes first and children
        // follow depth-first alphabetically. Depth is the number of "/" in the
        // dataset name — the pool's root dataset ("Tank") is depth 0, "Tank/x"
        // is depth 1, "Tank/x/y" is depth 2.
        const sorted = [...datasets].sort((a, b) => {
            const ah = (a.hostname || '').localeCompare(b.hostname || '');
            if (ah !== 0) return ah;
            const ap = (a.pool_name || '').localeCompare(b.pool_name || '');
            if (ap !== 0) return ap;
            return (a.dataset_name || '').localeCompare(b.dataset_name || '');
        });

        // Precompute "has children" per dataset. A row has children when any
        // other dataset on the same host+pool starts with `${name}/`. Using a
        // set of qualified names keeps the check O(1) per row.
        const pathSet = new Set(sorted.map(d => `${d.hostname}|${d.pool_name}|${d.dataset_name || ''}`));
        const hasChildren = (d) => {
            const prefix = `${d.hostname}|${d.pool_name}|${d.dataset_name || ''}/`;
            for (const p of pathSet) if (p.startsWith(prefix)) return true;
            return false;
        };

        // A row is hidden when any collapsed ancestor's dataset_name is a
        // prefix of this row's dataset_name (on the same host).
        const isHidden = (d) => {
            const name = d.dataset_name || '';
            for (const k of this._collapsedDatasets) {
                const sep = k.indexOf('|');
                if (sep < 0) continue;
                const host = k.slice(0, sep);
                const ancestor = k.slice(sep + 1);
                if (host !== d.hostname) continue;
                if (name !== ancestor && name.startsWith(ancestor + '/')) return true;
            }
            return false;
        };

        let lastKey = '';
        const rows = sorted.map(d => {
            const key = `${d.hostname}|${d.pool_name}`;
            const depth = Math.max(0, (d.dataset_name || '').split('/').length - 1);
            const isPoolBoundary = key !== lastKey;
            lastKey = key;

            const dsKey = this._datasetKey(d.hostname, d.dataset_name);
            const collapsed = this._collapsedDatasets.has(dsKey);
            const hidden = isHidden(d);
            const children = hasChildren(d);

            const toggleBtn = children
                ? `<button class="zfs-node-toggle"
                        onclick="event.stopPropagation(); ZFS.toggleDataset('${Utils.escapeHtml(dsKey)}')"
                        aria-label="${collapsed ? 'Expand' : 'Collapse'}">
                        ${this.icons.chevron}
                    </button>`
                : `<span class="zfs-node-toggle-placeholder"></span>`;

            const rowCls = [
                'drive-table-row',
                collapsed ? 'is-node-collapsed' : '',
                hidden ? 'is-hidden-by-ancestor' : '',
            ].filter(Boolean).join(' ');

            const groupHeader = isPoolBoundary ? `
                <tr class="zfs-dataset-pool-row">
                    <td colspan="${headers.length}">
                        ${this.icons.pool}
                        <span class="zfs-dataset-pool-label">${Utils.escapeHtml(d.pool_name || '')}</span>
                        <span class="zfs-dataset-pool-host">${Utils.escapeHtml(d.hostname || '')}</span>
                    </td>
                </tr>` : '';

            return groupHeader + `
                <tr class="${rowCls}" data-ds-key="${Utils.escapeHtml(dsKey)}">
                    <td class="drive-table-name zfs-tree-cell" data-depth="${depth}">
                        <span class="zfs-tree-indent" style="--depth:${depth}"></span>${toggleBtn}${Utils.escapeHtml(d.dataset_name || '')}
                    </td>
                    <td class="drive-table-host">${Utils.escapeHtml(d.hostname || '')}</td>
                    <td>${Utils.escapeHtml(d.pool_name || '')}</td>
                    <td>${this.formatStorageSize(d.used_bytes)}</td>
                    <td>${this.formatStorageSize(d.available_bytes)}</td>
                    <td>${this.formatStorageSize(d.referenced_bytes)}</td>
                    <td>${(d.compress_ratio || 1).toFixed(2)}x</td>
                    <td class="zfs-mountpoint">${Utils.escapeHtml(d.mountpoint || '--')}</td>
                </tr>
            `;
        }).join('');

        return this._tableSection('Datasets', this.icons.drive, datasets.length,
            headers.map(h => `<th>${h}</th>`).join(''),
            rows, 'zfs-datasets-table',
            { sectionId: 'zfs-section-datasets', collapsible: true, collapsed: this._isCollapsed('zfs-section-datasets') });
    },

    toggleDataset(key) {
        if (!key) return;
        if (this._collapsedDatasets.has(key)) {
            this._collapsedDatasets.delete(key);
        } else {
            this._collapsedDatasets.add(key);
        }
        // Re-render just the Datasets section so the other tables and scroll
        // position stay put. renderDatasetsTable reads _collapsedDatasets so
        // the new state is reflected in the fresh HTML.
        const section = document.getElementById('zfs-section-datasets');
        if (section && this._datasets) {
            section.outerHTML = this.renderDatasetsTable(this._datasets);
        }
    },

    // Build a parent→children map keyed by parent device_name so we can emit
    // mirror/raidz vdevs with their disks nested directly underneath.
    _buildDeviceTree(devices) {
        const childrenByParent = new Map();
        for (const d of devices) {
            const p = d.vdev_parent || '';
            if (!p) continue;
            if (!childrenByParent.has(p)) childrenByParent.set(p, []);
            childrenByParent.get(p).push(d);
        }
        const ordered = [];
        for (const d of devices) {
            if (d.vdev_parent) continue;   // children emitted via their parent
            ordered.push({ dev: d, depth: 0 });
            const kids = childrenByParent.get(d.device_name) || [];
            kids.sort((a, b) => (a.vdev_index || 0) - (b.vdev_index || 0));
            for (const k of kids) ordered.push({ dev: k, depth: 1 });
        }
        return ordered;
    },

    renderDevicesTable(devices) {
        if (devices.length === 0) return '';

        // Group by host+pool so each pool's vdev layout sits together; then
        // within each group, nest disks under their mirror/raidz vdev parent.
        const groups = new Map();
        for (const d of devices) {
            const key = `${d.hostname}|${d.pool_name}`;
            if (!groups.has(key)) groups.set(key, []);
            groups.get(key).push(d);
        }

        const headers = ['State', 'Device', 'Host', 'Pool', 'Role', 'Serial', 'Size', 'Read', 'Write', 'Cksum'];
        const orderedKeys = [...groups.keys()].sort();
        let lastKey = '';
        const rows = orderedKeys.flatMap(key => {
            const group = groups.get(key);
            const tree = this._buildDeviceTree(group);
            return tree.map(({ dev, depth }) => {
                const isBoundary = key !== lastKey;
                lastKey = key;
                return (isBoundary ? `
                    <tr class="zfs-dataset-pool-row">
                        <td colspan="${headers.length}">
                            ${this.icons.pool}
                            <span class="zfs-dataset-pool-label">${Utils.escapeHtml(dev.pool_name || '')}</span>
                            <span class="zfs-dataset-pool-host">${Utils.escapeHtml(dev.hostname || '')}</span>
                        </td>
                    </tr>` : '') + this._deviceRow(dev, depth);
            });
        }).join('');

        return this._tableSection('Pool Devices', this.icons.drive, devices.length,
            headers.map(h => `<th>${h}</th>`).join(''),
            rows, 'zfs-devices-table');
    },

    _deviceRow(d, depth) {
        const state = (d.state || 'UNKNOWN').toUpperCase();
        const stateClass = this.getStateClass(state);
        const role = d.is_spare ? 'spare'
                   : d.is_cache ? 'cache'
                   : d.is_log ? 'log'
                   : (d.vdev_type || '');
        const anyErrors = (d.read_errors || 0) + (d.write_errors || 0) + (d.checksum_errors || 0) > 0;

        // Only leaf disks can drill into a drive detail card — vdev rows
        // (mirror-0 etc.) are logical groupings with no physical backing.
        const link = (d.vdev_type === 'disk' && d.serial_number)
            ? this.findDriveBySerial(d.hostname, d.serial_number)
            : null;
        const clickable = link ? ' clickable' : '';
        const onclick = link
            ? `onclick="ZFS.navigateToDrive(${link.serverIdx}, ${link.driveIdx})"`
            : '';

        return `
            <tr class="drive-table-row ${stateClass}${clickable}" ${onclick}>
                <td><span class="drive-status-dot ${stateClass}" title="${state}"></span></td>
                <td class="drive-table-name zfs-tree-cell" data-depth="${depth}">
                    <span class="zfs-tree-indent" style="--depth:${depth}"></span>${Utils.escapeHtml(d.device_name || '')}
                </td>
                <td class="drive-table-host">${Utils.escapeHtml(d.hostname || '')}</td>
                <td>${Utils.escapeHtml(d.pool_name || '')}</td>
                <td>${Utils.escapeHtml(role)}</td>
                <td class="drive-table-serial">${Utils.escapeHtml(d.serial_number || '--')}</td>
                <td>${this.formatStorageSize(d.size_bytes)}</td>
                <td class="${anyErrors && d.read_errors ? 'zfs-cell-error' : ''}">${d.read_errors || 0}</td>
                <td class="${anyErrors && d.write_errors ? 'zfs-cell-error' : ''}">${d.write_errors || 0}</td>
                <td class="${anyErrors && d.checksum_errors ? 'zfs-cell-error' : ''}">${d.checksum_errors || 0}</td>
            </tr>
        `;
    },

    renderScrubsTable(scrubs) {
        const headers = ['State', 'Pool', 'Host', 'Type', 'Started', 'Duration', 'Examined', 'Repaired', 'Errors'];
        const limitOptions = [5, 10, 25, 50, 100];
        const limitSelector = `
            <label class="zfs-limit-label" onclick="event.stopPropagation();">
                <span>Show</span>
                <select class="zfs-limit-select"
                        onchange="ZFS.changeScrubLimit(this.value)"
                        onclick="event.stopPropagation();">
                    ${limitOptions.map(n => `<option value="${n}" ${n === this._scrubLimit ? 'selected' : ''}>${n}</option>`).join('')}
                </select>
            </label>`;

        if (scrubs.length === 0) {
            const emptyRow = `
                <tr class="drive-table-row">
                    <td colspan="${headers.length}" class="zfs-table-empty">
                        No scrub history yet — records appear after the first scrub or resilver.
                    </td>
                </tr>`;
            return this._tableSection('Scrub History', this.icons.scrub, 0,
                headers.map(h => `<th>${h}</th>`).join(''),
                emptyRow, 'zfs-scrubs-table',
                { sectionId: 'zfs-section-scrubs', extraHeader: limitSelector });
        }

        const rows = scrubs.map(s => {
            const state = (s.state || 'UNKNOWN').toLowerCase();
            const stateClass = state === 'finished' || state === 'completed' ? 'online'
                            : state === 'canceled' ? 'offline'
                            : state === 'scanning' || state === 'in_progress' ? 'active'
                            : 'unknown';
            const started = this.formatScrubDate(s.start_time);
            const errors = s.errors_found || 0;

            return `
                <tr class="drive-table-row ${stateClass}">
                    <td><span class="drive-status-dot ${stateClass}" title="${Utils.escapeHtml(state)}"></span></td>
                    <td class="drive-table-name">${Utils.escapeHtml(s.pool_name || '')}</td>
                    <td class="drive-table-host">${Utils.escapeHtml(s.hostname || '')}</td>
                    <td>${Utils.escapeHtml(s.scan_type || 'scrub')}</td>
                    <td>${Utils.escapeHtml(started)}</td>
                    <td>${this.formatDurationLong(s.duration_secs)}</td>
                    <td>${this.formatStorageSize(s.data_examined)}</td>
                    <td>${this.formatStorageSize(s.bytes_repaired)}</td>
                    <td class="${errors > 0 ? 'zfs-cell-error' : ''}">${errors}</td>
                </tr>
            `;
        }).join('');

        return this._tableSection('Scrub History', this.icons.scrub, scrubs.length,
            headers.map(h => `<th>${h}</th>`).join(''),
            rows, 'zfs-scrubs-table',
            { sectionId: 'zfs-section-scrubs', extraHeader: limitSelector });
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