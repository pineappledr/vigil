/**
 * Vigil Dashboard - API Client
 */

const API = {
    async get(endpoint) {
        const response = await fetch(endpoint);
        return response;
    },

    async post(endpoint, data) {
        return fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
    },

    async getHistory() {
        return this.get(State.API_URL);
    },

    async getAuthStatus() {
        return this.get('/api/auth/status');
    },

    async getVersion() {
        return this.get('/api/version');
    },

    async login(username, password) {
        return this.post('/api/auth/login', { username, password });
    },

    async logout() {
        return this.post('/api/auth/logout', {});
    },

    async changePassword(currentPassword, newPassword) {
        return this.post('/api/users/password', {
            current_password: currentPassword,
            new_password: newPassword
        });
    },

    async changeUsername(newUsername, currentPassword) {
        return this.post('/api/users/username', {
            new_username: newUsername,
            current_password: currentPassword
        });
    },

    async setAlias(hostname, serialNumber, alias) {
        return this.post('/api/aliases', {
            hostname,
            serial_number: serialNumber,
            alias
        });
    },

    // ─── ZFS Endpoints ───────────────────────────────────────────────────────

    /**
     * Get all ZFS pools, optionally filtered by hostname
     * @param {string} [hostname] - Filter by specific host
     * @returns {Promise<Response>}
     */
    async getZFSPools(hostname) {
        const query = hostname ? `?hostname=${encodeURIComponent(hostname)}` : '';
        return this.get(`/api/zfs/pools${query}`);
    },

    /**
     * Get single ZFS pool with devices and scrub history
     * @param {string} hostname
     * @param {string} poolName
     * @returns {Promise<Response>}
     */
    async getZFSPool(hostname, poolName) {
        return this.get(`/api/zfs/pools/${encodeURIComponent(hostname)}/${encodeURIComponent(poolName)}`);
    },

    /**
     * Get devices for a specific pool
     * @param {string} hostname
     * @param {string} poolName
     * @returns {Promise<Response>}
     */
    async getZFSPoolDevices(hostname, poolName) {
        return this.get(`/api/zfs/pools/${encodeURIComponent(hostname)}/${encodeURIComponent(poolName)}/devices`);
    },

    /**
     * Get scrub history for a pool
     * @param {string} hostname
     * @param {string} poolName
     * @param {number} [limit=5] - Number of records to return
     * @returns {Promise<Response>}
     */
    async getZFSScrubHistory(hostname, poolName, limit = 5) {
        return this.get(`/api/zfs/pools/${encodeURIComponent(hostname)}/${encodeURIComponent(poolName)}/scrubs?limit=${limit}`);
    },

    /**
     * Get ZFS health summary (pools needing attention)
     * @returns {Promise<Response>}
     */
    async getZFSHealth() {
        return this.get('/api/zfs/health');
    },

    /**
     * Get aggregate ZFS statistics
     * @param {string} [hostname] - Filter by specific host
     * @returns {Promise<Response>}
     */
    async getZFSSummary(hostname) {
        const query = hostname ? `?hostname=${encodeURIComponent(hostname)}` : '';
        return this.get(`/api/zfs/summary${query}`);
    },

    /**
     * Get ZFS pool info for a specific drive by serial number
     * @param {string} hostname
     * @param {string} serial
     * @returns {Promise<Response>}
     */
    async getZFSDriveInfo(hostname, serial) {
        return this.get(`/api/zfs/drive/${encodeURIComponent(hostname)}/${encodeURIComponent(serial)}`);
    }
};