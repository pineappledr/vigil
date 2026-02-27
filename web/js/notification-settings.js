/**
 * Vigil Dashboard - Notification Settings Page (Task 4.8)
 *
 * Per-service configuration, event rules, quiet hours, digest config,
 * test-fire button, and notification history viewer.
 */

const NotificationSettings = {
    services: [],
    activeServiceId: null,
    activeService: null,
    eventRules: [],
    quietHours: null,
    digest: null,

    async render() {
        const container = document.getElementById('dashboard-view');
        if (!container) return;

        container.innerHTML = '<div class="loading-spinner"><div class="spinner"></div>Loading notifications...</div>';

        try {
            const resp = await API.getNotificationServices();
            if (resp.ok) {
                this.services = await resp.json();
                if (!Array.isArray(this.services)) this.services = [];
            }
        } catch (e) {
            console.error('Failed to load notification services:', e);
            this.services = [];
        }

        container.innerHTML = this._buildView();
    },

    _buildView() {
        return `
            <div class="notif-header">
                <h2>Notification Services</h2>
                <button class="btn btn-primary" onclick="NotificationSettings.showAddService()">
                    ${this._icons.plus} Add Service
                </button>
            </div>
            <div class="notif-layout">
                <div class="notif-sidebar">
                    ${this._serviceList()}
                </div>
                <div class="notif-content" id="notif-content">
                    ${this.services.length > 0
                        ? '<p class="notif-hint">Select a service to configure</p>'
                        : this._emptyState()}
                </div>
            </div>
            <div class="notif-history-section">
                <h3>Recent Notifications</h3>
                <div id="notif-history">
                    <button class="btn btn-secondary" onclick="NotificationSettings.loadHistory()">Load History</button>
                </div>
            </div>
        `;
    },

    _serviceList() {
        if (this.services.length === 0) return '';

        return `<div class="notif-service-list">
            ${this.services.map(s => `
                <div class="notif-service-item ${s.id === this.activeServiceId ? 'active' : ''} ${s.enabled ? '' : 'disabled'}"
                     onclick="NotificationSettings.selectService(${s.id})">
                    <div class="notif-service-name">${this._escape(s.name)}</div>
                    <div class="notif-service-type">${this._escape(s.service_type)}</div>
                    <span class="notif-service-badge ${s.enabled ? 'enabled' : 'disabled'}">
                        ${s.enabled ? 'Active' : 'Disabled'}
                    </span>
                </div>
            `).join('')}
        </div>`;
    },

    _emptyState() {
        return `
            <div class="notif-empty">
                ${this._icons.bell}
                <p>No notification services configured</p>
                <span class="hint">Add a service to receive alerts for drive health events</span>
            </div>
        `;
    },

    // ─── Service Detail ───────────────────────────────────────────────────

    async selectService(id) {
        this.activeServiceId = id;

        const content = document.getElementById('notif-content');
        if (content) content.innerHTML = '<div class="loading-spinner"><div class="spinner"></div></div>';

        try {
            const resp = await API.getNotificationService(id);
            if (resp.ok) {
                const data = await resp.json();
                this.activeService = data.service;
                this.eventRules = data.event_rules || [];
                this.quietHours = data.quiet_hours || { enabled: false, start_time: '22:00', end_time: '07:00' };
                this.digest = data.digest || { enabled: false, send_at: '08:00' };
            }
        } catch (e) {
            console.error('Failed to load service:', e);
        }

        // Update sidebar active state
        document.querySelectorAll('.notif-service-item').forEach(el => {
            el.classList.toggle('active', el.querySelector('.notif-service-name')?.textContent === this.activeService?.name);
        });

        if (content) content.innerHTML = this._serviceDetail();
    },

    _serviceDetail() {
        const s = this.activeService;
        if (!s) return '<p>Service not found</p>';

        return `
            <div class="notif-detail">
                <div class="notif-detail-header">
                    <h3>${this._escape(s.name)}</h3>
                    <div class="notif-detail-actions">
                        <button class="btn btn-secondary" onclick="NotificationSettings.testFire(${s.id})">
                            ${this._icons.zap} Test
                        </button>
                        <button class="btn btn-danger-outline" onclick="NotificationSettings.deleteService(${s.id})">
                            ${this._icons.trash} Delete
                        </button>
                    </div>
                </div>

                <div class="notif-section">
                    <h4>General</h4>
                    <div class="notif-form-grid">
                        <div class="form-group">
                            <label>Service Type</label>
                            <input type="text" class="form-input" value="${this._escape(s.service_type)}" disabled>
                        </div>
                        <div class="form-group">
                            <label>Enabled</label>
                            <label class="addon-checkbox">
                                <input type="checkbox" id="notif-enabled" ${s.enabled ? 'checked' : ''}
                                    onchange="NotificationSettings.updateGeneral(${s.id})">
                                Send notifications
                            </label>
                        </div>
                    </div>
                    <div class="notif-severity-toggles">
                        <label class="addon-checkbox">
                            <input type="checkbox" id="notif-critical" ${s.notify_on_critical ? 'checked' : ''}
                                onchange="NotificationSettings.updateGeneral(${s.id})">
                            Critical
                        </label>
                        <label class="addon-checkbox">
                            <input type="checkbox" id="notif-warning" ${s.notify_on_warning ? 'checked' : ''}
                                onchange="NotificationSettings.updateGeneral(${s.id})">
                            Warning
                        </label>
                        <label class="addon-checkbox">
                            <input type="checkbox" id="notif-healthy" ${s.notify_on_healthy ? 'checked' : ''}
                                onchange="NotificationSettings.updateGeneral(${s.id})">
                            Healthy / Recovered
                        </label>
                    </div>
                </div>

                <div class="notif-section">
                    <h4>Event Rules</h4>
                    ${this._eventRulesTable()}
                </div>

                <div class="notif-section">
                    <h4>Quiet Hours</h4>
                    ${this._quietHoursForm(s.id)}
                </div>

                <div class="notif-section">
                    <h4>Daily Digest</h4>
                    ${this._digestForm(s.id)}
                </div>

                <div class="notif-status" id="notif-status"></div>
            </div>
        `;
    },

    _eventRulesTable() {
        if (this.eventRules.length === 0) {
            return '<p class="notif-hint">No event rules configured. All events will use the default severity filters above.</p>';
        }

        return `
            <table class="notif-rules-table">
                <thead>
                    <tr>
                        <th>Event Type</th>
                        <th>Enabled</th>
                        <th>Cooldown</th>
                    </tr>
                </thead>
                <tbody>
                    ${this.eventRules.map((rule, i) => `
                        <tr>
                            <td>${this._escape(rule.event_type)}</td>
                            <td>
                                <input type="checkbox" ${rule.enabled ? 'checked' : ''}
                                    onchange="NotificationSettings._updateRuleEnabled(${i}, this.checked)">
                            </td>
                            <td>
                                <input type="number" class="form-input form-input-sm" value="${rule.cooldown_secs || 0}"
                                    min="0" step="60" onchange="NotificationSettings._updateRuleCooldown(${i}, this.value)">
                                <span class="form-hint">seconds</span>
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
            <button class="btn btn-secondary btn-sm" onclick="NotificationSettings.saveEventRules()">Save Rules</button>
        `;
    },

    _quietHoursForm(serviceId) {
        const q = this.quietHours;
        return `
            <div class="notif-form-row">
                <label class="addon-checkbox">
                    <input type="checkbox" id="quiet-enabled" ${q.enabled ? 'checked' : ''}>
                    Enable quiet hours
                </label>
                <div class="notif-time-range">
                    <input type="time" id="quiet-start" class="form-input form-input-sm" value="${q.start_time || '22:00'}">
                    <span>to</span>
                    <input type="time" id="quiet-end" class="form-input form-input-sm" value="${q.end_time || '07:00'}">
                    <span class="form-hint">(UTC)</span>
                </div>
                <button class="btn btn-secondary btn-sm" onclick="NotificationSettings.saveQuietHours(${serviceId})">Save</button>
            </div>
        `;
    },

    _digestForm(serviceId) {
        const d = this.digest;
        return `
            <div class="notif-form-row">
                <label class="addon-checkbox">
                    <input type="checkbox" id="digest-enabled" ${d.enabled ? 'checked' : ''}>
                    Send daily digest
                </label>
                <div class="notif-time-range">
                    <label>Send at:</label>
                    <input type="time" id="digest-time" class="form-input form-input-sm" value="${d.send_at || '08:00'}">
                    <span class="form-hint">(UTC)</span>
                </div>
                <button class="btn btn-secondary btn-sm" onclick="NotificationSettings.saveDigest(${serviceId})">Save</button>
            </div>
        `;
    },

    // ─── Actions ──────────────────────────────────────────────────────────

    async updateGeneral(id) {
        const s = this.activeService;
        if (!s) return;

        const body = {
            name: s.name,
            service_type: s.service_type,
            config_json: s.config_json,
            enabled: document.getElementById('notif-enabled')?.checked ?? s.enabled,
            notify_on_critical: document.getElementById('notif-critical')?.checked ?? s.notify_on_critical,
            notify_on_warning: document.getElementById('notif-warning')?.checked ?? s.notify_on_warning,
            notify_on_healthy: document.getElementById('notif-healthy')?.checked ?? s.notify_on_healthy
        };

        try {
            const resp = await API.updateNotificationService(id, body);
            if (resp.ok) this._showStatus('Settings saved');
            else this._showStatus('Failed to save', true);
        } catch { this._showStatus('Connection error', true); }
    },

    _updateRuleEnabled(idx, enabled) {
        if (this.eventRules[idx]) this.eventRules[idx].enabled = enabled;
    },

    _updateRuleCooldown(idx, value) {
        if (this.eventRules[idx]) this.eventRules[idx].cooldown_secs = parseInt(value) || 0;
    },

    async saveEventRules() {
        if (!this.activeServiceId) return;
        try {
            const resp = await API.updateEventRules(this.activeServiceId, this.eventRules);
            if (resp.ok) this._showStatus('Event rules saved');
            else this._showStatus('Failed to save rules', true);
        } catch { this._showStatus('Connection error', true); }
    },

    async saveQuietHours(serviceId) {
        const body = {
            enabled: document.getElementById('quiet-enabled')?.checked ?? false,
            start_time: document.getElementById('quiet-start')?.value || '22:00',
            end_time: document.getElementById('quiet-end')?.value || '07:00'
        };

        try {
            const resp = await API.updateQuietHours(serviceId, body);
            if (resp.ok) this._showStatus('Quiet hours saved');
            else this._showStatus('Failed to save', true);
        } catch { this._showStatus('Connection error', true); }
    },

    async saveDigest(serviceId) {
        const body = {
            enabled: document.getElementById('digest-enabled')?.checked ?? false,
            send_at: document.getElementById('digest-time')?.value || '08:00'
        };

        try {
            const resp = await API.updateDigestConfig(serviceId, body);
            if (resp.ok) this._showStatus('Digest config saved');
            else this._showStatus('Failed to save', true);
        } catch { this._showStatus('Connection error', true); }
    },

    async testFire(serviceId) {
        this._showStatus('Sending test notification...');
        try {
            const resp = await API.testFireNotification(serviceId, 'Vigil test notification');
            const data = await resp.json().catch(() => ({}));
            if (data.success) {
                this._showStatus('Test notification sent');
            } else {
                this._showStatus(data.error || 'Test failed', true);
            }
        } catch { this._showStatus('Connection error', true); }
    },

    async deleteService(id) {
        if (!confirm('Delete this notification service? This cannot be undone.')) return;

        try {
            const resp = await API.deleteNotificationService(id);
            if (resp.ok) {
                this.activeServiceId = null;
                this.activeService = null;
                this.render();
            }
        } catch { this._showStatus('Failed to delete', true); }
    },

    // ─── Add Service Modal ────────────────────────────────────────────────

    showAddService() {
        Modals.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>Add Notification Service</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label>Name</label>
                        <input type="text" id="new-notif-name" class="form-input" placeholder="e.g., Discord Alerts">
                    </div>
                    <div class="form-group">
                        <label>Service Type</label>
                        <select id="new-notif-type" class="form-input">
                            <option value="discord">Discord</option>
                            <option value="slack">Slack</option>
                            <option value="telegram">Telegram</option>
                            <option value="email">Email (SMTP)</option>
                            <option value="pushover">Pushover</option>
                            <option value="gotify">Gotify</option>
                            <option value="generic">Generic Webhook</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label>Shoutrrr URL</label>
                        <input type="text" id="new-notif-url" class="form-input form-input-mono"
                               placeholder="discord://token@id or slack://token@channel">
                        <span class="form-hint">See <a href="https://containrrr.dev/shoutrrr/services/overview/" target="_blank" rel="noopener">Shoutrrr docs</a> for URL format</span>
                    </div>
                    <div id="new-notif-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-primary" onclick="NotificationSettings.submitAddService()">Add Service</button>
                </div>
            </div>
        `);
        document.getElementById('new-notif-name')?.focus();
    },

    async submitAddService() {
        const name = document.getElementById('new-notif-name')?.value.trim();
        const type = document.getElementById('new-notif-type')?.value;
        const url = document.getElementById('new-notif-url')?.value.trim();
        const errorEl = document.getElementById('new-notif-error');

        if (!name) { if (errorEl) errorEl.textContent = 'Name is required'; return; }
        if (!url) { if (errorEl) errorEl.textContent = 'Shoutrrr URL is required'; return; }

        try {
            const configJSON = JSON.stringify({ shoutrrr_url: url });
            const resp = await API.createNotificationService({
                name,
                service_type: type,
                config_json: configJSON,
                enabled: true,
                notify_on_critical: true,
                notify_on_warning: true,
                notify_on_healthy: false
            });

            if (resp.ok) {
                document.querySelector('.modal-overlay')?.remove();
                this.render();
            } else {
                const data = await resp.json().catch(() => ({}));
                if (errorEl) errorEl.textContent = data.error || 'Failed to create service';
            }
        } catch {
            if (errorEl) errorEl.textContent = 'Connection error';
        }
    },

    // ─── History ──────────────────────────────────────────────────────────

    async loadHistory() {
        const container = document.getElementById('notif-history');
        if (!container) return;

        container.innerHTML = '<div class="loading-spinner"><div class="spinner"></div></div>';

        try {
            const resp = await API.getNotificationHistory(50);
            if (resp.ok) {
                const records = await resp.json();
                container.innerHTML = this._historyTable(Array.isArray(records) ? records : []);
            } else {
                container.innerHTML = '<p>Failed to load history</p>';
            }
        } catch {
            container.innerHTML = '<p>Connection error</p>';
        }
    },

    _historyTable(records) {
        if (records.length === 0) {
            return '<p class="notif-hint">No notification history</p>';
        }

        return `
            <table class="notif-history-table">
                <thead>
                    <tr>
                        <th>Time</th>
                        <th>Event</th>
                        <th>Host</th>
                        <th>Message</th>
                        <th>Status</th>
                    </tr>
                </thead>
                <tbody>
                    ${records.map(r => `
                        <tr class="notif-history-${r.status}">
                            <td class="notif-time">${this._formatTime(r.created_at)}</td>
                            <td>${this._escape(r.event_type)}</td>
                            <td>${this._escape(r.hostname || '--')}</td>
                            <td class="notif-msg">${this._escape(r.message)}</td>
                            <td>
                                <span class="notif-status-badge ${r.status}">
                                    ${r.status === 'sent' ? 'Sent' : r.status === 'failed' ? 'Failed' : this._escape(r.status)}
                                </span>
                                ${r.error_message ? `<span class="notif-error-hint" title="${this._escape(r.error_message)}">!</span>` : ''}
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    },

    // ─── Helpers ──────────────────────────────────────────────────────────

    _showStatus(msg, isError) {
        const el = document.getElementById('notif-status');
        if (!el) return;
        el.textContent = msg;
        el.className = `notif-status ${isError ? 'error' : 'success'}`;
        setTimeout(() => { el.textContent = ''; el.className = 'notif-status'; }, 3000);
    },

    _formatTime(dateStr) {
        if (!dateStr) return '--';
        const d = new Date(dateStr);
        if (isNaN(d)) return '--';
        return d.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false });
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    },

    _icons: {
        plus: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>`,
        bell: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="48" height="48"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>`,
        zap: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>`,
        trash: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>`
    }
};
