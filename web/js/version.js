/**
 * Vigil Dashboard - Version Check Module
 * Checks for updates and displays notification banner
 */

const Version = {
    // State
    updateInfo: null,
    dismissedVersion: null,
    checkInterval: null,

    // Configuration
    CHECK_INTERVAL_MS: 60 * 60 * 1000, // 1 hour
    STORAGE_KEY: 'vigil_dismissed_version',

    /**
     * Initialize version checking
     */
    init() {
        // Load dismissed version from localStorage
        this.dismissedVersion = localStorage.getItem(this.STORAGE_KEY);

        // Check immediately on load
        this.checkForUpdates();

        // Set up periodic checks
        this.checkInterval = setInterval(() => {
            this.checkForUpdates();
        }, this.CHECK_INTERVAL_MS);
    },

    /**
     * Check for updates from the API
     */
    async checkForUpdates() {
        try {
            const response = await fetch('/api/version/check');
            if (!response.ok) {
                console.warn('[Version] Failed to check for updates:', response.status);
                return;
            }

            const info = await response.json();
            this.updateInfo = info;

            // Show banner if update available and not dismissed
            if (info.update_available && info.latest_version !== this.dismissedVersion) {
                this.showUpdateBanner(info);
            } else {
                this.hideUpdateBanner();
            }

            // Update version display in footer
            this.updateVersionDisplay(info.current_version);

        } catch (error) {
            console.error('[Version] Error checking for updates:', error);
        }
    },

    /**
     * Show the update notification banner
     */
    showUpdateBanner(info) {
        // Remove existing banner if any
        this.hideUpdateBanner();

        const banner = document.createElement('div');
        banner.id = 'update-banner';
        banner.className = 'update-banner';
        banner.innerHTML = `
            <div class="update-banner-content">
                <div class="update-banner-icon">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"/>
                        <line x1="12" y1="16" x2="12" y2="12"/>
                        <line x1="12" y1="8" x2="12.01" y2="8"/>
                    </svg>
                </div>
                <div class="update-banner-text">
                    <span class="update-banner-title">Update Available</span>
                    <span class="update-banner-version">
                        v${this.escapeHtml(info.current_version)} → 
                        <strong>v${this.escapeHtml(info.latest_version)}</strong>
                    </span>
                </div>
                <div class="update-banner-actions">
                    <a href="${this.escapeHtml(info.release_url)}" 
                       target="_blank" 
                       rel="noopener noreferrer"
                       class="update-banner-btn primary">
                        View Release
                    </a>
                    <button class="update-banner-btn secondary" onclick="Version.dismissUpdate()">
                        Dismiss
                    </button>
                </div>
            </div>
        `;

        // Insert at top of main content
        const mainContent = document.querySelector('.main-content');
        if (mainContent) {
            mainContent.insertBefore(banner, mainContent.firstChild);
        }
    },

    /**
     * Hide the update banner
     */
    hideUpdateBanner() {
        const banner = document.getElementById('update-banner');
        if (banner) {
            banner.remove();
        }
    },

    /**
     * Dismiss the current update notification
     */
    dismissUpdate() {
        if (this.updateInfo && this.updateInfo.latest_version) {
            this.dismissedVersion = this.updateInfo.latest_version;
            localStorage.setItem(this.STORAGE_KEY, this.dismissedVersion);
        }
        this.hideUpdateBanner();
    },

    /**
     * Update the version display in the sidebar footer
     */
    updateVersionDisplay(version) {
        const versionEl = document.getElementById('app-version');
        if (versionEl) {
            const displayVersion = version.startsWith('v') ? version : `v${version}`;
            versionEl.textContent = displayVersion;

            // Add update indicator if available
            if (this.updateInfo && this.updateInfo.update_available) {
                versionEl.classList.add('has-update');
                versionEl.title = `Update available: v${this.updateInfo.latest_version}`;
            } else {
                versionEl.classList.remove('has-update');
                versionEl.title = '';
            }
        }
    },

    /**
     * Get update info (for use by other modules)
     */
    getUpdateInfo() {
        return this.updateInfo;
    },

    /**
     * Check if an update is available
     */
    isUpdateAvailable() {
        return this.updateInfo?.update_available === true;
    },

    /**
     * Force a fresh check (bypasses cache)
     */
    async forceCheck() {
        try {
            const response = await fetch('/api/version/check?force=true');
            if (response.ok) {
                const info = await response.json();
                this.updateInfo = info;

                if (info.update_available && info.latest_version !== this.dismissedVersion) {
                    this.showUpdateBanner(info);
                }

                return info;
            }
        } catch (error) {
            console.error('[Version] Force check failed:', error);
        }
        return null;
    },

    /**
     * Clear dismissed version (for testing or settings)
     */
    clearDismissed() {
        this.dismissedVersion = null;
        localStorage.removeItem(this.STORAGE_KEY);

        // Re-check to show banner if update available
        if (this.updateInfo?.update_available) {
            this.showUpdateBanner(this.updateInfo);
        }
    },

    /**
     * Escape HTML to prevent XSS
     */
    escapeHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    },

    /**
     * Manual check triggered by user (shows result in UI)
     * Used by Settings page "Check for Updates" button
     */
    async manualCheck() {
        const button = document.getElementById('check-updates-btn');
        const statusEl = document.getElementById('update-check-status');
        
        if (button) {
            button.disabled = true;
            button.innerHTML = `
                <span class="spinner-small"></span>
                Checking...
            `;
        }
        
        try {
            const info = await this.forceCheck();
            
            if (statusEl) {
                if (info && info.update_available) {
                    statusEl.innerHTML = `
                        <div class="update-status update-available">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <circle cx="12" cy="12" r="10"/>
                                <line x1="12" y1="8" x2="12" y2="12"/>
                                <line x1="12" y1="16" x2="12.01" y2="16"/>
                            </svg>
                            <div class="update-status-text">
                                <strong>Update available!</strong>
                                <span>v${this.escapeHtml(info.current_version)} → v${this.escapeHtml(info.latest_version)}</span>
                            </div>
                            <a href="${this.escapeHtml(info.release_url)}" target="_blank" rel="noopener" class="btn btn-primary btn-sm">
                                View Release
                            </a>
                        </div>
                    `;
                } else if (info) {
                    statusEl.innerHTML = `
                        <div class="update-status up-to-date">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                                <polyline points="22 4 12 14.01 9 11.01"/>
                            </svg>
                            <div class="update-status-text">
                                <strong>You're up to date!</strong>
                                <span>Version ${this.escapeHtml(info.current_version)} is the latest</span>
                            </div>
                        </div>
                    `;
                } else {
                    statusEl.innerHTML = `
                        <div class="update-status update-error">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <circle cx="12" cy="12" r="10"/>
                                <line x1="15" y1="9" x2="9" y2="15"/>
                                <line x1="9" y1="9" x2="15" y2="15"/>
                            </svg>
                            <div class="update-status-text">
                                <strong>Couldn't check for updates</strong>
                                <span>Please try again later</span>
                            </div>
                        </div>
                    `;
                }
            }
        } catch (error) {
            console.error('[Version] Manual check failed:', error);
            if (statusEl) {
                statusEl.innerHTML = `
                    <div class="update-status update-error">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="10"/>
                            <line x1="15" y1="9" x2="9" y2="15"/>
                            <line x1="9" y1="9" x2="15" y2="15"/>
                        </svg>
                        <div class="update-status-text">
                            <strong>Connection error</strong>
                            <span>Please check your internet connection</span>
                        </div>
                    </div>
                `;
            }
        } finally {
            if (button) {
                button.disabled = false;
                button.innerHTML = `
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"/>
                        <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                    </svg>
                    Check for Updates
                `;
            }
        }
    },

    /**
     * Cleanup (call on page unload if needed)
     */
    destroy() {
        if (this.checkInterval) {
            clearInterval(this.checkInterval);
            this.checkInterval = null;
        }
    }
};

// Export for use in other modules
window.Version = Version;