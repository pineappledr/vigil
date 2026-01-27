/**
 * Vigil Dashboard - Navigation
 */

const Navigation = {
    resetDashboard() {
        State.reset();
        this.updateUI('Infrastructure Overview');
        document.getElementById('breadcrumbs').classList.add('hidden');
        document.getElementById('settings-view')?.classList.add('hidden');
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelector('.nav-item')?.classList.add('active');
        document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
        
        Renderer.dashboard(State.data);
    },

    showNeedsAttention() {
        State.setFilter('attention');
        this.showFilteredView('Needs Attention', 'Drives Needing Attention', 
            d => Utils.getHealthStatus(d) !== 'healthy', 'attention');
    },

    showHealthyDrives() {
        State.setFilter('healthy');
        this.showFilteredView('Healthy Drives', 'Healthy Drives',
            d => Utils.getHealthStatus(d) === 'healthy', 'healthy');
    },

    showAllDrives() {
        State.setFilter('all');
        this.showFilteredView('All Drives', 'All Drives', () => true, 'all');
    },

    showFilteredView(crumb, title, filterFn, filterType) {
        this.updateUI(title, crumb);
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('dashboard-view').classList.remove('hidden');
        this.clearNavSelection();
        Renderer.filteredDrives(filterFn, filterType);
    },

    showServer(serverIdx) {
        State.setServer(serverIdx);
        const server = State.data[serverIdx];
        if (!server) {
            this.resetDashboard();
            return;
        }

        this.updateUI(server.hostname, server.hostname);
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('dashboard-view').classList.remove('hidden');
        
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach((el, idx) => {
            el.classList.toggle('active', idx === serverIdx);
        });

        Renderer.serverDetail(server, serverIdx);
    },

    showDriveDetails(serverIdx, driveIdx) {
        Renderer.driveDetails(serverIdx, driveIdx);
        document.getElementById('dashboard-view').classList.add('hidden');
        document.getElementById('details-view').classList.remove('hidden');
    },

    goBackToContext() {
        if (State.activeFilter === 'attention') this.showNeedsAttention();
        else if (State.activeFilter === 'healthy') this.showHealthyDrives();
        else if (State.activeFilter === 'all') this.showAllDrives();
        else if (State.activeServerIndex !== null) this.showServer(State.activeServerIndex);
        else this.resetDashboard();
    },

    showSettings() {
        document.getElementById('dashboard-view').classList.add('hidden');
        document.getElementById('details-view').classList.add('hidden');
        this.updateUI('Settings', 'Settings');
        this.clearNavSelection();
        Renderer.settings();
    },

    updateUI(title, crumb = null) {
        document.getElementById('page-title').textContent = title;
        if (crumb) {
            document.getElementById('crumb-server').textContent = crumb;
            document.getElementById('breadcrumbs').classList.remove('hidden');
        }
    },

    clearNavSelection() {
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    }
};

// Global function aliases for onclick handlers
const resetDashboard = () => Navigation.resetDashboard();
const showNeedsAttention = () => Navigation.showNeedsAttention();
const showHealthyDrives = () => Navigation.showHealthyDrives();
const showAllDrives = () => Navigation.showAllDrives();
const showServer = (idx) => Navigation.showServer(idx);
const showDriveDetails = (s, d) => Navigation.showDriveDetails(s, d);
const goBackToContext = () => Navigation.goBackToContext();
const showSettings = () => Navigation.showSettings();
const fetchData = () => Data.fetch();
const showAliasModal = (h, s, a, d) => Modals.showAlias(h, s, a, d);
const showChangePasswordModal = () => Modals.showChangePassword();
const showChangeUsernameModal = () => Modals.showChangeUsername();
const toggleUserMenu = () => Auth.toggleMenu();
const logout = () => Auth.logout();
