/**
 * Vigil Dashboard - Data Management
 */

const Data = {
    async fetch() {
        try {
            const [historyResponse, zfsResponse] = await Promise.all([
                API.getHistory(),
                API.getZFSPools().catch(() => null)
            ]);
            
            if (!historyResponse.ok) {
                throw new Error(`HTTP ${historyResponse.status}`);
            }
            
            State.data = await historyResponse.json() || [];
            State.resolveActiveServer();
            
            if (zfsResponse && zfsResponse.ok) {
                const zfsData = await zfsResponse.json();
                State.zfsPools = Array.isArray(zfsData) ? zfsData : [];
                State.buildZFSDriveMap();
            } else {
                State.zfsPools = [];
                State.zfsDriveMap = {};
            }
            
            this.updateCurrentView();
            this.updateSidebar();
            this.updateStats();
            this.setOnlineStatus(true);
            this.updateLastRefresh();
            
        } catch (error) {
            console.error('Fetch error:', error);
            this.setOnlineStatus(false);
        }
    },

    updateCurrentView() {
        const dashboardView = document.getElementById('dashboard-view');
        const detailsView = document.getElementById('details-view');
        
        if (!detailsView.classList.contains('hidden')) return;
        if (dashboardView.classList.contains('hidden')) return;
        
        if (State.activeView === 'zfs') {
            if (typeof ZFS !== 'undefined' && ZFS.render) {
                ZFS.render();
            }
        } else if (State.activeServerIndex !== null && State.data[State.activeServerIndex]) {
            Renderer.serverDetail(State.data[State.activeServerIndex], State.activeServerIndex);
        } else if (State.activeFilter) {
            const filterFn = State.activeFilter === 'attention' 
                ? d => Utils.getHealthStatus(d) !== 'healthy'
                : State.activeFilter === 'healthy'
                ? d => Utils.getHealthStatus(d) === 'healthy'
                : () => true;
            Renderer.filteredDrives(filterFn, State.activeFilter);
        } else {
            Renderer.dashboard(State.data);
        }
    },

    updateSidebar() {
        this.updateServerList();
        this.updateZFSSidebar();
        this.updateSortIndicator();
    },

    updateServerList() {
        const serverNav = document.getElementById('server-nav-list');
        const serverCount = document.getElementById('server-count');
        
        if (!serverNav) return;
        
        serverCount.textContent = State.data.length;
        
        const sortedData = State.getSortedData();
        
        serverNav.innerHTML = sortedData.map((server, sortedIdx) => {
            const drives = server.details?.drives || [];
            const hasWarning = drives.some(d => Utils.getHealthStatus(d) === 'warning');
            const hasCritical = drives.some(d => Utils.getHealthStatus(d) === 'critical');
            const isOffline = State.isServerOffline(server);
            const timeSince = State.getTimeSinceUpdate(server);
            const isActive = State.activeServerHostname === server.hostname && State.activeView !== 'zfs';
            
            let statusClass = '';
            if (isOffline) statusClass = 'offline';
            else if (hasCritical) statusClass = 'critical';
            else if (hasWarning) statusClass = 'warning';
            
            return `
                <div class="server-nav-item ${isActive ? 'active' : ''} ${isOffline ? 'server-offline' : ''}" 
                     onclick="navShowServer(${sortedIdx})"
                     title="${isOffline ? 'Offline - last seen ' + timeSince : 'Online'}">
                    <span class="status-indicator ${statusClass}"></span>
                    <span class="server-name">${server.hostname}</span>
                    ${isOffline ? `<span class="offline-badge">OFFLINE</span>` : ''}
                </div>
            `;
        }).join('');
    },
    
    updateSortIndicator() {
        const sortBtn = document.getElementById('server-sort-btn');
        if (sortBtn) {
            const isAsc = State.serverSortOrder === 'asc';
            const iconEl = sortBtn.querySelector('.sort-icon');
            const textEl = sortBtn.querySelector('.sort-text');
            if (iconEl) iconEl.textContent = isAsc ? '↑' : '↓';
            if (textEl) textEl.textContent = isAsc ? 'A-Z' : 'Z-A';
        }
    },

    updateZFSSidebar() {
        const zfsNav = document.getElementById('zfs-nav-section');
        const zfsNavList = document.getElementById('zfs-nav-list');
        if (!zfsNav) return;

        const stats = State.getZFSStats();
        
        if (stats.totalPools === 0) {
            zfsNav.classList.add('hidden');
            return;
        }

        zfsNav.classList.remove('hidden');
        
        const poolCount = document.getElementById('zfs-pool-count');
        if (poolCount) {
            poolCount.textContent = stats.totalPools;
        }

        const navZfs = document.getElementById('nav-zfs');
        if (navZfs) {
            navZfs.classList.toggle('has-warning', stats.degradedPools > 0);
            navZfs.classList.toggle('has-critical', stats.faultedPools > 0);
            navZfs.classList.toggle('active', State.activeView === 'zfs');
        }

        // Build ZFS pool list - same format as servers
        if (zfsNavList) {
            const poolsByHost = State.getPoolsByHost();
            
            // Sort hosts same as servers
            const sortedHosts = Object.keys(poolsByHost).sort((a, b) => {
                const cmp = a.toLowerCase().localeCompare(b.toLowerCase());
                return State.serverSortOrder === 'asc' ? cmp : -cmp;
            });

            zfsNavList.innerHTML = sortedHosts.flatMap(hostname => {
                // Sort pools within host
                const pools = poolsByHost[hostname].sort((a, b) => {
                    const nameA = (a.name || a.pool_name || '').toLowerCase();
                    const nameB = (b.name || b.pool_name || '').toLowerCase();
                    const cmp = nameA.localeCompare(nameB);
                    return State.serverSortOrder === 'asc' ? cmp : -cmp;
                });
                
                return pools.map(pool => {
                    const poolName = pool.name || pool.pool_name || 'Unknown';
                    const state = (pool.status || pool.health || pool.state || 'UNKNOWN').toUpperCase();
                    let statusClass = '';
                    if (state === 'DEGRADED') statusClass = 'warning';
                    else if (state === 'FAULTED' || state === 'UNAVAIL') statusClass = 'critical';
                    
                    return `
                        <div class="server-nav-item ${statusClass}" 
                             onclick="navShowZFSPool('${hostname}', '${poolName}')"
                             title="${hostname} - ${poolName} (${state})">
                            <span class="status-indicator ${statusClass}"></span>
                            <span class="server-name">${poolName}</span>
                            <span class="pool-host-label">${hostname}</span>
                        </div>
                    `;
                });
            }).join('');
        }
    },

    updateStats() {
        const stats = State.getStats();
        const zfsStats = State.getZFSStats();
        
        document.getElementById('total-drives').textContent = stats.totalDrives;
        document.getElementById('healthy-count').textContent = stats.healthyDrives;
        
        const totalWarnings = stats.attentionDrives + zfsStats.attentionPools;
        const warningEl = document.getElementById('warning-count');
        warningEl.textContent = totalWarnings;
        
        if (zfsStats.faultedPools > 0 || stats.offlineServers > 0) {
            warningEl.classList.remove('warning');
            warningEl.classList.add('critical');
        } else if (totalWarnings > 0) {
            warningEl.classList.remove('critical');
            warningEl.classList.add('warning');
        } else {
            warningEl.classList.remove('warning', 'critical');
        }
    },

    setOnlineStatus(online) {
        const indicator = document.getElementById('status-indicator');
        indicator.classList.toggle('online', online);
        indicator.classList.toggle('offline', !online);
        indicator.title = online ? 'Connected' : 'Connection Lost';
    },

    updateLastRefresh() {
        document.getElementById('last-update-time').textContent = 
            new Date().toLocaleTimeString([], { 
                hour: '2-digit', 
                minute: '2-digit', 
                second: '2-digit' 
            });
    },

    async fetchVersion() {
        try {
            const resp = await API.getVersion();
            if (resp.ok) {
                const data = await resp.json();
                const versionEl = document.getElementById('app-version');
                if (versionEl && data.version) {
                    versionEl.textContent = data.version.startsWith('v') ? data.version : `v${data.version}`;
                }
            }
        } catch (e) {}
    },

    async fetchZFSPoolDetail(hostname, poolName) {
        try {
            const response = await API.getZFSPool(hostname, poolName);
            if (response.ok) {
                return await response.json();
            }
        } catch (e) {}
        return null;
    }
};