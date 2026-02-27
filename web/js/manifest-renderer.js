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

        const container = document.getElementById('dashboard-view');
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
                    ? ProgressComponent.render(comp.id, config)
                    : '<p class="component-unavailable">Progress component not loaded</p>';

            case 'chart':
                return typeof ChartComponent !== 'undefined'
                    ? ChartComponent.render(comp.id, config)
                    : '<p class="component-unavailable">Chart component not loaded</p>';

            case 'smart-table':
                return typeof SmartTableComponent !== 'undefined'
                    ? SmartTableComponent.render(comp.id, config)
                    : '<p class="component-unavailable">SMART Table component not loaded</p>';

            case 'log-viewer':
                return typeof LogViewerComponent !== 'undefined'
                    ? LogViewerComponent.render(comp.id, config)
                    : '<p class="component-unavailable">Log Viewer component not loaded</p>';

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
                break;

            case 'log':
                if (typeof LogViewerComponent !== 'undefined') {
                    LogViewerComponent.handleUpdate(payload);
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
