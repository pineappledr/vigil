/**
 * Vigil Dashboard - Application State
 */

const State = {
    data: [],
    activeServerIndex: null,
    activeFilter: null,
    refreshTimer: null,
    currentUser: null,
    mustChangePassword: false,

    API_URL: '/api/history',
    REFRESH_INTERVAL: 5000,

    reset() {
        this.activeServerIndex = null;
        this.activeFilter = null;
    },

    setFilter(filter) {
        this.activeServerIndex = null;
        this.activeFilter = filter;
    },

    setServer(index) {
        this.activeServerIndex = index;
        this.activeFilter = null;
    },

    getStats() {
        let totalServers = this.data.length;
        let totalDrives = 0;
        let healthyDrives = 0;
        let attentionDrives = 0;

        this.data.forEach(server => {
            const drives = server.details?.drives || [];
            totalDrives += drives.length;
            drives.forEach(drive => {
                if (Utils.getHealthStatus(drive) === 'healthy') {
                    healthyDrives++;
                } else {
                    attentionDrives++;
                }
            });
        });

        return { totalServers, totalDrives, healthyDrives, attentionDrives };
    }
};
