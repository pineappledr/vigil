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
        console.log('[State] init complete, activeView:', this.activeView);
    },

    toggleSortOrder() {
        this.serverSortOrder = this.serverSortOrder === 'asc' ? 'desc' : 'asc';
        localStorage.setItem('vigil_server_sort', this.serverSortOrder);
        
        if (typeof Data !== 'undefined') {
            Data.updateSidebar();
            if (this.activeView === 'drives' && !this.activeServerIndex && !this.activeFilter) {
                Renderer.dashboard(this.data);
            }
        }
    },

    getSortedData() {
        if (!this.data || !this.data.length) return [];
        return [...this.data].sort((a, b) => {
            const cmp = (a.hostname || '').toLowerCase().localeCompare((b.hostname || '').toLowerCase());
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
        const sorted = this.getSortedData();
        const server = sorted[index];
        const actual = this.data.findIndex(s => s.hostname === server?.hostname);
        this.activeServerIndex = actual >= 0 ? actual : index;
        this.activeServerHostname = server?.hostname || null;
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
            const idx = this.data.findIndex(s => s.hostname === this.activeServerHostname);
            this.activeServerIndex = idx >= 0 ? idx : null;
            if (idx < 0) this.activeServerHostname = null;
        }
    },

    isServerOffline(server) {
        const ts = server?.last_seen || server?.timestamp;
        if (!ts) return false;
        return (Date.now() - new Date(ts)) / 60000 > this.OFFLINE_THRESHOLD_MINUTES;
    },

    getTimeSinceUpdate(server) {
        const ts = server?.last_seen || server?.timestamp;
        if (!ts) return 'Unknown';
        const mins = Math.floor((Date.now() - new Date(ts)) / 60000);
        if (mins < 1) return 'Just now';
        if (mins < 60) return `${mins}m ago`;
        const hrs = Math.floor(mins / 60);
        if (hrs < 24) return `${hrs}h ago`;
        return `${Math.floor(hrs / 24)}d ago`;
    },

    getStats() {
        let totalDrives = 0, healthyDrives = 0, attentionDrives = 0, offlineServers = 0;
        this.data.forEach(s => {
            const drives = s.details?.drives || [];
            totalDrives += drives.length;
            if (this.isServerOffline(s)) offlineServers++;
            drives.forEach(d => {
                Utils.getHealthStatus(d) === 'healthy' ? healthyDrives++ : attentionDrives++;
            });
        });
        return { totalServers: this.data.length, totalDrives, healthyDrives, attentionDrives, offlineServers };
    },

    getZFSStats() {
        const pools = this.zfsPools || [];
        let healthyPools = 0, degradedPools = 0, faultedPools = 0, totalErrors = 0;
        pools.forEach(p => {
            if (!p) return;
            const st = (p.status || p.health || '').toUpperCase();
            if (st === 'ONLINE') healthyPools++;
            else if (st === 'DEGRADED') degradedPools++;
            else if (st === 'FAULTED' || st === 'UNAVAIL') faultedPools++;
            totalErrors += (p.read_errors || 0) + (p.write_errors || 0) + (p.checksum_errors || 0);
        });
        return { totalPools: pools.length, healthyPools, degradedPools, faultedPools, attentionPools: degradedPools + faultedPools, totalErrors };
    },

    getPoolsByHost() {
        const g = {};
        (this.zfsPools || []).forEach(p => {
            if (!p) return;
            const h = p.hostname || 'unknown';
            if (!g[h]) g[h] = [];
            g[h].push(p);
        });
        return g;
    },

    buildZFSDriveMap() {
        this.zfsDriveMap = {};
        (this.zfsPools || []).forEach(p => {
            if (!p) return;
            const hostname = p.hostname || '';
            const poolName = p.name || p.pool_name || '';
            const poolState = (p.status || p.state || p.health || 'UNKNOWN').toUpperCase();
            (p.devices || []).forEach(d => {
                const serial = d.serial_number || d.serial;
                if (serial) {
                    this.zfsDriveMap[`${hostname}:${serial}`] = {
                        poolName, poolState,
                        vdev: d.vdev_parent || d.vdev || '',
                        deviceName: d.device_name || d.name || '',
                        readErrors: d.read_errors || 0,
                        writeErrors: d.write_errors || 0,
                        checksumErrors: d.checksum_errors || 0
                    };
                }
            });
        });
    },

    getZFSInfoForDrive(hostname, serial) {
        return this.zfsDriveMap[`${hostname}:${serial}`] || null;
    },

    hasZFSAlerts() {
        const s = this.getZFSStats();
        return s.attentionPools > 0 || s.totalErrors > 0;
    }
};