/**
 * Vigil Dashboard - Log Viewer Component (Task 4.7)
 *
 * Scrollable, auto-tailing terminal-style log viewer with severity coloring.
 * Receives log telemetry events from SSE and appends lines.
 *
 * Supports two modes:
 *   1. Live-only — receives SSE log events (default).
 *   2. Source-backed — fetches historical logs from an addon API endpoint
 *      on initial render. Optionally supports a time_filter dropdown that
 *      re-fetches with a ?time_range= query parameter.
 *
 * Config schema:
 *   max_lines      - Maximum retained lines (default 500)
 *   show_source    - Show the source tag per line (default true)
 *   show_timestamp - Show timestamps (default true)
 *   source         - Data source identifier (e.g., "log_history")
 *   time_filter    - { default, options: [{value, label}] }
 */

const LogViewerComponent = {
    _viewers: {},  // keyed by compId → { maxLines, autoScroll, addonId, ... }

    /**
     * Render initial log viewer.
     * @param {string} compId  - Manifest component ID
     * @param {Object} config  - Component configuration from manifest
     * @param {number} addonId - Parent add-on ID (optional, for source fetching)
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        const maxLines = config.max_lines || config.maxLines || 500;

        this._viewers[compId] = {
            maxLines,
            autoScroll: true,
            showSource: config.show_source !== false && config.showSource !== false,
            addonId: addonId || null,
            config: config || {},
            timeRange: config.time_filter ? (config.time_filter.default || '') : ''
        };

        // Time filter dropdown (rendered only when configured)
        const timeFilter = config.time_filter
            ? this._renderTimeFilter(compId, config.time_filter)
            : '';

        // Fetch historical logs after DOM insertion
        if (config.source && addonId) {
            setTimeout(() => this._fetchSource(compId), 0);
        }

        const emptyText = (config.source && addonId)
            ? 'Loading logs...'
            : 'Waiting for log output...';

        return `
            <div class="log-viewer" id="log-viewer-${compId}">
                <div class="log-viewer-toolbar">
                    <div class="log-viewer-filters">
                        <button class="log-filter-btn active" data-level="all" onclick="LogViewerComponent._filterLevel('${compId}', 'all', this)">All</button>
                        <button class="log-filter-btn" data-level="error" onclick="LogViewerComponent._filterLevel('${compId}', 'error', this)">Error</button>
                        <button class="log-filter-btn" data-level="warn" onclick="LogViewerComponent._filterLevel('${compId}', 'warn', this)">Warn</button>
                        <button class="log-filter-btn" data-level="info" onclick="LogViewerComponent._filterLevel('${compId}', 'info', this)">Info</button>
                    </div>
                    <div class="log-viewer-actions">
                        ${timeFilter}
                        <label class="log-autoscroll">
                            <input type="checkbox" checked onchange="LogViewerComponent._toggleAutoScroll('${compId}', this.checked)">
                            Auto-scroll
                        </label>
                        <button class="btn-icon" onclick="LogViewerComponent._clear('${compId}')" title="Clear logs">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                                <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                            </svg>
                        </button>
                    </div>
                </div>
                <div class="log-viewer-body" id="log-body-${compId}">
                    <div class="log-empty">${emptyText}</div>
                </div>
            </div>
        `;
    },

    // ─── Time Filter ──────────────────────────────────────────────────────

    _renderTimeFilter(compId, filterConfig) {
        const options = filterConfig.options || [];
        const defaultVal = filterConfig.default || '';
        return `<select class="smart-time-filter" id="log-time-filter-${compId}"
                        onchange="LogViewerComponent._onTimeFilterChange('${compId}')">
                    ${options.map(opt =>
                        `<option value="${this._escape(opt.value)}"${opt.value === defaultVal ? ' selected' : ''}>${this._escape(opt.label)}</option>`
                    ).join('')}
                </select>`;
    },

    _onTimeFilterChange(compId) {
        const viewer = this._viewers[compId];
        if (!viewer) return;
        const sel = document.getElementById(`log-time-filter-${compId}`);
        if (!sel) return;
        viewer.timeRange = sel.value;
        this._fetchSource(compId);
    },

    // ─── Source Data Fetching ─────────────────────────────────────────────

    async _fetchSource(compId) {
        const viewer = this._viewers[compId];
        if (!viewer || !viewer.config.source || !viewer.addonId) return;

        const sourceMap = {
            'log_history': '/api/logs/history'
        };

        let path = sourceMap[viewer.config.source];
        if (!path) return;

        // Append time_range query parameter if a time filter is active.
        if (viewer.timeRange) {
            const sep = path.includes('?') ? '&' : '?';
            path += `${sep}time_range=${encodeURIComponent(viewer.timeRange)}`;
        }

        try {
            const resp = await fetch(`/api/addons/${viewer.addonId}/proxy?path=${encodeURIComponent(path)}`);
            if (!resp.ok) return;

            const logs = await resp.json();
            if (!Array.isArray(logs)) return;

            // Clear existing content and populate with historical entries.
            const body = document.getElementById(`log-body-${compId}`);
            if (!body) return;
            body.innerHTML = '';

            if (logs.length === 0) {
                body.innerHTML = '<div class="log-empty">No logs in this time range</div>';
                return;
            }

            for (const entry of logs) {
                this._appendLine(compId, entry, viewer);
            }
        } catch (e) {
            console.error(`[LogViewer] Failed to fetch source for ${compId}:`, e);
        }
    },

    // ─── SSE Telemetry Updates ───────────────────────────────────────────

    /**
     * Handle an incoming log telemetry event.
     * @param {Object} payload - LogPayload { level, message, source }
     */
    handleUpdate(payload) {
        if (!payload?.message) return;

        // Broadcast to all active log viewers
        for (const [compId, viewer] of Object.entries(this._viewers)) {
            this._appendLine(compId, payload, viewer);
        }
    },

    _appendLine(compId, payload, viewer) {
        const body = document.getElementById(`log-body-${compId}`);
        if (!body) return;

        // Remove empty placeholder
        const empty = body.querySelector('.log-empty');
        if (empty) empty.remove();

        const level = (payload.level || 'info').toLowerCase();
        // Use the payload timestamp if available (historical), otherwise generate one.
        let timestamp;
        if (payload.timestamp) {
            try {
                timestamp = new Date(payload.timestamp).toLocaleTimeString('en-US', { hour12: false });
            } catch {
                timestamp = new Date().toLocaleTimeString('en-US', { hour12: false });
            }
        } else {
            timestamp = new Date().toLocaleTimeString('en-US', { hour12: false });
        }
        const source = viewer.showSource && payload.source ? payload.source : '';

        const line = document.createElement('div');
        line.className = `log-line log-${level}`;
        line.dataset.level = level;

        line.innerHTML = `
            <span class="log-time">${timestamp}</span>
            <span class="log-level log-level-${level}">${level.toUpperCase()}</span>
            ${source ? `<span class="log-source">[${this._escape(source)}]</span>` : ''}
            <span class="log-msg">${this._escape(payload.message)}</span>
        `;

        body.appendChild(line);

        // Trim excess lines
        while (body.children.length > viewer.maxLines) {
            body.removeChild(body.firstChild);
        }

        // Auto-scroll
        if (viewer.autoScroll) {
            body.scrollTop = body.scrollHeight;
        }
    },

    _filterLevel(compId, level, btn) {
        const viewer = document.getElementById(`log-viewer-${compId}`);
        if (!viewer) return;

        // Update active filter button
        viewer.querySelectorAll('.log-filter-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');

        // Show/hide lines
        const body = document.getElementById(`log-body-${compId}`);
        if (!body) return;

        body.querySelectorAll('.log-line').forEach(line => {
            if (level === 'all' || line.dataset.level === level) {
                line.style.display = '';
            } else {
                line.style.display = 'none';
            }
        });
    },

    _toggleAutoScroll(compId, enabled) {
        const viewer = this._viewers[compId];
        if (viewer) viewer.autoScroll = enabled;
    },

    _clear(compId) {
        const body = document.getElementById(`log-body-${compId}`);
        if (body) {
            body.innerHTML = '<div class="log-empty">Logs cleared</div>';
        }
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};
