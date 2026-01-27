/**
 * Vigil Dashboard - Data Management
 */

const Data = {
    async fetch() {
        try {
            const response = await API.getHistory();
            
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            State.data = await response.json() || [];
            this.updateViews();
            this.updateSidebar();
            this.updateStats();
            this.setOnlineStatus(true);
            this.updateLastRefresh();
            
        } catch (error) {
            console.error('Fetch error:', error);
            this.setOnlineStatus(false);
        }
    },

    updateViews() {
        const dashboardView = document.getElementById('dashboard-view');
        if (dashboardView.classList.contains('hidden')) return;

        if (State.activeServerIndex !== null && State.data[State.activeServerIndex]) {
            Renderer.serverDetail(State.data[State.activeServerIndex], State.activeServerIndex);
        } else if (State.activeFilter === 'attention') {
            Renderer.filteredDrives(d => Utils.getHealthStatus(d) !== 'healthy', 'attention');
        } else if (State.activeFilter === 'healthy') {
            Renderer.filteredDrives(d => Utils.getHealthStatus(d) === 'healthy', 'healthy');
        } else if (State.activeFilter === 'all') {
            Renderer.filteredDrives(() => true, 'all');
        } else {
            Renderer.dashboard(State.data);
        }
    },

    updateSidebar() {
        const serverNav = document.getElementById('server-nav-list');
        const serverCount = document.getElementById('server-count');
        
        serverCount.textContent = State.data.length;
        
        serverNav.innerHTML = State.data.map((server, idx) => {
            const drives = server.details?.drives || [];
            const hasWarning = drives.some(d => Utils.getHealthStatus(d) === 'warning');
            const hasCritical = drives.some(d => Utils.getHealthStatus(d) === 'critical');
            
            let statusClass = '';
            if (hasCritical) statusClass = 'critical';
            else if (hasWarning) statusClass = 'warning';
            
            return `
                <div class="server-nav-item ${State.activeServerIndex === idx ? 'active' : ''}" 
                     onclick="Navigation.showServer(${idx})">
                    <span class="status-indicator ${statusClass}"></span>
                    ${server.hostname}
                </div>
            `;
        }).join('');
    },

    updateStats() {
        const stats = State.getStats();
        document.getElementById('total-drives').textContent = stats.totalDrives;
        document.getElementById('healthy-count').textContent = stats.healthyDrives;
        document.getElementById('warning-count').textContent = stats.attentionDrives;
    },

    setOnlineStatus(online) {
        const indicator = document.getElementById('status-indicator');
        indicator.classList.toggle('online', online);
        indicator.classList.toggle('offline', !online);
        indicator.title = online ? 'Connected' : 'Connection Lost';
    },

    updateLastRefresh() {
        document.getElementById('last-update-time').textContent = 
            new Date().toLocaleTimeString([], { 
                hour: '2-digit', 
                minute: '2-digit', 
                second: '2-digit' 
            });
    },

    async fetchVersion() {
        try {
            const resp = await API.getVersion();
            if (resp.ok) {
                const data = await resp.json();
                const versionEl = document.getElementById('app-version');
                if (versionEl && data.version) {
                    versionEl.textContent = data.version.startsWith('v') ? data.version : `v${data.version}`;
                }
            }
        } catch (e) {
            console.warn('Could not fetch version:', e);
        }
    }
};
