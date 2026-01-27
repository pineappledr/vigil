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
    }
};
