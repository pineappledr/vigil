/**
 * Vigil Dashboard - Disk Storage Component
 *
 * Visual disk storage display with progress bars and color-coded usage.
 * Supports disk aliases for friendly names.
 *
 * Config schema:
 *   source    - Data source identifier (e.g., "disk_storage")
 *   aliases   - Object mapping disk name → friendly alias (e.g., { "d1": "Movies" })
 *   thresholds - { warning: 70, danger: 90 } — usage % thresholds for color coding
 */

const DiskStorageComponent = {
    _instances: {},  // keyed by compId

    /**
     * @param {string} compId  - Manifest component ID
     * @param {Object} config  - Component configuration from manifest
     * @param {number} addonId - Parent add-on ID
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        this._instances[compId] = {
            config: config || {},
            addonId,
            rows: []
        };

        // Fetch data after DOM insertion
        if (config.source && addonId) {
            setTimeout(() => this._fetchSource(compId), 0);
        }

        return `
            <div class="disk-storage-container" id="disk-storage-${this._escapeAttr(compId)}">
                <div class="disk-storage-toolbar">
                    <button class="smart-table-refresh" id="disk-storage-refresh-${this._escapeAttr(compId)}" title="Refresh"
                            onclick="DiskStorageComponent.refresh('${this._escapeJS(compId)}')">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                            <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
                            <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                        </svg>
                    </button>
                </div>
                <div class="disk-storage-grid" id="disk-storage-grid-${this._escapeAttr(compId)}">
                    <div class="disk-storage-loading">Loading disk data...</div>
                </div>
            </div>
        `;
    },

    /** Public refresh — re-fetches source data. */
    refresh(compId) {
        const btn = document.getElementById(`disk-storage-refresh-${compId}`);
        if (btn) {
            btn.classList.add('spinning');
            const cleanup = () => btn.classList.remove('spinning');
            this._fetchSource(compId).then(cleanup).catch(cleanup);
            return;
        }
        this._fetchSource(compId);
    },

    /** Direct update for SSE telemetry. */
    update(compId, data) {
        const entry = this._instances[compId];
        if (!entry) return;
        if (Array.isArray(data)) {
            this._renderDisks(compId, entry, data);
        }
    },

    // ─── Data Fetching ────────────────────────────────────────────────────

    async _fetchSource(compId) {
        const entry = this._instances[compId];
        if (!entry || !entry.config.source || !entry.addonId) return;

        const sourceMap = {
            'disk_storage': '/api/disk_storage'
        };

        let path = sourceMap[entry.config.source];
        if (!path) return;

        let agentParam = '';
        // Append agent_id from the page-level agent selector
        if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
            const agentId = ManifestRenderer.getSelectedAgentId();
            if (agentId) {
                agentParam = `agent_id=${encodeURIComponent(agentId)}`;
                const sep = path.includes('?') ? '&' : '?';
                path += `${sep}${agentParam}`;
            }
        }

        try {
            // Fetch disk storage data and config (for aliases) in parallel
            const configPath = '/api/config' + (agentParam ? '?' + agentParam : '');
            const [storageResp, configResp] = await Promise.all([
                fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`),
                fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(configPath)}`)
            ]);

            // Parse aliases from agent config
            if (configResp.ok) {
                try {
                    const configData = await configResp.json();
                    const aliasStr = configData.disk_aliases || '';
                    entry.config.aliases = aliasStr ? this._parseAliases(aliasStr) : {};
                } catch { /* ignore config parse errors */ }
            }

            if (!storageResp.ok) {
                this._showError(compId, `Failed to load data (HTTP ${storageResp.status})`);
                return;
            }

            const data = await storageResp.json();
            if (Array.isArray(data)) {
                this._renderDisks(compId, entry, data);
            }
        } catch (e) {
            console.error(`[DiskStorage] Failed to fetch for ${compId}:`, e);
            const msg = e instanceof SyntaxError
                ? 'Invalid response from add-on (not JSON)'
                : 'Could not reach add-on — check that the add-on URL is reachable';
            this._showError(compId, msg);
        }
    },

    /**
     * Parse alias string like "d1=Movies, d2=TV Shows" into { d1: "Movies", d2: "TV Shows" }.
     * Supports comma-separated or newline-separated entries.
     */
    _parseAliases(str) {
        const aliases = {};
        if (!str) return aliases;
        const parts = str.split(/[,\n]+/);
        for (const part of parts) {
            const trimmed = part.trim();
            if (!trimmed) continue;
            const eqIdx = trimmed.indexOf('=');
            if (eqIdx > 0) {
                const key = trimmed.substring(0, eqIdx).trim();
                const val = trimmed.substring(eqIdx + 1).trim();
                if (key && val) aliases[key] = val;
            }
        }
        return aliases;
    },

    /**
     * Serialize aliases object back to "d1=Movies, d2=TV Shows" string.
     */
    _serializeAliases(aliases) {
        return Object.entries(aliases)
            .filter(([, v]) => v)
            .map(([k, v]) => `${k}=${v}`)
            .join(', ');
    },

    // ─── Alias Editing ────────────────────────────────────────────────────

    /** Called when the edit button on a card is clicked. */
    _startEditAlias(compId, diskName) {
        const entry = this._instances[compId];
        if (!entry) return;

        const aliases = entry.config.aliases || {};
        const currentAlias = aliases[diskName] || '';

        const nameEl = document.getElementById(`ds-name-${compId}-${diskName}`);
        if (!nameEl) return;

        nameEl.innerHTML = `
            <div class="ds-alias-edit">
                <input type="text" class="ds-alias-input" id="ds-alias-input-${compId}-${diskName}"
                       value="${this._escapeAttr(currentAlias)}"
                       placeholder="${this._escapeAttr(diskName)}"
                       maxlength="40" />
                <div class="ds-alias-actions">
                    <button class="ds-alias-btn ds-alias-save" title="Save"
                            onclick="DiskStorageComponent._saveAlias('${this._escapeJS(compId)}','${this._escapeJS(diskName)}')">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" width="14" height="14">
                            <polyline points="20 6 9 17 4 12"/>
                        </svg>
                    </button>
                    <button class="ds-alias-btn ds-alias-cancel" title="Cancel"
                            onclick="DiskStorageComponent._cancelEditAlias('${this._escapeJS(compId)}')">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" width="14" height="14">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                    ${currentAlias ? `<button class="ds-alias-btn ds-alias-remove" title="Remove alias"
                            onclick="DiskStorageComponent._removeAlias('${this._escapeJS(compId)}','${this._escapeJS(diskName)}')">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                            <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
                            <path d="M10 11v6"/><path d="M14 11v6"/>
                        </svg>
                    </button>` : ''}
                </div>
            </div>
        `;

        const input = document.getElementById(`ds-alias-input-${compId}-${diskName}`);
        if (input) {
            input.focus();
            input.select();
            input.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') this._saveAlias(compId, diskName);
                if (e.key === 'Escape') this._cancelEditAlias(compId);
            });
        }
    },

    /** Save the alias and persist to agent config. */
    async _saveAlias(compId, diskName) {
        const entry = this._instances[compId];
        if (!entry) return;

        const input = document.getElementById(`ds-alias-input-${compId}-${diskName}`);
        if (!input) return;

        const newAlias = input.value.trim();
        const aliases = entry.config.aliases || {};

        if (newAlias) {
            aliases[diskName] = newAlias;
        } else {
            delete aliases[diskName];
        }
        entry.config.aliases = aliases;

        await this._persistAliases(compId, entry);
        this._renderDisks(compId, entry, entry.rows);
    },

    /** Remove the alias for a disk. */
    async _removeAlias(compId, diskName) {
        const entry = this._instances[compId];
        if (!entry) return;

        const aliases = entry.config.aliases || {};
        delete aliases[diskName];
        entry.config.aliases = aliases;

        await this._persistAliases(compId, entry);
        this._renderDisks(compId, entry, entry.rows);
    },

    /** Cancel editing and re-render. */
    _cancelEditAlias(compId) {
        const entry = this._instances[compId];
        if (!entry) return;
        this._renderDisks(compId, entry, entry.rows);
    },

    /** Persist aliases to the agent config via POST /api/config. */
    async _persistAliases(compId, entry) {
        if (!entry.addonId) return;

        const aliasStr = this._serializeAliases(entry.config.aliases || {});

        // Hub expects flat: { agent_id, key1: val1, ... }
        const body = { disk_aliases: aliasStr };

        // Include agent_id if selected
        if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
            const agentId = ManifestRenderer.getSelectedAgentId();
            if (agentId) body.agent_id = agentId;
        }

        try {
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent('/api/config')}&method=POST`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });
            if (!resp.ok) {
                console.error(`[DiskStorage] Failed to save aliases: HTTP ${resp.status}`);
            }
        } catch (e) {
            console.error('[DiskStorage] Failed to save aliases:', e);
        }
    },

    // ─── Rendering ────────────────────────────────────────────────────────

    _renderDisks(compId, entry, rows) {
        const grid = document.getElementById(`disk-storage-grid-${compId}`);
        if (!grid) return;

        entry.rows = rows;

        if (!rows || rows.length === 0) {
            grid.innerHTML = '<div class="disk-storage-loading">No disk data yet — agent is collecting, try refreshing</div>';
            return;
        }

        const aliases = entry.config.aliases || {};
        const thresholds = entry.config.thresholds || { warning: 70, danger: 90 };

        // Compute totals
        let totalBytes = 0, usedBytes = 0;
        for (const disk of rows) {
            totalBytes += disk.total_bytes || 0;
            usedBytes += disk.used_bytes || 0;
        }
        const totalPct = totalBytes > 0 ? (usedBytes / totalBytes * 100) : 0;

        grid.innerHTML =
            this._renderSummaryBar(totalBytes, usedBytes, totalPct, rows.length, thresholds) +
            '<div class="disk-storage-cards">' +
            rows.map(disk => this._renderDiskCard(compId, disk, aliases, thresholds)).join('') +
            '</div>';
    },

    _renderSummaryBar(totalBytes, usedBytes, pct, diskCount, thresholds) {
        const freeBytes = totalBytes - usedBytes;
        const colorClass = this._usageColorClass(pct, thresholds);
        return `
            <div class="disk-storage-summary">
                <div class="disk-storage-summary-info">
                    <span class="disk-storage-summary-label">${diskCount} disk${diskCount !== 1 ? 's' : ''}</span>
                    <span class="disk-storage-summary-usage">
                        ${this._formatBytes(usedBytes)} used of ${this._formatBytes(totalBytes)}
                        <span class="disk-storage-summary-free">(${this._formatBytes(freeBytes)} free)</span>
                    </span>
                </div>
                <div class="disk-storage-summary-bar">
                    <div class="disk-storage-bar-track">
                        <div class="disk-storage-bar-fill ${colorClass}" style="width: ${Math.min(pct, 100).toFixed(1)}%"></div>
                    </div>
                    <span class="disk-storage-pct ${colorClass}">${pct.toFixed(1)}%</span>
                </div>
            </div>
        `;
    },

    _renderDiskCard(compId, disk, aliases, thresholds) {
        const name = disk.name || '';
        const alias = aliases[name] || '';
        const pct = disk.used_pct || 0;
        const colorClass = this._usageColorClass(pct, thresholds);
        const usedBytes = disk.used_bytes || 0;
        const totalBytes = disk.total_bytes || 0;
        const freeBytes = disk.free_bytes || 0;

        const displayName = alias || name;
        const subtitle = alias ? name : '';

        return `
            <div class="disk-storage-card">
                <div class="disk-storage-card-header">
                    <div class="disk-storage-card-name" id="ds-name-${this._escapeAttr(compId)}-${this._escapeAttr(name)}">
                        <div class="disk-storage-name-row">
                            <span class="disk-storage-disk-name">${this._escape(displayName)}</span>
                            <button class="ds-edit-alias-btn" title="Edit alias"
                                    onclick="DiskStorageComponent._startEditAlias('${this._escapeJS(compId)}','${this._escapeJS(name)}')">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
                                    <path d="M17 3a2.85 2.85 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/>
                                    <path d="m15 5 4 4"/>
                                </svg>
                            </button>
                        </div>
                        ${subtitle ? `<span class="disk-storage-disk-id">${this._escape(subtitle)}</span>` : ''}
                    </div>
                    <span class="disk-storage-card-pct ${colorClass}">${pct.toFixed(1)}%</span>
                </div>
                <div class="disk-storage-bar-track">
                    <div class="disk-storage-bar-fill ${colorClass}" style="width: ${Math.min(pct, 100).toFixed(1)}%"></div>
                </div>
                <div class="disk-storage-card-details">
                    <span class="disk-storage-detail">
                        <span class="disk-storage-detail-label">Used</span>
                        <span class="disk-storage-detail-value">${this._formatBytes(usedBytes)}</span>
                    </span>
                    <span class="disk-storage-detail">
                        <span class="disk-storage-detail-label">Free</span>
                        <span class="disk-storage-detail-value">${this._formatBytes(freeBytes)}</span>
                    </span>
                    <span class="disk-storage-detail">
                        <span class="disk-storage-detail-label">Total</span>
                        <span class="disk-storage-detail-value">${this._formatBytes(totalBytes)}</span>
                    </span>
                    ${disk.mount_path ? `<span class="disk-storage-detail disk-storage-detail-mount">
                        <span class="disk-storage-detail-label">Mount</span>
                        <span class="disk-storage-detail-value disk-storage-mount-path">${this._escape(disk.mount_path)}</span>
                    </span>` : ''}
                    ${disk.filesystem ? `<span class="disk-storage-detail">
                        <span class="disk-storage-detail-label">FS</span>
                        <span class="disk-storage-detail-value">${this._escape(disk.filesystem)}</span>
                    </span>` : ''}
                </div>
            </div>
        `;
    },

    _showError(compId, message) {
        const grid = document.getElementById(`disk-storage-grid-${compId}`);
        if (!grid) return;
        grid.innerHTML = `<div class="disk-storage-loading disk-storage-error">${this._escape(message)}</div>`;
    },

    // ─── Helpers ──────────────────────────────────────────────────────────

    _usageColorClass(pct, thresholds) {
        if (pct >= thresholds.danger) return 'ds-danger';
        if (pct >= thresholds.warning) return 'ds-warning';
        return 'ds-ok';
    },

    _formatBytes(bytes) {
        if (typeof bytes !== 'number' || bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = String(str);
        return div.innerHTML;
    },

    _escapeAttr(str) {
        return this._escape(str);
    },

    _escapeJS(str) {
        if (!str) return '';
        return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }
};
