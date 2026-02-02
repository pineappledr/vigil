/**
 * Vigil Dashboard - Navigation Controller
 */

const Navigation = {
    showDashboard() {
        State.reset();
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
        document.getElementById('page-title').textContent = 'Infrastructure Overview';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelector('.nav-item[onclick*="resetDashboard"]')?.classList.add('active');
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        
        Data.updateViews();
    },

    showServer(index) {
        State.setServer(index);
        const server = State.data[index];
        if (!server) return;
        
        document.getElementById('page-title').textContent = server.hostname;
        document.getElementById('crumb-server').textContent = server.hostname;
        document.getElementById('breadcrumbs').classList.remove('hidden');
        
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach((el, i) => {
            el.classList.toggle('active', i === index);
        });
        
        Data.updateViews();
    },

    showDriveDetails(serverIdx, driveIdx) {
        const server = State.data[serverIdx];
        const drive = server?.details?.drives?.[driveIdx];
        if (!drive) return;
        
        State.setServer(serverIdx);
        
        document.getElementById('dashboard-view').classList.add('hidden');
        document.getElementById('details-view').classList.remove('hidden');
        
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
        document.getElementById('page-title').textContent = 
            filter === 'attention' ? 'Drives Needing Attention' :
            filter === 'healthy' ? 'Healthy Drives' : 'All Drives';
        document.getElementById('breadcrumbs').classList.add('hidden');
        
        document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
        
        Data.updateViews();
    }
};

function resetDashboard() {
    Navigation.showDashboard();
}

function goBackToContext() {
    if (State.activeServerIndex !== null) {
        Navigation.showServer(State.activeServerIndex);
        document.getElementById('dashboard-view').classList.remove('hidden');
        document.getElementById('details-view').classList.add('hidden');
    } else {
        Navigation.showDashboard();
    }
}

function fetchData() {
    Data.fetch();
}