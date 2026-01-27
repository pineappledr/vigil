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
                        <div><strong>Drive:</strong> ${driveName}</div>
                        <div><strong>Serial:</strong> ${serialNumber}</div>
                        <div><strong>Server:</strong> ${hostname}</div>
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
                    <button class="btn btn-primary" onclick="Modals.submitAlias('${hostname}', '${serialNumber}')">Save Alias</button>
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
