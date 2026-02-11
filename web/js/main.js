/**
 * Vigil Dashboard - Application Initialization
 * DEBUG VERSION - with extensive logging
 */

document.addEventListener('DOMContentLoaded', async () => {
    console.log('[Main] DOM loaded, initializing...');
    
    // Initialize state first
    if (typeof State !== 'undefined' && State.init) {
        State.init();
    }
    
    // Set up event listeners BEFORE auth check
    setupEventListeners();
    
    // Check auth
    const isAuth = await Auth.checkStatus();
    if (!isAuth) {
        console.log('[Main] Not authenticated');
        return;
    }
    
    console.log('[Main] Authenticated, fetching data...');
    
    // Fetch initial data
    Data.fetchVersion();
    await Data.fetch();
    
    // Start refresh timer
    State.refreshTimer = setInterval(() => {
        console.log('[Main] Auto-refresh, activeView:', State.activeView);
        Data.fetch();
    }, State.REFRESH_INTERVAL);
    
    console.log('[Main] Initialization complete');
});

function setupEventListeners() {
    console.log('[Main] Setting up event listeners...');
    
    // Logo
    const logo = document.getElementById('logo-link');
    if (logo) {
        logo.style.cursor = 'pointer';
        logo.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            console.log('[Click] Logo clicked');
            Navigation.showDashboard();
        });
    }
    
    // Dashboard nav
    const navDashboard = document.getElementById('nav-dashboard');
    if (navDashboard) {
        navDashboard.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            console.log('[Click] Dashboard nav clicked, current activeView:', State.activeView);
            Navigation.showDashboard();
        });
    }
    
    // Servers nav
    const navServers = document.getElementById('nav-servers');
    if (navServers) {
        navServers.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            console.log('[Click] Servers nav clicked');
            Navigation.showDashboard();
        });
    }
    
    // ZFS nav
    const navZfs = document.getElementById('nav-zfs');
    if (navZfs) {
        navZfs.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            console.log('[Click] ZFS nav clicked');
            Navigation.showZFS();
        });
    }
    
    // Sort button
    const sortBtn = document.getElementById('server-sort-btn');
    if (sortBtn) {
        sortBtn.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();
            console.log('[Click] Sort button clicked');
            State.toggleSortOrder();
        });
    }
    
    // Refresh button
    const refreshBtn = document.getElementById('btn-refresh');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', function(e) {
            console.log('[Click] Refresh clicked');
            Data.fetch();
        });
    }
    
    // Back button
    const backBtn = document.getElementById('btn-back');
    if (backBtn) {
        backBtn.addEventListener('click', function(e) {
            console.log('[Click] Back clicked');
            Navigation.goBack();
        });
    }
    
    // Breadcrumb
    const breadcrumbHome = document.getElementById('breadcrumb-home');
    if (breadcrumbHome) {
        breadcrumbHome.addEventListener('click', function(e) {
            e.preventDefault();
            console.log('[Click] Breadcrumb home clicked');
            Navigation.showDashboard();
        });
    }
    
    console.log('[Main] Event listeners set up complete');
}

// Cleanup
window.addEventListener('beforeunload', () => {
    if (State.refreshTimer) {
        clearInterval(State.refreshTimer);
    }
});

// Global navigation functions for dynamically created elements
window.navShowServer = function(idx) {
    console.log('[Global] navShowServer:', idx);
    Navigation.showServer(idx);
};

window.navShowDriveDetails = function(serverIdx, driveIdx) {
    console.log('[Global] navShowDriveDetails:', serverIdx, driveIdx);
    Navigation.showDriveDetails(serverIdx, driveIdx);
};

window.navShowZFSPool = function(hostname, poolName) {
    console.log('[Global] navShowZFSPool:', hostname, poolName);
    if (typeof ZFS !== 'undefined' && ZFS.showPoolDetail) {
        ZFS.showPoolDetail(hostname, poolName);
    }
};