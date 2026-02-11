/**
 * Vigil Dashboard - Navigation Controller
 */

const Navigation = {
    showDashboard() {
        console.log('showDashboard called, current activeView:', State.activeView);
        
        // CRITICAL: Reset ALL state before anything else
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'drives';  // MUST reset from 'zfs' to 'drives'
        
        console.log('State after reset, activeView:', State.activeView);
        
        // Show dashboard view, hide others
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        // Update header
        document.getElementById('page-title').textContent = 'Infrastructure Overview';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        // Clear all nav selections and mark Dashboard as active
        this.clearNavSelection();
        document.querySelector('.nav-section .nav-item[onclick*="resetDashboard"]')?.classList.add('active');
        
        // IMPORTANT: Directly render dashboard, don't call updateViews()
        // This prevents the ZFS check in updateViews from interfering
        Renderer.dashboard(State.data);
        
        // Update sidebar
        Data.updateSidebar();
    },

    showServer(sortedIndex) {
        const sortedData = State.getSortedData();
        const server = sortedData[sortedIndex];
        if (!server) return;
        
        const actualIndex = State.data.findIndex(s => s.hostname === server.hostname);
        if (actualIndex === -1) return;
        
        // Reset state - exit from ZFS view
        State.activeServerIndex = actualIndex;
        State.activeServerHostname = server.hostname;
        State.activeFilter = null;
        State.activeView = 'drives';  // Exit ZFS view
        
        // Update UI
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        document.getElementById('page-title').textContent = server.hostname;
        document.getElementById('crumb-server').textContent = server.hostname;
        document.getElementById('breadcrumbs').classList.remove('hidden');
        
        this.clearNavSelection();
        
        // Highlight correct server in sidebar
        document.querySelectorAll('.server-nav-item').forEach((el, i) => {
            el.classList.toggle('active', i === sortedIndex);
        });
        
        // Directly render server detail
        Renderer.serverDetail(server, actualIndex);
    },

    showDriveDetails(serverIdx, driveIdx) {
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) return;
        
        State.activeServerIndex = serverIdx;
        State.activeServerHostname = server.hostname;
        State.activeView = 'drives';
        
        document.getElementById('dashboard-view').classList.add('hidden');
        document.getElementById('details-view').classList.remove('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        document.getElementById('page-title').textContent = Utils.getDriveName(drive);
        document.getElementById('crumb-server').textContent = `${server.hostname} › ${Utils.getDriveName(drive)}`;
        document.getElementById('breadcrumbs').classList.remove('hidden');
        
        Renderer.driveDetails(serverIdx, driveIdx);
    },

    showSettings() {
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

    showFilter(filter) {
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = filter;
        State.activeView = 'drives';  // Exit ZFS view
        
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        document.getElementById('page-title').textContent = 
            filter === 'attention' ? 'Drives Needing Attention' :
            filter === 'healthy' ? 'Healthy Drives' : 'All Drives';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        this.clearNavSelection();
        
        // Render filtered drives directly
        if (filter === 'attention') {
            Renderer.filteredDrives(d => Utils.getHealthStatus(d) !== 'healthy', 'attention');
        } else if (filter === 'healthy') {
            Renderer.filteredDrives(d => Utils.getHealthStatus(d) === 'healthy', 'healthy');
        } else {
            Renderer.filteredDrives(() => true, 'all');
        }
    },

    showZFS() {
        console.log('showZFS called');
        
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'zfs';  // Set to ZFS view
        
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        document.getElementById('page-title').textContent = 'ZFS Pools';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        this.clearNavSelection();
        document.querySelector('#zfs-nav-section .nav-item')?.classList.add('active');
        
        if (typeof ZFS !== 'undefined' && ZFS.render) {
            ZFS.render();
        }
    },

    clearNavSelection() {
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    }
};

function resetDashboard() {
    Navigation.showDashboard();
}

function goBackToContext() {
    if (State.activeServerIndex !== null) {
        const sortedData = State.getSortedData();
        const sortedIdx = sortedData.findIndex(s => s.hostname === State.activeServerHostname);
        if (sortedIdx !== -1) {
            Navigation.showServer(sortedIdx);
        } else {
            Navigation.showDashboard();
        }
    } else {
        Navigation.showDashboard();
    }
}

function fetchData() {
    Data.fetch();
}

function showZFSPools() {
    Navigation.showZFS();
}

function toggleServerSort() {
    State.toggleSortOrder();
}