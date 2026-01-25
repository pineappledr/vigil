const API_URL = '/api/history';
let globalData = []; // Store data locally to switch views instantly

// Formatters
const formatSize = (b) => {
    if (!b) return 'N/A';
    const s = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(b) / Math.log(1024));
    return `${(b / Math.pow(1024, i)).toFixed(2)} ${s[i]}`;
};
const formatAge = (h) => h ? `${(h / 8760).toFixed(1)} years` : 'N/A';

// --- Navigation ---
function showDetails(serverIdx, driveIdx) {
    const server = globalData[serverIdx];
    const drive = server.details.drives[driveIdx];
    
    // 1. Populate Sidebar
    const sb = document.getElementById('detail-sidebar');
    sb.innerHTML = `
        <div class="sidebar-group">
            <h3>Device Info</h3>
            <div class="spec-row"><span class="spec-label">Model</span><span class="spec-val">${drive.model_name || 'N/A'}</span></div>
            <div class="spec-row"><span class="spec-label">Serial</span><span class="spec-val">${drive.serial_number || 'N/A'}</span></div>
            <div class="spec-row"><span class="spec-label">Capacity</span><span class="spec-val">${formatSize(drive.user_capacity?.bytes)}</span></div>
            <div class="spec-row"><span class="spec-label">Firmware</span><span class="spec-val">${drive.firmware_version || 'N/A'}</span></div>
        </div>
        <div class="sidebar-group">
            <h3>Status</h3>
            <div class="spec-row"><span class="spec-label">Health</span><span class="spec-val" style="color:${drive.smart_status?.passed ? '#10B981':'#EF4444'}">${drive.smart_status?.passed ? 'PASSED':'FAILED'}</span></div>
            <div class="spec-row"><span class="spec-label">Temp</span><span class="spec-val">${drive.temperature?.current ?? 'N/A'}°C</span></div>
            <div class="spec-row"><span class="spec-label">Power On</span><span class="spec-val">${formatAge(drive.power_on_time?.hours)}</span></div>
            <div class="spec-row"><span class="spec-label">Cycles</span><span class="spec-val">${drive.power_cycle_count || 'N/A'}</span></div>
        </div>
        <div class="sidebar-group">
            <h3>System</h3>
            <div class="spec-row"><span class="spec-label">Host</span><span class="spec-val">${server.hostname}</span></div>
            <div class="spec-row"><span class="spec-label">Updated</span><span class="spec-val">${new Date(server.timestamp).toLocaleTimeString()}</span></div>
        </div>
    `;

    // 2. Populate Table
    const tbody = document.getElementById('detail-table');
    const attributes = drive.ata_smart_attributes?.table || [];
    
    if (attributes.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6">No standard ATA attributes found (NVMe drives use different logs)</td></tr>';
    } else {
        tbody.innerHTML = `
            <thead><tr><th>Status</th><th>ID</th><th>Name</th><th>Value</th><th>Thresh</th><th>Raw</th></tr></thead>
            <tbody>
            ${attributes.map(attr => {
                // Determine status logic (simplified)
                const isCrit = [5, 187, 197, 198].includes(attr.id);
                const isFail = (attr.raw?.value > 0 && isCrit) || (attr.thresh > 0 && attr.value <= attr.thresh);
                const pillClass = isFail ? 'bad' : 'ok';
                const pillText = isFail ? 'FAIL' : 'OK';

                return `
                <tr>
                    <td><span class="pill ${pillClass}">${pillText}</span></td>
                    <td>${attr.id}</td>
                    <td>${attr.name}</td>
                    <td>${attr.value}</td>
                    <td>${attr.thresh}</td>
                    <td>${attr.raw?.value}</td>
                </tr>`;
            }).join('')}
            </tbody>`;
    }

    // 3. Switch Views
    document.getElementById('dashboard-view').classList.add('hidden');
    document.getElementById('details-view').classList.remove('hidden');
}

function showDashboard() {
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
}

// --- Data Fetching ---
async function fetchData() {
    try {
        const res = await fetch(API_URL);
        globalData = await res.json(); // Store globally
        renderDashboard(globalData);
        
        const ind = document.getElementById('status-indicator');
        ind.className = 'status-badge online';
        ind.innerText = '🟢 System Online';
    } catch (e) {
        document.getElementById('status-indicator').className = 'status-badge offline';
    }
}

function renderDashboard(servers) {
    // Only re-render if we are currently looking at the dashboard
    if (document.getElementById('dashboard-view').classList.contains('hidden')) return;

    const list = document.getElementById('server-list');
    if (!servers.length) { list.innerHTML = 'Waiting...'; return; }

    list.innerHTML = servers.map((server, sIdx) => {
        const drivesHtml = (server.details.drives || []).map((d, dIdx) => {
            const passed = d.smart_status?.passed;
            return `
            <div class="drive-row" style="border-left: 4px solid ${passed ? '#10B981':'#EF4444'}">
                <div class="drive-link" onclick="showDetails(${sIdx}, ${dIdx})">
                    ${d.model_name || 'Unknown Drive'}
                </div>
                <div class="metrics">
                    <div class="metric"><span class="label">Status</span><span class="value ${passed?'passed':'failed'}">${passed?'Passed':'Failed'}</span></div>
                    <div class="metric"><span class="label">Temp</span><span class="value">${d.temperature?.current ?? 'N/A'}°C</span></div>
                    <div class="metric"><span class="label">Cap</span><span class="value">${formatSize(d.user_capacity?.bytes)}</span></div>
                    <div class="metric"><span class="label">Age</span><span class="value">${formatAge(d.power_on_time?.hours)}</span></div>
                </div>
            </div>`;
        }).join('');

        return `
            <div class="card">
                <div class="card-header"><span class="hostname">${server.hostname}</span><span class="timestamp">${new Date(server.timestamp).toLocaleTimeString()}</span></div>
                ${drivesHtml || '<div class="drive-row">No drives detected</div>'}
            </div>`;
    }).join('');
}

setInterval(fetchData, 2000);
fetchData();