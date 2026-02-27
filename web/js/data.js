/**
 * Vigil Dashboard - Data Management
 */

const Data = {
    async fetch() {
        try {
            const [historyResponse, zfsResponse, wearoutResponse] = await Promise.all([
                API.getHistory(),
                API.getZFSPools().catch(() => null),
                API.get('/api/wearout/all').catch(() => null)
            ]);

            if (!historyResponse.ok) {
                throw new Error(`HTTP ${historyResponse.status}`);
            }

            State.data = await historyResponse.json() || [];
            State.resolveActiveServer();

            if (zfsResponse && zfsResponse.ok) {
                State.zfsPools = await zfsResponse.json() || [];
                State.buildZFSDriveMap();
            } else {
                State.zfsPools = [];
                State.zfsDriveMap = {};
            }

            if (wearoutResponse && wearoutResponse.ok) {
                const wData = await wearoutResponse.json();
                State.buildWearoutMap(wData?.drives);
            } else {
                State.wearoutMap = {};
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
        // Views that manage their own content — don't overwrite
        if (['agents', 'settings', 'addons', 'notifications'].includes(State.activeView)) {
            return;
        }

        const dashboardView = document.getElementById('dashboard-view');
        const detailsView = document.getElementById('details-view');

        // Don't update if details view is showing
        if (detailsView && !detailsView.classList.contains('hidden')) return;

        // Don't update if dashboard is hidden
        if (dashboardView && dashboardView.classList.contains('hidden')) return;

        // Update based on current view
        if (State.activeView === 'temperature') {
            if (typeof Temperature !== 'undefined' && Temperature.loadData) {
                Temperature.loadData();
            }
        } else if (State.activeView === 'zfs') {
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
            const isActive = State.activeServerHostname === server.hostname && State.activeView === 'drives';

            let statusClass = '';
            if (isOffline) statusClass = 'offline';
            else if (hasCritical) statusClass = 'critical';
            else if (hasWarning) statusClass = 'warning';

            return `
                <div class="server-nav-item ${isActive ? 'active' : ''} ${isOffline ? 'server-offline' : ''}"
                     onclick="navShowServer(${sortedIdx})"
                     title="${isOffline ? 'Not reporting — no data received in 5+ minutes' : 'Online — last update ' + timeSince}">
                    <span class="status-indicator ${statusClass}"></span>
                    <span class="server-name">${Utils.escapeHtml(server.hostname)}</span>
                    ${isOffline ? '<span class="offline-badge">NOT REPORTING</span>' : ''}
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
        if (poolCount) poolCount.textContent = stats.totalPools;

        const navZfs = document.getElementById('nav-zfs');
        if (navZfs) {
            navZfs.classList.toggle('has-warning', stats.degradedPools > 0);
            navZfs.classList.toggle('has-critical', stats.faultedPools > 0);
            navZfs.classList.toggle('active', State.activeView === 'zfs');
        }

        // ZFS pool list - same format as servers
        if (zfsNavList) {
            const poolsByHost = State.getPoolsByHost();
            const sortedHosts = Object.keys(poolsByHost).sort((a, b) => {
                const cmp = a.toLowerCase().localeCompare(b.toLowerCase());
                return State.serverSortOrder === 'asc' ? cmp : -cmp;
            });

            zfsNavList.innerHTML = sortedHosts.flatMap(hostname => {
                return poolsByHost[hostname]
                    .sort((a, b) => {
                        const nameA = (a.name || a.pool_name || '').toLowerCase();
                        const nameB = (b.name || b.pool_name || '').toLowerCase();
                        const cmp = nameA.localeCompare(nameB);
                        return State.serverSortOrder === 'asc' ? cmp : -cmp;
                    })
                    .map(pool => {
                        const poolName = pool.name || pool.pool_name || 'Unknown';
                        const state = (pool.status || pool.health || 'UNKNOWN').toUpperCase();
                        let statusClass = '';
                        if (state === 'DEGRADED') statusClass = 'warning';
                        else if (state === 'FAULTED' || state === 'UNAVAIL') statusClass = 'critical';

                        return `
                            <div class="server-nav-item ${statusClass}"
                                 onclick="navShowZFSPool('${Utils.escapeJSString(hostname)}', '${Utils.escapeJSString(poolName)}')"
                                 title="${Utils.escapeHtml(hostname)} - ${Utils.escapeHtml(poolName)}">
                                <span class="status-indicator ${statusClass}"></span>
                                <span class="server-name">${Utils.escapeHtml(poolName)}</span>
                                <span class="pool-host-label">${Utils.escapeHtml(hostname)}</span>
                            </div>
                        `;
                    });
            }).join('');
        }
    },

    updateStats() {
        const stats = State.getStats();
        const zfsStats = State.getZFSStats();

        const totalDrives = document.getElementById('total-drives');
        const healthyCount = document.getElementById('healthy-count');
        const warningCount = document.getElementById('warning-count');

        if (totalDrives) totalDrives.textContent = stats.totalDrives;
        if (healthyCount) healthyCount.textContent = stats.healthyDrives;

        if (warningCount) {
            const total = stats.attentionDrives + zfsStats.attentionPools;
            warningCount.textContent = total;
            warningCount.classList.toggle('critical', zfsStats.faultedPools > 0);
            warningCount.classList.toggle('warning', total > 0 && zfsStats.faultedPools === 0);
        }
    },

    setOnlineStatus(online) {
        const indicator = document.getElementById('status-indicator');
        if (indicator) {
            indicator.classList.toggle('online', online);
            indicator.classList.toggle('offline', !online);
            indicator.title = online ? 'Connected' : 'Disconnected';
        }
    },

    updateLastRefresh() {
        const el = document.getElementById('last-update-time');
        if (el) {
            el.textContent = new Date().toLocaleTimeString([], {
                hour: '2-digit', minute: '2-digit', second: '2-digit'
            });
        }
    },

    async fetchVersion() {
        try {
            const resp = await API.getVersion();
            if (resp.ok) {
                const data = await resp.json();
                const el = document.getElementById('app-version');
                if (el && data.version) {
                    el.textContent = data.version.startsWith('v') ? data.version : `v${data.version}`;
                }
            }
        } catch (e) {}
    },

    async fetchZFSPoolDetail(hostname, poolName) {
        try {
            const response = await API.getZFSPool(hostname, poolName);
            if (response.ok) return await response.json();
        } catch (e) {}
        return null;
    }
};
