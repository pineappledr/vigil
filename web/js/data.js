/**
 * Vigil Dashboard - Data Management
 */

const Data = {
    async fetch() {
        try {
            // Fetch drives and ZFS pools in parallel
            const [historyResponse, zfsResponse] = await Promise.all([
                API.getHistory(),
                API.getZFSPools().catch(() => null)  // Don't fail if ZFS unavailable
            ]);
            
            if (!historyResponse.ok) {
                throw new Error(`HTTP ${historyResponse.status}`);
            }
            
            State.data = await historyResponse.json() || [];
            
            // Process ZFS data if available
            if (zfsResponse && zfsResponse.ok) {
                State.zfsPools = await zfsResponse.json() || [];
                State.buildZFSDriveMap();
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
        
        serverNav.innerHTML = State.data.map((server, idx) => {
            const drives = server.details?.drives || [];
            const hasWarning = drives.some(d => Utils.getHealthStatus(d) === 'warning');
            const hasCritical = drives.some(d => Utils.getHealthStatus(d) === 'critical');
            
            let statusClass = '';
            if (hasCritical) statusClass = 'critical';
            else if (hasWarning) statusClass = 'warning';
            
            return `
                <div class="server-nav-item ${State.activeServerIndex === idx ? 'active' : ''}" 
                     onclick="Navigation.showServer(${idx})">
                    <span class="status-indicator ${statusClass}"></span>
                    ${server.hostname}
                </div>
            `;
        }).join('');

        // Update ZFS sidebar if pools exist
        this.updateZFSSidebar();
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
        document.getElementById('total-drives').textContent = stats.totalDrives;
        document.getElementById('healthy-count').textContent = stats.healthyDrives;
        document.getElementById('warning-count').textContent = stats.attentionDrives;

        // Update ZFS stats if elements exist
        const zfsStats = State.getZFSStats();
        const zfsPoolsEl = document.getElementById('zfs-pools-count');
        const zfsAttentionEl = document.getElementById('zfs-attention-count');
        
        if (zfsPoolsEl) {
            zfsPoolsEl.textContent = zfsStats.totalPools;
        }
        if (zfsAttentionEl) {
            zfsAttentionEl.textContent = zfsStats.attentionPools;
            zfsAttentionEl.closest('.summary-card')?.classList.toggle('hidden', zfsStats.totalPools === 0);
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

    /**
     * Fetch detailed ZFS pool data (for modal)
     * @param {string} hostname
     * @param {string} poolName
     * @returns {Promise<Object|null>}
     */
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