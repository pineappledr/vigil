/**
 * Vigil Dashboard - Navigation Controller
 *
 * Uses a centralized _switchView() to hide ALL .view-section containers
 * and show only the target. Each major view has its own container element,
 * eliminating innerHTML conflicts between views.
 */

const Navigation = {
    /**
     * Central view switcher.  Hides every .view-section, then shows the
     * one identified by targetId.  Optionally resets shared state, updates
     * the page title, hides breadcrumbs, and highlights a nav item.
     */
    _switchView(targetId) {
        document.querySelectorAll('.view-section').forEach(el => {
            el.classList.add('hidden');
        });
        const target = document.getElementById(targetId);
        if (target) target.classList.remove('hidden');
    },

    showDashboard() {
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'drives';

        this._switchView('dashboard-view');

        const pageTitle = document.getElementById('page-title');
        const breadcrumbs = document.getElementById('breadcrumbs');
        if (pageTitle) pageTitle.textContent = 'Infrastructure Overview';
        if (breadcrumbs) breadcrumbs.classList.add('hidden');

        this._clearNavSelection();
        const navDashboard = document.getElementById('nav-dashboard');
        if (navDashboard) navDashboard.classList.add('active');

        Renderer.ensureDashboardStructure();
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

        this._switchView('dashboard-view');

        document.getElementById('page-title').textContent = server.hostname;
        document.getElementById('crumb-server').textContent = server.hostname;
        document.getElementById('breadcrumbs')?.classList.remove('hidden');

        this._clearNavSelection();
        document.querySelectorAll('.server-nav-item').forEach((el, i) => {
            el.classList.toggle('active', i === sortedIndex);
        });

        Renderer.ensureDashboardStructure();
        Renderer.serverDetail(server, actualIndex);
    },

    showDriveDetails(serverIdx, driveIdx) {
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) return;

        State.activeServerIndex = serverIdx;
        State.activeServerHostname = server.hostname;
        State.activeView = 'drives';

        this._switchView('details-view');

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

        this._switchView('zfs-view');

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

        this._switchView('dashboard-view');

        const titles = {
            attention: 'Drives Needing Attention',
            healthy: 'Healthy Drives',
            all: 'All Drives'
        };
        document.getElementById('page-title').textContent = titles[filter] || 'Drives';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();

        Renderer.ensureDashboardStructure();

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

        this._switchView('agents-view');

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

        this._switchView('addons-view');

        document.getElementById('page-title').textContent = 'Add-ons';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();
        const navAddons = document.getElementById('nav-addons');
        if (navAddons) navAddons.classList.add('active');

        // Set immediate loading state so the view is never blank
        const container = document.getElementById('addons-view');
        if (container && !container.innerHTML.trim()) {
            container.innerHTML = '<div class="loading-spinner"><div class="spinner"></div>Loading add-ons...</div>';
        }

        if (typeof Addons !== 'undefined' && Addons.render) {
            Addons.render();
        }
    },

    showNotifications() {
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'notifications';

        this._switchView('notifications-view');

        document.getElementById('page-title').textContent = 'Notifications';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();
        const navNotif = document.getElementById('nav-notifications');
        if (navNotif) navNotif.classList.add('active');

        if (typeof NotificationSettings !== 'undefined' && NotificationSettings.render) {
            NotificationSettings.render();
        }
    },

    showTemperature() {
        State.activeServerIndex = null;
        State.activeServerHostname = null;
        State.activeFilter = null;
        State.activeView = 'temperature';

        this._switchView('temperature-view');

        document.getElementById('page-title').textContent = 'Temperature Monitor';
        document.getElementById('breadcrumbs')?.classList.add('hidden');

        this._clearNavSelection();
        const navTemp = document.getElementById('nav-temperature');
        if (navTemp) navTemp.classList.add('active');

        if (typeof Temperature !== 'undefined' && Temperature.render) {
            Temperature.render();
        }
    },

    showSettings() {
        document.getElementById('dropdown-menu')?.classList.remove('show');
        State.activeView = 'settings';

        this._switchView('settings-view');

        const settingsView = document.getElementById('settings-view');
        if (settingsView) {
            // Safe: settingsPage() returns a static template with no user-controlled data
            const range = document.createRange();
            range.selectNodeContents(settingsView);
            range.deleteContents();
            settingsView.append(range.createContextualFragment(Renderer.settingsPage()));
        }

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
