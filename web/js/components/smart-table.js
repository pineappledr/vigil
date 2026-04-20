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

        const selectable = !!config.selectable;
        const keyField = config.key_field || 'id';
        const bulkActions = Array.isArray(config.bulk_actions) ? config.bulk_actions : [];

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
            bulkActions,
            selectable,
            keyField,
            selection: new Set(),
            optionsCache: {}  // keyed by options_from path
        };

        const selectHead = selectable
            ? `<th class="smart-col-select-head"><input type="checkbox" class="smart-select-all" onchange="SmartTableComponent._toggleSelectAll('${this._escapeJS(compId)}', this.checked)"></th>`
            : '';

        const expandHead = config.expand_key
            ? `<th class="smart-col-expand-head"></th>`
            : '';

        const headers = isStructured
            ? selectHead + expandHead + columns.map(c => {
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

        const colCount = columns.length + (config.expand_key ? 1 : 0) + (selectable ? 1 : 0);

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

        const visibleToolbarActions = this._tables[compId].toolbarActions.filter(a => !a.hidden);
        const toolbarActionsHtml = visibleToolbarActions.length > 0
            ? `<div class="smart-toolbar-actions">${this._tables[compId].toolbarActions.map((act, i) =>
                this._renderToolbarActionButton(compId, act, i)).join('')}</div>`
            : '';

        const bulkBarHtml = selectable && bulkActions.length > 0
            ? `<div class="smart-bulk-bar" id="smart-bulk-bar-${compId}" style="display:none">
                   <span class="smart-bulk-count"><span class="smart-bulk-count-num">0</span> selected</span>
                   <div class="smart-bulk-actions">${bulkActions.map((act, i) =>
                       this._renderBulkActionButton(compId, act, i)).join('')}</div>
                   <button class="smart-bulk-clear" onclick="SmartTableComponent._clearSelection('${this._escapeJS(compId)}')">Clear</button>
               </div>`
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
                ${bulkBarHtml}
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

        // Source-level fetch has no row context — falls back to the page-level
        // selector inside _appendAgentIdToPath.
        path = this._appendAgentIdToPath(path, null);

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
        const expandKey = entry.config && entry.config.expand_key;
        const totalCols = entry.columns.length + (expandKey ? 1 : 0) + (entry.selectable ? 1 : 0);
        tbody.innerHTML = `<tr><td colspan="${totalCols}" class="smart-table-empty smart-table-error">${Utils.escapeHtml(message)}</td></tr>`;
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

        const expandKey = entry.config && entry.config.expand_key;
        const totalCols = entry.columns.length + (expandKey ? 1 : 0) + (entry.selectable ? 1 : 0);

        if (!rows || rows.length === 0) {
            const msg = (entry.config && entry.config.empty_message)
                || 'No data yet — agent is collecting, try refreshing in a moment';
            const icon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="28" height="28" aria-hidden="true"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`;
            tbody.innerHTML = `<tr><td colspan="${totalCols}" class="smart-table-empty"><div class="smart-empty-state">${icon}<span>${Utils.escapeHtml(msg)}</span></div></td></tr>`;
            this._renderPagination(compId, 0, entry.pageSize, 1);
            return;
        }

        entry.rows = rows;
        const sorted = this._sortRows(rows, entry.sort);

        // Tree hierarchy (opt-in via `tree: { key, separator }` in the table
        // config). Renders TrueNAS-style parent/child nesting in the named
        // column: parents are placed above their descendants, each row is
        // tagged with a depth for CSS indentation, and rows with children get
        // a chevron that collapses the subtree.
        const treeCfg = entry.config && entry.config.tree;
        let preparedRows = sorted;
        if (treeCfg && treeCfg.key) {
            const sep = treeCfg.separator || '/';
            const hasChild = new Set();
            const names = new Set(sorted.map(r => String(r[treeCfg.key] ?? '')));
            names.forEach(n => {
                const idx = n.lastIndexOf(sep);
                if (idx > 0) {
                    const parent = n.slice(0, idx);
                    if (names.has(parent)) hasChild.add(parent);
                }
            });
            preparedRows = sorted
                .map(r => {
                    const name = String(r[treeCfg.key] ?? '');
                    const depth = name ? (name.split(sep).length - 1) : 0;
                    return { ...r, __treeName: name, __treeDepth: depth, __treeHasChildren: hasChild.has(name) };
                })
                .sort((a, b) => a.__treeName.localeCompare(b.__treeName));
            // Filter out rows whose ancestor is collapsed.
            entry.collapsedPaths = entry.collapsedPaths || new Set();
            preparedRows = preparedRows.filter(r => {
                if (!entry.collapsedPaths.size) return true;
                const parts = r.__treeName.split(sep);
                for (let i = 1; i < parts.length; i++) {
                    const anc = parts.slice(0, i).join(sep);
                    if (entry.collapsedPaths.has(anc)) return false;
                }
                return true;
            });
        }

        // Apply pagination if configured.
        let displayRows = preparedRows;
        if (entry.pageSize > 0) {
            const totalPages = Math.ceil(preparedRows.length / entry.pageSize);
            if (entry.currentPage > totalPages) entry.currentPage = totalPages;
            if (entry.currentPage < 1) entry.currentPage = 1;
            const start = (entry.currentPage - 1) * entry.pageSize;
            displayRows = preparedRows.slice(start, start + entry.pageSize);
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
            const children = expandKey ? row[expandKey] : null;
            const hasChildren = Array.isArray(children) && children.length > 0;
            const expandCell = expandKey
                ? (hasChildren
                    ? `<td class="smart-col-expand"><button type="button" class="smart-expand-toggle" onclick="SmartTableComponent._toggleExpand('${this._escapeJS(compId)}',${idx})" aria-expanded="false"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><polyline points="9 18 15 12 9 6"/></svg></button></td>`
                    : `<td class="smart-col-expand"></td>`)
                : '';
            const rowKey = entry.selectable ? String(row[entry.keyField] ?? '') : '';
            const selectCell = entry.selectable
                ? `<td class="smart-col-select"><input type="checkbox" class="smart-row-select" data-row-key="${Utils.escapeHtml(rowKey)}" ${entry.selection.has(rowKey) ? 'checked' : ''} onclick="event.stopPropagation()" onchange="SmartTableComponent._toggleRowSelection('${this._escapeJS(compId)}', this)"></td>`
                : '';
            const mainRow = `<tr data-row-idx="${idx}"${clickAttr}>${selectCell}${expandCell}${entry.columns.map(col => {
                const val = row[col.key];
                // Tree-column: inject indent + chevron before the cell value.
                if (treeCfg && col.key === treeCfg.key) {
                    const depth = row.__treeDepth || 0;
                    const isCollapsed = entry.collapsedPaths && entry.collapsedPaths.has(row.__treeName);
                    const chevron = row.__treeHasChildren
                        ? `<button type="button" class="smart-tree-toggle${isCollapsed ? ' smart-tree-collapsed' : ''}" onclick="SmartTableComponent._toggleTreeNode('${this._escapeJS(compId)}', this.dataset.path)" data-path="${Utils.escapeHtml(row.__treeName)}" aria-expanded="${isCollapsed ? 'false' : 'true'}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12"><polyline points="9 18 15 12 9 6"/></svg></button>`
                        : `<span class="smart-tree-spacer"></span>`;
                    return `<td class="smart-tree-cell" style="--tree-depth:${depth}">${chevron}<span class="smart-tree-label">${this._formatValue(val, col.format, row, col)}</span></td>`;
                }
                return `<td>${this._formatValue(val, col.format, row, col)}</td>`;
            }).join('')}</tr>`;
            const nestedRow = hasChildren
                ? `<tr class="smart-expand-row" data-expand-for="${idx}" style="display:none"><td colspan="${totalCols}">${this._renderExpandedChildren(children, entry.config)}</td></tr>`
                : '';
            return mainRow + nestedRow;
        }).join('');

        if (entry.selectable) {
            // Drop selections that no longer reference a visible row.
            const visibleKeys = new Set(rows.map(r => String(r[entry.keyField] ?? '')));
            for (const k of Array.from(entry.selection)) {
                if (!visibleKeys.has(k)) entry.selection.delete(k);
            }
            this._updateBulkBar(compId);
        }

        this._scheduleBusyPoll(compId, entry, rows);

        // Render pagination controls.
        this._renderPagination(compId, sorted.length, entry.pageSize, entry.currentPage);
    },

    // ─── Progress indicator (auto-refresh while any row is busy) ─────────
    // Schema: config.progress_indicator = { field, busy_values[], interval_seconds }
    _scheduleBusyPoll(compId, entry, rows) {
        const pi = entry.config && entry.config.progress_indicator;
        if (!pi || !pi.field || !Array.isArray(pi.busy_values)) return;

        const busyValues = pi.busy_values.map(v => String(v).toLowerCase());
        const isBusy = (raw) => {
            if (raw == null) return false;
            const s = String(raw).toLowerCase();
            return busyValues.some(bv => s === bv || s.startsWith(bv + ' ') || s.startsWith(bv + '('));
        };
        const anyBusy = rows.some(r => isBusy(r && r[pi.field]));

        if (entry._busyTimer) {
            clearTimeout(entry._busyTimer);
            entry._busyTimer = null;
        }

        if (anyBusy) {
            const interval = Math.max(2, Number(pi.interval_seconds) || 5) * 1000;
            entry._busyTimer = setTimeout(() => {
                entry._busyTimer = null;
                this._fetchSource(compId);
            }, interval);
        }

        // Decorate busy cells in the DOM so CSS can pulse them.
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;
        const colIdx = entry.columns.findIndex(c => c.key === pi.field);
        if (colIdx < 0) return;
        const offset = (entry.selectable ? 1 : 0) + (entry.config.expand_key ? 1 : 0);
        const targetColIdx = colIdx + offset;
        tbody.querySelectorAll('tr[data-row-idx]').forEach(tr => {
            const td = tr.children[targetColIdx];
            if (!td) return;
            const text = (td.textContent || '').trim();
            td.classList.toggle('smart-cell-busy', isBusy(text));
        });
    },

    // Collapse/expand a tree subtree. `path` is the value of the tree key for
    // the node being toggled (e.g. "Tank" to hide "Tank/data"). We just flip
    // the collapsed set and re-render — the tree prep in
    // `_updateStructuredTable` handles the rest.
    _toggleTreeNode(compId, path) {
        const entry = this._tables[compId];
        if (!entry) return;
        entry.collapsedPaths = entry.collapsedPaths || new Set();
        if (entry.collapsedPaths.has(path)) entry.collapsedPaths.delete(path);
        else entry.collapsedPaths.add(path);
        this._updateStructuredTable(compId, entry, entry.rows);
    },

    // Toggle the nested row sibling for the main row at `idx`. The chevron
    // rotates via `aria-expanded`; CSS handles the visual.
    _toggleExpand(compId, idx) {
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;
        const expandRow = tbody.querySelector(`tr.smart-expand-row[data-expand-for="${idx}"]`);
        const toggle = tbody.querySelector(`tr[data-row-idx="${idx}"] .smart-expand-toggle`);
        if (!expandRow || !toggle) return;
        const open = expandRow.style.display !== 'none';
        expandRow.style.display = open ? 'none' : '';
        toggle.setAttribute('aria-expanded', open ? 'false' : 'true');
        toggle.classList.toggle('smart-expand-open', !open);
    },

    // Render child rows as a nested smart-table using the same column
    // formatters. Child shape is determined by inspecting the first child's
    // keys, since manifest doesn't (yet) declare a `child_columns` section.
    _renderExpandedChildren(children, parentConfig) {
        if (!Array.isArray(children) || children.length === 0) return '';
        const childCols = parentConfig.child_columns
            || Object.keys(children[0]).map(k => ({ key: k, label: k }));
        const head = childCols.map(c =>
            `<th>${Utils.escapeHtml(c.label || c.key)}</th>`).join('');
        const body = children.map(child =>
            `<tr>${childCols.map(c =>
                `<td>${this._formatValue(child[c.key], c.format, child, c)}</td>`
            ).join('')}</tr>`
        ).join('');
        return `<table class="smart-table smart-table-nested"><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`;
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
        if (format === 'badge' || (col && (col.badge_map || col.badge))) {
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
            case 'error_count': {
                const n = Number(val) || 0;
                const cls = n > 0 ? 'smart-error-count smart-error-count-bad' : 'smart-error-count smart-error-count-ok';
                return `<span class="${cls}">${n}</span>`;
            }
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
                Utils.toast(err.error || `Delete failed (HTTP ${resp.status})`, 'error');
                return;
            }
            Utils.toast(`Deleted "${id}"`, 'success');
            this.refresh(compId);
        } catch (e) {
            Utils.toast('Failed to delete: ' + e.message, 'error');
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
        if (val == null) return '';
        if (typeof val !== 'number') {
            const n = Number(val);
            if (!Number.isFinite(n)) return String(val);
            val = n;
        }
        if (val < 0) return String(val);
        const h = Math.floor(val / 3600);
        const m = Math.floor((val % 3600) / 60);
        const s = Math.floor(val % 60);
        if (h > 0) return `${h}h ${m}m`;
        if (m > 0) return `${m}m ${s}s`;
        return `${s}s`;
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
        // `hidden: true` keeps the action in the config (callable via
        // invokeActionById) without rendering a toolbar button.
        if (action.hidden) return '';
        const tier = action.safety_tier || 'green';
        const variant = this._tierVariant(tier);
        const label = Utils.escapeHtml(action.label || action.id || 'Action');
        const icon = this._iconSvg(action.icon);
        return `<button class="smart-action-btn smart-action-${variant}"
                    title="${label}"
                    data-action-id="${Utils.escapeHtml(action.id || '')}"
                    onclick="SmartTableComponent._invokeToolbarAction('${this._escapeJS(compId)}', ${idx})">
                    ${icon}<span>${label}</span>
                </button>`;
    },

    // Public: invoke a toolbar action by its manifest id. Lets external
    // components (e.g., discovery-card) trigger an action-modal without
    // needing a rendered button to click.
    invokeActionById(compId, actionId) {
        const entry = this._tables[compId];
        if (!entry) return;
        const action = (entry.toolbarActions || []).find(a => a && a.id === actionId);
        if (!action) return;
        this._openActionModal(compId, action, null);
    },

    _renderRowActionButtons(compId, row) {
        const entry = this._tables[compId];
        if (!entry || !entry.rowActions || entry.rowActions.length === 0) return '';
        const rowIdx = row.__rowIdx;
        // Filter by `visible_when` so manifests can show e.g. Pause only when
        // a scrub is running and Resume only when it's paused — the same row
        // action slot swaps based on state instead of showing both.
        const visible = entry.rowActions
            .map((act, i) => ({ act, i }))
            .filter(({ act }) => this._evalVisibleWhen(act.visible_when, row));
        if (visible.length === 0) return '';
        return `<div class="smart-row-actions">${visible.map(({ act, i }) => {
            const tier = act.safety_tier || 'green';
            const variant = this._tierVariant(tier);
            const label = Utils.escapeHtml(act.label || act.id || 'Action');
            const icon = this._iconSvg(act.icon);
            return `<button class="smart-row-action-btn smart-row-action-${variant}"
                        title="${label}"
                        onclick="event.stopPropagation(); SmartTableComponent._invokeRowAction('${this._escapeJS(compId)}', ${i}, ${rowIdx})">
                        ${icon}<span>${label}</span>
                    </button>`;
        }).join('')}</div>`;
    },

    // Evaluate a manifest `visible_when` clause against a row. Supported forms:
    //   { "field": "scrub_status", "equals": "paused" }
    //   { "field": "scrub_status", "not_equals": "none" }
    //   { "field": "scrub_status", "in": ["in_progress", "paused"] }
    //   { "field": "scrub_status", "not_in": ["none", "completed"] }
    //   { "field": "scrub_status", "starts_with": "in_progress" }
    // starts_with is there because scrub_status can be "in_progress (42% done)".
    // Missing clause means the action is always visible.
    _evalVisibleWhen(rule, row) {
        if (!rule || typeof rule !== 'object') return true;
        if (!row) return true;
        const actual = row[rule.field];
        const s = actual == null ? '' : String(actual);
        if (rule.equals !== undefined) return s === String(rule.equals);
        if (rule.not_equals !== undefined) return s !== String(rule.not_equals);
        if (Array.isArray(rule.in)) return rule.in.map(String).includes(s);
        if (Array.isArray(rule.not_in)) return !rule.not_in.map(String).includes(s);
        if (rule.starts_with !== undefined) return s.startsWith(String(rule.starts_with));
        return true;
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

    // ─── Bulk selection ──────────────────────────────────────────────────

    _renderBulkActionButton(compId, action, idx) {
        const tier = action.safety_tier || 'green';
        const variant = this._tierVariant(tier);
        const label = Utils.escapeHtml(action.label || action.id || 'Bulk Action');
        const icon = this._iconSvg(action.icon);
        return `<button class="smart-action-btn smart-action-${variant}"
                    title="${label}"
                    data-bulk-action-id="${Utils.escapeHtml(action.id || '')}"
                    onclick="SmartTableComponent._invokeBulkAction('${this._escapeJS(compId)}', ${idx})">
                    ${icon}<span>${label}</span>
                </button>`;
    },

    _toggleRowSelection(compId, inputEl) {
        const entry = this._tables[compId];
        if (!entry) return;
        const key = inputEl.dataset.rowKey;
        if (inputEl.checked) entry.selection.add(key);
        else entry.selection.delete(key);
        this._updateBulkBar(compId);
    },

    _toggleSelectAll(compId, checked) {
        const entry = this._tables[compId];
        if (!entry) return;
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;
        tbody.querySelectorAll('input.smart-row-select').forEach(cb => {
            cb.checked = checked;
            const key = cb.dataset.rowKey;
            if (checked) entry.selection.add(key);
            else entry.selection.delete(key);
        });
        this._updateBulkBar(compId);
    },

    _clearSelection(compId) {
        const entry = this._tables[compId];
        if (!entry) return;
        entry.selection.clear();
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (tbody) tbody.querySelectorAll('input.smart-row-select').forEach(cb => { cb.checked = false; });
        const selectAll = document.querySelector(`#smart-table-${compId} .smart-select-all`);
        if (selectAll) selectAll.checked = false;
        this._updateBulkBar(compId);
    },

    _updateBulkBar(compId) {
        const bar = document.getElementById(`smart-bulk-bar-${compId}`);
        if (!bar) return;
        const entry = this._tables[compId];
        const count = entry ? entry.selection.size : 0;
        bar.style.display = count > 0 ? '' : 'none';
        const numEl = bar.querySelector('.smart-bulk-count-num');
        if (numEl) numEl.textContent = String(count);
        const selectAll = document.querySelector(`#smart-table-${compId} .smart-select-all`);
        if (selectAll && entry) {
            const visibleCount = entry.displayedRows ? entry.displayedRows.length : 0;
            selectAll.checked = count > 0 && count >= visibleCount;
            selectAll.indeterminate = count > 0 && count < visibleCount;
        }
    },

    async _invokeBulkAction(compId, actionIdx) {
        const entry = this._tables[compId];
        if (!entry) return;
        const action = entry.bulkActions[actionIdx];
        if (!action) return;
        const selectedKeys = Array.from(entry.selection);
        if (selectedKeys.length === 0) return;
        const rows = (entry.rows || []).filter(r => selectedKeys.includes(String(r[entry.keyField] ?? '')));
        if (rows.length === 0) return;

        const cfg = action.confirm || {};
        const count = rows.length;
        const title = this._interpolate(cfg.title || action.label || 'Bulk Action', { count });
        const message = this._interpolate(cfg.message || `Run ${action.label || 'this action'} on ${count} items?`, { count });
        const tier = action.safety_tier || 'green';
        const tierBadge = tier !== 'green'
            ? `<span class="smart-tier-badge smart-tier-${tier}">${tier.toUpperCase()} TIER</span>`
            : '';
        const submitVariant = cfg.button_variant || (tier === 'red' ? 'danger' : 'primary');
        const submitLabel = Utils.escapeHtml(cfg.button_label || `Run on ${count}`);

        const modal = Modals.create(`
            <div class="modal smart-action-modal">
                <div class="modal-header">
                    <h3>${Utils.escapeHtml(title)} ${tierBadge}</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                    </button>
                </div>
                <div class="modal-body">
                    <p class="smart-action-message">${Utils.escapeHtml(message)}</p>
                    <div class="smart-bulk-progress" id="smart-bulk-progress" style="display:none">
                        <div class="smart-bulk-progress-label">Processing <span class="smart-bulk-progress-current">0</span> of ${count}…</div>
                    </div>
                    <div class="form-error" id="smart-action-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-${submitVariant}" id="smart-bulk-submit">${submitLabel}</button>
                </div>
            </div>
        `);

        const submitBtn = modal.querySelector('#smart-bulk-submit');
        submitBtn.onclick = () => this._submitBulkAction(modal, compId, action, rows);
    },

    async _submitBulkAction(modalEl, compId, action, rows) {
        const entry = this._tables[compId];
        if (!entry || !entry.addonId) return;
        const submitBtn = modalEl.querySelector('#smart-bulk-submit');
        const errEl = modalEl.querySelector('#smart-action-error');
        const progress = modalEl.querySelector('#smart-bulk-progress');
        const currentEl = modalEl.querySelector('.smart-bulk-progress-current');
        submitBtn.disabled = true;
        progress.style.display = '';
        errEl.textContent = '';

        const actionSpec = action.action || {};
        const basePath = actionSpec.endpoint;
        const method = (actionSpec.method || 'POST').toUpperCase();
        let succeeded = 0;
        const failures = [];

        for (let i = 0; i < rows.length; i++) {
            const row = rows[i];
            currentEl.textContent = String(i + 1);

            const body = this._buildRequestBody(action, row, {}, actionSpec);
            const path = this._appendAgentIdToPath(basePath, row);
            const proxyURL = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                + (method !== 'GET' && method !== 'POST' ? `&method=${method}` : '');
            const init = { method: method === 'GET' ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' } };
            if (init.method !== 'GET') init.body = JSON.stringify(body);

            try {
                const resp = await fetch(proxyURL, init);
                if (resp.ok) {
                    succeeded++;
                } else {
                    const data = await resp.json().catch(() => ({}));
                    failures.push(`${row[entry.keyField]}: ${data.error || `HTTP ${resp.status}`}`);
                }
            } catch (e) {
                failures.push(`${row[entry.keyField]}: ${e.message}`);
            }
        }

        Modals.close(submitBtn);
        entry.selection.clear();
        const total = rows.length;
        const ctx = { count: total, total, succeeded, failed: failures.length };
        if (failures.length === 0) {
            const tmpl = action.success_message || `${action.label || 'Action'} completed for ${total} item${total === 1 ? '' : 's'}`;
            Utils.toast(this._interpolate(tmpl, ctx), 'success');
        } else if (succeeded > 0) {
            Utils.toast(`${succeeded} of ${total} succeeded. ${failures.length} failed: ${failures[0]}`, 'warning');
        } else {
            const tmpl = action.error_message || `All ${total} operations failed: ${failures[0]}`;
            Utils.toast(this._interpolate(tmpl, { ...ctx, error: failures[0] }), 'error');
        }
        this.refresh(compId);
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

        const fieldsHtml = cfg.sections
            ? cfg.sections.map((sec, i) => this._renderSection(sec, i, row)).join('')
            : cfg.fields
                .filter(f => f.type !== 'hidden')
                .map(f => this._renderField(f, row))
                .join('');
        // Hidden fields are always appended at the form tail so
        // _collectFormValues can read them. They carry `value_from: row.X`
        // values the backend requires but the user should never edit.
        const hiddenHtml = (cfg.fields || [])
            .filter(f => f.type === 'hidden')
            .map(f => this._renderField(f, row))
            .join('');

        const wideClass = cfg.sections ? ' smart-action-modal-wide' : '';

        const previewHtml = action.action && action.action.preview
            ? `<div class="smart-action-preview" id="smart-action-preview"><div class="smart-action-preview-label">Command preview</div><pre class="smart-action-preview-cmd">Loading…</pre></div>`
            : '';

        const tierBadge = tier !== 'green'
            ? `<span class="smart-tier-badge smart-tier-${tier}">${tier.toUpperCase()} TIER</span>`
            : '';

        const submitVariant = cfg.button_variant || (tier === 'red' ? 'danger' : 'primary');
        const submitLabel = Utils.escapeHtml(cfg.button_label || action.label || 'Submit');

        const modal = Modals.create(`
            <div class="modal smart-action-modal${wideClass}">
                <div class="modal-header">
                    <h3>${Utils.escapeHtml(title)} ${tierBadge}</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                    </button>
                </div>
                <div class="modal-body">
                    ${message ? `<p class="smart-action-message">${Utils.escapeHtml(message)}</p>` : ''}
                    <form class="smart-action-form" onsubmit="return false;">${fieldsHtml}${hiddenHtml}</form>
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
                this._loadFieldOptions(modal, field, entry.addonId, row);
            }
        }

        // Apply `visible_when` once to set initial state, then re-evaluate on
        // every input/change so form-field references react live.
        this._applyVisibleWhen(modal, row);
        this._updateCapacityPreviews(modal, cfg.fields);
        modal.querySelectorAll('input, select, textarea').forEach(el => {
            el.addEventListener('input', () => {
                this._applyVisibleWhen(modal, row);
                this._updateCapacityPreviews(modal, cfg.fields);
                this._updateCronPreviews(modal);
            });
            el.addEventListener('change', () => {
                this._applyVisibleWhen(modal, row);
                this._updateCapacityPreviews(modal, cfg.fields);
                this._updateCronPreviews(modal);
            });
        });

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

        // remote_value fields GET a sibling field's value through the proxy.
        // Debounced separately from preview because the watched field may
        // lazy-create agent-side state on first hit.
        if (modal.querySelector('.smart-remote-value-wrap')) {
            const refreshRemote = this._debounce(() => this._updateRemoteValues(modal), 400);
            modal.querySelectorAll('input, select, textarea').forEach(el => {
                el.addEventListener('input', refreshRemote);
                el.addEventListener('change', refreshRemote);
            });
            this._updateRemoteValues(modal);
        }
    },

    // Render a `{title, collapsed, layout, fields}` section as a collapsible
    // card. `layout: "columns"` arranges fields in a 2-column grid.
    _renderSection(section, idx, row) {
        const title = section.title ? Utils.escapeHtml(section.title) : '';
        const collapsed = !!section.collapsed;
        const layoutClass = section.layout === 'columns' ? ' smart-section-columns' : '';
        const fieldsHtml = (section.fields || [])
            .filter(f => f.type !== 'hidden')
            .map(f => this._renderField(f, row))
            .join('');
        const collapsedClass = collapsed ? ' smart-section-collapsed' : '';
        return `
            <section class="smart-section${collapsedClass}" data-section-idx="${idx}">
                ${title ? `<header class="smart-section-header" onclick="SmartTableComponent._toggleSection(this)">
                    <svg class="smart-section-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><polyline points="9 18 15 12 9 6"/></svg>
                    <span>${title}</span>
                </header>` : ''}
                <div class="smart-section-body${layoutClass}">${fieldsHtml}</div>
            </section>`;
    },

    _toggleSection(headerEl) {
        const section = headerEl.parentElement;
        if (!section) return;
        section.classList.toggle('smart-section-collapsed');
    },

    // Fetches the URL for each `remote_value` field in the modal whenever the
    // field named in `depends_on` changes. The `depVal` → last-fetched cache
    // on each wrap skips no-op refetches (e.g. when the user edits a
    // different field). Errors leave the cache unset so the next input
    // retries. Rendered via `_renderField` → case 'remote_value'.
    async _updateRemoteValues(modalEl) {
        const wraps = modalEl.querySelectorAll('.smart-remote-value-wrap');
        if (!wraps.length) return;
        const ctx = modalEl._smartAction;
        if (!ctx) return;
        const entry = this._tables[ctx.compId];
        if (!entry || !entry.addonId) return;

        const formValues = this._collectFormValues(modalEl);

        for (const wrap of wraps) {
            const key = wrap.dataset.remoteField;
            const depends = wrap.dataset.dependsOn;
            const endpoint = wrap.dataset.endpoint;
            const method = (wrap.dataset.method || 'GET').toUpperCase();
            const regex = wrap.dataset.validateRegex;
            const valuePath = wrap.dataset.valuePath;
            const displayEl = modalEl.querySelector(
                `.smart-remote-value-display[data-display-for="${CSS.escape(key)}"]`
            );
            const statusEl = modalEl.querySelector(
                `.smart-remote-value-status[data-status-for="${CSS.escape(key)}"]`
            );
            const copyBtn = wrap.querySelector('.smart-remote-value-copy');

            const depVal = String(formValues[depends] || '').trim();

            const clear = (msg, cls) => {
                if (displayEl) displayEl.value = '';
                if (statusEl) {
                    statusEl.textContent = msg || '';
                    statusEl.className = 'smart-remote-value-status' + (cls ? ' ' + cls : '');
                }
                if (copyBtn) copyBtn.disabled = true;
            };

            if (!depVal) {
                wrap.dataset.lastFetched = '';
                clear('', '');
                continue;
            }
            if (regex) {
                try {
                    if (!new RegExp(regex).test(depVal)) {
                        wrap.dataset.lastFetched = '';
                        clear(`Invalid ${depends}`, 'smart-remote-value-status-error');
                        continue;
                    }
                } catch (_) { /* bad regex in manifest — ignore, still fetch */ }
            }

            if (wrap.dataset.lastFetched === depVal) continue;
            wrap.dataset.lastFetched = depVal;

            if (statusEl) {
                statusEl.textContent = 'Fetching…';
                statusEl.className = 'smart-remote-value-status smart-remote-value-status-pending';
            }

            try {
                const interpCtx = { ...(ctx.row || {}), ...formValues, form: formValues };
                const path = this._interpolate(endpoint, interpCtx);
                const proxyURL = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                    + (method !== 'GET' && method !== 'POST' ? `&method=${method}` : '');
                const init = {
                    method: method === 'GET' ? 'GET' : 'POST',
                    headers: { 'X-Requested-With': 'XMLHttpRequest' }
                };
                const resp = await fetch(proxyURL, init);
                if (!resp.ok) {
                    const data = await resp.json().catch(() => ({}));
                    const msg = (data && (data.error || data.message)) || `HTTP ${resp.status}`;
                    wrap.dataset.lastFetched = '';
                    clear(msg, 'smart-remote-value-status-error');
                    continue;
                }
                const data = await resp.json().catch(() => ({}));
                let value = '';
                if (valuePath) {
                    const v = this._getNestedValue(data, valuePath);
                    value = v == null ? '' : String(v);
                } else if (typeof data === 'string') {
                    value = data;
                } else if (data && typeof data === 'object') {
                    value = String(data.public_key ?? data.value ?? data.key ?? data.message ?? '');
                }
                if (displayEl) displayEl.value = value;
                if (statusEl) {
                    statusEl.textContent = value ? '' : 'Empty response';
                    statusEl.className = 'smart-remote-value-status';
                }
                if (copyBtn) copyBtn.disabled = !value;
            } catch (e) {
                wrap.dataset.lastFetched = '';
                clear(`Failed: ${e.message}`, 'smart-remote-value-status-error');
            }
        }
    },

    async _copyRemoteValue(btnEl) {
        const wrap = btnEl.closest('.smart-remote-value-wrap');
        if (!wrap) return;
        const key = wrap.dataset.remoteField || '';
        const textarea = wrap.querySelector(
            `.smart-remote-value-display[data-display-for="${CSS.escape(key)}"]`
        );
        if (!textarea || !textarea.value) return;
        const origText = btnEl.textContent;
        try {
            await navigator.clipboard.writeText(textarea.value);
        } catch (_) {
            textarea.select();
            try { document.execCommand('copy'); } catch (_) { /* no-op */ }
        }
        btnEl.textContent = 'Copied!';
        setTimeout(() => { btnEl.textContent = origText; }, 1500);
    },

    _getNestedValue(obj, path) {
        if (!obj || !path) return undefined;
        return path.split('.').reduce((o, p) => (o == null ? undefined : o[p]), obj);
    },

    // Client-side capacity calculator for `type: "capacity_preview"` fields.
    // Pulls sizes from the devices field's loaded options (`options.size_bytes`
    // when the endpoint exposes it) and applies the vdev arithmetic.
    _updateCapacityPreviews(modal, allFields) {
        const panels = modal.querySelectorAll('.smart-capacity-preview');
        if (panels.length === 0) return;
        panels.forEach(panel => {
            const typeKey = panel.dataset.typeField;
            const devKey = panel.dataset.devicesField;
            const typeEl = modal.querySelector(`[data-field-key="${CSS.escape(typeKey)}"]`);
            const devEl = modal.querySelector(`[data-field-key="${CSS.escape(devKey)}"]`);
            const body = panel.querySelector('.smart-capacity-body');
            if (!typeEl || !devEl || !body) return;

            const vdevType = (typeEl.value || 'stripe').toLowerCase();
            const selectedValues = devEl.multiple
                ? Array.from(devEl.selectedOptions).map(o => o.value)
                : (devEl.value ? [devEl.value] : []);
            const n = selectedValues.length;
            if (n === 0) {
                body.textContent = 'Select drives to preview usable capacity.';
                return;
            }

            // Pull per-drive size from option dataset — populated by
            // _loadFieldOptions when the field declares `option_meta`.
            const sizes = selectedValues.map(v => {
                const opt = devEl.querySelector(`option[value="${CSS.escape(v)}"]`);
                const bytes = opt && opt.dataset.sizeBytes ? Number(opt.dataset.sizeBytes) : 0;
                return Number.isFinite(bytes) ? bytes : 0;
            });
            const smallest = sizes.length ? Math.min(...sizes.filter(s => s > 0)) : 0;

            let usableCount = 0;
            let layoutNote = '';
            switch (vdevType) {
                case 'mirror':
                    usableCount = 1;
                    layoutNote = `mirror of ${n} drives — ${n}-way redundancy`;
                    break;
                case 'raidz':
                case 'raidz1':
                    usableCount = Math.max(n - 1, 0);
                    layoutNote = `raidz1 — loses 1 drive to parity`;
                    break;
                case 'raidz2':
                    usableCount = Math.max(n - 2, 0);
                    layoutNote = `raidz2 — loses 2 drives to parity`;
                    break;
                case 'raidz3':
                    usableCount = Math.max(n - 3, 0);
                    layoutNote = `raidz3 — loses 3 drives to parity`;
                    break;
                default:
                    usableCount = n;
                    layoutNote = `stripe — no redundancy, any drive loss destroys the pool`;
            }
            const raw = n * smallest;
            const usable = usableCount * smallest;
            const pct = raw > 0 ? Math.round((usable / raw) * 100) : 0;
            body.innerHTML = smallest > 0
                ? `<strong>${this._formatBytes(usable)}</strong> usable &middot; ${this._formatBytes(raw)} raw &middot; ${pct}% efficiency<div class="smart-capacity-note">${Utils.escapeHtml(layoutNote)}</div>`
                : `<div class="smart-capacity-note">${Utils.escapeHtml(layoutNote)} &middot; drive sizes unknown — usable capacity will be shown after the agent reports them.</div>`;
        });
    },

    // Show/hide form fields based on `visible_when: {field, equals}`.
    // `field` may be `row.X` (static from the triggering row) or a form-field
    // key (live from another input in the same form).
    _applyVisibleWhen(modal, row) {
        const groups = modal.querySelectorAll('.form-group[data-visible-when]');
        groups.forEach(group => {
            let rule;
            try { rule = JSON.parse(group.dataset.visibleWhen); }
            catch { return; }
            if (!rule || !rule.field) return;
            let actual;
            if (rule.field.startsWith('row.')) {
                actual = row ? row[rule.field.slice(4)] : undefined;
            } else {
                const input = modal.querySelector(`[data-field-key="${CSS.escape(rule.field)}"]`);
                if (input) {
                    actual = input.type === 'checkbox' ? input.checked : input.value;
                }
            }
            const match = String(actual) === String(rule.equals);
            group.style.display = match ? '' : 'none';
        });
    },

    _normalizeActionConfig(action) {
        // `form` and `confirm` describe the same modal in different shapes.
        // Confirm dialogs may also carry `extra_fields` for inputs that aren't
        // the type-to-confirm box itself.
        if (action.form) {
            // A form may declare either a flat `fields` array, or a `sections`
            // array of `{title, collapsed, layout, fields}`. We normalize both
            // shapes: downstream loops (options loading, visible_when) walk
            // the flat `fields`, while the modal renders from `sections` when
            // present.
            const sections = Array.isArray(action.form.sections) ? action.form.sections : null;
            const flatFields = sections
                ? sections.flatMap(s => s.fields || [])
                : (action.form.fields || []);
            return {
                title: action.form.title,
                fields: flatFields,
                sections,
                button_label: action.form.button_label,
                button_variant: action.form.button_variant,
                show_command: action.form.show_command,
                require_type_confirm: action.form.require_type_confirm || false,
                confirm_value: action.form.confirm_value,
                confirm_key: action.form.confirm_key || 'confirm',
                confirm_label: action.form.confirm_label,
                message: action.form.message,
                success_message: action.form.success_message,
                error_message: action.form.error_message
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
                confirm_label: action.confirm.confirm_label,
                success_message: action.confirm.success_message,
                error_message: action.confirm.error_message
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
        const vwAttr = field.visible_when
            ? ` data-visible-when="${Utils.escapeHtml(JSON.stringify(field.visible_when))}"`
            : '';

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
                const opts = (field.options || []).map(o => {
                    const t = typeof o.value;
                    const typeAttr = (t === 'number' || t === 'boolean') ? ` data-value-type="${t}"` : '';
                    return `<option value="${Utils.escapeHtml(String(o.value))}"${typeAttr}${String(o.value) === String(initialVal) ? ' selected' : ''}>${Utils.escapeHtml(o.label)}</option>`;
                }).join('');
                const blank = field.required ? '' : `<option value="">— unchanged —</option>`;
                // `allow_custom` injects a sentinel option and a sibling text
                // input that appears only when the sentinel is chosen. On
                // submit, _collectFormValues substitutes the text value back.
                const customOpt = field.allow_custom
                    ? `<option value="__custom__">${Utils.escapeHtml(field.custom_label || 'Custom…')}</option>`
                    : '';
                input = `<select id="${id}" class="form-input" data-field-key="${Utils.escapeHtml(field.key)}"${field.allow_custom ? ` data-allow-custom="1"` : ''}>${blank}${opts}${customOpt}<option disabled>Loading…</option></select>`;
                if (field.allow_custom) {
                    const customId = `${id}__custom`;
                    const isCron = field.custom_input_type === 'cron';
                    // The custom input intentionally omits `data-field-key`;
                    // `_collectFormValues` substitutes its value into the parent
                    // select's key via `data-custom-for`. A `cron` variant adds
                    // `smart-cron-input` + a sibling `.smart-cron-preview` so
                    // live cron description works inside the custom wrap too.
                    const cronClass = isCron ? ' smart-cron-input' : '';
                    const placeholder = field.custom_placeholder || (isCron ? '*/15 * * * *' : 'e.g. */15 * * * *');
                    const cronPreview = isCron
                        ? `<div class="smart-cron-preview" data-cron-preview-for="${Utils.escapeHtml(field.key)}">Runs…</div>`
                        : '';
                    // Nested .form-group so visible_when hides it by display:none.
                    input += `<div class="form-group smart-custom-input-group" data-visible-when='${JSON.stringify({field: field.key, equals: '__custom__'})}'>
                        <input type="text" id="${customId}" class="form-input smart-custom-input${cronClass}" data-custom-for="${Utils.escapeHtml(field.key)}" placeholder="${Utils.escapeHtml(placeholder)}"${isCron ? ' autocomplete="off" spellcheck="false"' : ''}>
                        ${cronPreview}
                        ${field.custom_hint ? `<div class="form-hint">${Utils.escapeHtml(field.custom_hint)}</div>` : ''}
                    </div>`;
                }
                break;
            }
            case 'multi-select': {
                // `variant: "cards"` keeps a hidden <select multiple> as the
                // canonical source of truth (so _collectFormValues is
                // unchanged), and renders a visible grid of toggleable cards.
                // _loadFieldOptions populates both. Card clicks toggle the
                // matching <option>.selected and fire a `change` event so
                // visible_when + capacity preview re-evaluate.
                const variant = field.variant === 'cards' ? 'cards' : 'native';
                if (variant === 'cards') {
                    input = `<select id="${id}" class="form-input form-multi-select smart-hidden-select" multiple
                                data-field-key="${Utils.escapeHtml(field.key)}"
                                data-variant="cards"
                                aria-hidden="true"
                                tabindex="-1"><option disabled>Loading options…</option></select>
                            <div class="smart-disk-grid" data-for-field="${Utils.escapeHtml(field.key)}">
                                <div class="smart-disk-grid-empty">Loading drives…</div>
                            </div>
                            <div class="smart-disk-grid-count" data-count-for="${Utils.escapeHtml(field.key)}"></div>`;
                } else {
                    input = `<select id="${id}" class="form-input form-multi-select" multiple
                                data-field-key="${Utils.escapeHtml(field.key)}"
                                size="6"><option disabled>Loading options…</option></select>`;
                }
                break;
            }
            case 'test_button': {
                // Standalone action button inside a form (does not submit the
                // form). POSTs the current form values to `endpoint` and
                // renders the agent's {ok, message, result} response inline.
                // `require_fields` gates the button until the named sibling
                // fields all have non-empty values.
                const endpoint = Utils.escapeHtml(field.endpoint || '');
                const methodAttr = Utils.escapeHtml((field.method || 'POST').toUpperCase());
                const require = Array.isArray(field.require_fields) ? field.require_fields.join(',') : '';
                const btnLabel = Utils.escapeHtml(field.button_label || field.label || 'Run');
                return `<div class="form-group smart-test-button-group">
                    ${field.label ? `<div class="form-label">${Utils.escapeHtml(field.label)}</div>` : ''}
                    <button type="button" class="btn btn-secondary smart-test-button"
                            data-field-key="${Utils.escapeHtml(field.key)}"
                            data-endpoint="${endpoint}"
                            data-method="${methodAttr}"
                            data-require-fields="${Utils.escapeHtml(require)}"
                            onclick="SmartTableComponent._runTestButton(this)">
                        ${btnLabel}
                    </button>
                    <div class="smart-test-button-result" data-result-for="${Utils.escapeHtml(field.key)}"></div>
                    ${hint}
                </div>`;
            }
            case 'remote_value': {
                // Read-only display field populated by GETting a sibling
                // field's value through the add-on proxy. Watches
                // `depends_on`; when it matches `validate_regex` (if set),
                // interpolates `{form.X}` into `endpoint` and renders the
                // response (via `value_path` dot-path, or the `public_key` /
                // `value` / `key` / `message` fallback) in a readonly
                // textarea with a Copy button. Purely display — no form
                // value is emitted.
                const depends = Utils.escapeHtml(field.depends_on || '');
                const endpoint = Utils.escapeHtml(field.endpoint || '');
                const method = Utils.escapeHtml((field.method || 'GET').toUpperCase());
                const regex = Utils.escapeHtml(field.validate_regex || '');
                const valuePath = Utils.escapeHtml(field.value_path || '');
                const showCopy = field.copy_button !== false;
                const rows = Number.isFinite(field.rows) ? field.rows : 2;
                return `<div class="form-group smart-remote-value-group">
                    ${field.label ? `<div class="form-label">${Utils.escapeHtml(field.label)}</div>` : ''}
                    <div class="smart-remote-value-wrap"
                         data-remote-field="${Utils.escapeHtml(field.key)}"
                         data-depends-on="${depends}"
                         data-endpoint="${endpoint}"
                         data-method="${method}"
                         data-validate-regex="${regex}"
                         data-value-path="${valuePath}">
                        <textarea readonly class="form-input smart-remote-value-display"
                                  data-display-for="${Utils.escapeHtml(field.key)}"
                                  placeholder="${Utils.escapeHtml(placeholder || '')}"
                                  rows="${rows}"
                                  spellcheck="false"></textarea>
                        ${showCopy ? `<button type="button" class="btn btn-secondary smart-remote-value-copy"
                                               onclick="SmartTableComponent._copyRemoteValue(this)"
                                               disabled>Copy</button>` : ''}
                    </div>
                    <div class="smart-remote-value-status" data-status-for="${Utils.escapeHtml(field.key)}"></div>
                    ${hint}
                </div>`;
            }
            case 'cron': {
                // 5-field cron expression (minute hour day month weekday) with
                // live plain-language preview. `_updateCronPreviews` runs on
                // every input event (wired from `_openActionModal`) and fills
                // the preview element.
                const cronVal = initialVal != null ? String(initialVal) : '';
                input = `<input type="text" id="${id}" class="form-input smart-cron-input"
                            data-field-key="${Utils.escapeHtml(field.key)}"
                            value="${Utils.escapeHtml(cronVal)}"
                            placeholder="${Utils.escapeHtml(placeholder || '*/15 * * * *')}"
                            ${field.required ? 'required' : ''}
                            autocomplete="off"
                            spellcheck="false">
                        <div class="smart-cron-preview" data-cron-preview-for="${Utils.escapeHtml(field.key)}">
                            ${cronVal ? Utils.escapeHtml(this._describeCron(cronVal)) : 'Runs…'}
                        </div>`;
                break;
            }
            case 'checkbox':
            case 'toggle':
                input = `<label class="form-checkbox-row"><input type="checkbox" id="${id}"
                            data-field-key="${Utils.escapeHtml(field.key)}"
                            ${initialVal === true || initialVal === 'on' || initialVal === 'true' ? 'checked' : ''}>
                            <span>${Utils.escapeHtml(field.label || '')}</span></label>`;
                // Checkbox uses its own inline label; suppress the outer one.
                return `<div class="form-group"${tierAttr}${vwAttr}>${input}${hint}</div>`;
            case 'hidden':
                // Hidden fields are tracked separately so the form can still
                // hand their values to the body collector. Render none.
                return `<input type="hidden" id="${id}" data-field-key="${Utils.escapeHtml(field.key)}" value="${Utils.escapeHtml(String(initialVal ?? ''))}">`;
            case 'capacity_preview': {
                // Read-only panel that computes usable capacity from sibling
                // fields (type_field + devices_field + size_from). Rendered
                // inert; _updateCapacityPreview fills it live as the user
                // picks devices in the same form.
                const typeField = field.type_field || 'data_type';
                const devField = field.devices_field || 'data_devices';
                const sizeFrom = field.size_from || '';
                return `<div class="form-group smart-capacity-preview" data-type-field="${Utils.escapeHtml(typeField)}" data-devices-field="${Utils.escapeHtml(devField)}" data-size-from="${Utils.escapeHtml(sizeFrom)}">
                    <div class="form-label">${Utils.escapeHtml(field.label || 'Capacity preview')}</div>
                    <div class="smart-capacity-body">Select drives to preview usable capacity.</div>
                </div>`;
            }
            default:
                input = `<input type="text" id="${id}" class="form-input" data-field-key="${Utils.escapeHtml(field.key)}">`;
        }

        return `<div class="form-group"${tierAttr}${vwAttr}>${label}${input}${hint}</div>`;
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

    // Translate a 5-field cron expression into plain English. Handles the
    // common forms (`*`, `*/N`, `a,b,c`, `a-b`, single value) in each column;
    // anything exotic falls through to a literal echo so the user can at least
    // read back what they typed. Returns an error string for malformed input
    // so the user gets immediate feedback before submitting.
    _describeCron(expr) {
        if (!expr || typeof expr !== 'string') return 'Invalid — enter 5 space-separated fields';
        const parts = expr.trim().split(/\s+/);
        if (parts.length !== 5) return 'Invalid — expected 5 fields (minute hour day month weekday)';
        const [mn, hr, dom, mo, dow] = parts;
        const valid = /^(\*|\*\/\d+|\d+(-\d+)?(\/\d+)?|(\d+)(,\d+)*)$/;
        if (!parts.every(p => valid.test(p))) return 'Invalid cron expression';

        const dowNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
        const moNames = ['', 'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

        const timeOf = () => {
            if (hr === '*' && mn === '*') return 'every minute';
            if (hr === '*' && /^\*\/\d+$/.test(mn)) return `every ${mn.slice(2)} minutes`;
            if (/^\*\/\d+$/.test(hr) && mn === '0') return `every ${hr.slice(2)} hours`;
            if (mn === '0' && hr === '*') return 'at the top of every hour';
            if (/^\d+$/.test(mn) && /^\d+$/.test(hr)) {
                const h = String(hr).padStart(2, '0'), m = String(mn).padStart(2, '0');
                return `at ${h}:${m}`;
            }
            return `at minute ${mn}, hour ${hr}`;
        };

        const dayOf = () => {
            if (dom === '*' && mo === '*' && dow === '*') return 'every day';
            if (dom === '*' && mo === '*' && /^\d+$/.test(dow)) return `every ${dowNames[+dow] || dow}`;
            if (dom === '*' && mo === '*' && /^\d+(,\d+)+$/.test(dow)) {
                return `on ${dow.split(',').map(d => dowNames[+d] || d).join(', ')}`;
            }
            if (/^\d+$/.test(dom) && mo === '*' && dow === '*') return `on day ${dom} of every month`;
            if (/^\d+$/.test(dom) && /^\d+$/.test(mo) && dow === '*') return `on ${moNames[+mo] || mo} ${dom}`;
            return `dom=${dom} mo=${mo} dow=${dow}`;
        };

        return `Runs ${timeOf()}, ${dayOf()}.`;
    },

    // Refresh every cron preview in the modal. Called on any form input so
    // the preview tracks what the user is typing.
    _updateCronPreviews(modalEl) {
        if (!modalEl) return;
        modalEl.querySelectorAll('.smart-cron-input').forEach(inp => {
            // `data-field-key` is present on top-level cron fields; the
            // `custom_input_type: "cron"` variant lives under a select's
            // `allow_custom` wrap with `data-custom-for` instead.
            const key = inp.dataset.fieldKey || inp.dataset.customFor;
            if (!key) return;
            const preview = modalEl.querySelector(`[data-cron-preview-for="${key.replace(/"/g, '\\"')}"]`);
            if (!preview) return;
            const desc = this._describeCron(inp.value);
            preview.textContent = desc;
            preview.classList.toggle('smart-cron-preview-error', desc.startsWith('Invalid'));
        });
    },

    // Map a manifest `safety_tier` to the CSS variant used by action buttons.
    // `black` is the irreversible/destructive tier (destroy pool, force import)
    // — it renders as a solid red fill so it visually outweighs the outlined
    // red-bordered `red` tier.
    _tierVariant(tier) {
        if (tier === 'red') return 'danger';
        if (tier === 'yellow') return 'warning';
        if (tier === 'black') return 'irreversible';
        return 'primary';
    },

    // Resolve the agent_id for a given row: prefer `row.agent_id` so cross-
    // agent lists (e.g., aggregated drives) dispatch each action to its owning
    // host, and fall back to the page-level selector for single-agent tables.
    _resolveAgentId(row) {
        if (row && row.agent_id != null && row.agent_id !== '') {
            return String(row.agent_id);
        }
        if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
            const id = ManifestRenderer.getSelectedAgentId();
            if (id) return String(id);
        }
        return '';
    },

    _appendAgentIdToPath(path, row) {
        // Interpolate {row.X} tokens in the endpoint so manifests can declare
        // RESTful paths like `/api/tasks/{row.id}` without the UI sending
        // them literally. Without this the agent's router tries to parse
        // "{row.id}" as an integer and rejects the request.
        const resolved = this._interpolate(path, row || {});
        const id = this._resolveAgentId(row);
        if (!id) return resolved;
        const sep = resolved.includes('?') ? '&' : '?';
        return `${resolved}${sep}agent_id=${encodeURIComponent(id)}`;
    },

    _interpolate(template, ctx) {
        if (!template) return '';
        // Support `{row.X}` (first-class), `{form.X}` (when ctx has a `form`
        // object — callers spread formValues into both the top level and a
        // `form` namespace so either works), and bare `{X}` for top-level keys.
        return String(template)
            .replace(/\{row\.([a-zA-Z0-9_]+)\}/g, (_, key) =>
                ctx && ctx.row && ctx.row[key] != null ? String(ctx.row[key])
                : ctx && ctx[key] != null ? String(ctx[key]) : '')
            .replace(/\{form\.([a-zA-Z0-9_]+)\}/g, (_, key) =>
                ctx && ctx.form && ctx.form[key] != null ? String(ctx.form[key])
                : ctx && ctx[key] != null ? String(ctx[key]) : '')
            .replace(/\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g, (m, key) =>
                ctx && ctx[key] != null ? String(ctx[key]) : m);
    },

    // Render a grid of clickable cards for `multi-select` with
    // `variant: "cards"`. The hidden <select> is the canonical source of
    // truth; card clicks flip the matching option.selected and fire a
    // change event so visible_when + capacity preview react.
    _renderDiskCards(modalEl, field, items, metaMap) {
        const grid = modalEl.querySelector(`.smart-disk-grid[data-for-field="${CSS.escape(field.key)}"]`);
        if (!grid) return;
        const valueKey = field.option_value || 'value';
        const labelKey = field.option_label || 'label';
        const detailKey = field.option_detail;
        if (items.length === 0) {
            grid.innerHTML = `<div class="smart-disk-grid-empty">No available drives.</div>`;
            return;
        }
        grid.innerHTML = items.map(it => {
            const v = String(it[valueKey] ?? '');
            const name = Utils.escapeHtml(String(it[labelKey] ?? v));
            const detail = detailKey && it[detailKey] ? Utils.escapeHtml(String(it[detailKey])) : '';
            const size = it.size ? this._formatBytes(it.size) : '';
            const serial = it.serial ? Utils.escapeHtml(String(it.serial)) : '';
            return `
                <div class="smart-disk-card" data-value="${Utils.escapeHtml(v)}" onclick="SmartTableComponent._toggleDiskCard(this, '${this._escapeJS(field.key)}')">
                    <div class="smart-disk-card-check"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" width="14" height="14"><polyline points="20 6 9 17 4 12"/></svg></div>
                    <div class="smart-disk-card-body">
                        <div class="smart-disk-card-name">${name}</div>
                        ${detail ? `<div class="smart-disk-card-detail">${detail}</div>` : ''}
                        <div class="smart-disk-card-meta">
                            ${size ? `<span>${size}</span>` : ''}
                            ${serial ? `<span class="smart-disk-card-serial">${serial}</span>` : ''}
                        </div>
                        <div class="smart-disk-card-conflict"></div>
                    </div>
                </div>`;
        }).join('');
        this._refreshDiskConflicts(modalEl);
    },

    _toggleDiskCard(cardEl, fieldKey) {
        const modalEl = cardEl.closest('.modal');
        if (!modalEl) return;
        if (cardEl.classList.contains('smart-disk-card-conflict-disabled')) return;
        const sel = modalEl.querySelector(`select[data-field-key="${CSS.escape(fieldKey)}"]`);
        if (!sel) return;
        const value = cardEl.dataset.value;
        const opt = sel.querySelector(`option[value="${CSS.escape(value)}"]`);
        if (!opt) return;
        opt.selected = !opt.selected;
        cardEl.classList.toggle('smart-disk-card-selected', opt.selected);
        sel.dispatchEvent(new Event('change', { bubbles: true }));
        this._refreshDiskConflicts(modalEl);
        this._updateDiskSelectionCount(modalEl, fieldKey);
    },

    // When multiple disk pickers exist in the same form (Data / Log / Cache /
    // Spare), a drive picked in one should render disabled + "in use as X" in
    // the others. This runs after any card toggle.
    _refreshDiskConflicts(modalEl) {
        const grids = modalEl.querySelectorAll('.smart-disk-grid');
        if (grids.length === 0) return;
        const claimed = {}; // value → field label
        grids.forEach(grid => {
            const fieldKey = grid.dataset.forField;
            const sel = modalEl.querySelector(`select[data-field-key="${CSS.escape(fieldKey)}"]`);
            if (!sel) return;
            const label = this._fieldLabelFor(modalEl, fieldKey);
            Array.from(sel.selectedOptions).forEach(o => {
                if (o.value) claimed[o.value] = label;
            });
        });
        grids.forEach(grid => {
            const fieldKey = grid.dataset.forField;
            const label = this._fieldLabelFor(modalEl, fieldKey);
            grid.querySelectorAll('.smart-disk-card').forEach(card => {
                const v = card.dataset.value;
                const owner = claimed[v];
                const selfSelected = card.classList.contains('smart-disk-card-selected');
                const conflictEl = card.querySelector('.smart-disk-card-conflict');
                if (owner && owner !== label && !selfSelected) {
                    card.classList.add('smart-disk-card-conflict-disabled');
                    if (conflictEl) conflictEl.textContent = `In use as ${owner}`;
                } else {
                    card.classList.remove('smart-disk-card-conflict-disabled');
                    if (conflictEl) conflictEl.textContent = '';
                }
            });
            this._updateDiskSelectionCount(modalEl, fieldKey);
        });
    },

    _fieldLabelFor(modalEl, fieldKey) {
        const sel = modalEl.querySelector(`select[data-field-key="${CSS.escape(fieldKey)}"]`);
        if (!sel) return fieldKey;
        const labelEl = modalEl.querySelector(`label[for="${sel.id}"]`);
        return labelEl ? labelEl.textContent.trim().replace(/\s*\*$/, '') : fieldKey;
    },

    _updateDiskSelectionCount(modalEl, fieldKey) {
        const sel = modalEl.querySelector(`select[data-field-key="${CSS.escape(fieldKey)}"]`);
        const counter = modalEl.querySelector(`.smart-disk-grid-count[data-count-for="${CSS.escape(fieldKey)}"]`);
        if (!sel || !counter) return;
        const n = Array.from(sel.selectedOptions).filter(o => o.value).length;
        counter.textContent = n > 0 ? `${n} selected` : '';
    },

    // ─── options_from loader ─────────────────────────────────────────────

    async _loadFieldOptions(modalEl, field, addonId, row) {
        const sel = modalEl.querySelector(`[data-field-key="${field.key.replace(/"/g, '\\"')}"]`);
        if (!sel) return;
        if (!addonId) return;

        let path = field.options_from;
        if (typeof path !== 'string') return;
        // Prefer the row's agent_id (cross-agent aggregated tables) over the
        // page-level selector so dropdowns show options from the host that
        // owns the row being acted on.
        const agentId = this._resolveAgentId(row);
        if (agentId) {
            const sep = path.includes('?') ? '&' : '?';
            path += `${sep}agent_id=${encodeURIComponent(agentId)}`;
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

            // `option_meta` maps item-fields → data-* attributes so downstream
            // code (capacity preview, grouped disk picker) can read metadata
            // without re-fetching. Declare `{"size": "size-bytes"}` on disk
            // pickers to make the capacity calculator work.
            const metaMap = { ...(field.option_meta || {}) };

            const opts = items.map(it => {
                const raw = it[valueKey];
                const v = String(raw ?? '');
                let l = String(it[labelKey] ?? v);
                if (detailKey && it[detailKey]) l += ` — ${it[detailKey]}`;
                const attrs = Object.entries(metaMap)
                    .filter(([k]) => it[k] != null && it[k] !== '')
                    .map(([k, attr]) => `data-${attr}="${Utils.escapeHtml(String(it[k]))}"`)
                    .join(' ');
                const t = typeof raw;
                const typeAttr = (t === 'number' || t === 'boolean') ? ` data-value-type="${t}"` : '';
                return `<option value="${Utils.escapeHtml(v)}"${typeAttr} ${attrs}>${Utils.escapeHtml(l)}</option>`;
            }).join('');

            // Single-select: leading blank if not required.
            const blank = (field.type === 'select' && !field.required) ? `<option value="">— unchanged —</option>` : '';
            sel.innerHTML = blank + opts;

            // Cards variant: render the visible grid from the same items. The
            // hidden <select> remains the source of truth for form collection.
            if (sel.dataset.variant === 'cards') {
                this._renderDiskCards(modalEl, field, items, metaMap);
            }

            // Capacity preview may need to recalc now that size metadata is available.
            this._updateCapacityPreviews(modalEl, []);
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

        const path = this._appendAgentIdToPath(preview.endpoint, ctx.row);

        try {
            // Preview endpoints are read-only POSTs by convention (they receive
            // a JSON body describing the desired change and return a `command`
            // string). Don't inherit the parent action's method — PUT/DELETE
            // would 405 against a handler registered for POST.
            const method = (preview.method || 'POST').toUpperCase();
            const proxyURL = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                + (method !== 'GET' && method !== 'POST' ? `&method=${method}` : '');
            const init = { method: method === 'GET' ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' } };
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

    // Standalone in-form action button (e.g. "Test Connection"). Validates
    // that `require_fields` are filled, POSTs the current form values to the
    // agent endpoint through the add-on proxy, and renders the JSON response
    // inline below the button. Never closes the modal or refreshes tables —
    // this is a diagnostic, not a write.
    async _runTestButton(btnEl) {
        const modalEl = btnEl.closest('.smart-action-modal');
        if (!modalEl) return;
        const ctx = modalEl._smartAction;
        if (!ctx) return;
        const entry = this._tables[ctx.compId];
        if (!entry || !entry.addonId) return;

        const fieldKey = btnEl.dataset.fieldKey || '';
        const resultEl = modalEl.querySelector(
            `.smart-test-button-result[data-result-for="${CSS.escape(fieldKey)}"]`
        );
        const endpoint = btnEl.dataset.endpoint || '';
        const method = (btnEl.dataset.method || 'POST').toUpperCase();
        const requireKeys = (btnEl.dataset.requireFields || '')
            .split(',').map(s => s.trim()).filter(Boolean);

        const formValues = this._collectFormValues(modalEl);
        const missing = requireKeys.filter(k => {
            const v = formValues[k];
            return v == null || v === '' || (Array.isArray(v) && v.length === 0);
        });
        if (missing.length && resultEl) {
            resultEl.className = 'smart-test-button-result smart-test-button-result-error';
            resultEl.textContent = `Fill required fields first: ${missing.join(', ')}`;
            return;
        }

        if (resultEl) {
            resultEl.className = 'smart-test-button-result smart-test-button-result-pending';
            resultEl.textContent = 'Testing…';
        }
        const origLabel = btnEl.textContent;
        btnEl.disabled = true;
        btnEl.textContent = 'Testing…';

        try {
            const path = this._appendAgentIdToPath(endpoint, ctx.row);
            const proxyURL = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                + (method !== 'GET' && method !== 'POST' ? `&method=${method}` : '');
            const init = {
                method: method === 'GET' ? 'GET' : 'POST',
                headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' }
            };
            if (init.method !== 'GET') init.body = JSON.stringify(formValues);
            const resp = await fetch(proxyURL, init);
            const data = await resp.json().catch(() => ({}));
            if (resultEl) {
                const ok = resp.ok && data.ok !== false;
                resultEl.className = `smart-test-button-result smart-test-button-result-${ok ? 'ok' : 'error'}`;
                const successMsg = data.message
                    || (data.dataset ? `Connected — remote dataset ${data.dataset} is reachable` : 'Success');
                const errorMsg = data.message || data.error || `Request failed (HTTP ${resp.status})`;
                resultEl.textContent = ok ? successMsg : errorMsg;
            }
        } catch (e) {
            if (resultEl) {
                resultEl.className = 'smart-test-button-result smart-test-button-result-error';
                resultEl.textContent = `Request failed: ${e.message}`;
            }
        } finally {
            btnEl.disabled = false;
            btnEl.textContent = origLabel;
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

        // Client-side cron validation for any field declared as type=cron or
        // a select with `allow_custom` + `custom_input_type: "cron"`. For
        // select+cron the substituted value only reaches here when the user
        // chose __custom__; presets are always valid so they fall through.
        for (const f of (ctx.cfg.fields || [])) {
            const isCronField = f.type === 'cron'
                || (f.type === 'select' && f.allow_custom && f.custom_input_type === 'cron');
            if (!isCronField) continue;
            const raw = String(formValues[f.key] || '').trim();
            if (!raw) {
                if (f.required) { errEl.textContent = `${f.label || f.key} is required.`; return; }
                continue;
            }
            const desc = this._describeCron(raw);
            if (desc.startsWith('Invalid')) { errEl.textContent = `${f.label || f.key}: ${desc}`; return; }
        }

        const path = this._appendAgentIdToPath(ctx.action.action.endpoint, ctx.row);
        const method = (ctx.action.action.method || 'POST').toUpperCase();

        submitBtn.disabled = true;
        const origLabel = submitBtn.textContent;
        submitBtn.textContent = 'Working…';
        try {
            const proxyURL = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                + (method !== 'GET' && method !== 'POST' ? `&method=${method}` : '');
            const init = { method: method === 'GET' ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' } };
            if (init.method !== 'GET') init.body = JSON.stringify(body);
            const resp = await fetch(proxyURL, init);
            const data = await resp.json().catch(() => ({}));
            if (!resp.ok) {
                const serverMsg = (data && (data.error || data.message)) || `Request failed (HTTP ${resp.status})`;
                errEl.textContent = serverMsg;
                if (data && data.command) {
                    errEl.textContent += `\nCommand: ${data.command}`;
                }
                const errorTmpl = ctx.cfg.error_message;
                const toastMsg = errorTmpl
                    ? this._interpolate(errorTmpl, { ...(ctx.row || {}), ...formValues, error: serverMsg })
                    : serverMsg;
                Utils.toast(toastMsg, 'error');
                return;
            }
            // Async actions: agent returned a job handle instead of a final
            // result. Swap the modal into a progress overlay that polls the
            // job endpoint until a terminal status, then close + toast.
            // Manifest opts in via `action.async: true`. Response must carry
            // `{job_id}`; optional `action.poll_endpoint` / `poll_interval_seconds`
            // / `cancel_endpoint` / `cancel_method` overrides control the
            // polling + cancel paths (defaults: `/api/jobs/{job_id}`, 2 s,
            // DELETE to the same path).
            if (ctx.action.action.async && data && data.job_id) {
                this._startProgressOverlay(modalEl, ctx, formValues, data.job_id);
                return;
            }

            Modals.close(submitBtn);
            const successTmpl = ctx.cfg.success_message;
            const successMsg = successTmpl
                ? this._interpolate(successTmpl, { ...(ctx.row || {}), ...formValues })
                : `${ctx.action.label || 'Action'} completed`;
            Utils.toast(successMsg, 'success');
            // Refresh every smart-table on the page that belongs to the same
            // add-on. Creating a pool makes a root dataset appear in the
            // datasets table; deleting a dataset removes entries from the
            // snapshots table; etc. Users shouldn't have to hit refresh.
            //
            // Double refresh: immediate + delayed. Add-ons typically refresh
            // their local state asynchronously after a write (zfs-manager's
            // agent does `go refreshAndFlush()`), so the hub-side cache the
            // first refresh reads from may still be stale. The delayed one
            // catches the freshly-pushed frame without needing the user to
            // click refresh.
            const addonId = entry.addonId;
            const refreshAll = () => {
                Object.keys(this._tables).forEach(id => {
                    if (this._tables[id] && this._tables[id].addonId === addonId) this.refresh(id);
                });
            };
            refreshAll();
            setTimeout(refreshAll, 1500);
        } catch (e) {
            errEl.textContent = `Request failed: ${e.message}`;
            Utils.toast(`Request failed: ${e.message}`, 'error');
        } finally {
            submitBtn.disabled = false;
            submitBtn.textContent = origLabel;
        }
    },

    // ─── Async progress overlay ──────────────────────────────────────────
    //
    // Generic in-modal progress reporter for actions declared with
    // `action.async: true`. Replaces the form body (keeping the modal
    // chrome) with a progress bar + phase/ETA readout, polls the job
    // endpoint on `poll_interval_seconds` (default 2), and closes the
    // modal + toasts on terminal status. Cancel button fires the
    // `cancel_endpoint` (default `DELETE /api/jobs/{job_id}`); no-op on
    // already-terminal jobs.
    //
    // Expected poll response shape — all fields optional except `status`:
    //   { status, phase, phase_detail, progress_percent, message,
    //     fail_reason, eta_sec, elapsed_sec }
    // Terminal statuses: completed, failed, cancelled (plus defensive
    // aliases: succeeded, success, done, error).
    _startProgressOverlay(modalEl, ctx, formValues, jobId) {
        const entry = this._tables[ctx.compId];
        if (!entry || !entry.addonId) return;
        const cfg = ctx.action.action;
        const pollPath = cfg.poll_endpoint || '/api/jobs/{job_id}';
        const cancelPath = cfg.cancel_endpoint || '/api/jobs/{job_id}';
        const cancelMethod = (cfg.cancel_method || 'DELETE').toUpperCase();
        const intervalMs = Math.max(500,
            Number(cfg.poll_interval_seconds) ? Number(cfg.poll_interval_seconds) * 1000 : 2000);
        const interpCtx = { ...(ctx.row || {}), ...formValues, form: formValues, job_id: jobId };

        // Replace the whole modal-body with the overlay — the form, preview
        // pane, and error region are all superseded.
        const body = modalEl.querySelector('.modal-body');
        const cancelDisabled = cfg.cancel_endpoint === false || cfg.cancelable === false;
        if (body) {
            body.innerHTML = `
                <div class="smart-progress-overlay">
                    <div class="smart-progress-phase">Queued…</div>
                    <div class="smart-progress-bar-outer">
                        <div class="smart-progress-bar-inner" style="width: 0%"></div>
                    </div>
                    <div class="smart-progress-meta">
                        <span class="smart-progress-percent">0%</span>
                        <span class="smart-progress-elapsed"></span>
                        <span class="smart-progress-eta"></span>
                    </div>
                    <div class="smart-progress-message"></div>
                    <div class="smart-progress-error" hidden></div>
                </div>`;
        }

        // The footer Cancel (close modal without running cancel API) stays
        // available as a "detach" option. The primary button becomes the
        // in-flight cancel.
        const submitBtn = modalEl.querySelector('#smart-action-submit');
        if (submitBtn) {
            submitBtn.textContent = 'Cancel Operation';
            submitBtn.className = 'btn btn-warning smart-progress-cancel';
            submitBtn.disabled = !!cancelDisabled;
            submitBtn.onclick = async () => {
                submitBtn.disabled = true;
                submitBtn.textContent = 'Cancelling…';
                try {
                    const path = this._interpolate(cancelPath, interpCtx);
                    const url = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`
                        + (cancelMethod !== 'GET' && cancelMethod !== 'POST' ? `&method=${cancelMethod}` : '');
                    const init = {
                        method: cancelMethod === 'GET' ? 'GET' : cancelMethod === 'POST' ? 'POST' : 'POST',
                        headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' }
                    };
                    const r = await fetch(url, init);
                    if (!r.ok) throw new Error(`HTTP ${r.status}`);
                    // Don't change status locally — the next poll will see
                    // `cancelled` and trigger the terminal branch.
                } catch (e) {
                    submitBtn.disabled = false;
                    submitBtn.textContent = 'Cancel Operation';
                    const errBox = modalEl.querySelector('.smart-progress-error');
                    if (errBox) {
                        errBox.hidden = false;
                        errBox.textContent = `Cancel failed: ${e.message}`;
                    }
                }
            };
        }

        // Poll loop. Stored on the modal so we can clear on close; also
        // self-clears when the modal is detached from the DOM.
        let pollBusy = false;
        const tick = async () => {
            if (!modalEl.isConnected) {
                clearInterval(modalEl._progressTimer);
                modalEl._progressTimer = null;
                return;
            }
            if (pollBusy) return;
            pollBusy = true;
            try {
                const path = this._interpolate(pollPath, interpCtx);
                const url = `/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`;
                const r = await fetch(url, { headers: { 'X-Requested-With': 'XMLHttpRequest' } });
                if (!r.ok) {
                    if (r.status === 404) {
                        // Job vanished (GC'd after completion in some agents).
                        this._finishProgressOverlay(modalEl, ctx, formValues,
                            { status: 'completed', message: 'Job finished' });
                    }
                    return;
                }
                const state = await r.json().catch(() => ({}));
                this._renderProgressState(modalEl, state);
                if (this._isTerminalStatus(state.status)) {
                    this._finishProgressOverlay(modalEl, ctx, formValues, state);
                }
            } catch (_) {
                // Transient network failures — keep polling.
            } finally {
                pollBusy = false;
            }
        };
        modalEl._progressTimer = setInterval(tick, intervalMs);
        tick(); // fire immediately so the first frame isn't blank for `intervalMs`
    },

    _isTerminalStatus(status) {
        if (!status) return false;
        const s = String(status).toLowerCase();
        return s === 'completed' || s === 'failed' || s === 'cancelled' || s === 'canceled'
            || s === 'succeeded' || s === 'success' || s === 'done' || s === 'error';
    },

    _renderProgressState(modalEl, state) {
        const phaseEl = modalEl.querySelector('.smart-progress-phase');
        const barEl = modalEl.querySelector('.smart-progress-bar-inner');
        const pctEl = modalEl.querySelector('.smart-progress-percent');
        const elapsedEl = modalEl.querySelector('.smart-progress-elapsed');
        const etaEl = modalEl.querySelector('.smart-progress-eta');
        const msgEl = modalEl.querySelector('.smart-progress-message');
        if (!phaseEl) return;

        const pct = Number(state.progress_percent ?? state.progress ?? state.percent);
        const pctClamped = Number.isFinite(pct) ? Math.max(0, Math.min(100, pct)) : 0;
        if (barEl) barEl.style.width = `${pctClamped}%`;
        if (pctEl) pctEl.textContent = Number.isFinite(pct) ? `${pctClamped.toFixed(pctClamped < 10 ? 1 : 0)}%` : '';

        const phase = state.phase ? String(state.phase) : '';
        const phaseDetail = state.phase_detail ? String(state.phase_detail) : '';
        const phaseText = [phase, phaseDetail].filter(Boolean).join(' — ');
        if (phaseEl) phaseEl.textContent = phaseText || (state.status || 'Running');

        if (elapsedEl) elapsedEl.textContent = state.elapsed_sec != null
            ? `Elapsed ${this._formatDuration(state.elapsed_sec)}` : '';
        if (etaEl) etaEl.textContent = state.eta_sec != null && state.eta_sec > 0
            ? `ETA ${this._formatDuration(state.eta_sec)}` : '';
        if (msgEl) msgEl.textContent = state.message || '';
    },

    _finishProgressOverlay(modalEl, ctx, formValues, state) {
        if (modalEl._progressTimer) {
            clearInterval(modalEl._progressTimer);
            modalEl._progressTimer = null;
        }
        const submitBtn = modalEl.querySelector('#smart-action-submit');
        if (submitBtn) {
            submitBtn.textContent = 'Close';
            submitBtn.className = 'btn btn-primary';
            submitBtn.disabled = false;
            submitBtn.onclick = () => Modals.close(submitBtn);
        }

        const s = String(state.status || '').toLowerCase();
        const ok = s === 'completed' || s === 'succeeded' || s === 'success' || s === 'done';
        const errBox = modalEl.querySelector('.smart-progress-error');
        if (!ok && errBox) {
            errBox.hidden = false;
            errBox.textContent = state.fail_reason || state.error || state.message || `Job ${s || 'failed'}`;
        }
        // Fill final progress state one last time for the visual.
        this._renderProgressState(modalEl, state);

        const entry = this._tables[ctx.compId];
        if (ok) {
            const successTmpl = ctx.cfg.success_message;
            const successMsg = successTmpl
                ? this._interpolate(successTmpl, { ...(ctx.row || {}), ...formValues })
                : `${ctx.action.label || 'Action'} completed`;
            Utils.toast(successMsg, 'success');
        } else {
            const reason = state.fail_reason || state.error || state.message || 'failed';
            Utils.toast(`${ctx.action.label || 'Action'} ${s || 'failed'}: ${reason}`, 'error');
        }
        if (entry && entry.addonId) {
            const addonId = entry.addonId;
            const refreshAll = () => {
                Object.keys(this._tables).forEach(id => {
                    if (this._tables[id] && this._tables[id].addonId === addonId) this.refresh(id);
                });
            };
            refreshAll();
            setTimeout(refreshAll, 1500);
        }
    },

    _collectFormValues(modalEl) {
        const out = {};
        modalEl.querySelectorAll('[data-field-key]').forEach(el => {
            const key = el.dataset.fieldKey;
            if (!key) return;
            // Skip non-input elements that happen to carry data-field-key
            // (e.g. test_button, smart-disk-grid wrappers).
            if (el.tagName === 'BUTTON') return;
            // Skip fields hidden by `visible_when` — the user never chose them.
            const group = el.closest('.form-group[data-visible-when]');
            if (group && group.style.display === 'none') return;
            let value;
            if (el.type === 'checkbox') {
                value = el.checked;
            } else if (el.tagName === 'SELECT' && el.multiple) {
                value = Array.from(el.selectedOptions).map(o => {
                    const t = o.dataset.valueType;
                    if (t === 'number') return Number(o.value);
                    if (t === 'boolean') return o.value === 'true';
                    return o.value;
                }).filter(v => v !== '');
            } else if (el.tagName === 'SELECT') {
                const opt = el.selectedOptions[0];
                const t = opt && opt.dataset.valueType;
                if (el.value === '') value = '';
                else if (t === 'number') { const n = Number(el.value); value = Number.isFinite(n) ? n : el.value; }
                else if (t === 'boolean') value = el.value === 'true';
                else value = el.value;
            } else if (el.type === 'number') {
                const n = el.value === '' ? null : Number(el.value);
                value = Number.isFinite(n) ? n : null;
            } else {
                value = el.value;
            }
            // `allow_custom` sentinel → substitute the sibling text input.
            if (value === '__custom__' && el.dataset.allowCustom === '1') {
                const custom = modalEl.querySelector(`.smart-custom-input[data-custom-for="${CSS.escape(key)}"]`);
                value = custom ? custom.value : '';
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
