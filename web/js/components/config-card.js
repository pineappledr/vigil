/**
 * Vigil Dashboard - Config Card Component
 *
 * Displays addon configuration as a read-only key-value summary card.
 * Fetches config from the addon's API and renders it with friendly labels.
 * Includes an edit button that navigates to the Automation tab.
 *
 * Config schema (from manifest):
 *   source       - API source, must be "agent_config"
 *   labels       - Object mapping config keys to display labels, e.g. {"maintenance_cron": "Maintenance Schedule"}
 *   link_page    - Page ID to navigate to when edit is clicked (default: "automation")
 */

const ConfigCardComponent = {
    _cards: {},  // keyed by compId → { config, addonId, data }

    /**
     * @param {string} compId  - Manifest component ID
     * @param {Object} config  - Component configuration from manifest
     * @param {number} addonId - Parent add-on ID
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        this._cards[compId] = { config: config || {}, addonId, data: null };

        if (config.source && addonId) {
            setTimeout(() => this._fetchConfig(compId), 0);
        }

        return `
            <div class="config-card-container" id="config-card-${Utils.escapeHtml(compId)}">
                <div class="config-card-toolbar">
                    <button class="config-card-edit-btn" title="Edit configuration"
                            onclick="ConfigCardComponent._onEdit('${this._escapeJS(compId)}')">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                        </svg>
                    </button>
                </div>
                <div class="config-card-grid" id="config-card-grid-${Utils.escapeHtml(compId)}">
                    <span class="config-card-loading">Loading configuration...</span>
                </div>
            </div>
        `;
    },

    async _fetchConfig(compId) {
        const entry = this._cards[compId];
        if (!entry || !entry.addonId) return;

        try {
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent('/api/config')}`);
            if (!resp.ok) {
                this._showError(compId, `Failed to load config (HTTP ${resp.status})`);
                return;
            }

            const data = await resp.json();
            entry.data = data;
            this._renderGrid(compId);
        } catch (e) {
            console.error(`[ConfigCard] Failed to fetch config for ${compId}:`, e);
            this._showError(compId, 'Could not load configuration');
        }
    },

    _renderGrid(compId) {
        const entry = this._cards[compId];
        const grid = document.getElementById(`config-card-grid-${compId}`);
        if (!grid || !entry || !entry.data) return;

        const labels = entry.config.labels || {};
        const data = entry.data;

        // Filter to only keys that have labels defined (skip internal/unknown keys)
        const keys = Object.keys(labels);
        if (keys.length === 0) {
            // No labels defined — show all keys
            keys.push(...Object.keys(data));
        }

        if (keys.length === 0) {
            grid.innerHTML = '<span class="config-card-loading">No configuration found</span>';
            return;
        }

        grid.innerHTML = keys.map(key => {
            const label = labels[key] || key;
            const raw = data[key];
            const display = this._formatValue(key, raw);
            return `
                <div class="config-card-item">
                    <span class="config-card-label">${Utils.escapeHtml(label)}</span>
                    <span class="config-card-value">${Utils.escapeHtml(display)}</span>
                </div>
            `;
        }).join('');
    },

    _formatValue(key, val) {
        if (val === undefined || val === null || val === '') return 'Not set';
        if (val === 'true') return 'Enabled';
        if (val === 'false') return 'Disabled';
        return String(val);
    },

    _showError(compId, message) {
        const grid = document.getElementById(`config-card-grid-${compId}`);
        if (!grid) return;
        grid.innerHTML = `<span class="config-card-loading config-card-error">${Utils.escapeHtml(message)}</span>`;
    },

    _onEdit(compId) {
        const entry = this._cards[compId];
        const page = entry?.config?.link_page || 'automation';
        if (typeof ManifestRenderer !== 'undefined') {
            ManifestRenderer.switchPage(page);
        }
    },

    refresh(compId) {
        this._fetchConfig(compId);
    },

    _escapeJS(str) {
        if (!str) return '';
        return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }
};
