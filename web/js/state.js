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

    zfsPools: [],
    zfsDriveMap: {},
    activeView: 'drives',
    serverSortOrder: 'asc',

    API_URL: '/api/history',
    REFRESH_INTERVAL: 5000,
    OFFLINE_THRESHOLD_MINUTES: 5,

    init() {
        const savedSort = localStorage.getItem('vigil_server_sort');
        if (savedSort === 'asc' || savedSort === 'desc') {
            this.serverSortOrder = savedSort;
        }
        console.log('[State] Initialized');
    },

    toggleSortOrder() {
        this.serverSortOrder = this.serverSortOrder === 'asc' ? 'desc' : 'asc';
        localStorage.setItem('vigil_server_sort', this.serverSortOrder);
        console.log('[State] Sort:', this.serverSortOrder);
        
        if (typeof Data !== 'undefined') {
            Data.updateSidebar();
            
            // Re-render dashboard if showing it
            if (this.activeView === 'drives' && this.activeServerIndex === null && !this.activeFilter) {
                Renderer.dashboard(this.data);
            }
        }
    },

    getSortedData() {
        if (!this.data || this.data.length === 0) return [];
        
        return [...this.data].sort((a, b) => {
            const nameA = (a.hostname || '').toLowerCase();
            const nameB = (b.hostname || '').toLowerCase();
            const cmp = nameA.localeCompare(nameB);
            return this.serverSortOrder === 'asc' ? cmp : -cmp;
        });
    },

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
        const sortedData = this.getSortedData();
        const server = sortedData[index];
        const actualIndex = this.data.findIndex(s => s.hostname === server?.hostname);
        
        this.activeServerIndex = actualIndex >= 0 ? actualIndex : index;
        this.activeServerHostname = server?.hostname || this.data[index]?.hostname || null;
        this.activeFilter = null;
        this.activeView = 'drives';
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

    isServerOffline(server) {
        if (!server || !server.last_seen) return false;
        const lastSeen = new Date(server.last_seen);
        const now = new Date();
        return (now - lastSeen) / (1000 * 60) > this.OFFLINE_THRESHOLD_MINUTES;
    },

    getTimeSinceUpdate(server) {
        if (!server || !server.last_seen) return 'Unknown';
        const lastSeen = new Date(server.last_seen);
        const now = new Date();
        const diffMs = now - lastSeen;
        const diffMinutes = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMinutes / 60);
        const diffDays = Math.floor(diffHours / 24);
        
        if (diffDays > 0) return `${diffDays}d ago`;
        if (diffHours > 0) return `${diffHours}h ago`;
        if (diffMinutes > 0) return `${diffMinutes}m ago`;
        return 'Just now';
    },

    getStats() {
        let totalDrives = 0, healthyDrives = 0, attentionDrives = 0, offlineServers = 0;

        this.data.forEach(server => {
            const drives = server.details?.drives || [];
            totalDrives += drives.length;
            if (this.isServerOffline(server)) offlineServers++;
            
            drives.forEach(drive => {
                if (Utils.getHealthStatus(drive) === 'healthy') {
                    healthyDrives++;
                } else {
                    attentionDrives++;
                }
            });
        });

        return { totalServers: this.data.length, totalDrives, healthyDrives, attentionDrives, offlineServers };
    },

    getZFSStats() {
        const pools = Array.isArray(this.zfsPools) ? this.zfsPools : [];
        let healthyPools = 0, degradedPools = 0, faultedPools = 0, totalErrors = 0;

        pools.forEach(pool => {
            if (!pool) return;
            const state = (pool.status || pool.health || pool.state || '').toUpperCase();
            
            if (state === 'ONLINE') healthyPools++;
            else if (state === 'DEGRADED') degradedPools++;
            else if (state === 'FAULTED' || state === 'UNAVAIL') faultedPools++;

            totalErrors += (pool.read_errors || 0) + (pool.write_errors || 0) + (pool.checksum_errors || 0);
        });

        return { 
            totalPools: pools.length, 
            healthyPools, 
            degradedPools, 
            faultedPools,
            attentionPools: degradedPools + faultedPools,
            totalErrors
        };
    },

    getPoolsByHost() {
        const grouped = {};
        (this.zfsPools || []).forEach(pool => {
            if (!pool) return;
            const host = pool.hostname || 'unknown';
            if (!grouped[host]) grouped[host] = [];
            grouped[host].push(pool);
        });
        return grouped;
    },

    buildZFSDriveMap() {
        this.zfsDriveMap = {};
        (this.zfsPools || []).forEach(pool => {
            if (!pool) return;
            const hostname = pool.hostname || '';
            const poolName = pool.name || pool.pool_name || 'unknown';
            const poolState = (pool.status || pool.state || pool.health || 'UNKNOWN').toUpperCase();
            
            (pool.devices || []).forEach(device => {
                if (!device) return;
                const serial = device.serial_number || device.serial;
                if (!serial) return;
                
                this.zfsDriveMap[`${hostname}:${serial}`] = {
                    poolName, poolState,
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
        return this.zfsDriveMap[`${hostname}:${serial}`] || null;
    },

    hasZFSAlerts() {
        const stats = this.getZFSStats();
        return stats.attentionPools > 0 || stats.totalErrors > 0;
    }
};