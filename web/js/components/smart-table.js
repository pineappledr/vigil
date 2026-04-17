/**
 * Vigil Dashboard - Smart Table Component
 *
 * Renders a configurable data table. Supports two modes:
 *
 * 1. SMART attributes — Classic mode with hardcoded columns and delta highlighting.
 *    Receives telemetry updates via SSE to show live attribute changes.
 *
 * 2. Source-backed tables — Columns defined as {key, label} objects in the manifest.
 *    Optionally fetches initial data from the addon's API via the proxy route.
 *    Also supports live updates via SSE telemetry targeting the component ID.
 *
 * Config schema:
 *   columns       - Array of strings (SMART mode) or {key, label, format?} objects
 *   source        - Data source identifier (e.g., "addon_agents", "agent_drives")
 *   highlight_threshold - For SMART mode delta highlighting
 *   sortable      - Enable column sorting
 *   default_sort  - { key, direction }
 *   filterable    - Enable filtering
 *   filter_keys   - Array of column keys to filter on
 */

const SmartTableComponent = {
    _tables: {},  // keyed by compId → { rows, prevRows, config, columns, isStructured }

    /**
     * @param {string} compId  - Manifest component ID
     * @param {Object} config  - Table configuration from manifest
     * @param {number} addonId - Parent add-on ID (optional, for source fetching)
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        let columns = config.columns || ['ID', 'Attribute', 'Value', 'Worst', 'Threshold', 'Raw Value'];
        const isStructured = columns.length > 0 && typeof columns[0] === 'object';

        // Auto-append an "Actions" column when row_actions are declared and the
        // manifest didn't already provide an explicit `format: "actions"` column.
        const rowActions = Array.isArray(config.row_actions) ? config.row_actions : [];
        if (isStructured && rowActions.length > 0 && !columns.some(c => c.format === 'actions')) {
            columns = [...columns, { key: '__actions', label: '', format: 'row_actions' }];
        }

        const sortState = config.default_sort
            ? { key: config.default_sort.key, dir: config.default_sort.direction || 'asc' }
            : null;

        this._tables[compId] = {
            rows: [],
            prevRows: {},
            config: config || {},
            columns,
            isStructured,
            addonId,
            sort: sortState,
            timeRange: config.time_filter ? (config.time_filter.default || '') : '',
            pageSize: config.page_size || 0,  // 0 = show all (no pagination)
            currentPage: 1,
            rowActions,
            toolbarActions: Array.isArray(config.toolbar_actions) ? config.toolbar_actions : [],
            optionsCache: {}  // keyed by options_from path
        };

        const headers = isStructured
            ? columns.map(c => {
                const isActionCol = c.format === 'actions' || c.format === 'row_actions';
                if (config.sortable && c.key && !isActionCol) {
                    const arrow = sortState && sortState.key === c.key
                        ? (sortState.dir === 'asc' ? ' &#9650;' : ' &#9660;')
                        : '';
                    return `<th class="smart-th-sortable" onclick="SmartTableComponent._toggleSort('${this._escapeJS(compId)}','${this._escapeJS(c.key)}')">${Utils.escapeHtml(c.label || c.key)}${arrow}</th>`;
                }
                if (isActionCol) {
                    return `<th class="smart-col-actions-head">${Utils.escapeHtml(c.label || '')}</th>`;
                }
                return `<th>${Utils.escapeHtml(c.label || c.key)}</th>`;
            }).join('')
            : columns.map(c => `<th>${Utils.escapeHtml(c)}</th>`).join('');

        const colCount = columns.length;

        // Fetch source data after DOM insertion
        if (config.source && addonId) {
            setTimeout(() => this._fetchSource(compId), 0);
        }

        const emptyText = (config.source && addonId)
            ? 'Loading data...'
            : (isStructured ? 'No data' : 'Waiting for SMART data...');

        const refreshBtn = (config.source && addonId)
            ? `<button class="smart-table-refresh" id="smart-refresh-${compId}" title="Refresh"
                       onclick="SmartTableComponent.refresh('${this._escapeJS(compId)}')">
                   <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                       <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
                       <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                   </svg>
               </button>`
            : '';

        // Time filter dropdown
        const timeFilter = config.time_filter ? this._renderTimeFilter(compId, config.time_filter) : '';

        // Page size selector (only when page_size is configured)
        const pageSizeSelector = config.page_size
            ? this._renderPageSizeSelector(compId, config.page_size)
            : '';

        const toolbarActionsHtml = this._tables[compId].toolbarActions.length > 0
            ? `<div class="smart-toolbar-actions">${this._tables[compId].toolbarActions.map((act, i) =>
                this._renderToolbarActionButton(compId, act, i)).join('')}</div>`
            : '';

        return `
            <div class="smart-table-container" id="smart-table-${compId}">
                <div class="smart-table-toolbar">
                    ${toolbarActionsHtml}
                    <div class="smart-toolbar-spacer"></div>
                    ${pageSizeSelector}
                    ${timeFilter}
                    ${refreshBtn}
                </div>
                <table class="smart-table">
                    <thead>
                        <tr>${headers}</tr>
                    </thead>
                    <tbody id="smart-tbody-${compId}">
                        <tr><td colspan="${colCount}" class="smart-table-empty">${emptyText}</td></tr>
                    </tbody>
                </table>
                <div class="smart-table-pagination" id="smart-pagination-${compId}"></div>
            </div>
        `;
    },

    /** Public refresh — re-fetches source data with a spin animation. */
    refresh(compId) {
        const btn = document.getElementById(`smart-refresh-${compId}`);
        if (btn) {
            btn.classList.add('spinning');
            // Remove spinning class after fetch completes (or after timeout)
            const cleanup = () => btn.classList.remove('spinning');
            this._fetchSource(compId).then(cleanup).catch(cleanup);
            return;
        }
        this._fetchSource(compId);
    },

    // ─── Time Filter ──────────────────────────────────────────────────────

    _renderTimeFilter(compId, filterConfig) {
        const options = filterConfig.options || [];
        const defaultVal = filterConfig.default || '';
        return `<select class="smart-time-filter" id="smart-time-filter-${compId}"
                        onchange="SmartTableComponent._onTimeFilterChange('${this._escapeJS(compId)}')">
                    ${options.map(opt =>
                        `<option value="${Utils.escapeHtml(opt.value)}"${opt.value === defaultVal ? ' selected' : ''}>${Utils.escapeHtml(opt.label)}</option>`
                    ).join('')}
                </select>`;
    },

    _onTimeFilterChange(compId) {
        const entry = this._tables[compId];
        if (!entry) return;
        const sel = document.getElementById(`smart-time-filter-${compId}`);
        if (!sel) return;
        entry.timeRange = sel.value;
        this._fetchSource(compId);
    },

    // ─── Pagination ──────────────────────────────────────────────────────

    _renderPageSizeSelector(compId, defaultSize) {
        const options = [10, 25, 50, 100];
        return `<select class="smart-time-filter" id="smart-pagesize-${compId}"
                        onchange="SmartTableComponent._onPageSizeChange('${this._escapeJS(compId)}')">
                    ${options.map(n =>
                        `<option value="${n}"${n === defaultSize ? ' selected' : ''}>${n} per page</option>`
                    ).join('')}
                    <option value="0"${!options.includes(defaultSize) && defaultSize === 0 ? ' selected' : ''}>Show all</option>
                </select>`;
    },

    _onPageSizeChange(compId) {
        const entry = this._tables[compId];
        if (!entry) return;
        const sel = document.getElementById(`smart-pagesize-${compId}`);
        if (!sel) return;
        entry.pageSize = parseInt(sel.value, 10) || 0;
        entry.currentPage = 1;
        if (entry.rows.length > 0) {
            this._updateStructuredTable(compId, entry, entry.rows);
        }
    },

    _goToPage(compId, page) {
        const entry = this._tables[compId];
        if (!entry) return;
        entry.currentPage = page;
        if (entry.rows.length > 0) {
            this._updateStructuredTable(compId, entry, entry.rows);
        }
    },

    _renderPagination(compId, totalRows, pageSize, currentPage) {
        const container = document.getElementById(`smart-pagination-${compId}`);
        if (!container) return;

        if (!pageSize || pageSize <= 0 || totalRows <= pageSize) {
            container.innerHTML = '';
            return;
        }

        const totalPages = Math.ceil(totalRows / pageSize);
        const start = (currentPage - 1) * pageSize + 1;
        const end = Math.min(currentPage * pageSize, totalRows);

        let buttons = '';

        // Previous button
        buttons += `<button class="smart-page-btn" ${currentPage <= 1 ? 'disabled' : ''}
                        onclick="SmartTableComponent._goToPage('${this._escapeJS(compId)}', ${currentPage - 1})">
                        &laquo; Prev</button>`;

        // Page numbers (show at most 5 pages around current)
        const maxVisible = 5;
        let startPage = Math.max(1, currentPage - Math.floor(maxVisible / 2));
        let endPage = Math.min(totalPages, startPage + maxVisible - 1);
        if (endPage - startPage < maxVisible - 1) {
            startPage = Math.max(1, endPage - maxVisible + 1);
        }

        if (startPage > 1) {
            buttons += `<button class="smart-page-btn" onclick="SmartTableComponent._goToPage('${this._escapeJS(compId)}', 1)">1</button>`;
            if (startPage > 2) buttons += `<span class="smart-page-ellipsis">&hellip;</span>`;
        }

        for (let i = startPage; i <= endPage; i++) {
            buttons += `<button class="smart-page-btn${i === currentPage ? ' smart-page-active' : ''}"
                            onclick="SmartTableComponent._goToPage('${this._escapeJS(compId)}', ${i})">${i}</button>`;
        }

        if (endPage < totalPages) {
            if (endPage < totalPages - 1) buttons += `<span class="smart-page-ellipsis">&hellip;</span>`;
            buttons += `<button class="smart-page-btn" onclick="SmartTableComponent._goToPage('${this._escapeJS(compId)}', ${totalPages})">${totalPages}</button>`;
        }

        // Next button
        buttons += `<button class="smart-page-btn" ${currentPage >= totalPages ? 'disabled' : ''}
                        onclick="SmartTableComponent._goToPage('${this._escapeJS(compId)}', ${currentPage + 1})">
                        Next &raquo;</button>`;

        container.innerHTML = `
            <div class="smart-pagination-info">Showing ${start}–${end} of ${totalRows}</div>
            <div class="smart-pagination-buttons">${buttons}</div>
        `;
    },

    // ─── Source Data Fetching ─────────────────────────────────────────────

    async _fetchSource(compId) {
        const entry = this._tables[compId];
        if (!entry || !entry.config.source || !entry.addonId) return;

        // Map source names to addon API paths
        const sourceMap = {
            'addon_agents': '/api/agents',
            'agent_drives': '/api/agents',   // drives come from agents
            'job_history': '/api/jobs/history',
            'smart_deltas': '/api/smart/deltas',
            'disk_status': '/api/disk_status',
            'smart_status': '/api/smart_status',
            'disk_storage': '/api/disk_storage',
            'active_job': '/api/active_job'
        };

        const rawSource = entry.config.source;
        let path = sourceMap[rawSource];
        if (!path && typeof rawSource === 'string' && rawSource.startsWith('/api/')) {
            path = rawSource;
        }
        if (!path) return;

        // Append agent_id from the page-level agent selector (if available).
        if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
            const agentId = ManifestRenderer.getSelectedAgentId();
            if (agentId) {
                const sep = path.includes('?') ? '&' : '?';
                path += `${sep}agent_id=${encodeURIComponent(agentId)}`;
            }
        }

        // Append time_range query parameter if a time filter is active.
        if (entry.timeRange) {
            const sep = path.includes('?') ? '&' : '?';
            path += `${sep}time_range=${encodeURIComponent(entry.timeRange)}`;
        }

        try {
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`);
            if (!resp.ok) {
                let msg = `Failed to load data (HTTP ${resp.status})`;
                try {
                    const err = await resp.json();
                    if (err?.error) msg = err.error;
                } catch {}
                this._showTableError(compId, entry, msg);
                return;
            }

            let data = await resp.json();

            // Transform source data based on source type
            if (entry.config.source === 'smart_deltas' && data && !Array.isArray(data)) {
                // Convert enriched deltas map to rows:
                // { "5": { name, baseline, current }, ... } → [{ id, name, baseline, current, delta }, ...]
                data = Object.entries(data).map(([id, d]) => ({
                    id,
                    name: d.name || '',
                    baseline: d.baseline ?? 0,
                    current: d.current ?? 0,
                    delta: (d.current ?? 0) - (d.baseline ?? 0)
                }));
            } else if (entry.config.source === 'agent_drives' && Array.isArray(data)) {
                // Flatten drives from all agents
                const drives = [];
                for (const agent of data) {
                    if (agent.drives && Array.isArray(agent.drives)) {
                        for (const drive of agent.drives) {
                            drives.push({ agent_id: agent.agent_id, ...drive });
                        }
                    }
                }
                data = drives;
            } else if (entry.config.source === 'addon_agents' && Array.isArray(data)) {
                // Add drive_count to each agent
                data = data.map(a => ({
                    ...a,
                    drive_count: Array.isArray(a.drives) ? a.drives.length : 0
                }));
            }

            if (Array.isArray(data)) {
                this._updateStructuredTable(compId, entry, data);
            }
        } catch (e) {
            console.error(`[SmartTable] Failed to fetch source for ${compId}:`, e);
            const msg = e instanceof SyntaxError
                ? 'Invalid response from add-on (not JSON)'
                : 'Could not reach add-on — check that the add-on URL is reachable from the Vigil server';
            this._showTableError(compId, entry, msg);
        }
    },

    _showTableError(compId, entry, message) {
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;
        tbody.innerHTML = `<tr><td colspan="${entry.columns.length}" class="smart-table-empty smart-table-error">${Utils.escapeHtml(message)}</td></tr>`;
    },

    // ─── Telemetry Updates ───────────────────────────────────────────────

    /**
     * Handle an incoming SSE telemetry event with SMART data.
     * @param {Object|Array} payload
     */
    handleUpdate(payload) {
        let attributes;
        let targetComp = null;

        if (Array.isArray(payload)) {
            attributes = payload;
        } else if (payload.attributes && Array.isArray(payload.attributes)) {
            attributes = payload.attributes;
            targetComp = payload.component_id || null;
        } else if (payload.component_id && payload.rows) {
            // Structured table update: { component_id, rows: [...] }
            const entry = this._tables[payload.component_id];
            if (entry && entry.isStructured) {
                this._updateStructuredTable(payload.component_id, entry, payload.rows);
            }
            return;
        } else {
            return;
        }

        // Route to specific component or broadcast to all SMART tables
        for (const [compId, entry] of Object.entries(this._tables)) {
            if (entry.isStructured) continue; // Skip structured tables for SMART data
            if (targetComp && targetComp !== compId) continue;
            this._updateSmartTable(compId, entry, attributes);
        }
    },

    /**
     * Direct update for a specific component ID.
     * @param {string} compId
     * @param {Array} data - Array of row objects or SMART attributes
     */
    update(compId, data) {
        const entry = this._tables[compId];
        if (!entry) return;

        if (entry.isStructured) {
            this._updateStructuredTable(compId, entry, data);
        } else {
            this._updateSmartTable(compId, entry, data);
        }
    },

    // ─── Structured Table Rendering ─────────────────────────────────────

    _updateStructuredTable(compId, entry, rows) {
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;

        if (!rows || rows.length === 0) {
            tbody.innerHTML = `<tr><td colspan="${entry.columns.length}" class="smart-table-empty">No data yet — agent is collecting, try refreshing in a moment</td></tr>`;
            this._renderPagination(compId, 0, entry.pageSize, 1);
            return;
        }

        entry.rows = rows;
        const sorted = this._sortRows(rows, entry.sort);

        // Apply pagination if configured.
        let displayRows = sorted;
        if (entry.pageSize > 0) {
            const totalPages = Math.ceil(sorted.length / entry.pageSize);
            if (entry.currentPage > totalPages) entry.currentPage = totalPages;
            if (entry.currentPage < 1) entry.currentPage = 1;
            const start = (entry.currentPage - 1) * entry.pageSize;
            displayRows = sorted.slice(start, start + entry.pageSize);
        }

        const isJobHistory = entry.config.source === 'job_history';
        // Tag each row with the table id + its index in the displayed slice so
        // row-action buttons can recover the row context on click.
        entry.displayedRows = displayRows;
        tbody.innerHTML = displayRows.map((row, idx) => {
            row.__compId = compId;
            row.__rowIdx = idx;
            const clickAttr = isJobHistory && row.id
                ? ` class="smart-row-clickable" onclick="Modals.showJobDetail(${entry.addonId}, ${row.id})"`
                : '';
            return `<tr${clickAttr}>${entry.columns.map(col => {
                const val = row[col.key];
                return `<td>${this._formatValue(val, col.format, row, col)}</td>`;
            }).join('')}</tr>`;
        }).join('');

        // Render pagination controls.
        this._renderPagination(compId, sorted.length, entry.pageSize, entry.currentPage);
    },

    _formatValue(val, format, row, col) {
        if (format === 'status_dot') {
            return this._formatStatusDot(val);
        }
        if (format === 'actions') {
            return this._formatActions(row, col);
        }
        if (format === 'row_actions') {
            // Locate the table this row belongs to via `__compId` we tag in
            // _updateStructuredTable below.
            return this._renderRowActionButtons(row.__compId, row);
        }
        if (format === 'badge') {
            return this._formatBadge(val, col);
        }

        if (format === 'warning_badge') {
            return this._formatWarningBadge(val);
        }

        if (val === undefined || val === null) return '';

        switch (format) {
            case 'bytes':
                return this._formatBytes(val);
            case 'percent':
                return typeof val === 'number' ? val.toFixed(1) + '%' : Utils.escapeHtml(String(val));
            case 'duration':
                return this._formatDuration(val);
            case 'datetime':
                return this._formatDatetime(val);
            case 'relative_time':
                return this._formatRelativeTime(val);
            default:
                if (Array.isArray(val)) return Utils.escapeHtml(val.join(', '));
                return Utils.escapeHtml(String(val));
        }
    },

    _formatStatusDot(status) {
        const colorMap = {
            online:  { color: 'var(--success, #10b981)', label: 'Online' },
            offline: { color: 'var(--danger, #ef4444)',  label: 'Offline' },
            busy:    { color: 'var(--warning, #f59e0b)', label: 'Busy' }
        };
        const info = colorMap[status] || colorMap.offline;
        return `<span class="status-dot-wrap" title="${info.label}">` +
               `<span class="status-dot" style="background:${info.color}"></span> ` +
               `<span class="status-dot-label">${info.label}</span></span>`;
    },

    _formatWarningBadge(val) {
        if (!val) return '';
        return `<span class="warning-badge" style="background:var(--danger,#ef4444);color:#fff;padding:2px 8px;border-radius:4px;font-size:0.75rem;font-weight:600">OS</span>`;
    },

    _formatActions(row, col) {
        if (!col || !col.actions) return '';
        return col.actions.map(action => {
            if (action.type === 'delete') {
                const idKey = action.id_key || 'id';
                const idVal = row[idKey] || '';
                return `<button class="btn-table-action btn-table-delete" title="${Utils.escapeHtml(action.label || 'Delete')}"
                            onclick="SmartTableComponent._handleAction('delete','${this._escapeJS(idVal)}',this)">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                                <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
                                <path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
                            </svg>
                        </button>`;
            }
            return '';
        }).join('');
    },

    async _handleAction(type, id, btnEl) {
        if (type !== 'delete') return;
        if (!confirm(`Are you sure you want to delete "${id}"?`)) return;

        // Find which table this button belongs to
        const container = btnEl.closest('.smart-table-container');
        if (!container) return;
        const compId = container.id.replace('smart-table-', '');
        const entry = this._tables[compId];
        if (!entry || !entry.addonId) return;

        btnEl.disabled = true;
        try {
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent('/api/agents/' + id)}&method=DELETE`);
            if (!resp.ok) {
                const err = await resp.json().catch(() => ({}));
                alert(err.error || `Delete failed (HTTP ${resp.status})`);
                return;
            }
            // Refresh table
            this.refresh(compId);
        } catch (e) {
            alert('Failed to delete: ' + e.message);
        } finally {
            btnEl.disabled = false;
        }
    },

    _toggleSort(compId, key) {
        const entry = this._tables[compId];
        if (!entry) return;

        if (entry.sort && entry.sort.key === key) {
            entry.sort.dir = entry.sort.dir === 'asc' ? 'desc' : 'asc';
        } else {
            entry.sort = { key, dir: 'asc' };
        }

        // Re-render headers with updated arrows
        const thead = document.querySelector(`#smart-table-${compId} thead tr`);
        if (thead) {
            thead.innerHTML = entry.columns.map(c => {
                const isActionCol = c.format === 'actions' || c.format === 'row_actions';
                if (entry.config.sortable && c.key && !isActionCol) {
                    const arrow = entry.sort && entry.sort.key === c.key
                        ? (entry.sort.dir === 'asc' ? ' &#9650;' : ' &#9660;')
                        : '';
                    return `<th class="smart-th-sortable" onclick="SmartTableComponent._toggleSort('${this._escapeJS(compId)}','${this._escapeJS(c.key)}')">${Utils.escapeHtml(c.label || c.key)}${arrow}</th>`;
                }
                if (isActionCol) {
                    return `<th class="smart-col-actions-head">${Utils.escapeHtml(c.label || '')}</th>`;
                }
                return `<th>${Utils.escapeHtml(c.label || c.key)}</th>`;
            }).join('');
        }

        // Re-render with sorted data
        if (entry.rows.length > 0) {
            this._updateStructuredTable(compId, entry, entry.rows);
        }
    },

    _sortRows(rows, sort) {
        if (!sort) return rows;
        const sorted = [...rows];
        sorted.sort((a, b) => {
            let va = a[sort.key], vb = b[sort.key];
            if (va === undefined || va === null) va = '';
            if (vb === undefined || vb === null) vb = '';
            if (typeof va === 'string') va = va.toLowerCase();
            if (typeof vb === 'string') vb = vb.toLowerCase();
            if (va < vb) return sort.dir === 'asc' ? -1 : 1;
            if (va > vb) return sort.dir === 'asc' ? 1 : -1;
            return 0;
        });
        return sorted;
    },

    _formatBytes(bytes) {
        if (typeof bytes !== 'number' || bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
    },

    _formatDuration(val) {
        if (typeof val === 'number') {
            const h = Math.floor(val / 3600);
            const m = Math.floor((val % 3600) / 60);
            const s = Math.floor(val % 60);
            if (h > 0) return `${h}h ${m}m`;
            if (m > 0) return `${m}m ${s}s`;
            return `${s}s`;
        }
        return String(val);
    },

    _formatDatetime(val) {
        try {
            const d = new Date(val);
            return d.toLocaleString();
        } catch {
            return String(val);
        }
    },

    _formatRelativeTime(val) {
        try {
            const d = new Date(val);
            const now = new Date();
            const diffMs = now - d;
            const diffS = Math.floor(diffMs / 1000);
            if (diffS < 60) return 'just now';
            if (diffS < 3600) return `${Math.floor(diffS / 60)}m ago`;
            if (diffS < 86400) return `${Math.floor(diffS / 3600)}h ago`;
            return `${Math.floor(diffS / 86400)}d ago`;
        } catch {
            return String(val);
        }
    },

    // ─── SMART Table Rendering (Classic Mode) ───────────────────────────

    _updateSmartTable(compId, entry, attributes) {
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;

        const threshold = entry.config.highlight_threshold || 0;

        tbody.innerHTML = attributes.map(attr => {
            const key = attr.id ?? attr.name;
            const prevRaw = entry.prevRows[key];
            const currentRaw = this._rawValue(attr);
            const delta = prevRaw !== undefined ? currentRaw - prevRaw : 0;
            const deltaClass = this._deltaClass(delta, threshold);
            const deltaText = delta !== 0
                ? ` <span class="${deltaClass}">(${delta > 0 ? '+' : ''}${delta})</span>`
                : '';

            return `<tr class="${this._rowClass(attr)}">
                <td class="smart-col-id">${attr.id ?? ''}</td>
                <td class="smart-col-name">${Utils.escapeHtml(attr.name || attr.attribute || '')}</td>
                <td class="smart-col-value">${attr.value ?? ''}</td>
                <td class="smart-col-worst">${attr.worst ?? ''}</td>
                <td class="smart-col-thresh">${attr.threshold ?? attr.thresh ?? ''}</td>
                <td class="smart-col-raw">${currentRaw}${deltaText}</td>
            </tr>`;
        }).join('');

        // Snapshot for next delta
        entry.prevRows = {};
        for (const attr of attributes) {
            entry.prevRows[attr.id ?? attr.name] = this._rawValue(attr);
        }
        entry.rows = attributes;
    },

    _rawValue(attr) {
        return attr.raw_value ?? attr.rawValue ?? attr.raw ?? 0;
    },

    _rowClass(attr) {
        if (attr.failing_now || attr.failingNow) return 'smart-row-critical';
        const val = attr.value;
        const thresh = attr.threshold ?? attr.thresh;
        if (val !== undefined && thresh !== undefined && val <= thresh && thresh > 0) {
            return 'smart-row-warning';
        }
        return '';
    },

    _deltaClass(delta, threshold) {
        if (delta === 0) return '';
        if (threshold > 0 && Math.abs(delta) >= threshold) return 'smart-delta-high';
        return delta > 0 ? 'smart-delta-up' : 'smart-delta-down';
    },

    _escapeJS(str) {
        if (!str) return '';
        return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    },

    // ─── Manifest-driven Actions ─────────────────────────────────────────
    //
    // The frontend renders action buttons declared in the manifest as
    // `toolbar_actions` (above the table) and `row_actions` (per row). Each
    // action either opens a `form` modal (multi-field input) or a `confirm`
    // modal (single yes/no with optional type-to-confirm), then POSTs to the
    // addon API through the proxy route.

    _renderToolbarActionButton(compId, action, idx) {
        const tier = action.safety_tier || 'green';
        const variant = tier === 'red' ? 'danger' : (tier === 'yellow' ? 'warning' : 'primary');
        const label = Utils.escapeHtml(action.label || action.id || 'Action');
        const icon = this._iconSvg(action.icon);
        return `<button class="smart-action-btn smart-action-${variant}"
                    title="${label}"
                    onclick="SmartTableComponent._invokeToolbarAction('${this._escapeJS(compId)}', ${idx})">
                    ${icon}<span>${label}</span>
                </button>`;
    },

    _renderRowActionButtons(compId, row) {
        const entry = this._tables[compId];
        if (!entry || !entry.rowActions || entry.rowActions.length === 0) return '';
        const rowIdx = row.__rowIdx;
        return `<div class="smart-row-actions">${entry.rowActions.map((act, i) => {
            const tier = act.safety_tier || 'green';
            const variant = tier === 'red' ? 'danger' : (tier === 'yellow' ? 'warning' : 'primary');
            const label = Utils.escapeHtml(act.label || act.id || 'Action');
            const icon = this._iconSvg(act.icon);
            return `<button class="smart-row-action-btn smart-row-action-${variant}"
                        title="${label}"
                        onclick="event.stopPropagation(); SmartTableComponent._invokeRowAction('${this._escapeJS(compId)}', ${i}, ${rowIdx})">
                        ${icon}<span>${label}</span>
                    </button>`;
        }).join('')}</div>`;
    },

    _invokeToolbarAction(compId, actionIdx) {
        const entry = this._tables[compId];
        if (!entry) return;
        const action = entry.toolbarActions[actionIdx];
        if (!action) return;
        this._openActionModal(compId, action, null);
    },

    _invokeRowAction(compId, actionIdx, rowIdx) {
        const entry = this._tables[compId];
        if (!entry) return;
        const action = entry.rowActions[actionIdx];
        const row = entry.displayedRows ? entry.displayedRows[rowIdx] : null;
        if (!action || !row) return;
        this._openActionModal(compId, action, row);
    },

    // ─── Modal ───────────────────────────────────────────────────────────

    _openActionModal(compId, action, row) {
        const entry = this._tables[compId];
        if (!entry) return;

        // Two shapes: `form` (multi-field) or `confirm` (single dialog,
        // possibly with extra_fields and type-to-confirm). We unify them so
        // the rest of the pipeline only sees one structure.
        const cfg = this._normalizeActionConfig(action);
        const title = this._interpolate(cfg.title || action.label || 'Action', row);
        const message = cfg.message ? this._interpolate(cfg.message, row) : '';
        const tier = action.safety_tier || 'green';

        const fieldsHtml = cfg.fields
            .filter(f => f.type !== 'hidden')
            .map(f => this._renderField(f, row))
            .join('');

        const previewHtml = action.action && action.action.preview
            ? `<div class="smart-action-preview" id="smart-action-preview"><div class="smart-action-preview-label">Command preview</div><pre class="smart-action-preview-cmd">Loading…</pre></div>`
            : '';

        const tierBadge = tier !== 'green'
            ? `<span class="smart-tier-badge smart-tier-${tier}">${tier.toUpperCase()} TIER</span>`
            : '';

        const submitVariant = cfg.button_variant || (tier === 'red' ? 'danger' : 'primary');
        const submitLabel = Utils.escapeHtml(cfg.button_label || action.label || 'Submit');

        const modal = Modals.create(`
            <div class="modal smart-action-modal">
                <div class="modal-header">
                    <h3>${Utils.escapeHtml(title)} ${tierBadge}</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                    </button>
                </div>
                <div class="modal-body">
                    ${message ? `<p class="smart-action-message">${Utils.escapeHtml(message)}</p>` : ''}
                    <form class="smart-action-form" onsubmit="return false;">${fieldsHtml}</form>
                    ${previewHtml}
                    <div class="form-error" id="smart-action-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-${submitVariant}" id="smart-action-submit">${submitLabel}</button>
                </div>
            </div>
        `);

        // Stash everything the submit handler needs on the DOM element.
        modal._smartAction = { compId, action, row, cfg };

        // Wire submit + initial preview + initial options_from loads.
        const submitBtn = modal.querySelector('#smart-action-submit');
        submitBtn.onclick = () => this._submitActionFromModal(modal);

        // Keep submit gated until type-to-confirm passes (if required).
        if (cfg.require_type_confirm) {
            this._wireTypeConfirm(modal);
        }

        // Hidden fields with value_from go into the form values directly when
        // collected — load options and prime the preview.
        for (const field of cfg.fields) {
            if (field.options_from) {
                this._loadFieldOptions(modal, field, entry.addonId);
            }
        }

        if (action.action && action.action.preview) {
            // Debounce preview so typing doesn't hammer the agent.
            const refreshPreview = this._debounce(() => this._refreshPreview(modal), 250);
            modal.querySelectorAll('input, select, textarea').forEach(el => {
                el.addEventListener('input', refreshPreview);
                el.addEventListener('change', refreshPreview);
            });
            // First fetch.
            this._refreshPreview(modal);
        }
    },

    _normalizeActionConfig(action) {
        // `form` and `confirm` describe the same modal in different shapes.
        // Confirm dialogs may also carry `extra_fields` for inputs that aren't
        // the type-to-confirm box itself.
        if (action.form) {
            return {
                title: action.form.title,
                fields: action.form.fields || [],
                button_label: action.form.button_label,
                button_variant: action.form.button_variant,
                show_command: action.form.show_command,
                require_type_confirm: action.form.require_type_confirm || false,
                confirm_value: action.form.confirm_value,
                confirm_key: action.form.confirm_key || 'confirm',
                confirm_label: action.form.confirm_label,
                message: action.form.message
            };
        }
        if (action.confirm) {
            const fields = [...(action.confirm.extra_fields || [])];
            if (action.confirm.require_type_confirm) {
                fields.push({
                    key: action.confirm.confirm_key || 'confirm',
                    label: action.confirm.confirm_label || 'Type to confirm',
                    type: 'text',
                    placeholder: action.confirm.confirm_value || ''
                });
            }
            return {
                title: action.confirm.title || action.label,
                message: action.confirm.message || '',
                fields,
                button_label: action.confirm.button_label || 'Confirm',
                button_variant: action.confirm.button_variant,
                show_command: action.confirm.show_command,
                require_type_confirm: action.confirm.require_type_confirm || false,
                confirm_value: action.confirm.confirm_value,
                confirm_key: action.confirm.confirm_key || 'confirm',
                confirm_label: action.confirm.confirm_label
            };
        }
        // No form, no confirm — bare action. Treat as a one-tap confirm.
        return {
            title: action.label,
            message: `Run ${action.label}?`,
            fields: [],
            button_label: action.label || 'Run'
        };
    },

    // ─── Field rendering ────────────────────────────────────────────────

    _renderField(field, row) {
        const id = `smart-field-${field.key.replace(/\./g, '_')}`;
        const label = field.label
            ? `<label for="${id}" class="form-label">${Utils.escapeHtml(field.label)}${field.required ? ' <span class="form-req">*</span>' : ''}</label>`
            : '';
        const hint = field.hint
            ? `<div class="form-hint">${Utils.escapeHtml(field.hint)}</div>`
            : '';
        const tierAttr = field.safety_tier ? ` data-tier="${field.safety_tier}"` : '';

        let input = '';
        const initialVal = this._resolveValueFrom(field, row);
        const placeholder = field.placeholder ? this._interpolate(field.placeholder, row) : '';

        switch (field.type) {
            case 'text':
            case 'number':
                input = `<input type="${field.type}" id="${id}" class="form-input"
                            data-field-key="${Utils.escapeHtml(field.key)}"
                            ${initialVal != null ? `value="${Utils.escapeHtml(String(initialVal))}"` : ''}
                            ${placeholder ? `placeholder="${Utils.escapeHtml(placeholder)}"` : ''}
                            ${field.required ? 'required' : ''}>`;
                break;
            case 'select': {
                const opts = (field.options || []).map(o =>
                    `<option value="${Utils.escapeHtml(String(o.value))}"${String(o.value) === String(initialVal) ? ' selected' : ''}>${Utils.escapeHtml(o.label)}</option>`
                ).join('');
                const blank = field.required ? '' : `<option value="">— unchanged —</option>`;
                input = `<select id="${id}" class="form-input" data-field-key="${Utils.escapeHtml(field.key)}">${blank}${opts}<option disabled>Loading…</option></select>`;
                break;
            }
            case 'multi-select':
                input = `<select id="${id}" class="form-input form-multi-select" multiple
                            data-field-key="${Utils.escapeHtml(field.key)}"
                            size="6"><option disabled>Loading options…</option></select>`;
                break;
            case 'checkbox':
            case 'toggle':
                input = `<label class="form-checkbox-row"><input type="checkbox" id="${id}"
                            data-field-key="${Utils.escapeHtml(field.key)}"
                            ${initialVal === true || initialVal === 'on' || initialVal === 'true' ? 'checked' : ''}>
                            <span>${Utils.escapeHtml(field.label || '')}</span></label>`;
                // Checkbox uses its own inline label; suppress the outer one.
                return `<div class="form-group"${tierAttr}>${input}${hint}</div>`;
            case 'hidden':
                // Hidden fields are tracked separately so the form can still
                // hand their values to the body collector. Render none.
                return `<input type="hidden" id="${id}" data-field-key="${Utils.escapeHtml(field.key)}" value="${Utils.escapeHtml(String(initialVal ?? ''))}">`;
            default:
                input = `<input type="text" id="${id}" class="form-input" data-field-key="${Utils.escapeHtml(field.key)}">`;
        }

        return `<div class="form-group"${tierAttr}>${label}${input}${hint}</div>`;
    },

    _resolveValueFrom(field, row) {
        if (field.value !== undefined) return field.value;
        if (!field.value_from) return undefined;
        if (typeof field.value_from === 'string' && field.value_from.startsWith('row.')) {
            const key = field.value_from.slice(4);
            return row ? row[key] : undefined;
        }
        return undefined;
    },

    _interpolate(template, row) {
        if (!template) return '';
        return String(template).replace(/\{row\.([a-zA-Z0-9_]+)\}/g, (_, key) =>
            row && row[key] != null ? String(row[key]) : '');
    },

    // ─── options_from loader ─────────────────────────────────────────────

    async _loadFieldOptions(modalEl, field, addonId) {
        const sel = modalEl.querySelector(`[data-field-key="${field.key.replace(/"/g, '\\"')}"]`);
        if (!sel) return;
        if (!addonId) return;

        let path = field.options_from;
        if (typeof path !== 'string') return;
        if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
            const agentId = ManifestRenderer.getSelectedAgentId();
            if (agentId) {
                const sep = path.includes('?') ? '&' : '?';
                path += `${sep}agent_id=${encodeURIComponent(agentId)}`;
            }
        }

        try {
            const resp = await fetch(`/api/addons/${addonId}/proxy?path=${encodeURIComponent(path)}`);
            if (!resp.ok) {
                sel.innerHTML = `<option disabled>Failed to load (HTTP ${resp.status})</option>`;
                return;
            }
            const data = await resp.json();
            const items = Array.isArray(data) ? data : (data && data.items) || [];
            const valueKey = field.option_value || 'value';
            const labelKey = field.option_label || 'label';
            const detailKey = field.option_detail;

            if (items.length === 0) {
                sel.innerHTML = `<option disabled>No options available</option>`;
                return;
            }

            const opts = items.map(it => {
                const v = String(it[valueKey] ?? '');
                let l = String(it[labelKey] ?? v);
                if (detailKey && it[detailKey]) l += ` — ${it[detailKey]}`;
                return `<option value="${Utils.escapeHtml(v)}">${Utils.escapeHtml(l)}</option>`;
            }).join('');

            // Single-select: leading blank if not required.
            const blank = (field.type === 'select' && !field.required) ? `<option value="">— unchanged —</option>` : '';
            sel.innerHTML = blank + opts;
        } catch (e) {
            console.error(`[SmartTable] options_from ${path}:`, e);
            sel.innerHTML = `<option disabled>Could not load options</option>`;
        }
    },

    // ─── Preview ─────────────────────────────────────────────────────────

    async _refreshPreview(modalEl) {
        const ctx = modalEl._smartAction;
        if (!ctx) return;
        const previewEl = modalEl.querySelector('#smart-action-preview .smart-action-preview-cmd');
        if (!previewEl) return;
        const preview = ctx.action.action && ctx.action.action.preview;
        if (!preview) return;

        const entry = this._tables[ctx.compId];
        if (!entry || !entry.addonId) return;

        const formValues = this._collectFormValues(modalEl);
        const body = this._buildRequestBody(ctx.action, ctx.row, formValues, preview);

        let path = preview.endpoint;
        if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
            const agentId = ManifestRenderer.getSelectedAgentId();
            if (agentId) {
                const sep = path.includes('?') ? '&' : '?';
                path += `${sep}agent_id=${encodeURIComponent(agentId)}`;
            }
        }

        try {
            const method = (preview.method || ctx.action.action.method || 'POST').toUpperCase();
            const proxyURL = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                + (method !== 'GET' && method !== 'POST' ? `&method=${method}` : '');
            const init = { method: method === 'GET' ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json' } };
            if (init.method !== 'GET') init.body = JSON.stringify(body);
            const resp = await fetch(proxyURL, init);
            if (!resp.ok) {
                previewEl.textContent = `Preview unavailable (HTTP ${resp.status})`;
                return;
            }
            const data = await resp.json().catch(() => ({}));
            previewEl.textContent = data.command || data.preview || JSON.stringify(data, null, 2);
        } catch (e) {
            previewEl.textContent = `Preview unavailable: ${e.message}`;
        }
    },

    // ─── Submit ──────────────────────────────────────────────────────────

    async _submitActionFromModal(modalEl) {
        const ctx = modalEl._smartAction;
        if (!ctx) return;
        const entry = this._tables[ctx.compId];
        if (!entry || !entry.addonId) return;

        const submitBtn = modalEl.querySelector('#smart-action-submit');
        const errEl = modalEl.querySelector('#smart-action-error');
        errEl.textContent = '';

        const formValues = this._collectFormValues(modalEl);
        const body = this._buildRequestBody(ctx.action, ctx.row, formValues, ctx.action.action);

        // Type-to-confirm gate for red-tier actions.
        if (ctx.cfg.require_type_confirm) {
            const expected = this._interpolate(ctx.cfg.confirm_value || '', ctx.row);
            const got = String(formValues[ctx.cfg.confirm_key] || '');
            if (expected && got !== expected) {
                errEl.textContent = `Type "${expected}" exactly to confirm.`;
                return;
            }
        }

        let path = ctx.action.action.endpoint;
        const method = (ctx.action.action.method || 'POST').toUpperCase();

        if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
            const agentId = ManifestRenderer.getSelectedAgentId();
            if (agentId) {
                const sep = path.includes('?') ? '&' : '?';
                path += `${sep}agent_id=${encodeURIComponent(agentId)}`;
            }
        }

        submitBtn.disabled = true;
        const origLabel = submitBtn.textContent;
        submitBtn.textContent = 'Working…';
        try {
            const proxyURL = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                + (method !== 'GET' && method !== 'POST' ? `&method=${method}` : '');
            const init = { method: method === 'GET' ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json' } };
            if (init.method !== 'GET') init.body = JSON.stringify(body);
            const resp = await fetch(proxyURL, init);
            const data = await resp.json().catch(() => ({}));
            if (!resp.ok) {
                errEl.textContent = (data && (data.error || data.message)) || `Request failed (HTTP ${resp.status})`;
                if (data && data.command) {
                    errEl.textContent += `\nCommand: ${data.command}`;
                }
                return;
            }
            Modals.close(submitBtn);
            this.refresh(ctx.compId);
        } catch (e) {
            errEl.textContent = `Request failed: ${e.message}`;
        } finally {
            submitBtn.disabled = false;
            submitBtn.textContent = origLabel;
        }
    },

    _collectFormValues(modalEl) {
        const out = {};
        modalEl.querySelectorAll('[data-field-key]').forEach(el => {
            const key = el.dataset.fieldKey;
            if (!key) return;
            let value;
            if (el.type === 'checkbox') {
                value = el.checked;
            } else if (el.tagName === 'SELECT' && el.multiple) {
                value = Array.from(el.selectedOptions).map(o => o.value).filter(v => v !== '');
            } else if (el.type === 'number') {
                const n = el.value === '' ? null : Number(el.value);
                value = Number.isFinite(n) ? n : null;
            } else {
                value = el.value;
            }
            this._setNestedValue(out, key, value);
        });
        return out;
    },

    _setNestedValue(obj, dottedKey, value) {
        const parts = dottedKey.split('.');
        let cur = obj;
        for (let i = 0; i < parts.length - 1; i++) {
            if (cur[parts[i]] == null || typeof cur[parts[i]] !== 'object') cur[parts[i]] = {};
            cur = cur[parts[i]];
        }
        cur[parts[parts.length - 1]] = value;
    },

    _buildRequestBody(action, row, formValues, actionOrPreview) {
        // Compose: static body fields + body_map (row→body) + form values.
        // Form values take precedence over body_map for the same key.
        const body = {};
        if (actionOrPreview && actionOrPreview.body && typeof actionOrPreview.body === 'object') {
            Object.assign(body, actionOrPreview.body);
        }
        if (actionOrPreview && actionOrPreview.body_map && typeof actionOrPreview.body_map === 'object') {
            for (const [bodyKey, src] of Object.entries(actionOrPreview.body_map)) {
                if (typeof src === 'string' && src.startsWith('row.') && row) {
                    const v = row[src.slice(4)];
                    if (v !== undefined) this._setNestedValue(body, bodyKey, v);
                }
            }
        }
        // Strip empty-string optional values so the backend gets {} rather than
        // {"new_name": ""} which would be interpreted as "rename to empty".
        for (const [k, v] of Object.entries(formValues)) {
            if (v === '' || v === null) continue;
            if (Array.isArray(v) && v.length === 0) continue;
            this._setNestedValue(body, k, v);
        }
        return body;
    },

    _wireTypeConfirm(modalEl) {
        const ctx = modalEl._smartAction;
        if (!ctx) return;
        const expected = this._interpolate(ctx.cfg.confirm_value || '', ctx.row);
        if (!expected) return;
        const key = ctx.cfg.confirm_key || 'confirm';
        const sel = modalEl.querySelector(`[data-field-key="${key.replace(/"/g, '\\"')}"]`);
        const submit = modalEl.querySelector('#smart-action-submit');
        if (!sel || !submit) return;
        const update = () => {
            submit.disabled = String(sel.value) !== expected;
        };
        sel.addEventListener('input', update);
        update();
    },

    // ─── Helpers ─────────────────────────────────────────────────────────

    _formatBadge(val, col) {
        if (val == null || val === '') return '';
        const map = (col && (col.badge_map || col.badge)) || {};
        const variant = map[val] || 'muted';
        return `<span class="smart-badge smart-badge-${variant}">${Utils.escapeHtml(String(val))}</span>`;
    },

    _iconSvg(name) {
        // Tiny inline icon registry for the icon names the manifest uses. Any
        // name not listed falls back to a small dot. We don't pull in a full
        // icon library — the dashboard only needs a handful.
        const ICONS = {
            'edit':         '<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>',
            'plus-square':  '<rect x="3" y="3" width="18" height="18" rx="2"/><line x1="12" y1="8" x2="12" y2="16"/><line x1="8" y1="12" x2="16" y2="12"/>',
            'log-in':       '<path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><polyline points="10 17 15 12 10 7"/><line x1="15" y1="12" x2="3" y2="12"/>',
            'log-out':      '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/>',
            'trash-2':      '<polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>',
            'refresh-cw':   '<polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>',
            'rotate-ccw':   '<polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/>',
            'pause':        '<rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/>',
            'x-circle':     '<circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>',
            'camera':       '<path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/>',
            'eraser':       '<path d="M20 20H7L3 16a2 2 0 0 1 0-3l9-9a2 2 0 0 1 3 0l6 6a2 2 0 0 1 0 3l-7 7"/>',
            'play':         '<polygon points="5 3 19 12 5 21 5 3"/>'
        };
        const path = name && ICONS[name] ? ICONS[name] : '<circle cx="12" cy="12" r="3"/>';
        return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14" stroke-linecap="round" stroke-linejoin="round">${path}</svg>`;
    },

    _debounce(fn, ms) {
        let timer = null;
        return function (...args) {
            clearTimeout(timer);
            timer = setTimeout(() => fn.apply(this, args), ms);
        };
    }
};
