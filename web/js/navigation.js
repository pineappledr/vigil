/**
 * Vigil Dashboard - Navigation Controller
 * DEBUG VERSION
 */

const Navigation = {
    showDashboard() {
        console.log('[Nav] === showDashboard START ===');
        console.log('[Nav] Before - activeView:', State.activeView);
        
        // CRITICAL: Reset state FIRST
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'drives';
        
        console.log('[Nav] After reset - activeView:', State.activeView);
        
        // Get elements
        const dashboardView = document.getElementById('dashboard-view');
        const detailsView = document.getElementById('details-view');
        const settingsView = document.getElementById('settings-view');
        
        console.log('[Nav] dashboardView hidden?', dashboardView?.classList.contains('hidden'));
        
        // Show/hide views
        if (dashboardView) dashboardView.classList.remove('hidden');
        if (detailsView) detailsView.classList.add('hidden');
        if (settingsView) settingsView.classList.add('hidden');
        
        // Update header
        const pageTitle = document.getElementById('page-title');
        const breadcrumbs = document.getElementById('breadcrumbs');
        if (pageTitle) pageTitle.textContent = 'Infrastructure Overview';
        if (breadcrumbs) breadcrumbs.classList.add('hidden');
        
        // Update nav
        this._clearNavSelection();
        const navDashboard = document.getElementById('nav-dashboard');
        if (navDashboard) navDashboard.classList.add('active');
        
        // CRITICAL: Render the dashboard content
        console.log('[Nav] Calling Renderer.dashboard with', State.data.length, 'servers');
        Renderer.dashboard(State.data);
        
        // Update sidebar
        Data.updateSidebar();
        
        console.log('[Nav] === showDashboard END, activeView:', State.activeView, '===');
    },

    showServer(sortedIndex) {
        console.log('[Nav] showServer:', sortedIndex);
        
        const sortedData = State.getSortedData();
        const server = sortedData[sortedIndex];
        if (!server) {
            console.error('[Nav] No server at sorted index:', sortedIndex);
            return;
        }
        
        const actualIndex = State.data.findIndex(s => s.hostname === server.hostname);
        console.log('[Nav] Server:', server.hostname, 'actualIndex:', actualIndex);
        
        // Update state
        State.activeServerIndex = actualIndex;
        State.activeServerHostname = server.hostname;
        State.activeFilter = null;
        State.activeView = 'drives';
        
        // Update UI
        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        document.getElementById('page-title').textContent = server.hostname;
        document.getElementById('crumb-server').textContent = server.hostname;
        document.getElementById('breadcrumbs')?.classList.remove('hidden');
        
        // Nav highlight
        this._clearNavSelection();
        document.querySelectorAll('.server-nav-item').forEach((el, i) => {
            el.classList.toggle('active', i === sortedIndex);
        });
        
        // Render
        Renderer.serverDetail(server, actualIndex);
    },

    showDriveDetails(serverIdx, driveIdx) {
        console.log('[Nav] showDriveDetails:', serverIdx, driveIdx);
        
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) {
            console.error('[Nav] Drive not found');
            return;
        }
        
        State.activeServerIndex = serverIdx;
        State.activeServerHostname = server.hostname;
        State.activeView = 'drives';
        
        document.getElementById('dashboard-view')?.classList.add('hidden');
        document.getElementById('details-view')?.classList.remove('hidden');
        
        document.getElementById('page-title').textContent = Utils.getDriveName(drive);
        document.getElementById('crumb-server').textContent = `${server.hostname} › ${Utils.getDriveName(drive)}`;
        document.getElementById('breadcrumbs')?.classList.remove('hidden');
        
        Renderer.driveDetails(serverIdx, driveIdx);
    },

    showZFS() {
        console.log('[Nav] === showZFS START ===');
        
        // Update state
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'zfs';
        
        console.log('[Nav] Set activeView to:', State.activeView);
        
        // Update UI
        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        document.getElementById('page-title').textContent = 'ZFS Pools';
        document.getElementById('breadcrumbs')?.classList.add('hidden');
        
        // Nav highlight
        this._clearNavSelection();
        const navZfs = document.getElementById('nav-zfs');
        if (navZfs) navZfs.classList.add('active');
        
        // Render ZFS
        if (typeof ZFS !== 'undefined' && ZFS.render) {
            console.log('[Nav] Calling ZFS.render()');
            ZFS.render();
        } else {
            console.error('[Nav] ZFS module not found!');
        }
        
        // Update sidebar
        Data.updateSidebar();
        
        console.log('[Nav] === showZFS END ===');
    },

    showFilter(filter) {
        console.log('[Nav] showFilter:', filter);
        
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = filter;
        State.activeView = 'drives';
        
        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        
        const titles = {
            attention: 'Drives Needing Attention',
            healthy: 'Healthy Drives',
            all: 'All Drives'
        };
        document.getElementById('page-title').textContent = titles[filter] || 'Drives';
        document.getElementById('breadcrumbs')?.classList.add('hidden');
        
        this._clearNavSelection();
        
        const filterFns = {
            attention: d => Utils.getHealthStatus(d) !== 'healthy',
            healthy: d => Utils.getHealthStatus(d) === 'healthy',
            all: () => true
        };
        Renderer.filteredDrives(filterFns[filter] || (() => true), filter);
    },

    showAgents() {
        console.log('[Nav] showAgents');

        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'agents';

        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');

        document.getElementById('page-title').textContent = 'Agents';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();
        const navAgents = document.getElementById('nav-agents');
        if (navAgents) navAgents.classList.add('active');

        if (typeof Agents !== 'undefined' && Agents.render) {
            Agents.render();
        }
    },

    showSettings() {
        console.log('[Nav] showSettings');
        
        document.getElementById('dropdown-menu')?.classList.remove('show');
        State.activeView = 'settings';
        
        document.getElementById('dashboard-view')?.classList.add('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        
        let settingsView = document.getElementById('settings-view');
        if (!settingsView) {
            settingsView = document.createElement('div');
            settingsView.id = 'settings-view';
            settingsView.className = 'view settings-view';
            document.querySelector('.main-content')?.appendChild(settingsView);
        }
        
        settingsView.classList.remove('hidden');
        settingsView.innerHTML = Renderer.settingsPage();
        
        document.getElementById('page-title').textContent = 'Settings';
        document.getElementById('breadcrumbs')?.classList.add('hidden');
    },

    goBack() {
        console.log('[Nav] goBack');
        
        if (State.activeServerIndex !== null) {
            const sortedData = State.getSortedData();
            const sortedIdx = sortedData.findIndex(s => s.hostname === State.activeServerHostname);
            if (sortedIdx !== -1) {
                this.showServer(sortedIdx);
                return;
            }
        }
        this.showDashboard();
    },

    _clearNavSelection() {
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    }
};

// Legacy function aliases
function resetDashboard() { 
    console.log('[Legacy] resetDashboard called');
    Navigation.showDashboard(); 
}
function goBackToContext() { Navigation.goBack(); }
function fetchData() { Data.fetch(); }
function showZFSPools() { Navigation.showZFS(); }
function toggleServerSort() { State.toggleSortOrder(); }