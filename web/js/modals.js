/**
 * Vigil Dashboard - Modal Dialogs
 */

const Modals = {
    create(content) {
        const modal = document.createElement('div');
        modal.className = 'modal-overlay';
        modal.innerHTML = content;
        document.body.appendChild(modal);
        return modal;
    },

    close(element) {
        element?.closest('.modal-overlay')?.remove();
    },

    showChangePassword() {
        const modal = this.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>Change Password</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label>Current Password</label>
                        <input type="password" id="current-password" class="form-input">
                    </div>
                    <div class="form-group">
                        <label>New Password</label>
                        <input type="password" id="new-password" class="form-input">
                    </div>
                    <div class="form-group">
                        <label>Confirm New Password</label>
                        <input type="password" id="confirm-password" class="form-input">
                    </div>
                    <div id="password-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-primary" onclick="Modals.submitPasswordChange()">Change Password</button>
                </div>
            </div>
        `);
        document.getElementById('current-password').focus();
    },

    showForcePasswordChange() {
        const modal = this.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>🔐 Password Change Required</h3>
                </div>
                <div class="modal-body">
                    <p class="modal-message">For security, you must change your password before continuing.</p>
                    <div class="form-group">
                        <label>Current Password</label>
                        <input type="password" id="force-current-password" class="form-input">
                    </div>
                    <div class="form-group">
                        <label>New Password</label>
                        <input type="password" id="force-new-password" class="form-input">
                    </div>
                    <div class="form-group">
                        <label>Confirm New Password</label>
                        <input type="password" id="force-confirm-password" class="form-input">
                    </div>
                    <div id="force-password-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-primary" onclick="Modals.submitForcePasswordChange()">Set New Password</button>
                </div>
            </div>
        `);
        modal.id = 'force-password-modal';
        document.getElementById('force-current-password').focus();
    },

    showChangeUsername() {
        const modal = this.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>Change Username</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <p class="form-hint" style="margin-bottom: 16px;">
                        Changing your username will update it immediately. You will need to use the new username for future logins.
                    </p>
                    <div class="form-group">
                        <label>New Username</label>
                        <input type="text" id="new-username" class="form-input" placeholder="Enter new username">
                    </div>
                    <div class="form-group">
                        <label>Current Password</label>
                        <input type="password" id="current-password-for-user" class="form-input" placeholder="Verify your password">
                    </div>
                    <div id="username-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-primary" onclick="Modals.submitUsernameChange()">Save Changes</button>
                </div>
            </div>
        `);
        document.getElementById('new-username').focus();
    },

    showAlias(hostname, serialNumber, currentAlias, driveName) {
        const modal = this.create(`
            <div class="modal">
                <div class="modal-header">
                    <h3>Set Drive Alias</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body">
                    <div class="alias-drive-info">
                        <div><strong>Drive:</strong> ${Utils.escapeHtml(driveName)}</div>
                        <div><strong>Serial:</strong> ${Utils.escapeHtml(serialNumber)}</div>
                        <div><strong>Server:</strong> ${Utils.escapeHtml(hostname)}</div>
                    </div>
                    <div class="form-group">
                        <label>Alias (friendly name)</label>
                        <input type="text" id="alias-input" class="form-input" 
                               value="${currentAlias || ''}" 
                               placeholder="e.g., Plex Media, VM Storage, Backup Drive">
                    </div>
                    <p class="form-hint">Leave empty to remove the alias</p>
                    <div id="alias-error" class="form-error"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="Modals.close(this)">Cancel</button>
                    <button class="btn btn-primary" onclick="Modals.submitAlias('${Utils.escapeJSString(hostname)}', '${Utils.escapeJSString(serialNumber)}')">Save Alias</button>
                </div>
            </div>
        `);
        document.getElementById('alias-input').focus();
    },

    async submitPasswordChange() {
        const currentPassword = document.getElementById('current-password').value;
        const newPassword = document.getElementById('new-password').value;
        const confirmPassword = document.getElementById('confirm-password').value;
        const errorEl = document.getElementById('password-error');

        const error = this.validatePassword(newPassword, confirmPassword);
        if (error) {
            errorEl.textContent = error;
            return;
        }

        try {
            const response = await API.changePassword(currentPassword, newPassword);
            if (response.ok) {
                document.querySelector('.modal-overlay').remove();
                alert('Password changed successfully');
            } else {
                const data = await response.json();
                errorEl.textContent = data.error || 'Failed to change password';
            }
        } catch (e) {
            errorEl.textContent = 'Connection error';
        }
    },

    async submitForcePasswordChange() {
        const currentPassword = document.getElementById('force-current-password').value;
        const newPassword = document.getElementById('force-new-password').value;
        const confirmPassword = document.getElementById('force-confirm-password').value;
        const errorEl = document.getElementById('force-password-error');

        if (!currentPassword) {
            errorEl.textContent = 'Please enter your current password';
            return;
        }

        const error = this.validatePassword(newPassword, confirmPassword, currentPassword);
        if (error) {
            errorEl.textContent = error;
            return;
        }

        try {
            const response = await API.changePassword(currentPassword, newPassword);
            if (response.ok) {
                document.getElementById('force-password-modal').remove();
                State.mustChangePassword = false;
                alert('Password changed successfully! Welcome to Vigil.');
            } else {
                const data = await response.json();
                errorEl.textContent = data.error || 'Failed to change password';
            }
        } catch (e) {
            errorEl.textContent = 'Connection error';
        }
    },

    async submitUsernameChange() {
        const newUsername = document.getElementById('new-username').value.trim();
        const currentPassword = document.getElementById('current-password-for-user').value;
        const errorEl = document.getElementById('username-error');

        if (!newUsername) {
            errorEl.textContent = 'Username cannot be empty';
            return;
        }
        if (!currentPassword) {
            errorEl.textContent = 'Please enter your password to confirm';
            return;
        }

        try {
            const response = await API.changeUsername(newUsername, currentPassword);
            const data = await response.json();

            if (response.ok) {
                document.querySelector('.modal-overlay').remove();
                State.currentUser = newUsername;
                Navigation.showSettings();
                Auth.updateUserUI();
                alert(`Username successfully changed to ${newUsername}`);
            } else {
                errorEl.textContent = data.error || 'Failed to change username';
            }
        } catch (e) {
            errorEl.textContent = 'Connection error';
        }
    },

    async submitAlias(hostname, serialNumber) {
        const alias = document.getElementById('alias-input').value.trim();
        const errorEl = document.getElementById('alias-error');

        try {
            const response = await API.setAlias(hostname, serialNumber, alias);
            if (response.ok) {
                document.querySelector('.modal-overlay').remove();
                Data.fetch();
            } else {
                const data = await response.json();
                errorEl.textContent = data.error || 'Failed to save alias';
            }
        } catch (e) {
            errorEl.textContent = 'Connection error';
        }
    },

    // ─── Add Agent Modal ────────────────────────────────────────────────────

    async showAddAgent() {
        let serverPubKey = '';
        try {
            const resp = await API.getServerPubKey();
            const data = await resp.json();
            serverPubKey = data.public_key || '';
        } catch (e) {
            console.error('Failed to fetch server public key:', e);
        }

        let token = '';
        try {
            const resp = await API.createRegistrationToken('');
            const data = await resp.json();
            token = data.token || '';
        } catch (e) {
            console.error('Failed to create registration token:', e);
        }

        const serverURL = window.location.origin;

        const modal = this.create(`
            <div class="modal modal-add-agent">
                <div class="modal-header">
                    <h3>Add System</h3>
                    <button class="modal-close" onclick="Modals.close(this)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
                <div class="modal-body" style="padding-bottom: 0;">
                    <div class="agent-tab-bar">
                        <button class="agent-tab active" data-tab="docker" onclick="Modals.switchAgentTab('docker')">Docker</button>
                        <button class="agent-tab" data-tab="binary" onclick="Modals.switchAgentTab('binary')">Binary</button>
                    </div>
                    <div class="agent-platform-bar" id="agent-platform-bar">
                        <button class="agent-platform-btn active" data-platform="linux" onclick="Modals.switchAgentPlatform('linux')">Standard Linux</button>
                        <button class="agent-platform-btn" data-platform="truenas" onclick="Modals.switchAgentPlatform('truenas')">TrueNAS</button>
                    </div>
                    <p class="agent-tab-hint" id="agent-tab-hint">
                        Copy the <code>docker-compose.yml</code> to deploy the agent on a standard Linux host.
                    </p>
                    <div class="form-group">
                        <label>Name</label>
                        <input type="text" id="agent-name" class="form-input" placeholder="my-server-agent-01">
                    </div>
                    <div class="form-group">
                        <label>Server URL</label>
                        <div class="form-input-with-copy">
                            <input type="text" id="agent-server-url" class="form-input form-input-mono" value="${serverURL}" readonly>
                            <button class="btn-copy" onclick="Modals.copyField('agent-server-url')" title="Copy">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                                    <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                                </svg>
                            </button>
                        </div>
                    </div>
                    <div class="form-group">
                        <label>Public Key</label>
                        <div class="form-input-with-copy">
                            <input type="text" id="agent-pubkey" class="form-input form-input-mono" value="${serverPubKey}" readonly>
                            <button class="btn-copy" onclick="Modals.copyField('agent-pubkey')" title="Copy">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                                    <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                                </svg>
                            </button>
                        </div>
                    </div>
                    <div class="form-group">
                        <label>Token</label>
                        <div class="form-input-with-copy">
                            <input type="text" id="agent-token" class="form-input form-input-mono" value="${token}" readonly>
                            <button class="btn-copy" onclick="Modals.copyField('agent-token')" title="Copy">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                                    <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                                </svg>
                            </button>
                        </div>
                    </div>
                    <div class="form-group">
                        <label>Version</label>
                        <input type="text" id="agent-version" class="form-input" placeholder="latest" value="">
                        <span class="form-hint" style="margin-top: 4px;">Leave empty for latest. Use a tag like <code>v2.4.0</code> for a specific version.</span>
                    </div>
                    <div class="agent-option-row">
                        <label class="agent-checkbox">
                            <input type="checkbox" id="agent-zfs" onchange="Modals.toggleZFS(this.checked)">
                            ZFS Monitoring
                        </label>
                        <span class="form-hint">Include ZFS volumes and dependencies</span>
                    </div>
                    <div id="agent-error" class="form-error"></div>
                </div>
                <div class="modal-footer agent-modal-footer">
                    <div class="agent-copy-group">
                        <button class="btn btn-secondary btn-with-icon" id="agent-copy-btn" onclick="Modals.copyAgentInstall()">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                                <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                            </svg>
                            <span id="agent-copy-label">Copy docker compose</span>
                        </button>
                    </div>
                    <button class="btn btn-primary" onclick="Modals.submitAddAgent()">Add system</button>
                </div>
            </div>
        `);

        modal._agentState = { tab: 'docker', platform: 'linux', zfs: false, serverURL, serverPubKey, token };
        document.getElementById('agent-name').focus();
    },

    switchAgentTab(tab) {
        document.querySelectorAll('.agent-tab').forEach(el => {
            el.classList.toggle('active', el.dataset.tab === tab);
        });

        const overlay = document.querySelector('.modal-overlay');
        if (overlay?._agentState) overlay._agentState.tab = tab;

        const platformBar = document.getElementById('agent-platform-bar');
        const label = document.getElementById('agent-copy-label');

        if (tab === 'docker') {
            platformBar.innerHTML = `
                <button class="agent-platform-btn active" data-platform="linux" onclick="Modals.switchAgentPlatform('linux')">Standard Linux</button>
                <button class="agent-platform-btn" data-platform="truenas" onclick="Modals.switchAgentPlatform('truenas')">TrueNAS</button>
            `;
            if (overlay?._agentState) overlay._agentState.platform = 'linux';
            label.textContent = 'Copy docker compose';
            this.switchAgentPlatform('linux');
        } else {
            platformBar.innerHTML = `
                <button class="agent-platform-btn active" data-platform="debian" onclick="Modals.switchAgentPlatform('debian')">Debian / Ubuntu</button>
                <button class="agent-platform-btn" data-platform="fedora" onclick="Modals.switchAgentPlatform('fedora')">Fedora / RHEL</button>
                <button class="agent-platform-btn" data-platform="arch" onclick="Modals.switchAgentPlatform('arch')">Arch</button>
            `;
            if (overlay?._agentState) overlay._agentState.platform = 'debian';
            label.textContent = 'Copy install command';
            this.switchAgentPlatform('debian');
        }
    },

    switchAgentPlatform(platform) {
        document.querySelectorAll('.agent-platform-btn').forEach(el => {
            el.classList.toggle('active', el.dataset.platform === platform);
        });

        const overlay = document.querySelector('.modal-overlay');
        const prevPlatform = overlay?._agentState?.platform;
        if (overlay?._agentState) overlay._agentState.platform = platform;

        // Auto-toggle ZFS for TrueNAS
        const zfsCheckbox = document.getElementById('agent-zfs');
        if (platform === 'truenas') {
            if (zfsCheckbox) zfsCheckbox.checked = true;
            this.toggleZFS(true);
        } else if (prevPlatform === 'truenas' && zfsCheckbox) {
            zfsCheckbox.checked = false;
            this.toggleZFS(false);
        }

        const hint = document.getElementById('agent-tab-hint');
        const hints = {
            linux: 'Copy the <code>docker-compose.yml</code> to deploy the agent on a standard Linux host.',
            truenas: 'Copy the <code>docker-compose.yml</code> for <strong>TrueNAS</strong> SCALE/CORE with host ZFS tool mounts.',
            debian: 'Install dependencies and the agent binary on <strong>Debian / Ubuntu / Proxmox</strong>.',
            fedora: 'Install dependencies and the agent binary on <strong>Fedora / RHEL / CentOS</strong>.',
            arch: 'Install dependencies and the agent binary on <strong>Arch Linux</strong>.'
        };
        if (hint) hint.innerHTML = hints[platform] || '';
    },

    toggleZFS(enabled) {
        const overlay = document.querySelector('.modal-overlay');
        if (overlay?._agentState) overlay._agentState.zfs = enabled;
    },

    copyField(inputId) {
        const input = document.getElementById(inputId);
        if (!input) return;
        this._copyToClipboard(input.value, () => {
            const btn = input.parentElement.querySelector('.btn-copy');
            if (btn) {
                const orig = btn.innerHTML;
                btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="var(--success)" stroke-width="2" width="16" height="16"><polyline points="20 6 9 17 4 12"/></svg>';
                setTimeout(() => { btn.innerHTML = orig; }, 2000);
            }
        });
    },

    copyAgentInstall() {
        const overlay = document.querySelector('.modal-overlay');
        const st = overlay?._agentState;
        if (!st) return;

        const name = document.getElementById('agent-name')?.value.trim() || 'vigil-agent';
        const token = document.getElementById('agent-token')?.value || '';
        const serverURL = document.getElementById('agent-server-url')?.value || st.serverURL;
        const version = document.getElementById('agent-version')?.value.trim() || '';
        const text = this._generateInstallContent(st.tab, st.platform, serverURL, token, name, st.zfs, version);

        this._copyToClipboard(text, () => {
            const label = document.getElementById('agent-copy-label');
            if (label) {
                const orig = label.textContent;
                label.textContent = 'Copied!';
                setTimeout(() => { label.textContent = orig; }, 2000);
            }
        });
    },

    _generateInstallContent(tab, platform, serverURL, token, name, zfs, version) {
        if (tab === 'docker') {
            return this._generateDockerCompose(platform, serverURL, token, name, zfs, version);
        }
        return this._generateBinaryInstall(serverURL, token, name, zfs, version);
    },

    _generateDockerCompose(platform, serverURL, token, name, zfs, version) {
        const isTrueNAS = platform === 'truenas';
        const tag = version || (isTrueNAS ? 'debian' : 'latest');
        const image = `ghcr.io/pineappledr/vigil-agent:${tag}`;

        let volumes = `      - /dev:/dev:ro`;

        if (zfs && isTrueNAS) {
            volumes += `
      - /dev/zfs:/dev/zfs
      - /sbin/zpool:/sbin/zpool:ro
      - /sbin/zfs:/sbin/zfs:ro
      - /lib:/lib:ro
      - /lib64:/lib64:ro
      - /usr/lib:/usr/lib:ro`;
        } else if (zfs) {
            volumes += `
      - /sys:/sys:ro
      - /proc:/proc:ro
      - /dev/zfs:/dev/zfs`;
        }

        let extras = '';
        if (isTrueNAS) extras = '\n    pid: host';

        let deploy = '';
        if (isTrueNAS) {
            deploy = `
    deploy:
      resources:
        limits:
          cpus: '0.50'
          memory: 512M
        reservations:
          cpus: '0.10'
          memory: 128M`;
        }

        return `# Vigil Agent - docker-compose.yml (${isTrueNAS ? 'TrueNAS' : 'Standard Linux'})
services:
  vigil-agent:
    image: ${image}
    container_name: vigil-agent
    restart: unless-stopped
    network_mode: host${extras}
    privileged: true
    environment:
      SERVER: ${serverURL}
      TOKEN: ${token}
      HOSTNAME: ${name}
      TZ: \${TZ:-UTC}
    volumes:
      - vigil_agent_data:/var/lib/vigil-agent
${volumes}${deploy}

volumes:
  vigil_agent_data:`;
    },

    _generateBinaryInstall(serverURL, token, name, zfs, version) {
        const zfsFlag = zfs ? ' -z' : '';
        const versionFlag = version ? ` -v "${version}"` : '';
        return `curl -sL https://raw.githubusercontent.com/pineappledr/vigil/main/scripts/install-agent.sh | bash -s -- -s "${serverURL}" -t "${token}" -n "${name}"${zfsFlag}${versionFlag}`;
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

    async submitAddAgent() {
        const name = document.getElementById('agent-name')?.value.trim();
        const errorEl = document.getElementById('agent-error');
        if (!name) {
            if (errorEl) errorEl.textContent = 'Name is required';
            return;
        }
        document.querySelector('.modal-overlay')?.remove();
        if (typeof Agents !== 'undefined' && Agents.render) Agents.render();
    },

    validatePassword(newPassword, confirmPassword, currentPassword = null) {
        if (newPassword !== confirmPassword) {
            return 'New passwords do not match';
        }
        if (newPassword.length < 6) {
            return 'Password must be at least 6 characters';
        }
        if (currentPassword && currentPassword === newPassword) {
            return 'New password must be different from current password';
        }
        return null;
    }
};
