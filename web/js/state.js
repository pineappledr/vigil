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

    // ZFS State
    zfsPools: [],
    zfsDriveMap: {},        // Map of serial -> pool info for badge display
    activeView: 'drives',   // 'drives' | 'zfs' | 'settings'

    API_URL: '/api/history',
    REFRESH_INTERVAL: 5000,

    reset() {
        this.activeServerIndex = null;
        this.activeFilter = null;
        this.activeView = 'drives';
    },

    setFilter(filter) {
        this.activeServerIndex = null;
        this.activeFilter = filter;
        this.activeView = 'drives';
    },

    setServer(index) {
        this.activeServerIndex = index;
        this.activeFilter = null;
        this.activeView = 'drives';
    },

    setView(view) {
        this.activeView = view;
        if (view !== 'drives') {
            this.activeServerIndex = null;
            this.activeFilter = null;
        }
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
    },

    // ─── ZFS State Methods ───────────────────────────────────────────────────

    /**
     * Get ZFS statistics for display
     * @returns {Object} ZFS stats
     */
    getZFSStats() {
        const pools = this.zfsPools || [];
        
        let totalPools = pools.length;
        let healthyPools = 0;
        let degradedPools = 0;
        let faultedPools = 0;
        let totalErrors = 0;

        pools.forEach(pool => {
            const state = (pool.state || pool.health || '').toUpperCase();
            
            if (state === 'ONLINE') {
                healthyPools++;
            } else if (state === 'DEGRADED') {
                degradedPools++;
            } else if (state === 'FAULTED' || state === 'UNAVAIL') {
                faultedPools++;
            }

            // Count device errors
            if (pool.devices) {
                pool.devices.forEach(device => {
                    totalErrors += (device.read_errors || 0) + 
                                   (device.write_errors || 0) + 
                                   (device.checksum_errors || 0);
                });
            }
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

    /**
     * Get pools grouped by hostname
     * @returns {Object} Pools grouped by hostname
     */
    getPoolsByHost() {
        const grouped = {};
        
        (this.zfsPools || []).forEach(pool => {
            const host = pool.hostname || 'unknown';
            if (!grouped[host]) {
                grouped[host] = [];
            }
            grouped[host].push(pool);
        });

        return grouped;
    },

    /**
     * Build drive-to-pool mapping for badge display
     * Called after ZFS data is fetched
     */
    buildZFSDriveMap() {
        this.zfsDriveMap = {};
        
        (this.zfsPools || []).forEach(pool => {
            const hostname = pool.hostname || '';
            const poolState = (pool.state || pool.health || 'UNKNOWN').toUpperCase();
            
            (pool.devices || []).forEach(device => {
                if (device.serial) {
                    const key = `${hostname}:${device.serial}`;
                    this.zfsDriveMap[key] = {
                        poolName: pool.name,
                        poolState: poolState,
                        vdev: device.vdev || '',
                        deviceName: device.device_name || device.name || '',
                        readErrors: device.read_errors || 0,
                        writeErrors: device.write_errors || 0,
                        checksumErrors: device.checksum_errors || 0
                    };
                }
            });
        });
    },

    /**
     * Get ZFS pool info for a specific drive
     * @param {string} hostname
     * @param {string} serial
     * @returns {Object|null} Pool info or null
     */
    getZFSInfoForDrive(hostname, serial) {
        const key = `${hostname}:${serial}`;
        return this.zfsDriveMap[key] || null;
    },

    /**
     * Check if any ZFS pools need attention
     * @returns {boolean}
     */
    hasZFSAlerts() {
        const stats = this.getZFSStats();
        return stats.attentionPools > 0 || stats.totalErrors > 0;
    }
};