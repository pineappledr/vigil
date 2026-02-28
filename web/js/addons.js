/**
 * Vigil Dashboard - Add-ons View
 */

const Addons = {
    addons: [],
    tokens: [],
    activeAddonId: null,

    // ─── Data Fetching & Render ──────────────────────────────────────────

    async render() {
        const container = document.getElementById('addons-view');
        if (!container) return;

        container.innerHTML = this._loadingState();

        let fetchError = null;

        try {
            const [addonsResp, tokensResp] = await Promise.all([
                this._fetch('/api/addons'),
                this._fetch('/api/addons/tokens')
            ]);

            if (addonsResp?.ok) {
                const parsed = await addonsResp.json().catch(() => null);
                this.addons = Array.isArray(parsed) ? parsed : [];
            } else {
                this.addons = [];
            }

            if (tokensResp?.ok) {
                const data = await tokensResp.json().catch(() => null);
                this.tokens = Array.isArray(data?.tokens) ? data.tokens : [];
            } else {
                this.tokens = [];
            }
        } catch (e) {
            fetchError = e;
            this.addons = [];
            this.tokens = [];
        } finally {
            // Bail if user navigated away during fetch
            if (State.activeView !== 'addons') return;

            try {
                container.innerHTML = fetchError
                    ? this._errorState(fetchError.message || 'Failed to load add-ons')
                    : this._buildView();
            } catch (e) {
                container.innerHTML = this._emptyState();
            }
        }
    },

    async _fetch(url) {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 8000);
        try {
            return await fetch(url, { signal: controller.signal });
        } catch (e) {
            if (e.name === 'AbortError') throw new Error(`Request timed out`);
            throw e;
        } finally {
            clearTimeout(timer);
        }
    },

    // ─── View States ─────────────────────────────────────────────────────

    _loadingState() {
        return '<div class="loading-spinner"><div class="spinner"></div>Loading add-ons...</div>';
    },

    _emptyState() {
        return `
            <div class="addons-empty">
                ${this._icons.addonLarge}
                <p>No add-ons registered</p>
                <span class="hint">Click "Add Add-on" to register your first add-on</span>
            </div>
        `;
    },

    _errorState(message) {
        return `
            <div class="addons-empty">
                <svg viewBox="0 0 24 24" fill="none" stroke="var(--danger, #ef4444)" stroke-width="1.5" width="48" height="48">
                    <circle cx="12" cy="12" r="10"/>
                    <line x1="12" y1="8" x2="12" y2="12"/>
                    <line x1="12" y1="16" x2="12.01" y2="16"/>
                </svg>
                <p>Failed to load add-ons</p>
                <span class="hint">${this._escape(message)}</span>
                <button class="btn btn-secondary" style="margin-top:16px" onclick="Addons.render()">Retry</button>
            </div>
        `;
    },

    // ─── View Builder ────────────────────────────────────────────────────

    _buildView() {
        const cards = this.addons.length > 0
            ? this.addons.map(a => this._addonCard(a)).join('')
            : this._emptyState();

        const tokenRows = this.tokens.length > 0
            ? this.tokens.map(t => this._tokenRow(t)).join('')
            : '<div class="token-row" style="justify-content:center; color:var(--text-muted);">No registration tokens</div>';

        return `
            <div class="addons-header">
                <h2>Add-ons</h2>
                <button class="btn-add-agent" onclick="Addons.showAddAddon()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="12" y1="5" x2="12" y2="19"/>
                        <line x1="5" y1="12" x2="19" y2="12"/>
                    </svg>
                    Add Add-on
                </button>
            </div>
            <div class="addons-summary">
                ${this._summaryCards()}
            </div>
            <div class="addons-grid">${cards}</div>
            <div class="tokens-section">
                <h3>Registration Tokens</h3>
                <div class="tokens-grid">${tokenRows}</div>
            </div>
        `;
    },

    _summaryCards() {
        const total = this.addons.length;
        const online = this.addons.filter(a => a.status === 'online').length;
        const degraded = this.addons.filter(a => a.status === 'degraded').length;
        const offline = this.addons.filter(a => a.status === 'offline').length;

        return `
            <div class="summary-grid">
                ${Components.summaryCard({ icon: this._icons.addon, iconClass: 'blue', value: total, label: 'Total Add-ons' })}
                ${Components.summaryCard({ icon: this._icons.check, iconClass: 'green', value: online, label: 'Online' })}
                ${Components.summaryCard({ icon: this._icons.warning, iconClass: 'yellow', value: degraded, label: 'Degraded' })}
                ${Components.summaryCard({ icon: this._icons.offline, iconClass: 'red', value: offline, label: 'Offline' })}
            </div>
        `;
    },

    _addonCard(addon) {
        const statusClass = this._statusClass(addon.status);
        const statusLabel = addon.status.charAt(0).toUpperCase() + addon.status.slice(1);
        const lastSeen = addon.last_seen ? this._timeAgo(addon.last_seen) : 'never';
        const enabledLabel = addon.enabled ? 'Enabled' : 'Disabled';
        const enabledClass = addon.enabled ? 'enabled' : 'disabled';
        const urlMeta = addon.url ? `<span class="dot"></span><span>${this._escape(addon.url)}</span>` : '';

        const token = this.tokens.find(t => t.used_by_addon_id === addon.id);
        const tokenRow = token ? `
                <div class="addon-token-row" onclick="event.stopPropagation()">
                    <label>Token</label>
                    <div class="addon-token-field">
                        <span class="addon-token-value" id="addon-token-${addon.id}" data-token="${this._escape(token.token)}" data-masked="true">${'*'.repeat(20)}</span>
                        <button class="btn-token-action" onclick="Addons.toggleTokenVisibility(${addon.id})" title="Show/hide token">
                            ${this._icons.eye}
                        </button>
                        <button class="btn-token-action" onclick="Addons.copyToken(${addon.id})" title="Copy token">
                            ${this._icons.copy}
                        </button>
                    </div>
                </div>
        ` : '';

        return `
            <div class="addon-card ${statusClass}" onclick="Addons.openAddon(${addon.id})" role="button" tabindex="0">
                <div class="addon-card-top">
                    <div class="addon-card-left">
                        <div class="addon-icon">
                            ${this._icons.addon}
                        </div>
                        <div class="addon-info">
                            <h4>${this._escape(addon.name)}</h4>
                            <div class="addon-info-meta">
                                <span>v${this._escape(addon.version)}</span>
                                <span class="dot"></span>
                                <span>Last seen ${lastSeen}</span>
                                ${urlMeta}
                            </div>
                        </div>
                    </div>
                    <div class="addon-card-right">
                        <span class="addon-status-badge ${statusClass}">${statusLabel}</span>
                        <span class="addon-enabled-badge ${enabledClass}">${enabledLabel}</span>
                        <button class="btn-addon-toggle" onclick="event.stopPropagation(); Addons.toggleEnabled(${addon.id}, ${!addon.enabled})" title="${addon.enabled ? 'Disable' : 'Enable'}">
                            ${addon.enabled ? this._icons.toggleOn : this._icons.toggleOff}
                        </button>
                    </div>
                </div>
                ${addon.description ? `<p class="addon-description">${this._escape(addon.description)}</p>` : ''}
                ${tokenRow}
            </div>
        `;
    },

    _tokenRow(token) {
        const truncated = token.token; // Already masked by the server (first 16 chars)
        const now = new Date();
        const isUsed = !!token.used_at;
        const isExpired = token.expires_at ? new Date(token.expires_at + 'Z') < now : false;

        let badgeClass = 'available';
        let badgeLabel = 'Available';
        if (isUsed) { badgeClass = 'used'; badgeLabel = 'Used'; }
        else if (isExpired) { badgeClass = 'expired'; badgeLabel = 'Expired'; }

        return `
            <div class="token-row">
                <span class="token-value">${truncated}</span>
                <div class="token-meta">
                    <span class="token-badge ${badgeClass}">${badgeLabel}</span>
                    <span>${this._timeAgo(token.created_at)}</span>
                    <button class="btn-agent-delete" onclick="Addons.deleteToken(${token.id})" title="Delete token">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
            </div>
        `;
    },

    // ─── Add Add-on Modal ────────────────────────────────────────────────

    async showAddAddon() {
        let token = '';
        try {
            const resp = await API.createAddonToken('');
            if (!resp.ok) throw new Error('Token creation failed');
            const data = await resp.json();
            token = data.token || '';
        } catch {
            alert('Failed to generate registration token. Please try again.');
            return;
        }

        if (!token) {
            alert('Failed to generate registration token. Please try again.');
            return;
        }

        const copyIcon = this._icons.copy;

        Modals.create(`
            <div class="modal modal-add-agent">
                <div class="modal-header">
                    <h3>Add Add-on</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body" style="padding-bottom: 0;">
                    <p class="agent-tab-hint">
                        Register an add-on to extend Vigil with new capabilities. The add-on will connect to this server using the token below.
                    </p>
                    <div class="form-group">
                        <label>Name</label>
                        <input type="text" id="addon-name" class="form-input" placeholder="e.g., Burn-in Hub" maxlength="128">
                    </div>
                    <div class="form-group">
                        <label>Add-on URL</label>
                        <input type="text" id="addon-url" class="form-input form-input-mono" placeholder="e.g., http://192.168.1.50:9100">
                        <span class="form-hint">Network address where the add-on is running</span>
                    </div>
                    <div class="form-group">
                        <label>Token</label>
                        <div class="form-input-with-copy">
                            <input type="text" id="addon-token" class="form-input form-input-mono" value="${token}" readonly>
                            <button class="btn-copy" onclick="Addons._copyInput('addon-token')" title="Copy">
                                ${copyIcon}
                            </button>
                        </div>
                        <span class="form-hint form-hint-warning">This token expires in 1 hour. Save it — it will not be shown again.</span>
                    </div>
                    <div id="addon-reg-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-primary" onclick="Addons.submitAddAddon()">Register Add-on</button>
                </div>
            </div>
        `);

        document.getElementById('addon-name')?.focus();
    },

    async submitAddAddon() {
        const name = document.getElementById('addon-name')?.value.trim();
        const url = document.getElementById('addon-url')?.value.trim();
        const token = document.getElementById('addon-token')?.value.trim();
        const errorEl = document.getElementById('addon-reg-error');

        if (!name) {
            if (errorEl) errorEl.textContent = 'Name is required';
            return;
        }
        if (name.length > 128) {
            if (errorEl) errorEl.textContent = 'Name must be 128 characters or fewer';
            return;
        }
        if (!token) {
            if (errorEl) errorEl.textContent = 'Token is missing — close and reopen the modal';
            return;
        }
        if (url && url.length > 512) {
            if (errorEl) errorEl.textContent = 'URL must be 512 characters or fewer';
            return;
        }

        try {
            const resp = await fetch('/api/addons/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, url, token })
            });
            const data = await resp.json().catch(() => ({}));

            if (resp.ok) {
                document.querySelector('.modal-overlay')?.remove();
                this.render();
            } else {
                if (errorEl) errorEl.textContent = data.error || 'Failed to register add-on';
            }
        } catch {
            if (errorEl) errorEl.textContent = 'Connection error';
        }
    },

    // ─── Addon Actions ───────────────────────────────────────────────────

    async openAddon(id) {
        this.activeAddonId = id;
        try {
            const resp = await API.getAddon(id);
            if (resp.ok) {
                const data = await resp.json();
                if (typeof ManifestRenderer !== 'undefined') {
                    ManifestRenderer.render(data.addon, data.manifest);
                }
            }
        } catch {}
    },

    closeAddon() {
        this.activeAddonId = null;
        this.render();
    },

    async toggleEnabled(id, enabled) {
        try {
            const resp = await API.setAddonEnabled(id, enabled);
            if (resp.ok) this.render();
        } catch {}
    },

    async deleteToken(id) {
        try {
            await API.deleteAddonToken(id);
            this.render();
        } catch {}
    },

    // ─── Clipboard (single implementation) ───────────────────────────────

    _copyInput(inputId) {
        const input = document.getElementById(inputId);
        if (!input) return;
        this._copyText(input.value, input.parentElement?.querySelector('.btn-copy'));
    },

    copyToken(addonId) {
        const el = document.getElementById(`addon-token-${addonId}`);
        if (!el) return;
        this._copyText(el.dataset.token, el.closest('.addon-token-field')?.querySelectorAll('.btn-token-action')[1]);
    },

    _copyText(text, btn) {
        const flash = () => {
            if (!btn) return;
            const orig = btn.innerHTML;
            btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="var(--success, #4ade80)" stroke-width="2" width="16" height="16"><polyline points="20 6 9 17 4 12"/></svg>';
            setTimeout(() => { btn.innerHTML = orig; }, 2000);
        };

        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(text).then(flash).catch(() => {
                this._fallbackCopy(text);
                flash();
            });
        } else {
            this._fallbackCopy(text);
            flash();
        }
    },

    _fallbackCopy(text) {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
    },

    // ─── Token Visibility Toggle ─────────────────────────────────────────

    toggleTokenVisibility(addonId) {
        const el = document.getElementById(`addon-token-${addonId}`);
        if (!el) return;
        const isMasked = el.dataset.masked === 'true';
        el.textContent = isMasked ? el.dataset.token : '*'.repeat(20);
        el.dataset.masked = isMasked ? 'false' : 'true';
        const eyeBtn = el.closest('.addon-token-field')?.querySelector('.btn-token-action');
        if (eyeBtn) eyeBtn.innerHTML = isMasked ? this._icons.eyeOff : this._icons.eye;
    },

    // ─── Helpers ──────────────────────────────────────────────────────────

    _statusClass(status) {
        const map = { online: 'addon-online', degraded: 'addon-degraded', offline: 'addon-offline' };
        return map[status] || '';
    },

    _timeAgo(dateStr) {
        if (!dateStr) return 'never';
        const date = Utils.parseUTC(dateStr);
        if (!date || isNaN(date)) return 'never';
        if (date.getFullYear() < 2000) return 'never';
        const mins = Math.floor((Date.now() - date.getTime()) / 60000);
        if (mins < 1) return 'just now';
        if (mins < 60) return `${mins}m ago`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}h ago`;
        return `${Math.floor(hours / 24)}d ago`;
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    },

    _icons: {
        addon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>`,
        addonLarge: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="48" height="48"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>`,
        check: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>`,
        warning: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
        offline: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`,
        toggleOn: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><rect x="1" y="5" width="22" height="14" rx="7"/><circle cx="16" cy="12" r="3" fill="currentColor"/></svg>`,
        toggleOff: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><rect x="1" y="5" width="22" height="14" rx="7"/><circle cx="8" cy="12" r="3"/></svg>`,
        copy: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`,
        eye: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>`,
        eyeOff: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>`
    }
};
