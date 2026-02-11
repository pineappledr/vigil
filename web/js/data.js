/**
 * Vigil Dashboard - Data Management
 */

const Data = {
    async fetch() {
        try {
            const [historyResponse, zfsResponse] = await Promise.all([
                API.getHistory(),
                API.getZFSPools().catch(err => {
                    console.debug('ZFS API unavailable:', err?.message || 'endpoint not found');
                    return null;
                })
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
            
            // Update UI based on current state
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

    /**
     * Update the current view without changing navigation state
     * This is called on data refresh to update content in place
     */
    updateCurrentView() {
        const dashboardView = document.getElementById('dashboard-view');
        const detailsView = document.getElementById('details-view');
        
        // If we're in details view, update drive details
        if (!detailsView.classList.contains('hidden')) {
            if (State.activeServerIndex !== null) {
                const server = State.data[State.activeServerIndex];
                if (server) {
                    // Re-render if viewing drive details
                    // The driveIdx would need to be tracked separately
                }
            }
            return;
        }
        
        // If dashboard is hidden, don't update
        if (dashboardView.classList.contains('hidden')) return;
        
        // Update based on current view state
        if (State.activeView === 'zfs') {
            if (typeof ZFS !== 'undefined' && ZFS.render) {
                ZFS.render();
            }
        } else if (State.activeServerIndex !== null && State.data[State.activeServerIndex]) {
            Renderer.serverDetail(State.data[State.activeServerIndex], State.activeServerIndex);
        } else if (State.activeFilter === 'attention') {
            Renderer.filteredDrives(d => Utils.getHealthStatus(d) !== 'healthy', 'attention');
        } else if (State.activeFilter === 'healthy') {
            Renderer.filteredDrives(d => Utils.getHealthStatus(d) === 'healthy', 'healthy');
        } else if (State.activeFilter === 'all') {
            Renderer.filteredDrives(() => true, 'all');
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
            
            const statusTitle = isOffline 
                ? `Offline - last seen ${timeSince}`
                : hasCritical ? 'Critical issues detected'
                : hasWarning ? 'Warnings detected'
                : `Online - updated ${timeSince}`;
            
            return `
                <div class="server-nav-item ${isActive ? 'active' : ''} ${isOffline ? 'server-offline' : ''}" 
                     onclick="Navigation.showServer(${sortedIdx})"
                     title="${statusTitle}">
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
            sortBtn.title = isAsc ? 'Sorted A to Z (click to reverse)' : 'Sorted Z to A (click to reverse)';
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
        
        // Update count badge
        const poolCount = document.getElementById('zfs-pool-count');
        if (poolCount) {
            poolCount.textContent = stats.totalPools;
        }

        // Update nav item status
        const zfsNavItem = document.getElementById('nav-zfs');
        if (zfsNavItem) {
            zfsNavItem.classList.toggle('has-warning', stats.degradedPools > 0);
            zfsNavItem.classList.toggle('has-critical', stats.faultedPools > 0);
            zfsNavItem.classList.toggle('active', State.activeView === 'zfs');
        }

        // Build pool list grouped by hostname (sorted)
        if (zfsNavList) {
            const poolsByHost = State.getPoolsByHost();
            const sortedHosts = Object.keys(poolsByHost).sort((a, b) => {
                if (State.serverSortOrder === 'asc') {
                    return a.toLowerCase().localeCompare(b.toLowerCase());
                } else {
                    return b.toLowerCase().localeCompare(a.toLowerCase());
                }
            });

            zfsNavList.innerHTML = sortedHosts.map(hostname => {
                const pools = poolsByHost[hostname];
                return pools.map(pool => {
                    const poolName = pool.name || pool.pool_name || 'Unknown';
                    const state = (pool.status || pool.health || pool.state || 'UNKNOWN').toUpperCase();
                    const stateClass = state === 'ONLINE' ? '' : state === 'DEGRADED' ? 'warning' : 'critical';
                    
                    return `
                        <div class="zfs-pool-nav-item ${stateClass}" 
                             onclick="ZFS.showPoolDetail('${hostname}', '${poolName}')"
                             title="${hostname} - ${poolName} (${state})">
                            <span class="status-indicator ${stateClass}"></span>
                            <span class="pool-name">${poolName}</span>
                            <span class="pool-host">${hostname}</span>
                        </div>
                    `;
                }).join('');
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
        } catch (e) {
            console.warn('Could not fetch version:', e);
        }
    },

    async fetchZFSPoolDetail(hostname, poolName) {
        try {
            const response = await API.getZFSPool(hostname, poolName);
            if (response.ok) {
                return await response.json();
            }
        } catch (e) {
            console.error('Failed to fetch ZFS pool detail:', e);
        }
        return null;
    }
};