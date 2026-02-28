/**
 * Vigil Dashboard - SMART Table Component (Task 4.6)
 *
 * Renders a table of SMART attributes with delta highlighting.
 * Receives telemetry updates via SSE to show live attribute changes.
 */

const SmartTableComponent = {
    _tables: {},  // keyed by compId → { rows, prevRows, config }

    /**
     * @param {string} compId - Manifest component ID
     * @param {Object} config - { columns?: string[], highlight_threshold?: number }
     * @returns {string} HTML
     */
    render(compId, config) {
        this._tables[compId] = { rows: [], prevRows: {}, config: config || {} };

        const columns = config.columns || ['ID', 'Attribute', 'Value', 'Worst', 'Threshold', 'Raw Value'];

        return `
            <div class="smart-table-container" id="smart-table-${compId}">
                <table class="smart-table">
                    <thead>
                        <tr>${columns.map(c => `<th>${this._escape(c)}</th>`).join('')}</tr>
                    </thead>
                    <tbody id="smart-tbody-${compId}">
                        <tr><td colspan="${columns.length}" class="smart-table-empty">Waiting for SMART data...</td></tr>
                    </tbody>
                </table>
            </div>
        `;
    },

    /**
     * Handle an incoming SSE telemetry event with SMART data.
     * Accepts either:
     *  - { attributes: [...] }             (single drive)
     *  - { component_id, attributes: [...] } (targeted)
     *  - raw array of attribute objects
     *
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
        } else {
            return;
        }

        // Route to specific component or broadcast to all
        for (const [compId, entry] of Object.entries(this._tables)) {
            if (targetComp && targetComp !== compId) continue;
            this._updateTable(compId, entry, attributes);
        }
    },

    /**
     * Direct update for a specific component ID.
     * @param {string} compId
     * @param {Array} attributes
     */
    update(compId, attributes) {
        const entry = this._tables[compId];
        if (!entry) return;
        this._updateTable(compId, entry, attributes);
    },

    _updateTable(compId, entry, attributes) {
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
