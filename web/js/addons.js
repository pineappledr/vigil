/**
 * Vigil Dashboard - Add-ons Tab Shell (Task 4.1)
 */

const Addons = {
    addons: [],
    activeAddonId: null,

    async render() {
        const container = document.getElementById('dashboard-view');
        if (!container) return;

        container.innerHTML = '<div class="loading-spinner"><div class="spinner"></div>Loading add-ons...</div>';

        try {
            const resp = await API.getAddons();
            if (resp.ok) {
                this.addons = await resp.json();
                if (!Array.isArray(this.addons)) this.addons = [];
            }
        } catch (e) {
            console.error('Failed to load add-ons:', e);
            this.addons = [];
        }

        container.innerHTML = this._buildView();
    },

    _buildView() {
        const cards = this.addons.length > 0
            ? this.addons.map(a => this._addonCard(a)).join('')
            : this._emptyState();

        return `
            <div class="addons-header">
                <h2>Add-ons</h2>
            </div>
            <div class="addons-summary">
                ${this._summaryCards()}
            </div>
            <div class="addons-grid">${cards}</div>
        `;
    },

    _summaryCards() {
        const total = this.addons.length;
        const online = this.addons.filter(a => a.status === 'online').length;
        const degraded = this.addons.filter(a => a.status === 'degraded').length;
        const offline = this.addons.filter(a => a.status === 'offline').length;

        return `
            <div class="summary-grid">
                ${Components.summaryCard({ icon: this._icons.addon, iconClass: 'accent', value: total, label: 'Total Add-ons' })}
                ${Components.summaryCard({ icon: this._icons.check, iconClass: 'healthy', value: online, label: 'Online' })}
                ${Components.summaryCard({ icon: this._icons.warning, iconClass: 'warning', value: degraded, label: 'Degraded' })}
                ${Components.summaryCard({ icon: this._icons.offline, iconClass: 'danger', value: offline, label: 'Offline' })}
            </div>
        `;
    },

    _addonCard(addon) {
        const statusClass = this._statusClass(addon.status);
        const statusLabel = addon.status.charAt(0).toUpperCase() + addon.status.slice(1);
        const lastSeen = addon.last_seen ? this._timeAgo(addon.last_seen) : 'never';
        const enabledLabel = addon.enabled ? 'Enabled' : 'Disabled';
        const enabledClass = addon.enabled ? 'enabled' : 'disabled';

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
            </div>
        `;
    },

    _emptyState() {
        return `
            <div class="addons-empty">
                ${this._icons.addonLarge}
                <p>No add-ons registered</p>
                <span class="hint">Add-ons extend Vigil with custom monitoring workflows</span>
            </div>
        `;
    },

    async openAddon(id) {
        this.activeAddonId = id;

        let addon = null;
        try {
            const resp = await API.getAddon(id);
            if (resp.ok) {
                const data = await resp.json();
                addon = data.addon;
                const manifest = data.manifest;

                if (typeof ManifestRenderer !== 'undefined') {
                    ManifestRenderer.render(addon, manifest);
                }
            }
        } catch (e) {
            console.error('Failed to open add-on:', e);
        }
    },

    closeAddon() {
        this.activeAddonId = null;
        this.render();
    },

    async toggleEnabled(id, enabled) {
        try {
            const resp = await API.setAddonEnabled(id, enabled);
            if (resp.ok) {
                this.render();
            }
        } catch (e) {
            console.error('Failed to toggle add-on:', e);
        }
    },

    _statusClass(status) {
        switch (status) {
            case 'online': return 'addon-online';
            case 'degraded': return 'addon-degraded';
            case 'offline': return 'addon-offline';
            default: return '';
        }
    },

    _timeAgo(dateStr) {
        if (!dateStr) return 'never';
        const date = Utils.parseUTC(dateStr);
        if (!date || isNaN(date)) return 'never';
        if (date.getFullYear() < 2000) return 'never';
        const diff = Date.now() - date.getTime();
        const mins = Math.floor(diff / 60000);
        if (mins < 1) return 'just now';
        if (mins < 60) return `${mins}m ago`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}h ago`;
        const days = Math.floor(hours / 24);
        return `${days}d ago`;
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
        toggleOff: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><rect x="1" y="5" width="22" height="14" rx="7"/><circle cx="8" cy="12" r="3"/></svg>`
    }
};
