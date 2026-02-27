/**
 * Vigil Dashboard - Log Viewer Component (Task 4.7)
 *
 * Scrollable, auto-tailing terminal-style log viewer with severity coloring.
 * Receives log telemetry events from SSE and appends lines.
 */

const LogViewerComponent = {
    _viewers: {},  // keyed by compId → { maxLines, autoScroll }

    /**
     * Render initial log viewer.
     * @param {string} compId - Manifest component ID
     * @param {Object} config - { maxLines?: number, showSource?: boolean }
     * @returns {string} HTML
     */
    render(compId, config) {
        const maxLines = config.maxLines || 500;

        this._viewers[compId] = {
            maxLines,
            autoScroll: true,
            showSource: config.showSource !== false
        };

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
                    <div class="log-empty">Waiting for log output...</div>
                </div>
            </div>
        `;
    },

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
        const timestamp = new Date().toLocaleTimeString('en-US', { hour12: false });
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
