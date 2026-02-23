/**
 * Vigil Dashboard - Application Initialization
 */

document.addEventListener('DOMContentLoaded', async () => {
    // Initialize state first
    if (typeof State !== 'undefined' && State.init) {
        State.init();
    }

    // Set up event listeners BEFORE auth check
    setupEventListeners();

    // Check auth
    const isAuth = await Auth.checkStatus();
    if (!isAuth) return;

    // Fetch initial data
    Data.fetchVersion();
    await Data.fetch();

    // Start refresh timer
    State.refreshTimer = setInterval(() => Data.fetch(), State.REFRESH_INTERVAL);
});

function setupEventListeners() {
    // Logo
    const logo = document.getElementById('logo-link');
    if (logo) {
        logo.style.cursor = 'pointer';
        logo.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            Navigation.showDashboard();
        });
    }

    // Dashboard nav
    const navDashboard = document.getElementById('nav-dashboard');
    if (navDashboard) {
        navDashboard.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            Navigation.showDashboard();
        });
    }

    // Servers nav
    const navServers = document.getElementById('nav-servers');
    if (navServers) {
        navServers.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            Navigation.showDashboard();
        });
    }

    // ZFS nav
    const navZfs = document.getElementById('nav-zfs');
    if (navZfs) {
        navZfs.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            Navigation.showZFS();
        });
    }

    // Sort button
    const sortBtn = document.getElementById('server-sort-btn');
    if (sortBtn) {
        sortBtn.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            State.toggleSortOrder();
        });
    }

    // Refresh button
    const refreshBtn = document.getElementById('btn-refresh');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', () => Data.fetch());
    }

    // Back button
    const backBtn = document.getElementById('btn-back');
    if (backBtn) {
        backBtn.addEventListener('click', () => Navigation.goBack());
    }

    // Breadcrumb
    const breadcrumbHome = document.getElementById('breadcrumb-home');
    if (breadcrumbHome) {
        breadcrumbHome.addEventListener('click', function(e) {
            e.preventDefault();
            Navigation.showDashboard();
        });
    }

    // Close sidebar on mobile when a nav item is clicked
    const sidebarNav = document.querySelector('.sidebar-nav');
    if (sidebarNav) {
        sidebarNav.addEventListener('click', (e) => {
            if (e.target.closest('.nav-item') || e.target.closest('.server-nav-item')) {
                document.querySelector('.sidebar')?.classList.remove('open');
                document.querySelector('.sidebar-overlay')?.classList.remove('active');
            }
        });
    }
}

// Cleanup
window.addEventListener('beforeunload', () => {
    if (State.refreshTimer) {
        clearInterval(State.refreshTimer);
    }
});

// Global navigation functions for dynamically created elements
window.navShowServer = function(idx) {
    Navigation.showServer(idx);
};

window.navShowDriveDetails = function(serverIdx, driveIdx) {
    Navigation.showDriveDetails(serverIdx, driveIdx);
};

window.navShowZFSPool = function(hostname, poolName) {
    if (typeof ZFS !== 'undefined' && ZFS.showPoolDetail) {
        ZFS.showPoolDetail(hostname, poolName);
    }
};
