/**
 * Vigil Dashboard - Navigation Controller
 * Handles all view transitions and state management
 */

const Navigation = {
    /**
     * Show main dashboard with all servers
     */
    showDashboard() {
        console.log('[Nav] showDashboard called');
        
        // Reset state
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'drives';
        
        // Update DOM
        this._showView('dashboard-view');
        document.getElementById('page-title').textContent = 'Infrastructure Overview';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        // Update nav highlighting
        this._clearNavSelection();
        document.getElementById('nav-dashboard')?.classList.add('active');
        
        // Render
        Renderer.dashboard(State.data);
    },

    /**
     * Show individual server detail
     * @param {number} sortedIndex - Index in sorted server list
     */
    showServer(sortedIndex) {
        console.log('[Nav] showServer called with index:', sortedIndex);
        
        const sortedData = State.getSortedData();
        const server = sortedData[sortedIndex];
        if (!server) {
            console.error('[Nav] Server not found at index:', sortedIndex);
            return;
        }
        
        const actualIndex = State.data.findIndex(s => s.hostname === server.hostname);
        if (actualIndex === -1) {
            console.error('[Nav] Could not find actual index for:', server.hostname);
            return;
        }
        
        // Update state
        State.activeServerIndex = actualIndex;
        State.activeServerHostname = server.hostname;
        State.activeFilter = null;
        State.activeView = 'drives';
        
        // Update DOM
        this._showView('dashboard-view');
        document.getElementById('page-title').textContent = server.hostname;
        document.getElementById('crumb-server').textContent = server.hostname;
        document.getElementById('breadcrumbs').classList.remove('hidden');
        
        // Update nav highlighting
        this._clearNavSelection();
        this._highlightServer(sortedIndex);
        
        // Render
        Renderer.serverDetail(server, actualIndex);
    },

    /**
     * Show drive details
     * @param {number} serverIdx - Server index in State.data
     * @param {number} driveIdx - Drive index in server's drives array
     */
    showDriveDetails(serverIdx, driveIdx) {
        console.log('[Nav] showDriveDetails called:', serverIdx, driveIdx);
        
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) {
            console.error('[Nav] Drive not found');
            return;
        }
        
        // Update state
        State.activeServerIndex = serverIdx;
        State.activeServerHostname = server.hostname;
        State.activeView = 'drives';
        
        // Update DOM
        this._showView('details-view');
        document.getElementById('page-title').textContent = Utils.getDriveName(drive);
        document.getElementById('crumb-server').textContent = `${server.hostname} › ${Utils.getDriveName(drive)}`;
        document.getElementById('breadcrumbs').classList.remove('hidden');
        
        // Render
        Renderer.driveDetails(serverIdx, driveIdx);
    },

    /**
     * Show ZFS pools view
     */
    showZFS() {
        console.log('[Nav] showZFS called');
        
        // Update state
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'zfs';
        
        // Update DOM
        this._showView('dashboard-view');
        document.getElementById('page-title').textContent = 'ZFS Pools';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        // Update nav highlighting
        this._clearNavSelection();
        document.getElementById('nav-zfs')?.classList.add('active');
        
        // Render
        if (typeof ZFS !== 'undefined' && ZFS.render) {
            ZFS.render();
        }
    },

    /**
     * Show filtered drives view
     * @param {string} filter - 'all', 'healthy', or 'attention'
     */
    showFilter(filter) {
        console.log('[Nav] showFilter called:', filter);
        
        // Update state
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = filter;
        State.activeView = 'drives';
        
        // Update DOM
        this._showView('dashboard-view');
        document.getElementById('page-title').textContent = 
            filter === 'attention' ? 'Drives Needing Attention' :
            filter === 'healthy' ? 'Healthy Drives' : 'All Drives';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        // Update nav highlighting
        this._clearNavSelection();
        
        // Render
        const filterFn = filter === 'attention' 
            ? d => Utils.getHealthStatus(d) !== 'healthy'
            : filter === 'healthy'
            ? d => Utils.getHealthStatus(d) === 'healthy'
            : () => true;
        
        Renderer.filteredDrives(filterFn, filter);
    },

    /**
     * Show settings page
     */
    showSettings() {
        console.log('[Nav] showSettings called');
        
        document.getElementById('dropdown-menu')?.classList.remove('show');
        State.activeView = 'settings';
        
        const dashboardView = document.getElementById('dashboard-view');
        const detailsView = document.getElementById('details-view');
        
        dashboardView.classList.add('hidden');
        detailsView.classList.add('hidden');
        
        let settingsView = document.getElementById('settings-view');
        if (!settingsView) {
            settingsView = document.createElement('div');
            settingsView.id = 'settings-view';
            settingsView.className = 'view settings-view';
            dashboardView.parentNode.appendChild(settingsView);
        }
        
        settingsView.classList.remove('hidden');
        settingsView.innerHTML = Renderer.settingsPage();
        
        document.getElementById('page-title').textContent = 'Settings';
        document.getElementById('breadcrumbs').classList.add('hidden');
    },

    /**
     * Go back to appropriate context
     */
    goBack() {
        if (State.activeServerIndex !== null) {
            const sortedData = State.getSortedData();
            const sortedIdx = sortedData.findIndex(s => s.hostname === State.activeServerHostname);
            if (sortedIdx !== -1) {
                this.showServer(sortedIdx);
            } else {
                this.showDashboard();
            }
        } else {
            this.showDashboard();
        }
    },

    // ─── Private Helpers ─────────────────────────────────────────────────────

    _showView(viewId) {
        document.getElementById('dashboard-view').classList.toggle('hidden', viewId !== 'dashboard-view');
        document.getElementById('details-view').classList.toggle('hidden', viewId !== 'details-view');
        document.getElementById('settings-view')?.classList.add('hidden');
    },

    _clearNavSelection() {
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.zfs-pool-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    },

    _highlightServer(sortedIndex) {
        document.querySelectorAll('.server-nav-item').forEach((el, i) => {
            el.classList.toggle('active', i === sortedIndex);
        });
    }
};

// Legacy function aliases for backwards compatibility
function resetDashboard() { Navigation.showDashboard(); }
function goBackToContext() { Navigation.goBack(); }
function fetchData() { Data.fetch(); }
function showZFSPools() { Navigation.showZFS(); }
function toggleServerSort() { State.toggleSortOrder(); }