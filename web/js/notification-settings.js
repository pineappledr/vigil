/**
 * Vigil Dashboard - Notification Settings Page
 *
 * Uptime-Kuma-style dynamic provider wizard with per-provider form fields,
 * test-before-save, secret masking, and full service configuration.
 */

const NotificationSettings = {
    services: [],
    activeServiceId: null,
    activeService: null,
    eventRules: [],
    quietHours: null,
    digest: null,
    providerDefs: null,

    async render() {
        const container = document.getElementById('notifications-view');
        if (!container) return;

        container.innerHTML = '<div class="loading-spinner"><div class="spinner"></div>Loading notifications...</div>';

        try {
            const [svcResp] = await Promise.all([
                API.getNotificationServices(),
                this._ensureProviderDefs()
            ]);
            if (svcResp.ok) {
                this.services = await svcResp.json();
                if (!Array.isArray(this.services)) this.services = [];
            }
        } catch (e) {
            console.error('Failed to load notification services:', e);
            this.services = [];
        }

        container.innerHTML = this._buildView();
    },

    async _ensureProviderDefs() {
        if (this.providerDefs) return;
        try {
            const resp = await API.getNotificationProviders();
            if (resp.ok) this.providerDefs = await resp.json();
        } catch (e) {
            console.error('Failed to load provider definitions:', e);
        }
    },

    _buildView() {
        return `
            <div class="notif-header">
                <h2>Notification Services</h2>
                <button class="btn-add-agent" onclick="NotificationSettings.showAddService()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="12" y1="5" x2="12" y2="19"/>
                        <line x1="5" y1="12" x2="19" y2="12"/>
                    </svg>
                    Add Service
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
                    <div class="notif-service-type">${this._providerLabel(s.service_type)}</div>
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
                        <button class="btn btn-secondary" onclick="NotificationSettings.editProviderConfig(${s.id})">
                            ${this._icons.edit} Edit Config
                        </button>
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
                            <input type="text" class="form-input" value="${this._providerLabel(s.service_type)}" disabled>
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

    // ─── Add Service Modal (Dynamic Provider Wizard) ─────────────────────

    async showAddService() {
        await this._ensureProviderDefs();

        const types = this.providerDefs ? Object.keys(this.providerDefs) : [];
        const firstType = types[0] || 'discord';

        Modals.create(`
            <div class="modal modal-provider-wizard">
                <div class="modal-header">
                    <h3>Setup Notification</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label>Notification Type</label>
                        <select id="prov-type" class="form-input"
                                onchange="NotificationSettings._onProviderChange()">
                            ${types.map(t => {
                                const def = this.providerDefs[t];
                                return `<option value="${t}">${this._escape(def.label)}</option>`;
                            }).join('')}
                        </select>
                    </div>
                    <div class="form-group">
                        <label>Friendly Name</label>
                        <input type="text" id="prov-name" class="form-input" placeholder="e.g., My Discord Alert (1)">
                    </div>
                    <div id="prov-fields-container"></div>

                    <hr class="modal-divider">
                    <div class="form-group">
                        <label class="addon-checkbox">
                            <input type="checkbox" id="prov-default-enabled" checked>
                            Default enabled
                        </label>
                        <span class="form-hint">This notification will be enabled by default for new monitors. You can still disable the notification separately for each monitor.</span>
                    </div>

                    <div id="prov-status" class="notif-test-status"></div>
                    <div id="prov-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-outline" id="prov-test-btn"
                            onclick="NotificationSettings._testProvider()">
                        ${this._icons.zap} Test
                    </button>
                    <button class="btn btn-primary" id="prov-save-btn"
                            onclick="NotificationSettings._submitProvider()">Save</button>
                </div>
            </div>
        `);

        this._renderProviderFields(firstType);
        document.getElementById('prov-name')?.focus();
    },

    _onProviderChange() {
        const type = document.getElementById('prov-type')?.value;
        if (type) this._renderProviderFields(type);

        // Clear status/error
        const statusEl = document.getElementById('prov-status');
        const errorEl = document.getElementById('prov-error');
        if (statusEl) { statusEl.textContent = ''; statusEl.className = 'notif-test-status'; }
        if (errorEl) errorEl.textContent = '';
    },

    _renderProviderFields(type, prefillFields) {
        const container = document.getElementById('prov-fields-container');
        if (!container || !this.providerDefs) return;

        const def = this.providerDefs[type];
        if (!def) { container.innerHTML = ''; return; }

        container.innerHTML = def.fields.map(f => {
            const id = `prov-field-${f.key}`;
            const req = f.required ? ' <span class="form-required">*</span>' : '';
            const prefillVal = prefillFields ? (prefillFields[f.key] || '') : '';

            let input = '';
            switch (f.type) {
                case 'password':
                    input = `
                        <div class="form-input-password-wrap">
                            <input type="password" id="${id}" class="form-input"
                                   placeholder="${this._escape(f.placeholder || '')}"
                                   value="${this._escape(prefillVal)}"
                                   data-field-key="${f.key}" ${f.required ? 'required' : ''}>
                            <button type="button" class="btn-eye-toggle"
                                    onclick="NotificationSettings._togglePasswordVisibility('${id}')" title="Toggle visibility">
                                ${this._icons.eye}
                            </button>
                        </div>`;
                    break;
                case 'select':
                    input = `<select id="${id}" class="form-input" data-field-key="${f.key}">
                        ${(f.options || []).map(o => {
                            const selected = prefillVal
                                ? o.value === prefillVal
                                : o.value === (f.default || '');
                            return `<option value="${this._escape(o.value)}" ${selected ? 'selected' : ''}>
                                ${this._escape(o.label)}
                            </option>`;
                        }).join('')}
                    </select>`;
                    break;
                case 'checkbox':
                    input = `<label class="addon-checkbox">
                        <input type="checkbox" id="${id}" data-field-key="${f.key}"
                               ${prefillVal === 'true' ? 'checked' : ''}>
                        ${this._escape(f.label)}
                    </label>`;
                    break;
                case 'number':
                    input = `<input type="number" id="${id}" class="form-input"
                               placeholder="${this._escape(f.placeholder || '')}"
                               value="${this._escape(prefillVal || f.default || '')}"
                               data-field-key="${f.key}" ${f.required ? 'required' : ''}>`;
                    break;
                default:
                    input = `<input type="text" id="${id}" class="form-input"
                               placeholder="${this._escape(f.placeholder || '')}"
                               value="${this._escape(prefillVal)}"
                               data-field-key="${f.key}" ${f.required ? 'required' : ''}>`;
            }

            const labelHtml = f.type === 'checkbox' ? '' :
                `<label>${this._escape(f.label)}${req}</label>`;
            const docsLink = f.docs_url ?
                ` <a href="${this._escape(f.docs_url)}" target="_blank" rel="noopener noreferrer">Shoutrrr Services</a>` : '';
            const helpHtml = f.help_text ?
                `<span class="form-hint">${this._escape(f.help_text)}${docsLink}</span>` : '';

            return `<div class="form-group">${labelHtml}${input}${helpHtml}</div>`;
        }).join('');
    },

    _collectProviderFields() {
        const type = document.getElementById('prov-type')?.value;
        const def = this.providerDefs?.[type];
        if (!def) return {};

        const fields = {};
        for (const f of def.fields) {
            const el = document.getElementById(`prov-field-${f.key}`);
            if (!el) continue;
            if (f.type === 'checkbox') {
                fields[f.key] = el.checked ? 'true' : 'false';
            } else {
                fields[f.key] = el.value.trim();
            }
        }
        return fields;
    },

    async _submitProvider() {
        const name = document.getElementById('prov-name')?.value.trim();
        const type = document.getElementById('prov-type')?.value;
        const fields = this._collectProviderFields();
        const errorEl = document.getElementById('prov-error');
        const saveBtn = document.getElementById('prov-save-btn');
        const defaultEnabled = document.getElementById('prov-default-enabled')?.checked ?? true;

        if (!name) { if (errorEl) errorEl.textContent = 'Friendly Name is required'; return; }
        if (errorEl) errorEl.textContent = '';
        if (saveBtn) { saveBtn.disabled = true; saveBtn.textContent = 'Saving...'; }

        try {
            const resp = await API.createNotificationService({
                name,
                service_type: type,
                config_fields: fields,
                enabled: defaultEnabled,
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
                if (saveBtn) { saveBtn.disabled = false; saveBtn.textContent = 'Save'; }
            }
        } catch {
            if (errorEl) errorEl.textContent = 'Connection error';
            if (saveBtn) { saveBtn.disabled = false; saveBtn.textContent = 'Save'; }
        }
    },

    async _testProvider() {
        const type = document.getElementById('prov-type')?.value;
        const fields = this._collectProviderFields();
        const statusEl = document.getElementById('prov-status');
        const errorEl = document.getElementById('prov-error');
        const btn = document.getElementById('prov-test-btn');

        if (errorEl) errorEl.textContent = '';
        if (btn) { btn.disabled = true; btn.innerHTML = 'Sending...'; }
        if (statusEl) { statusEl.textContent = ''; statusEl.className = 'notif-test-status'; }

        try {
            const resp = await API.testNotificationFields(
                type, fields,
                'Vigil test notification \u2014 if you see this, it works!'
            );
            const data = await resp.json().catch(() => ({}));
            if (statusEl) {
                statusEl.textContent = data.success
                    ? 'Test sent successfully! Check your service.'
                    : (data.error || 'Test failed \u2014 check your settings');
                statusEl.className = `notif-test-status ${data.success ? 'success' : 'error'}`;
            }
        } catch {
            if (statusEl) {
                statusEl.textContent = 'Connection error';
                statusEl.className = 'notif-test-status error';
            }
        }

        if (btn) {
            btn.disabled = false;
            btn.innerHTML = `${this._icons.zap} Test`;
        }
    },

    _togglePasswordVisibility(id) {
        const input = document.getElementById(id);
        if (!input) return;
        input.type = input.type === 'password' ? 'text' : 'password';
    },

    // ─── Edit Provider Config Modal ──────────────────────────────────────

    async editProviderConfig(id) {
        await this._ensureProviderDefs();

        const s = this.activeService;
        if (!s) return;

        // Extract stored fields from config_json
        let storedFields = {};
        try {
            const cfg = JSON.parse(s.config_json);
            if (cfg.fields) storedFields = cfg.fields;
        } catch { /* legacy config */ }

        const types = this.providerDefs ? Object.keys(this.providerDefs) : [];

        Modals.create(`
            <div class="modal modal-provider-wizard">
                <div class="modal-header">
                    <h3>Edit Configuration</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label>Notification Type</label>
                        <select id="prov-type" class="form-input" disabled>
                            ${types.map(t => {
                                const def = this.providerDefs[t];
                                return `<option value="${t}" ${t === s.service_type ? 'selected' : ''}>${this._escape(def.label)}</option>`;
                            }).join('')}
                        </select>
                    </div>
                    <div class="form-group">
                        <label>Friendly Name</label>
                        <input type="text" id="prov-name" class="form-input" value="${this._escape(s.name)}">
                    </div>
                    <div id="prov-fields-container"></div>

                    <div id="prov-status" class="notif-test-status"></div>
                    <div id="prov-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-outline" id="prov-test-btn"
                            onclick="NotificationSettings._testProvider()">
                        ${this._icons.zap} Test
                    </button>
                    <button class="btn btn-primary" id="prov-save-btn"
                            onclick="NotificationSettings._submitEditProvider(${id})">Save</button>
                </div>
            </div>
        `);

        this._renderProviderFields(s.service_type, storedFields);
    },

    async _submitEditProvider(id) {
        const name = document.getElementById('prov-name')?.value.trim();
        const type = document.getElementById('prov-type')?.value;
        const fields = this._collectProviderFields();
        const errorEl = document.getElementById('prov-error');
        const saveBtn = document.getElementById('prov-save-btn');

        if (!name) { if (errorEl) errorEl.textContent = 'Friendly Name is required'; return; }
        if (errorEl) errorEl.textContent = '';
        if (saveBtn) { saveBtn.disabled = true; saveBtn.textContent = 'Saving...'; }

        const s = this.activeService;
        try {
            const resp = await API.updateNotificationService(id, {
                name,
                service_type: type,
                config_fields: fields,
                enabled: s?.enabled ?? true,
                notify_on_critical: s?.notify_on_critical ?? true,
                notify_on_warning: s?.notify_on_warning ?? true,
                notify_on_healthy: s?.notify_on_healthy ?? false
            });

            if (resp.ok) {
                document.querySelector('.modal-overlay')?.remove();
                this.selectService(id);
            } else {
                const data = await resp.json().catch(() => ({}));
                if (errorEl) errorEl.textContent = data.error || 'Failed to update';
                if (saveBtn) { saveBtn.disabled = false; saveBtn.textContent = 'Save'; }
            }
        } catch {
            if (errorEl) errorEl.textContent = 'Connection error';
            if (saveBtn) { saveBtn.disabled = false; saveBtn.textContent = 'Save'; }
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

    _providerLabel(type) {
        if (this.providerDefs?.[type]) return this._escape(this.providerDefs[type].label);
        return this._escape(type);
    },

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
        trash: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>`,
        edit: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>`,
        eye: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>`
    }
};
