/**
 * Vigil Dashboard - Navigation Controller
 */

const Navigation = {
    showDashboard() {
        console.log('[Nav] showDashboard');
        
        // Reset state completely
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'drives';
        
        // Show correct view
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        const settingsView = document.getElementById('settings-view');
        if (settingsView) settingsView.classList.add('hidden');
        
        // Update header
        document.getElementById('page-title').textContent = 'Infrastructure Overview';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        // Update nav highlighting
        this._clearNavSelection();
        const navDashboard = document.getElementById('nav-dashboard');
        if (navDashboard) navDashboard.classList.add('active');
        
        // Render dashboard
        Renderer.dashboard(State.data);
        
        // Update sidebar
        Data.updateSidebar();
    },

    showServer(sortedIndex) {
        console.log('[Nav] showServer:', sortedIndex);
        
        const sortedData = State.getSortedData();
        const server = sortedData[sortedIndex];
        if (!server) {
            console.error('[Nav] No server at index:', sortedIndex);
            return;
        }
        
        const actualIndex = State.data.findIndex(s => s.hostname === server.hostname);
        if (actualIndex === -1) return;
        
        // Update state
        State.activeServerIndex = actualIndex;
        State.activeServerHostname = server.hostname;
        State.activeFilter = null;
        State.activeView = 'drives';
        
        // Show correct view
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        
        // Update header
        document.getElementById('page-title').textContent = server.hostname;
        document.getElementById('crumb-server').textContent = server.hostname;
        document.getElementById('breadcrumbs').classList.remove('hidden');
        
        // Update nav highlighting
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
        if (!drive) return;
        
        State.activeServerIndex = serverIdx;
        State.activeServerHostname = server.hostname;
        State.activeView = 'drives';
        
        document.getElementById('dashboard-view').classList.add('hidden');
        document.getElementById('details-view').classList.remove('hidden');
        
        document.getElementById('page-title').textContent = Utils.getDriveName(drive);
        document.getElementById('crumb-server').textContent = `${server.hostname} › ${Utils.getDriveName(drive)}`;
        document.getElementById('breadcrumbs').classList.remove('hidden');
        
        Renderer.driveDetails(serverIdx, driveIdx);
    },

    showZFS() {
        console.log('[Nav] showZFS');
        
        // Update state
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'zfs';
        
        // Show correct view
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        
        // Update header
        document.getElementById('page-title').textContent = 'ZFS Pools';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        // Update nav highlighting
        this._clearNavSelection();
        const navZfs = document.getElementById('nav-zfs');
        if (navZfs) navZfs.classList.add('active');
        
        // Render ZFS
        if (typeof ZFS !== 'undefined' && ZFS.render) {
            ZFS.render();
        }
        
        // Update sidebar
        Data.updateSidebar();
    },

    showFilter(filter) {
        console.log('[Nav] showFilter:', filter);
        
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = filter;
        State.activeView = 'drives';
        
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        
        document.getElementById('page-title').textContent = 
            filter === 'attention' ? 'Drives Needing Attention' :
            filter === 'healthy' ? 'Healthy Drives' : 'All Drives';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        this._clearNavSelection();
        
        const filterFn = filter === 'attention' 
            ? d => Utils.getHealthStatus(d) !== 'healthy'
            : filter === 'healthy'
            ? d => Utils.getHealthStatus(d) === 'healthy'
            : () => true;
        
        Renderer.filteredDrives(filterFn, filter);
    },

    showSettings() {
        document.getElementById('dropdown-menu')?.classList.remove('show');
        State.activeView = 'settings';
        
        document.getElementById('dashboard-view').classList.add('hidden');
        document.getElementById('details-view').classList.add('hidden');
        
        let settingsView = document.getElementById('settings-view');
        if (!settingsView) {
            settingsView = document.createElement('div');
            settingsView.id = 'settings-view';
            settingsView.className = 'view settings-view';
            document.getElementById('dashboard-view').parentNode.appendChild(settingsView);
        }
        
        settingsView.classList.remove('hidden');
        settingsView.innerHTML = Renderer.settingsPage();
        
        document.getElementById('page-title').textContent = 'Settings';
        document.getElementById('breadcrumbs').classList.add('hidden');
    },

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

    _clearNavSelection() {
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.zfs-pool-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    }
};

// Legacy aliases
function resetDashboard() { Navigation.showDashboard(); }
function goBackToContext() { Navigation.goBack(); }
function fetchData() { Data.fetch(); }
function showZFSPools() { Navigation.showZFS(); }
function toggleServerSort() { State.toggleSortOrder(); }