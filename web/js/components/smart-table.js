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

        return `
            <div class="smart-table-container" id="smart-table-${compId}">
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
                return `<td>${this._formatValue(val, col.format)}</td>`;
            }).join('')}</tr>`;
        }).join('');

        entry.rows = rows;
    },

    _formatValue(val, format) {
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
    }
};
