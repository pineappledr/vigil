/**
 * Vigil Dashboard - Navigation Controller
 */

const Navigation = {
    showDashboard() {
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'drives';

        const dashboardView = document.getElementById('dashboard-view');
        const detailsView = document.getElementById('details-view');
        const settingsView = document.getElementById('settings-view');

        if (dashboardView) dashboardView.classList.remove('hidden');
        if (detailsView) detailsView.classList.add('hidden');
        if (settingsView) settingsView.classList.add('hidden');

        const pageTitle = document.getElementById('page-title');
        const breadcrumbs = document.getElementById('breadcrumbs');
        if (pageTitle) pageTitle.textContent = 'Infrastructure Overview';
        if (breadcrumbs) breadcrumbs.classList.add('hidden');

        this._clearNavSelection();
        const navDashboard = document.getElementById('nav-dashboard');
        if (navDashboard) navDashboard.classList.add('active');

        Renderer.dashboard(State.data);
        Data.updateSidebar();
    },

    showServer(sortedIndex) {
        const sortedData = State.getSortedData();
        const server = sortedData[sortedIndex];
        if (!server) return;

        const actualIndex = State.data.findIndex(s => s.hostname === server.hostname);

        State.activeServerIndex = actualIndex;
        State.activeServerHostname = server.hostname;
        State.activeFilter = null;
        State.activeView = 'drives';

        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');

        document.getElementById('page-title').textContent = server.hostname;
        document.getElementById('crumb-server').textContent = server.hostname;
        document.getElementById('breadcrumbs')?.classList.remove('hidden');

        this._clearNavSelection();
        document.querySelectorAll('.server-nav-item').forEach((el, i) => {
            el.classList.toggle('active', i === sortedIndex);
        });

        Renderer.serverDetail(server, actualIndex);
    },

    showDriveDetails(serverIdx, driveIdx) {
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) return;

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
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'zfs';

        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');

        document.getElementById('page-title').textContent = 'ZFS Pools';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();
        const navZfs = document.getElementById('nav-zfs');
        if (navZfs) navZfs.classList.add('active');

        if (typeof ZFS !== 'undefined' && ZFS.render) {
            ZFS.render();
        }

        Data.updateSidebar();
    },

    showFilter(filter) {
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

    showAddons() {
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'addons';

        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');

        document.getElementById('page-title').textContent = 'Add-ons';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();
        const navAddons = document.getElementById('nav-addons');
        if (navAddons) navAddons.classList.add('active');

        if (typeof Addons !== 'undefined' && Addons.render) {
            Addons.render();
        }
    },

    showNotifications() {
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'notifications';

        document.getElementById('dashboard-view')?.classList.remove('hidden');
        document.getElementById('details-view')?.classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');

        document.getElementById('page-title').textContent = 'Notifications';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();
        const navNotif = document.getElementById('nav-notifications');
        if (navNotif) navNotif.classList.add('active');

        if (typeof NotificationSettings !== 'undefined' && NotificationSettings.render) {
            NotificationSettings.render();
        }
    },

    showSettings() {
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
        // Safe: settingsPage() returns a static template with no user-controlled data
        const range = document.createRange();
        range.selectNodeContents(settingsView);
        range.deleteContents();
        settingsView.append(range.createContextualFragment(Renderer.settingsPage()));

        document.getElementById('page-title').textContent = 'Settings';
        document.getElementById('breadcrumbs')?.classList.add('hidden');
    },

    goBack() {
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
function resetDashboard() { Navigation.showDashboard(); }
function goBackToContext() { Navigation.goBack(); }
function fetchData() { Data.fetch(); }
function showZFSPools() { Navigation.showZFS(); }
function toggleServerSort() { State.toggleSortOrder(); }
