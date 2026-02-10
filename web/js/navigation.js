/**
 * Vigil Dashboard - Navigation Controller
 */

const Navigation = {
    showDashboard() {
        // Reset all state
        State.reset();
        
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        document.getElementById('page-title').textContent = 'Infrastructure Overview';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        this.clearNavSelection();
        document.querySelector('.nav-item[onclick*="resetDashboard"]')?.classList.add('active');
        
        // Render dashboard with sorted servers
        Renderer.dashboard(State.data);
    },

    showServer(sortedIndex) {
        const sortedData = State.getSortedData();
        const server = sortedData[sortedIndex];
        if (!server) return;
        
        const actualIndex = State.data.findIndex(s => s.hostname === server.hostname);
        if (actualIndex === -1) return;
        
        // Update state
        State.activeServerIndex = actualIndex;
        State.activeServerHostname = server.hostname;
        State.activeFilter = null;
        State.activeView = 'drives';
        
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
        
        // Render server detail directly (not through updateViews to avoid ZFS check)
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
        State.setFilter(filter);
        
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        
        document.getElementById('page-title').textContent = 
            filter === 'attention' ? 'Drives Needing Attention' :
            filter === 'healthy' ? 'Healthy Drives' : 'All Drives';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        this.clearNavSelection();
        
        Data.updateViews();
    },

    showZFS() {
        State.setView('zfs');
        
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