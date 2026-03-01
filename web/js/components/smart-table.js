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
        const columns = config.columns || ['ID', 'Attribute', 'Value', 'Worst', 'Threshold', 'Raw Value'];
        const isStructured = columns.length > 0 && typeof columns[0] === 'object';

        this._tables[compId] = {
            rows: [],
            prevRows: {},
            config: config || {},
            columns,
            isStructured,
            addonId
        };

        const headers = isStructured
            ? columns.map(c => `<th>${this._escape(c.label || c.key)}</th>`).join('')
            : columns.map(c => `<th>${this._escape(c)}</th>`).join('');

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

        return `
            <div class="smart-table-container" id="smart-table-${compId}">
                ${refreshBtn}
                <table class="smart-table">
                    <thead>
                        <tr>${headers}</tr>
                    </thead>
                    <tbody id="smart-tbody-${compId}">
                        <tr><td colspan="${colCount}" class="smart-table-empty">${emptyText}</td></tr>
                    </tbody>
                </table>
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

    // ─── Source Data Fetching ─────────────────────────────────────────────

    async _fetchSource(compId) {
        const entry = this._tables[compId];
        if (!entry || !entry.config.source || !entry.addonId) return;

        // Map source names to addon API paths
        const sourceMap = {
            'addon_agents': '/api/agents',
            'agent_drives': '/api/agents',   // drives come from agents
            'job_history': '/api/jobs/history'
        };

        const path = sourceMap[entry.config.source];
        if (!path) return;

        try {
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`);
            if (!resp.ok) {
                this._showTableError(compId, entry, `Failed to load data (HTTP ${resp.status})`);
                return;
            }

            let data = await resp.json();

            // Transform source data based on source type
            if (entry.config.source === 'agent_drives' && Array.isArray(data)) {
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
            this._showTableError(compId, entry, 'Could not reach add-on — check that the add-on URL is reachable from the Vigil server');
        }
    },

    _showTableError(compId, entry, message) {
        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;
        tbody.innerHTML = `<tr><td colspan="${entry.columns.length}" class="smart-table-empty smart-table-error">${this._escape(message)}</td></tr>`;
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
            tbody.innerHTML = `<tr><td colspan="${entry.columns.length}" class="smart-table-empty">No data</td></tr>`;
            return;
        }

        tbody.innerHTML = rows.map(row => {
            return `<tr>${entry.columns.map(col => {
                const val = row[col.key];
                return `<td>${this._formatValue(val, col.format, row, col)}</td>`;
            }).join('')}</tr>`;
        }).join('');

        entry.rows = rows;
    },

    _formatValue(val, format, row, col) {
        if (format === 'status_dot') {
            return this._formatStatusDot(val);
        }
        if (format === 'actions') {
            return this._formatActions(row, col);
        }

        if (val === undefined || val === null) return '';

        switch (format) {
            case 'bytes':
                return this._formatBytes(val);
            case 'duration':
                return this._formatDuration(val);
            case 'datetime':
                return this._formatDatetime(val);
            case 'relative_time':
                return this._formatRelativeTime(val);
            default:
                return this._escape(String(val));
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

    _formatActions(row, col) {
        if (!col || !col.actions) return '';
        return col.actions.map(action => {
            if (action.type === 'delete') {
                const idKey = action.id_key || 'id';
                const idVal = row[idKey] || '';
                return `<button class="btn-table-action btn-table-delete" title="${this._escape(action.label || 'Delete')}"
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
                <td class="smart-col-name">${this._escape(attr.name || attr.attribute || '')}</td>
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

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = String(str);
        return div.innerHTML;
    },

    _escapeJS(str) {
        if (!str) return '';
        return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }
};
