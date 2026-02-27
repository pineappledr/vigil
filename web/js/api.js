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
    },

    // ─── Agent Management ─────────────────────────────────────────────────────

    async delete(endpoint) {
        return fetch(endpoint, { method: 'DELETE' });
    },

    async getServerPubKey() {
        return this.get('/api/v1/server/pubkey');
    },

    async getAgents() {
        return this.get('/api/v1/agents');
    },

    async deleteAgent(id) {
        return this.delete(`/api/v1/agents/${id}`);
    },

    async createRegistrationToken(name) {
        return this.post('/api/v1/tokens', { name });
    },

    async getRegistrationTokens() {
        return this.get('/api/v1/tokens');
    },

    async deleteRegistrationToken(id) {
        return this.delete(`/api/v1/tokens/${id}`);
    },

    // ─── Add-on Endpoints ────────────────────────────────────────────────────

    async put(endpoint, data) {
        return fetch(endpoint, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
    },

    async getAddons() {
        return this.get('/api/addons');
    },

    async getAddon(id) {
        return this.get(`/api/addons/${id}`);
    },

    async registerAddon(manifest) {
        return this.post('/api/addons', { manifest });
    },

    async deregisterAddon(id) {
        return this.delete(`/api/addons/${id}`);
    },

    async setAddonEnabled(id, enabled) {
        return this.put(`/api/addons/${id}/enabled`, { enabled });
    },

    // ─── Notification Endpoints ──────────────────────────────────────────────

    async getNotificationServices() {
        return this.get('/api/notifications/services');
    },

    async getNotificationService(id) {
        return this.get(`/api/notifications/services/${id}`);
    },

    async createNotificationService(service) {
        return this.post('/api/notifications/services', service);
    },

    async updateNotificationService(id, service) {
        return this.put(`/api/notifications/services/${id}`, service);
    },

    async deleteNotificationService(id) {
        return this.delete(`/api/notifications/services/${id}`);
    },

    async updateEventRules(serviceId, rules) {
        return this.put(`/api/notifications/services/${serviceId}/rules`, { rules });
    },

    async updateQuietHours(serviceId, quietHours) {
        return this.put(`/api/notifications/services/${serviceId}/quiet-hours`, quietHours);
    },

    async updateDigestConfig(serviceId, digest) {
        return this.put(`/api/notifications/services/${serviceId}/digest`, digest);
    },

    async testFireNotification(serviceId, message) {
        return this.post('/api/notifications/test', { service_id: serviceId, message });
    },

    async getNotificationHistory(limit = 50) {
        return this.get(`/api/notifications/history?limit=${limit}`);
    }
};