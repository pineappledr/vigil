/**
 * Vigil Dashboard - Discovery Card Component
 *
 * Renders a TrueNAS-style callout when the agent reports unused disks.
 * Two primary actions:
 *   - "Create New Pool" — scrolls to and opens the existing smart-table
 *     create-pool toolbar action (manifest: toolbar_actions[id=create-pool]).
 *   - "Add to Existing Pool" — opens an inline modal that picks a pool +
 *     a vdev type and submits to /api/pool/add-vdev.
 *
 * Config schema (from manifest):
 *   source                  - "/api/disks" (or any endpoint returning an array of disks)
 *   poll_interval_seconds   - polling cadence, default 60
 *   target_table            - compId of the smart-table to reuse for the
 *                             Create Pool flow (it must declare a
 *                             toolbar_action with id "create-pool")
 *   expand_endpoint         - defaults to "/api/pool/add-vdev"
 *   pools_endpoint          - defaults to "/api/pools"
 */

const DiscoveryCardComponent = {
    _cards: {},  // compId → { config, addonId, disks, timer }

    render(compId, config, addonId) {
        this._cards[compId] = {
            config: config || {},
            addonId,
            disks: [],
            timer: null
        };

        // Polling is kicked off after the DOM mounts so first paint is
        // "Scanning…" rather than an empty container.
        setTimeout(() => this._startPolling(compId), 0);

        return `
            <div class="discovery-card-container" id="discovery-card-${Utils.escapeHtml(compId)}">
                <div class="discovery-card" id="discovery-card-body-${Utils.escapeHtml(compId)}" style="display:none">
                    <div class="discovery-card-icon">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="24" height="24">
                            <rect x="2" y="4" width="20" height="16" rx="2"/>
                            <line x1="2" y1="10" x2="22" y2="10"/>
                            <circle cx="7" cy="15" r="1.5"/>
                        </svg>
                    </div>
                    <div class="discovery-card-body">
                        <div class="discovery-card-title" id="discovery-card-title-${Utils.escapeHtml(compId)}">Unused drives detected</div>
                        <div class="discovery-card-summary" id="discovery-card-summary-${Utils.escapeHtml(compId)}"></div>
                    </div>
                    <div class="discovery-card-actions">
                        <button class="btn btn-primary" onclick="DiscoveryCardComponent._onCreatePool('${this._escapeJS(compId)}')">Create New Pool</button>
                        <button class="btn btn-secondary" onclick="DiscoveryCardComponent._onAddToPool('${this._escapeJS(compId)}')">Add to Existing Pool</button>
                    </div>
                </div>
            </div>
        `;
    },

    refresh(compId) {
        this._fetchOnce(compId);
    },

    _startPolling(compId) {
        const entry = this._cards[compId];
        if (!entry) return;
        this._fetchOnce(compId);
        const seconds = Number(entry.config.poll_interval_seconds || 60);
        if (entry.timer) clearInterval(entry.timer);
        entry.timer = setInterval(() => this._fetchOnce(compId), Math.max(seconds, 15) * 1000);
    },

    async _fetchOnce(compId) {
        const entry = this._cards[compId];
        if (!entry || !entry.addonId) return;
        const path = entry.config.source || '/api/disks';
        try {
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(path)}`);
            if (!resp.ok) {
                this._hide(compId);
                return;
            }
            const data = await resp.json();
            const disks = Array.isArray(data) ? data : (data && data.items) || [];
            entry.disks = disks;
            this._render(compId);
        } catch {
            this._hide(compId);
        }
    },

    _render(compId) {
        const entry = this._cards[compId];
        if (!entry) return;
        const body = document.getElementById(`discovery-card-body-${compId}`);
        if (!body) return;
        if (!entry.disks || entry.disks.length === 0) {
            body.style.display = 'none';
            return;
        }
        body.style.display = '';
        const title = document.getElementById(`discovery-card-title-${compId}`);
        const summary = document.getElementById(`discovery-card-summary-${compId}`);
        if (title) title.textContent = `${entry.disks.length} unused drive${entry.disks.length === 1 ? '' : 's'} detected`;
        if (summary) summary.innerHTML = this._summarize(entry.disks);
    },

    _hide(compId) {
        const body = document.getElementById(`discovery-card-body-${compId}`);
        if (body) body.style.display = 'none';
    },

    _summarize(disks) {
        // Group by size+model so "4 MiB SSD × 2" renders for uniform sets.
        const groups = new Map();
        for (const d of disks) {
            const size = d.size ? this._formatBytes(d.size) : '?';
            const key = `${size}|${d.model || 'unknown model'}`;
            groups.set(key, (groups.get(key) || 0) + 1);
        }
        return Array.from(groups.entries())
            .map(([key, count]) => {
                const [size, model] = key.split('|');
                return `<span class="discovery-card-group">${Utils.escapeHtml(size)} ${Utils.escapeHtml(model)} × ${count}</span>`;
            })
            .join('');
    },

    _onCreatePool(compId) {
        const entry = this._cards[compId];
        if (!entry) return;
        // Find the create-pool toolbar button on the target smart-table and
        // click it. Prefill via SmartTableComponent's optional hook if
        // present; otherwise the user selects disks manually.
        const targetTable = entry.config.target_table || 'pool-list';
        const btn = document.querySelector(`#smart-table-${targetTable} .smart-toolbar-actions button[data-action-id="create-pool"]`);
        if (btn) {
            btn.scrollIntoView({ behavior: 'smooth', block: 'center' });
            btn.click();
            return;
        }
        // Fallback: jump to first create-pool button anywhere in the dashboard.
        const anyBtn = document.querySelector('.smart-toolbar-actions button[data-action-id="create-pool"]');
        if (anyBtn) {
            anyBtn.scrollIntoView({ behavior: 'smooth', block: 'center' });
            anyBtn.click();
        }
    },

    async _onAddToPool(compId) {
        const entry = this._cards[compId];
        if (!entry) return;
        // Fetch pools for the selector. Show a loading-state modal first so
        // the click feels responsive on slow links.
        const poolsPath = entry.config.pools_endpoint || '/api/pools';
        let pools = [];
        try {
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(poolsPath)}`);
            if (resp.ok) {
                const data = await resp.json();
                pools = Array.isArray(data) ? data : (data && data.items) || [];
            }
        } catch {
            pools = [];
        }
        this._openAddToPoolModal(compId, pools);
    },

    _openAddToPoolModal(compId, pools) {
        const entry = this._cards[compId];
        if (!entry) return;
        const disks = entry.disks || [];

        const poolOpts = pools.length > 0
            ? pools.map(p => `<option value="${Utils.escapeHtml(p.name || p.id || '')}">${Utils.escapeHtml(p.name || '')}</option>`).join('')
            : `<option value="" disabled>No existing pools</option>`;

        const diskCards = disks.map(d => `
            <div class="smart-disk-card smart-disk-card-selected" data-value="${Utils.escapeHtml(d.path || '')}" onclick="DiscoveryCardComponent._toggleCard(this)">
                <div class="smart-disk-card-check"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" width="14" height="14"><polyline points="20 6 9 17 4 12"/></svg></div>
                <div class="smart-disk-card-body">
                    <div class="smart-disk-card-name">${Utils.escapeHtml(d.name || d.path || '')}</div>
                    ${d.model ? `<div class="smart-disk-card-detail">${Utils.escapeHtml(d.model)}</div>` : ''}
                    <div class="smart-disk-card-meta">${d.size ? `<span>${Utils.escapeHtml(this._formatBytes(d.size))}</span>` : ''}</div>
                </div>
            </div>`).join('');

        const modal = Modals.create(`
            <div class="modal smart-action-modal smart-action-modal-wide">
                <div class="modal-header">
                    <h3>Add Drives to Existing Pool</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                    </button>
                </div>
                <div class="modal-body">
                    <form class="smart-action-form" onsubmit="return false;">
                        <div class="form-group">
                            <label class="form-label">Target Pool <span class="form-req">*</span></label>
                            <select class="form-input" id="discovery-target-pool">${poolOpts}</select>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Vdev Type <span class="form-req">*</span></label>
                            <select class="form-input" id="discovery-vdev-type">
                                <option value="mirror">Mirror — safer, lose 50% capacity</option>
                                <option value="raidz1">RAIDZ1 — survive 1 failure</option>
                                <option value="raidz2">RAIDZ2 — survive 2 failures</option>
                                <option value="raidz3">RAIDZ3 — survive 3 failures</option>
                                <option value="stripe">Stripe — no redundancy (not recommended)</option>
                            </select>
                            <div class="form-hint">Matching the pool's existing vdev type is strongly recommended.</div>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Drives (click to deselect)</label>
                            <div class="smart-disk-grid">${diskCards}</div>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Type the pool name to confirm <span class="form-req">*</span></label>
                            <input type="text" class="form-input" id="discovery-confirm" placeholder="pool name">
                            <div class="form-hint">Adding a vdev is irreversible — it cannot be removed without destroying the pool.</div>
                        </div>
                        <div class="form-error" id="discovery-error"></div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-danger" onclick="DiscoveryCardComponent._submitAddToPool('${this._escapeJS(compId)}', this)">Add Drives</button>
                </div>
            </div>
        `);
        modal._discoveryCompId = compId;
    },

    _toggleCard(cardEl) {
        cardEl.classList.toggle('smart-disk-card-selected');
    },

    async _submitAddToPool(compId, btnEl) {
        const entry = this._cards[compId];
        if (!entry) return;
        const modal = btnEl.closest('.modal');
        if (!modal) return;
        const errEl = modal.querySelector('#discovery-error');
        const setError = (m) => { if (errEl) errEl.textContent = m; };
        setError('');

        const pool = modal.querySelector('#discovery-target-pool').value;
        const vdevType = modal.querySelector('#discovery-vdev-type').value;
        const confirm = modal.querySelector('#discovery-confirm').value.trim();
        const devices = Array.from(modal.querySelectorAll('.smart-disk-card.smart-disk-card-selected'))
            .map(c => c.dataset.value)
            .filter(Boolean);

        if (!pool) { setError('Pick a pool.'); return; }
        if (devices.length === 0) { setError('Select at least one drive.'); return; }
        if (confirm !== pool) { setError('Type the pool name exactly to confirm.'); return; }

        btnEl.disabled = true;
        btnEl.textContent = 'Adding…';
        try {
            const endpoint = entry.config.expand_endpoint || '/api/pool/add-vdev';
            const resp = await fetch(`/api/addons/${entry.addonId}/proxy?path=${encodeURIComponent(endpoint)}&method=POST`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    pool,
                    vdev_type: vdevType,
                    devices,
                    confirm
                })
            });
            if (!resp.ok) {
                let msg = `HTTP ${resp.status}`;
                try {
                    const err = await resp.json();
                    if (err && err.error) msg = err.error;
                } catch { /* keep the default */ }
                setError(msg);
                btnEl.disabled = false;
                btnEl.textContent = 'Add Drives';
                return;
            }
            Modals.close(btnEl);
            // Nudge the pool table to refresh so the new vdev shows up.
            if (typeof SmartTableComponent !== 'undefined') {
                SmartTableComponent.refresh(entry.config.target_table || 'pool-list');
            }
            this._fetchOnce(compId);
        } catch (e) {
            setError(String(e));
            btnEl.disabled = false;
            btnEl.textContent = 'Add Drives';
        }
    },

    _formatBytes(n) {
        if (!Number.isFinite(n) || n <= 0) return '';
        const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
        let i = 0, v = n;
        while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
        return `${v.toFixed(v >= 10 ? 0 : 1)} ${units[i]}`;
    },

    _escapeJS(s) {
        return String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }
};
