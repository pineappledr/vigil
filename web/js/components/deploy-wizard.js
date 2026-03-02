/**
 * Vigil Dashboard - Deploy Wizard Component
 *
 * Renders a deployment helper inside addon pages. Provides Docker/Binary
 * tabs with platform selection, pre-filled connection details from the
 * addon's own API, and one-click copy of docker-compose or install commands.
 *
 * Config schema (from manifest.json):
 *   target_label   - Display label (e.g., "Agent")
 *   docker         - { image, default_tag, container_name, ports[], privileged,
 *                      network_mode, volumes[], environment{}, platforms{} }
 *   binary         - { install_url, platforms{} }
 *   prefill_endpoint - Path on addon to fetch pre-fill data (e.g., "/api/deploy-info")
 */

const DeployWizardComponent = {
    _wizards: {},   // keyed by compId

    /**
     * @param {string} compId   - Manifest component ID
     * @param {Object} config   - Deploy wizard config from manifest
     * @param {number} addonId  - Parent add-on ID
     * @param {string} addonUrl - Registered add-on URL (fallback for prefill)
     * @returns {string} HTML
     */
    render(compId, config, addonId, addonUrl) {
        const hasDocker = !!config.docker;
        const hasBinary = !!config.binary;

        if (!hasDocker && !hasBinary) {
            return '<p class="component-unavailable">No deployment options configured</p>';
        }

        const defaultTab = hasDocker ? 'docker' : 'binary';
        const dockerPlatforms = hasDocker ? Object.keys(config.docker.platforms) : [];
        const binaryPlatforms = hasBinary ? Object.keys(config.binary.platforms) : [];
        const defaultPlatform = hasDocker ? dockerPlatforms[0] : binaryPlatforms[0];

        this._wizards[compId] = {
            config, addonId, addonUrl,
            tab: defaultTab,
            platform: defaultPlatform,
            prefill: {},
            loading: true
        };

        // Fetch prefill data after DOM insertion
        if (config.prefill_endpoint) {
            setTimeout(() => this._fetchPrefill(compId), 0);
        } else {
            setTimeout(() => {
                this._wizards[compId].loading = false;
                this._updateDisplay(compId);
            }, 0);
        }

        const label = config.target_label || 'Component';

        // Build tab bar
        const tabs = [];
        if (hasDocker) tabs.push({ key: 'docker', label: 'Docker' });
        if (hasBinary) tabs.push({ key: 'binary', label: 'Binary' });

        const tabBar = tabs.length > 1
            ? `<div class="agent-tab-bar">
                ${tabs.map(t =>
                    `<button class="agent-tab${t.key === defaultTab ? ' active' : ''}"
                            data-tab="${t.key}"
                            onclick="DeployWizardComponent.switchTab('${this._escapeJS(compId)}', '${t.key}')">${t.label}</button>`
                ).join('')}
               </div>`
            : '';

        // Build platform bar
        const platformBar = this._buildPlatformBar(compId, config, defaultTab);

        // Build hint
        const hintText = this._getHint(config, defaultTab, defaultPlatform);

        // Build user-input fields from environment config
        const userFields = this._buildUserFields(compId, config);

        // Build readonly prefill fields
        const prefillFields = this._buildPrefillFields(compId, config);

        // Copy button icon
        const copyIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
        </svg>`;

        return `
            <div class="deploy-wizard" id="dw-${compId}">
                ${tabBar}
                <div class="agent-platform-bar" id="dw-platform-${compId}">
                    ${platformBar}
                </div>
                <p class="agent-tab-hint" id="dw-hint-${compId}">${hintText}</p>

                ${prefillFields}
                ${userFields}

                <div class="form-group">
                    <label>Version</label>
                    <input type="text" id="dw-version-${compId}" class="form-input"
                           placeholder="latest" value="">
                    <span class="form-hint" style="margin-top: 4px;">Leave empty for latest. Use a tag like <code>v1.0.0</code> for a specific version.</span>
                </div>

                <div class="deploy-wizard-footer" style="display: flex; justify-content: flex-start; margin-top: 16px;">
                    <button class="btn btn-secondary btn-with-icon" id="dw-copy-btn-${compId}"
                            onclick="DeployWizardComponent.copyInstall('${this._escapeJS(compId)}')">
                        ${copyIcon}
                        <span id="dw-copy-label-${compId}">Copy docker compose</span>
                    </button>
                </div>
            </div>
        `;
    },

    // ─── Prefill ─────────────────────────────────────────────────────────

    async _fetchPrefill(compId) {
        const wiz = this._wizards[compId];
        if (!wiz) return;

        let errorMsg = null;
        try {
            const endpoint = wiz.config.prefill_endpoint;
            const resp = await fetch(`/api/addons/${wiz.addonId}/proxy?path=${encodeURIComponent(endpoint)}`);
            if (resp.ok) {
                wiz.prefill = await resp.json();
            } else {
                errorMsg = `Could not auto-fill from add-on (HTTP ${resp.status}). The Vigil server cannot reach the add-on URL. Values below have been pre-filled from the registered add-on URL where possible.`;
            }
        } catch (e) {
            console.error(`[DeployWizard] Failed to fetch prefill for ${compId}:`, e);
            errorMsg = 'Could not auto-fill from add-on. The Vigil server cannot reach the add-on URL.';
        }

        // Always prefer the registered addon URL for hub_url — the Hub's
        // auto-detected hostname is often a Docker container ID.
        if (wiz.addonUrl) {
            wiz.prefill.hub_url = wiz.addonUrl;
        }

        wiz.loading = false;
        this._updatePrefillFields(compId);

        if (errorMsg) {
            this._showPrefillError(compId, errorMsg);
        }
    },

    _showPrefillError(compId, message) {
        const wiz = this._wizards[compId];
        if (!wiz) return;

        const env = wiz.config.docker?.environment || {};
        for (const [envKey, envDef] of Object.entries(env)) {
            if (envDef.source !== 'prefill') continue;
            const input = document.getElementById(`dw-pf-${compId}-${envKey}`);
            if (input) {
                // If this field already has a value (from fallback), keep it readonly
                if (input.value) continue;
                input.placeholder = 'Enter manually';
                input.removeAttribute('readonly');
                input.classList.add('prefill-error');
            }
        }

        // Show error banner above all fields
        const container = document.getElementById(`dw-${compId}`);
        if (container) {
            const existing = container.querySelector('.deploy-wizard-error');
            if (!existing) {
                const errDiv = document.createElement('div');
                errDiv.className = 'deploy-wizard-error';
                errDiv.textContent = message;
                // Insert after the hint paragraph
                const hint = container.querySelector('.agent-tab-hint');
                if (hint) {
                    hint.insertAdjacentElement('afterend', errDiv);
                } else {
                    container.prepend(errDiv);
                }
            }
        }
    },

    _updatePrefillFields(compId) {
        const wiz = this._wizards[compId];
        if (!wiz) return;

        const env = wiz.config.docker?.environment || {};
        for (const [envKey, envDef] of Object.entries(env)) {
            if (envDef.source === 'prefill' && envDef.key) {
                const input = document.getElementById(`dw-pf-${compId}-${envKey}`);
                if (input) {
                    input.value = wiz.prefill[envDef.key] || '';
                }
            }
        }
    },

    // ─── Tab & Platform Switching ────────────────────────────────────────

    switchTab(compId, tab) {
        const wiz = this._wizards[compId];
        if (!wiz) return;
        wiz.tab = tab;

        // Update tab buttons
        const container = document.getElementById(`dw-${compId}`);
        if (container) {
            container.querySelectorAll('.agent-tab').forEach(el => {
                el.classList.toggle('active', el.dataset.tab === tab);
            });
        }

        // Rebuild platform bar
        const platformBar = document.getElementById(`dw-platform-${compId}`);
        if (platformBar) {
            platformBar.innerHTML = this._buildPlatformBar(compId, wiz.config, tab);
        }

        // Set default platform for new tab
        const platforms = tab === 'docker'
            ? Object.keys(wiz.config.docker?.platforms || {})
            : Object.keys(wiz.config.binary?.platforms || {});
        wiz.platform = platforms[0] || '';

        // Update hint
        this._updateHint(compId);

        // Update copy button label
        const label = document.getElementById(`dw-copy-label-${compId}`);
        if (label) {
            label.textContent = tab === 'docker' ? 'Copy docker compose' : 'Copy install command';
        }
    },

    switchPlatform(compId, platform) {
        const wiz = this._wizards[compId];
        if (!wiz) return;
        wiz.platform = platform;

        const bar = document.getElementById(`dw-platform-${compId}`);
        if (bar) {
            bar.querySelectorAll('.agent-platform-btn').forEach(el => {
                el.classList.toggle('active', el.dataset.platform === platform);
            });
        }

        this._updateHint(compId);
    },

    _buildPlatformBar(compId, config, tab) {
        const platforms = tab === 'docker'
            ? config.docker?.platforms || {}
            : config.binary?.platforms || {};

        const keys = Object.keys(platforms);
        if (keys.length <= 1 && tab === 'docker') {
            // Single platform — no bar needed for docker
            return '';
        }

        return keys.map((key, i) =>
            `<button class="agent-platform-btn${i === 0 ? ' active' : ''}"
                    data-platform="${this._escape(key)}"
                    onclick="DeployWizardComponent.switchPlatform('${this._escapeJS(compId)}', '${this._escapeJS(key)}')">${this._escape(platforms[key].label || key)}</button>`
        ).join('');
    },

    _getHint(config, tab, platform) {
        const platforms = tab === 'docker'
            ? config.docker?.platforms || {}
            : config.binary?.platforms || {};
        return platforms[platform]?.hint || '';
    },

    _updateHint(compId) {
        const wiz = this._wizards[compId];
        if (!wiz) return;
        const hint = document.getElementById(`dw-hint-${compId}`);
        if (hint) {
            hint.innerHTML = this._getHint(wiz.config, wiz.tab, wiz.platform);
        }
    },

    _updateDisplay(compId) {
        this._updatePrefillFields(compId);
    },

    // ─── Field Building ──────────────────────────────────────────────────

    _buildPrefillFields(compId, config) {
        const env = config.docker?.environment || {};
        const copyIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
        </svg>`;
        const eyeIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
        </svg>`;
        const eyeOffIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/>
            <line x1="1" y1="1" x2="23" y2="23"/>
        </svg>`;

        let html = '';
        for (const [envKey, envDef] of Object.entries(env)) {
            if (envDef.source !== 'prefill') continue;
            const fieldId = `dw-pf-${compId}-${envKey}`;
            const label = envDef.label || envKey;
            const isSecret = envDef.secret === true || envKey.includes('PSK') || envKey.includes('SECRET') || envKey.includes('KEY');

            html += `
                <div class="form-group">
                    <label>${this._escape(label)}</label>
                    <div class="form-input-with-copy">
                        <input type="${isSecret ? 'password' : 'text'}" id="${fieldId}" class="form-input form-input-mono"
                               value="" readonly placeholder="Loading..."
                               style="padding-right: ${isSecret ? '68px' : '38px'}">
                        <div class="form-input-actions">
                            ${isSecret ? `<button class="btn-copy" onclick="DeployWizardComponent._toggleSecret('${fieldId}')" title="Show/Hide" id="dw-eye-${fieldId}">
                                ${eyeIcon}
                            </button>` : ''}
                            <button class="btn-copy" onclick="DeployWizardComponent._copyField('${fieldId}')" title="Copy">
                                ${copyIcon}
                            </button>
                        </div>
                    </div>
                    ${envDef.hint ? `<span class="form-hint">${this._escape(envDef.hint)}</span>` : ''}
                </div>
            `;
        }
        return html;
    },

    _buildUserFields(compId, config) {
        const env = config.docker?.environment || {};
        let html = '';

        for (const [envKey, envDef] of Object.entries(env)) {
            if (envDef.source !== 'user_input') continue;
            const fieldId = `dw-ui-${compId}-${envKey}`;
            const label = envDef.label || envKey;
            const placeholder = envDef.placeholder || '';

            html += `
                <div class="form-group">
                    <label>${this._escape(label)}</label>
                    <input type="text" id="${fieldId}" class="form-input"
                           placeholder="${this._escape(placeholder)}">
                    ${envDef.hint ? `<span class="form-hint">${this._escape(envDef.hint)}</span>` : ''}
                </div>
            `;
        }
        return html;
    },

    // ─── Copy & Generation ───────────────────────────────────────────────

    copyInstall(compId) {
        const wiz = this._wizards[compId];
        if (!wiz) return;

        const text = wiz.tab === 'docker'
            ? this._generateDockerCompose(compId)
            : this._generateBinaryInstall(compId);

        this._copyToClipboard(text, () => {
            const label = document.getElementById(`dw-copy-label-${compId}`);
            if (label) {
                const orig = label.textContent;
                label.textContent = 'Copied!';
                setTimeout(() => { label.textContent = orig; }, 2000);
            }
            // Reset user-input fields so the wizard is ready for the next deployment
            this._resetUserFields(compId);
        });
    },

    /** Clear user-input fields after a successful copy, keeping prefill values intact. */
    _resetUserFields(compId) {
        const wiz = this._wizards[compId];
        if (!wiz) return;

        const env = wiz.config.docker?.environment || {};
        for (const [envKey, envDef] of Object.entries(env)) {
            if (envDef.source !== 'user_input') continue;
            const input = document.getElementById(`dw-ui-${compId}-${envKey}`);
            if (input) input.value = '';
        }

        // Also reset the version field
        const version = document.getElementById(`dw-version-${compId}`);
        if (version) version.value = '';
    },

    _generateDockerCompose(compId) {
        const wiz = this._wizards[compId];
        if (!wiz || !wiz.config.docker) return '';

        const docker = wiz.config.docker;
        const platform = wiz.config.docker.platforms[wiz.platform] || {};
        const version = document.getElementById(`dw-version-${compId}`)?.value.trim();
        const tag = version || docker.default_tag || 'latest';
        const image = `${docker.image}:${tag}`;
        const containerName = docker.container_name || docker.image.split('/').pop();

        // Build environment block
        const envLines = [];
        const env = docker.environment || {};
        for (const [envKey, envDef] of Object.entries(env)) {
            const val = this._resolveEnvValue(compId, envKey, envDef);
            envLines.push(`      ${envKey}: ${val}`);
        }

        // Build volumes
        let volumes = (docker.volumes || []).map(v => `      - ${v}`);
        if (platform.extra_volumes) {
            volumes = volumes.concat(platform.extra_volumes.map(v => `      - ${v}`));
        }

        // Build ports
        const ports = (docker.ports || []).map(p => `      - "${p}"`);

        // Build extras
        let extras = '';
        if (docker.privileged) extras += '\n    privileged: true';
        if (docker.network_mode) extras += `\n    network_mode: ${docker.network_mode}`;
        if (platform.pid) extras += `\n    pid: ${platform.pid}`;

        const platformLabel = platform.label || wiz.platform;

        let yaml = `# ${containerName} - docker-compose.yml (${platformLabel})
services:
  ${containerName}:
    image: ${image}
    container_name: ${containerName}
    restart: unless-stopped${extras}`;

        if (!docker.network_mode && ports.length > 0) {
            yaml += `\n    ports:\n${ports.join('\n')}`;
        }

        if (envLines.length > 0) {
            yaml += `\n    environment:\n${envLines.join('\n')}`;
        }

        if (volumes.length > 0) {
            yaml += `\n    volumes:\n${volumes.join('\n')}`;
        }

        return yaml;
    },

    _generateBinaryInstall(compId) {
        const wiz = this._wizards[compId];
        if (!wiz || !wiz.config.binary) return '';

        const binary = wiz.config.binary;
        const version = document.getElementById(`dw-version-${compId}`)?.value.trim();
        const versionFlag = version ? ` -v "${version}"` : '';

        // Collect env values for flags
        const env = wiz.config.docker?.environment || {};
        let flags = '';
        for (const [envKey, envDef] of Object.entries(env)) {
            if (envDef.source === 'literal') continue;
            const val = this._resolveEnvValue(compId, envKey, envDef);
            if (val) {
                flags += ` -e ${envKey}="${val}"`;
            }
        }

        return `curl -sL ${binary.install_url} | bash -s --${flags}${versionFlag}`;
    },

    _resolveEnvValue(compId, envKey, envDef) {
        const wiz = this._wizards[compId];
        if (!wiz) return '';

        switch (envDef.source) {
            case 'prefill': {
                const input = document.getElementById(`dw-pf-${compId}-${envKey}`);
                return input?.value || wiz.prefill[envDef.key] || '';
            }
            case 'user_input': {
                const input = document.getElementById(`dw-ui-${compId}-${envKey}`);
                return input?.value || envDef.placeholder || '';
            }
            case 'literal':
                return envDef.value || '';
            default:
                return '';
        }
    },

    // ─── Clipboard Helpers ───────────────────────────────────────────────

    _toggleSecret(inputId) {
        const input = document.getElementById(inputId);
        if (!input) return;
        const isHidden = input.type === 'password';
        input.type = isHidden ? 'text' : 'password';

        const btn = document.getElementById(`dw-eye-${inputId}`);
        if (btn) {
            const eyeIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
            </svg>`;
            const eyeOffIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/>
                <line x1="1" y1="1" x2="23" y2="23"/>
            </svg>`;
            btn.innerHTML = isHidden ? eyeOffIcon : eyeIcon;
        }
    },

    _copyField(inputId) {
        const input = document.getElementById(inputId);
        if (!input) return;
        this._copyToClipboard(input.value, () => {
            // Find the copy button (last .btn-copy in the wrapper, not the eye toggle).
            const btns = input.parentElement.querySelectorAll('.btn-copy[title="Copy"]');
            const btn = btns.length ? btns[0] : input.parentElement.querySelector('.btn-copy:last-of-type');
            if (btn) {
                const orig = btn.innerHTML;
                btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="var(--success)" stroke-width="2" width="16" height="16"><polyline points="20 6 9 17 4 12"/></svg>';
                setTimeout(() => { btn.innerHTML = orig; }, 2000);
            }
        });
    },

    _copyToClipboard(text, onSuccess) {
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(text).then(onSuccess).catch(() => {
                this._fallbackCopy(text);
                if (onSuccess) onSuccess();
            });
        } else {
            this._fallbackCopy(text);
            if (onSuccess) onSuccess();
        }
    },

    _fallbackCopy(text) {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
    },

    // ─── Helpers ─────────────────────────────────────────────────────────

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = String(str);
        return div.innerHTML;
    },

    _escapeJS(str) {
        if (!str) return '';
        return String(str).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }
};
