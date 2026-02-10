/**
 * Vigil Dashboard - Application State
 */

const State = {
    data: [],
    activeServerIndex: null,
    activeServerHostname: null,
    activeFilter: null,
    refreshTimer: null,
    currentUser: null,
    mustChangePassword: false,

    // ZFS State
    zfsPools: [],
    zfsDriveMap: {},
    activeView: 'drives',   // 'drives' | 'zfs' | 'settings'

    API_URL: '/api/history',
    REFRESH_INTERVAL: 5000,
    
    // Offline threshold in minutes
    OFFLINE_THRESHOLD_MINUTES: 5,

    reset() {
        this.activeServerIndex = null;
        this.activeServerHostname = null;
        this.activeFilter = null;
        this.activeView = 'drives';
    },

    setFilter(filter) {
        this.activeServerIndex = null;
        this.activeServerHostname = null;
        this.activeFilter = filter;
        this.activeView = 'drives';
    },

    setServer(index) {
        this.activeServerIndex = index;
        this.activeServerHostname = this.data[index]?.hostname || null;
        this.activeFilter = null;
        this.activeView = 'drives';  // Always reset to drives view
    },

    setView(view) {
        this.activeView = view;
        if (view !== 'drives') {
            this.activeServerIndex = null;
            this.activeServerHostname = null;
            this.activeFilter = null;
        }
    },

    resolveActiveServer() {
        if (this.activeServerHostname) {
            const newIndex = this.data.findIndex(s => s.hostname === this.activeServerHostname);
            if (newIndex !== -1) {
                this.activeServerIndex = newIndex;
            } else {
                this.activeServerIndex = null;
                this.activeServerHostname = null;
            }
        }
    },

    /**
     * Check if a server is offline based on last_seen timestamp
     */
    isServerOffline(server) {
        if (!server || !server.last_seen) return false;
        const lastSeen = new Date(server.last_seen);
        const now = new Date();
        const diffMinutes = (now - lastSeen) / (1000 * 60);
        return diffMinutes > this.OFFLINE_THRESHOLD_MINUTES;
    },

    /**
     * Get time since last update for a server
     */
    getTimeSinceUpdate(server) {
        if (!server || !server.last_seen) return 'Unknown';
        const lastSeen = new Date(server.last_seen);
        const now = new Date();
        const diffMs = now - lastSeen;
        const diffSeconds = Math.floor(diffMs / 1000);
        const diffMinutes = Math.floor(diffSeconds / 60);
        const diffHours = Math.floor(diffMinutes / 60);
        const diffDays = Math.floor(diffHours / 24);
        
        if (diffDays > 0) return `${diffDays}d ago`;
        if (diffHours > 0) return `${diffHours}h ago`;
        if (diffMinutes > 0) return `${diffMinutes}m ago`;
        return 'Just now';
    },

    getStats() {
        let totalServers = this.data.length;
        let totalDrives = 0;
        let healthyDrives = 0;
        let attentionDrives = 0;
        let offlineServers = 0;

        this.data.forEach(server => {
            const drives = server.details?.drives || [];
            totalDrives += drives.length;
            
            if (this.isServerOffline(server)) {
                offlineServers++;
            }
            
            drives.forEach(drive => {
                if (Utils.getHealthStatus(drive) === 'healthy') {
                    healthyDrives++;
                } else {
                    attentionDrives++;
                }
            });
        });

        return { totalServers, totalDrives, healthyDrives, attentionDrives, offlineServers };
    },

    // ─── ZFS State Methods ───────────────────────────────────────────────────

    getZFSStats() {
        const pools = Array.isArray(this.zfsPools) ? this.zfsPools : [];
        
        let totalPools = pools.length;
        let healthyPools = 0;
        let degradedPools = 0;
        let faultedPools = 0;
        let totalErrors = 0;

        pools.forEach(pool => {
            if (!pool) return;
            const state = (pool.status || pool.health || pool.state || '').toUpperCase();
            
            if (state === 'ONLINE') {
                healthyPools++;
            } else if (state === 'DEGRADED') {
                degradedPools++;
            } else if (state === 'FAULTED' || state === 'UNAVAIL') {
                faultedPools++;
            }

            totalErrors += (pool.read_errors || 0) + (pool.write_errors || 0) + (pool.checksum_errors || 0);

            const devices = Array.isArray(pool.devices) ? pool.devices : [];
            devices.forEach(device => {
                if (!device) return;
                totalErrors += (device.read_errors || 0) + (device.write_errors || 0) + (device.checksum_errors || 0);
            });
        });

        return { 
            totalPools, 
            healthyPools, 
            degradedPools, 
            faultedPools,
            attentionPools: degradedPools + faultedPools,
            totalErrors
        };
    },

    getPoolsByHost() {
        const grouped = {};
        const pools = Array.isArray(this.zfsPools) ? this.zfsPools : [];
        
        pools.forEach(pool => {
            if (!pool) return;
            const host = pool.hostname || 'unknown';
            if (!grouped[host]) {
                grouped[host] = [];
            }
            grouped[host].push(pool);
        });

        return grouped;
    },

    buildZFSDriveMap() {
        this.zfsDriveMap = {};
        const pools = Array.isArray(this.zfsPools) ? this.zfsPools : [];
        
        pools.forEach(pool => {
            if (!pool) return;
            
            const hostname = pool.hostname || '';
            const poolName = pool.name || pool.pool_name || 'unknown';
            const poolState = (pool.status || pool.state || pool.health || 'UNKNOWN').toUpperCase();
            const devices = Array.isArray(pool.devices) ? pool.devices : [];
            
            devices.forEach(device => {
                if (!device) return;
                const serial = device.serial_number || device.serial;
                if (!serial) return;
                
                const key = `${hostname}:${serial}`;
                this.zfsDriveMap[key] = {
                    poolName: poolName,
                    poolState: poolState,
                    vdev: device.vdev_parent || device.vdev || '',
                    deviceName: device.device_name || device.name || '',
                    readErrors: device.read_errors || 0,
                    writeErrors: device.write_errors || 0,
                    checksumErrors: device.checksum_errors || 0
                };
            });
        });
    },

    getZFSInfoForDrive(hostname, serial) {
        const key = `${hostname}:${serial}`;
        return this.zfsDriveMap[key] || null;
    },

    hasZFSAlerts() {
        const stats = this.getZFSStats();
        return stats.attentionPools > 0 || stats.totalErrors > 0;
    }
};