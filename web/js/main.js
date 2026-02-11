/**
 * Vigil Dashboard - Application Initialization
 * All event listeners are set up here to avoid inline onclick issues
 */

document.addEventListener('DOMContentLoaded', async () => {
    console.log('[Main] Initializing Vigil Dashboard');
    
    // Initialize state
    State.init();
    
    // Set up all navigation event listeners
    setupNavigationListeners();
    
    // Check authentication
    const isAuth = await Auth.checkStatus();
    if (!isAuth) return;
    
    // Fetch initial data
    Data.fetchVersion();
    Data.fetch();
    
    // Set up refresh timer
    State.refreshTimer = setInterval(() => Data.fetch(), State.REFRESH_INTERVAL);
    
    console.log('[Main] Initialization complete');
});

/**
 * Set up all click event listeners for navigation
 */
function setupNavigationListeners() {
    // Logo click -> Dashboard
    const logo = document.getElementById('logo-link');
    if (logo) {
        logo.addEventListener('click', (e) => {
            e.preventDefault();
            Navigation.showDashboard();
        });
    }
    
    // Dashboard nav item
    const navDashboard = document.getElementById('nav-dashboard');
    if (navDashboard) {
        navDashboard.addEventListener('click', (e) => {
            e.preventDefault();
            Navigation.showDashboard();
        });
    }
    
    // Servers nav item
    const navServers = document.getElementById('nav-servers');
    if (navServers) {
        navServers.addEventListener('click', (e) => {
            e.preventDefault();
            Navigation.showDashboard();
        });
    }
    
    // ZFS nav item
    const navZfs = document.getElementById('nav-zfs');
    if (navZfs) {
        navZfs.addEventListener('click', (e) => {
            e.preventDefault();
            Navigation.showZFS();
        });
    }
    
    // Sort button
    const sortBtn = document.getElementById('server-sort-btn');
    if (sortBtn) {
        sortBtn.addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            State.toggleSortOrder();
        });
    }
    
    // Refresh button
    const refreshBtn = document.getElementById('btn-refresh');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', () => {
            Data.fetch();
        });
    }
    
    // Back button
    const backBtn = document.getElementById('btn-back');
    if (backBtn) {
        backBtn.addEventListener('click', () => {
            Navigation.goBack();
        });
    }
    
    // Breadcrumb home link
    const breadcrumbHome = document.getElementById('breadcrumb-home');
    if (breadcrumbHome) {
        breadcrumbHome.addEventListener('click', (e) => {
            e.preventDefault();
            Navigation.showDashboard();
        });
    }
    
    console.log('[Main] Navigation listeners set up');
}

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    if (State.refreshTimer) {
        clearInterval(State.refreshTimer);
    }
});

// Global functions for dynamically created elements (server list, etc.)
window.navShowServer = function(index) {
    Navigation.showServer(index);
};

window.navShowDriveDetails = function(serverIdx, driveIdx) {
    Navigation.showDriveDetails(serverIdx, driveIdx);
};

window.navShowZFSPool = function(hostname, poolName) {
    if (typeof ZFS !== 'undefined' && ZFS.showPoolDetail) {
        ZFS.showPoolDetail(hostname, poolName);
    }
};