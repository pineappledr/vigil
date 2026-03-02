/**
 * Vigil Dashboard - Manifest Renderer Core (Task 4.2)
 *
 * Reads a parsed manifest JSON, renders page tabs, dispatches
 * each component to its type-specific renderer, and wires the
 * SSE telemetry stream for live updates.
 */

const ManifestRenderer = {
    addon: null,
    manifest: null,
    activePage: null,
    eventSource: null,
    _chartInstances: [],

    /**
     * Main entry point — called from Addons.openAddon().
     * @param {Object} addon  - The Addon row (id, name, version, status …)
     * @param {Object} manifest - Parsed manifest JSON (pages[], name, version)
     */
    render(addon, manifest) {
        this.addon = addon;
        this.manifest = manifest;
        this.activePage = manifest.pages[0]?.id || null;

        const container = document.getElementById('addons-view');
        if (!container) return;

        container.innerHTML = this._buildShell();
        this._renderActivePage();
        this._connectSSE();
    },

    /** Tear down SSE and chart instances, return to add-on list. */
    close() {
        this._disconnectSSE();
        this._destroyCharts();
        Addons.closeAddon();
    },

    // ─── Shell ────────────────────────────────────────────────────────────

    _buildShell() {
        const m = this.manifest;
        const a = this.addon;
        const tabs = m.pages.length > 1
            ? m.pages.map(p => this._tabButton(p)).join('')
            : '';

        return `
            <div class="manifest-shell">
                <div class="manifest-header">
                    <button class="btn-back manifest-back" onclick="ManifestRenderer.close()">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="19" y1="12" x2="5" y2="12"/>
                            <polyline points="12 19 5 12 12 5"/>
                        </svg>
                        Add-ons
                    </button>
                    <div class="manifest-title-row">
                        <h2>${this._escape(a.name)}</h2>
                        <span class="manifest-version">v${this._escape(m.version)}</span>
                        <span class="addon-status-badge addon-${a.status}">${this._capitalize(a.status)}</span>
                        <span class="addon-update-badge" id="addon-update-badge" style="display:none;">
                            Update available
                        </span>
                        <button class="btn btn-sm btn-secondary" id="addon-check-updates-btn"
                                onclick="ManifestRenderer.checkForUpdates()"
                                title="Check container registry for newer image tags">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                                <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
                                <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                            </svg>
                            Check for updates
                        </button>
                    </div>
                    ${m.description ? `<p class="manifest-desc">${this._escape(m.description)}</p>` : ''}
                </div>
                ${tabs ? `<div class="manifest-tabs">${tabs}</div>` : ''}
                <div class="manifest-page" id="manifest-page-container"></div>
            </div>
        `;
    },

    _tabButton(page) {
        const active = page.id === this.activePage ? 'active' : '';
        return `<button class="manifest-tab ${active}" data-page="${this._escape(page.id)}"
                    onclick="ManifestRenderer.switchPage('${this._escapeJS(page.id)}')">
                    ${this._escape(page.title)}
                </button>`;
    },

    // ─── Page / Component rendering ───────────────────────────────────────

    switchPage(pageId) {
        this.activePage = pageId;
        document.querySelectorAll('.manifest-tab').forEach(el => {
            el.classList.toggle('active', el.dataset.page === pageId);
        });
        this._destroyCharts();
        this._renderActivePage();
    },

    _renderActivePage() {
        const page = this.manifest.pages.find(p => p.id === this.activePage);
        const container = document.getElementById('manifest-page-container');
        if (!page || !container) return;

        container.innerHTML = page.components.map(comp => {
            const title = comp.title ? `<h3 class="component-title">${this._escape(comp.title)}</h3>` : '';
            return `
                <div class="manifest-component" id="mc-${this._escape(comp.id)}" data-type="${comp.type}" data-source="${comp.source || ''}">
                    ${title}
                    <div class="component-body" id="mc-body-${this._escape(comp.id)}">
                        ${this._renderComponent(comp)}
                    </div>
                </div>
            `;
        }).join('');
    },

    /**
     * Dispatch to the correct component renderer.
     * Each renderer returns an HTML string for initial state.
     */
    _renderComponent(comp) {
        const config = comp.config ? (typeof comp.config === 'string' ? JSON.parse(comp.config) : comp.config) : {};

        switch (comp.type) {
            case 'form':
                return typeof FormComponent !== 'undefined'
                    ? FormComponent.render(comp.id, config, this.addon.id)
                    : '<p class="component-unavailable">Form component not loaded</p>';

            case 'progress':
                return typeof ProgressComponent !== 'undefined'
                    ? ProgressComponent.render(comp.id, config, this.addon.id)
                    : '<p class="component-unavailable">Progress component not loaded</p>';

            case 'chart':
                return typeof ChartComponent !== 'undefined'
                    ? ChartComponent.render(comp.id, config)
                    : '<p class="component-unavailable">Chart component not loaded</p>';

            case 'smart-table':
                return typeof SmartTableComponent !== 'undefined'
                    ? SmartTableComponent.render(comp.id, config, this.addon.id)
                    : '<p class="component-unavailable">SMART Table component not loaded</p>';

            case 'log-viewer':
                return typeof LogViewerComponent !== 'undefined'
                    ? LogViewerComponent.render(comp.id, config)
                    : '<p class="component-unavailable">Log Viewer component not loaded</p>';

            case 'deploy-wizard':
                return typeof DeployWizardComponent !== 'undefined'
                    ? DeployWizardComponent.render(comp.id, config, this.addon.id, this.addon.url)
                    : '<p class="component-unavailable">Deploy Wizard component not loaded</p>';

            default:
                return `<p class="component-unavailable">Unknown component type: ${this._escape(comp.type)}</p>`;
        }
    },

    // ─── SSE Telemetry ────────────────────────────────────────────────────

    _connectSSE() {
        this._disconnectSSE();
        if (!this.addon?.id) return;

        const url = `/api/addons/${this.addon.id}/telemetry`;
        this.eventSource = new EventSource(url);

        this.eventSource.addEventListener('connected', () => {
            console.log(`[ManifestRenderer] SSE connected for addon ${this.addon.id}`);
        });

        this.eventSource.addEventListener('progress', (e) => {
            this._handleTelemetry('progress', e);
        });

        this.eventSource.addEventListener('log', (e) => {
            this._handleTelemetry('log', e);
        });

        this.eventSource.addEventListener('notification', (e) => {
            this._handleTelemetry('notification', e);
        });

        this.eventSource.addEventListener('metric', (e) => {
            this._handleTelemetry('metric', e);
        });

        // SMART attribute telemetry — routes to SmartTableComponent
        this.eventSource.addEventListener('smart', (e) => {
            this._handleTelemetry('smart', e);
        });
        this.eventSource.addEventListener('attributes', (e) => {
            this._handleTelemetry('smart', e);
        });

        this.eventSource.onerror = () => {
            console.warn('[ManifestRenderer] SSE connection error, will retry');
        };
    },

    _disconnectSSE() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
    },

    /**
     * Route an SSE event to the appropriate component updater.
     */
    _handleTelemetry(type, event) {
        let data;
        try { data = JSON.parse(event.data); } catch { return; }

        const payload = data.payload ? (typeof data.payload === 'string' ? JSON.parse(data.payload) : data.payload) : data;

        switch (type) {
            case 'progress':
                if (typeof ProgressComponent !== 'undefined') {
                    ProgressComponent.handleUpdate(payload);
                }
                // Extract temperature from progress frames → feed to chart
                if (payload.temp_c > 0 && typeof ChartComponent !== 'undefined') {
                    ChartComponent.handleUpdate({ key: 'temp_c', value: payload.temp_c });
                }
                // Extract SMART deltas from progress frames → feed to table
                if (payload.smart_deltas && typeof SmartTableComponent !== 'undefined') {
                    const rows = this._smartDeltasToRows(payload.smart_deltas);
                    if (rows.length > 0) {
                        SmartTableComponent.handleUpdate({ component_id: 'smart-deltas', rows });
                    }
                }
                break;

            case 'log':
                if (typeof LogViewerComponent !== 'undefined') {
                    // Translate hub log fields to LogViewerComponent format
                    const logPayload = {
                        level: payload.level || payload.severity || 'info',
                        message: payload.message || '',
                        source: payload.source || (payload.agent_id
                            ? payload.agent_id + (payload.job_id ? ':' + payload.job_id : '')
                            : '')
                    };
                    LogViewerComponent.handleUpdate(logPayload);
                }
                break;

            case 'metric':
                if (typeof ChartComponent !== 'undefined') {
                    ChartComponent.handleUpdate(payload);
                }
                break;

            case 'smart':
                if (typeof SmartTableComponent !== 'undefined') {
                    SmartTableComponent.handleUpdate(payload);
                }
                break;

            case 'notification':
                // Notifications are handled server-side via event bus.
                // Optionally show a toast in the UI.
                this._showToast(payload);
                break;
        }
    },

    /**
     * Convert SMART deltas map from progress payload to table rows.
     * Input: { "5": { name: "Reallocated_Sector_Ct", baseline: 0, current: 0 }, ... }
     * Output: [{ id: "5", name: "Reallocated_Sector_Ct", baseline: 0, current: 0, delta: 0 }, ...]
     */
    _smartDeltasToRows(deltas) {
        if (!deltas || typeof deltas !== 'object') return [];
        return Object.entries(deltas).map(([id, d]) => ({
            id,
            name: d.name || '',
            baseline: d.baseline ?? 0,
            current: d.current ?? 0,
            delta: (d.current ?? 0) - (d.baseline ?? 0)
        }));
    },

    _showToast(payload) {
        if (!payload?.message) return;
        const toast = document.createElement('div');
        toast.className = `manifest-toast toast-${payload.severity || 'info'}`;
        toast.textContent = payload.message;
        document.body.appendChild(toast);
        requestAnimationFrame(() => toast.classList.add('visible'));
        setTimeout(() => {
            toast.classList.remove('visible');
            setTimeout(() => toast.remove(), 300);
        }, 4000);
    },

    // ─── Chart instance tracking ──────────────────────────────────────────

    registerChart(instance) {
        this._chartInstances.push(instance);
    },

    _destroyCharts() {
        this._chartInstances.forEach(c => { try { c.destroy(); } catch {} });
        this._chartInstances = [];
    },

    // ─── Update Check ────────────────────────────────────────────────────

    async checkForUpdates() {
        if (!this.addon?.id) return;

        const btn = document.getElementById('addon-check-updates-btn');
        if (btn) {
            btn.disabled = true;
            btn.querySelector('svg')?.classList.add('spin');
        }

        try {
            const resp = await fetch(`/api/addons/${this.addon.id}/check-updates`);
            if (!resp.ok) throw new Error(`HTTP ${resp.status}`);

            const data = await resp.json();
            const badge = document.getElementById('addon-update-badge');

            if (data.update_available && badge) {
                badge.style.display = '';
                badge.textContent = `Update: ${data.latest_tag}`;
                badge.title = `Current: ${data.current_tag} → Latest: ${data.latest_tag} (${data.image})`;
            } else if (badge) {
                badge.style.display = 'none';
                this._showToast({ message: 'No updates available', severity: 'info' });
            }
        } catch (e) {
            console.error('[ManifestRenderer] Update check failed:', e);
            this._showToast({ message: 'Update check failed', severity: 'warning' });
        } finally {
            if (btn) {
                btn.disabled = false;
                btn.querySelector('svg')?.classList.remove('spin');
            }
        }
    },

    // ─── Helpers ──────────────────────────────────────────────────────────

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    },

    _escapeJS(str) {
        if (!str) return '';
        return str.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    },

    _capitalize(s) {
        if (!s) return '';
        return s.charAt(0).toUpperCase() + s.slice(1);
    }
};
