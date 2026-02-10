/**
 * Vigil Dashboard - Data Management
 */

const Data = {
    async fetch() {
        try {
            // Fetch drives and ZFS pools in parallel
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
            
            // Resolve active server by hostname after data refresh
            State.resolveActiveServer();
            
            // Process ZFS data if available
            if (zfsResponse && zfsResponse.ok) {
                const zfsData = await zfsResponse.json();
                State.zfsPools = Array.isArray(zfsData) ? zfsData : [];
                State.buildZFSDriveMap();
                
                if (State.zfsPools.length > 0) {
                    console.log(`ZFS: ${State.zfsPools.length} pool(s), ${Object.keys(State.zfsDriveMap).length} drive mapping(s)`);
                }
            } else {
                State.zfsPools = [];
                State.zfsDriveMap = {};
            }
            
            this.updateViews();
            this.updateSidebar();
            this.updateStats();
            this.setOnlineStatus(true);
            this.updateLastRefresh();
            
        } catch (error) {
            console.error('Fetch error:', error);
            this.setOnlineStatus(false);
        }
    },

    updateViews() {
        const dashboardView = document.getElementById('dashboard-view');
        if (dashboardView.classList.contains('hidden')) return;

        // Handle ZFS view
        if (State.activeView === 'zfs') {
            if (typeof ZFS !== 'undefined' && ZFS.render) {
                ZFS.render();
            }
            return;
        }

        // Handle drive views
        if (State.activeServerIndex !== null && State.data[State.activeServerIndex]) {
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
        const serverNav = document.getElementById('server-nav-list');
        const serverCount = document.getElementById('server-count');
        
        serverCount.textContent = State.data.length;
        
        // Use sorted data for display
        const sortedData = State.getSortedData();
        
        serverNav.innerHTML = sortedData.map((server, idx) => {
            const drives = server.details?.drives || [];
            const hasWarning = drives.some(d => Utils.getHealthStatus(d) === 'warning');
            const hasCritical = drives.some(d => Utils.getHealthStatus(d) === 'critical');
            const isOffline = State.isServerOffline(server);
            const timeSince = State.getTimeSinceUpdate(server);
            
            // Check if this is the active server by hostname
            const isActive = State.activeServerHostname === server.hostname;
            
            let statusClass = '';
            let statusTitle = '';
            
            if (isOffline) {
                statusClass = 'offline';
                statusTitle = `Offline - last seen ${timeSince}`;
            } else if (hasCritical) {
                statusClass = 'critical';
                statusTitle = 'Critical issues detected';
            } else if (hasWarning) {
                statusClass = 'warning';
                statusTitle = 'Warnings detected';
            } else {
                statusTitle = `Online - updated ${timeSince}`;
            }
            
            return `
                <div class="server-nav-item ${isActive ? 'active' : ''} ${isOffline ? 'server-offline' : ''}" 
                     onclick="Navigation.showServer(${idx})"
                     title="${statusTitle}">
                    <span class="status-indicator ${statusClass}"></span>
                    <span class="server-name">${server.hostname}</span>
                    ${isOffline ? `<span class="offline-badge" title="Last seen ${timeSince}">OFFLINE</span>` : ''}
                </div>
            `;
        }).join('');

        // Update ZFS sidebar if pools exist
        this.updateZFSSidebar();
        
        // Update sort indicator
        this.updateSortIndicator();
    },
    
    updateSortIndicator() {
        const sortBtn = document.getElementById('server-sort-btn');
        if (sortBtn) {
            const isAsc = State.serverSortOrder === 'asc';
            sortBtn.innerHTML = isAsc ? '↑ A-Z' : '↓ Z-A';
            sortBtn.title = isAsc ? 'Sorted A to Z (click to reverse)' : 'Sorted Z to A (click to reverse)';
        }
    },

    updateZFSSidebar() {
        const zfsNav = document.getElementById('zfs-nav-section');
        if (!zfsNav) return;

        const stats = State.getZFSStats();
        
        if (stats.totalPools === 0) {
            zfsNav.classList.add('hidden');
            return;
        }

        zfsNav.classList.remove('hidden');
        
        // Update pool count
        const poolCount = document.getElementById('zfs-pool-count');
        if (poolCount) {
            poolCount.textContent = stats.totalPools;
        }

        // Update attention indicator
        const zfsNavItem = zfsNav.querySelector('.nav-item');
        if (zfsNavItem) {
            zfsNavItem.classList.toggle('has-warning', stats.degradedPools > 0);
            zfsNavItem.classList.toggle('has-critical', stats.faultedPools > 0);
        }
    },

    updateStats() {
        const stats = State.getStats();
        const zfsStats = State.getZFSStats();
        
        document.getElementById('total-drives').textContent = stats.totalDrives;
        document.getElementById('healthy-count').textContent = stats.healthyDrives;
        
        // Combine drive warnings with ZFS pool issues
        const totalWarnings = stats.attentionDrives + zfsStats.attentionPools;
        const warningEl = document.getElementById('warning-count');
        warningEl.textContent = totalWarnings;
        
        // Change color to critical if ZFS pools are faulted or servers offline
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