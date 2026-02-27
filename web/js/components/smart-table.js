/**
 * Vigil Dashboard - SMART Table Component (Task 4.6)
 *
 * Renders a table of SMART attributes with delta highlighting.
 * Designed to show drive health attributes reported by add-on telemetry.
 */

const SmartTableComponent = {
    _tables: {},  // keyed by compId → { rows, prevRows }

    /**
     * Render initial SMART table.
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
     * Update table with new attribute data.
     * @param {string} compId - Component ID
     * @param {Array} attributes - Array of SMART attribute objects
     */
    update(compId, attributes) {
        const entry = this._tables[compId];
        if (!entry) return;

        const tbody = document.getElementById(`smart-tbody-${compId}`);
        if (!tbody) return;

        const threshold = entry.config.highlight_threshold || 0;

        tbody.innerHTML = attributes.map(attr => {
            const prevRaw = entry.prevRows[attr.id || attr.name];
            const currentRaw = attr.raw_value ?? attr.rawValue ?? 0;
            const delta = prevRaw !== undefined ? currentRaw - prevRaw : 0;
            const deltaClass = this._deltaClass(delta, threshold);
            const deltaText = delta !== 0 ? ` (${delta > 0 ? '+' : ''}${delta})` : '';

            return `<tr class="${this._rowClass(attr)}">
                <td class="smart-col-id">${attr.id ?? ''}</td>
                <td class="smart-col-name">${this._escape(attr.name || attr.attribute || '')}</td>
                <td class="smart-col-value">${attr.value ?? ''}</td>
                <td class="smart-col-worst">${attr.worst ?? ''}</td>
                <td class="smart-col-thresh">${attr.threshold ?? attr.thresh ?? ''}</td>
                <td class="smart-col-raw ${deltaClass}">
                    ${currentRaw}${deltaText}
                </td>
            </tr>`;
        }).join('');

        // Store current values for next delta calculation
        entry.prevRows = {};
        for (const attr of attributes) {
            const key = attr.id || attr.name;
            entry.prevRows[key] = attr.raw_value ?? attr.rawValue ?? 0;
        }
        entry.rows = attributes;
    },

    _rowClass(attr) {
        if (attr.failing_now || attr.failingNow) return 'smart-row-critical';
        if (attr.value !== undefined && attr.threshold !== undefined && attr.value <= attr.threshold && attr.threshold > 0) {
            return 'smart-row-warning';
        }
        return '';
    },

    _deltaClass(delta, threshold) {
        if (delta === 0) return '';
        if (Math.abs(delta) >= threshold && threshold > 0) return 'smart-delta-high';
        return delta > 0 ? 'smart-delta-up' : 'smart-delta-down';
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};
