const API_URL = '/api/history';
let globalData = [];
let activeServerIndex = null; // Track if we are viewing a specific server

// Formatters
const formatSize = (b) => {
    if (!b) return 'N/A';
    const s = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(b) / Math.log(1024));
    return `${(b / Math.pow(1024, i)).toFixed(2)} ${s[i]}`;
};
const formatAge = (h) => h ? `${(h / 8760).toFixed(1)} years` : 'N/A';

// --- Navigation Logic ---

// 1. Show All Servers (Home)
function resetDashboard() {
    activeServerIndex = null;
    document.getElementById('breadcrumbs').classList.add('hidden');
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    renderDashboard(globalData); // Render ALL
}

// 2. Show Single Server (Filtered)
function showServer(serverIdx) {
    activeServerIndex = serverIdx;
    const server = globalData[serverIdx];
    
    // Update Breadcrumbs
    document.getElementById('crumb-server').innerText = server.hostname;
    document.getElementById('breadcrumbs').classList.remove('hidden');

    // Show Dashboard View, but render ONLY this server
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    
    renderDashboard([server], true); // true = "filtered mode"
}

// 3. Show Drive Details
function showDriveDetails(serverIdx, driveIdx) {
    const server = globalData[serverIdx];
    const drive = server.details.drives[driveIdx];
    
    // If we are in "Single Server Mode", we need to map the passed driveIdx correctly
    // But since we pass global indices, it's fine.

    // Populate Sidebar
    let rotation = drive.rotation_rate || 'N/A';
    if (rotation === 0) rotation = 'SSD';
    if (typeof rotation === 'number') rotation += ' RPM';

    const sb = document.getElementById('detail-sidebar');
    sb.innerHTML = `
        <div class="sidebar-group">
            <h3>Device Info</h3>
            <div class="spec-row"><span class="spec-label">Model</span><span class="spec-val">${drive.model_name || 'N/A'}</span></div>
            <div class="spec-row"><span class="spec-label">Serial</span><span class="spec-val">${drive.serial_number || 'N/A'}</span></div>
            <div class="spec-row"><span class="spec-label">Capacity</span><span class="spec-val">${formatSize(drive.user_capacity?.bytes)}</span></div>
            <div class="spec-row"><span class="spec-label">Firmware</span><span class="spec-val">${drive.firmware_version || 'N/A'}</span></div>
            <div class="spec-row"><span class="spec-label">Rotation</span><span class="spec-val">${rotation}</span></div>
        </div>
        <div class="sidebar-group">
            <h3>Status</h3>
            <div class="spec-row"><span class="spec-label">Health</span><span class="spec-val" style="color:${drive.smart_status?.passed ? '#10B981':'#EF4444'}">${drive.smart_status?.passed ? 'PASSED':'FAILED'}</span></div>
            <div class="spec-row"><span class="spec-label">Temp</span><span class="spec-val">${drive.temperature?.current ?? 'N/A'}°C</span></div>
            <div class="spec-row"><span class="spec-label">Power On</span><span class="spec-val">${formatAge(drive.power_on_time?.hours)}</span></div>
        </div>
    `;

    // Populate Table
    const tbody = document.getElementById('detail-table');
    const attributes = drive.ata_smart_attributes?.table || [];
    
    if (attributes.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="padding:20px; text-align:center; color:#666">No standard ATA attributes found (NVMe drives use different logs)</td></tr>';
    } else {
        tbody.innerHTML = `
            <thead><tr><th>Status</th><th>ID</th><th>Name</th><th>Value</th><th>Thresh</th><th>Raw</th></tr></thead>
            <tbody>
            ${attributes.map(attr => {
                const isCrit = [5, 187, 197, 198].includes(attr.id);
                const isFail = (attr.raw?.value > 0 && isCrit) || (attr.thresh > 0 && attr.value <= attr.thresh);
                const pillClass = isFail ? 'bad' : 'ok';
                return `
                <tr>
                    <td><span class="pill ${pillClass}">${isFail ? 'FAIL':'OK'}</span></td>
                    <td>${attr.id}</td>
                    <td>${attr.name}</td>
                    <td>${attr.value}</td>
                    <td>${attr.thresh}</td>
                    <td>${attr.raw?.value}</td>
                </tr>`;
            }).join('')}
            </tbody>`;
    }

    document.getElementById('dashboard-view').classList.add('hidden');
    document.getElementById('details-view').classList.remove('hidden');
}

// "Smart" Back Button
function goBackToContext() {
    if (activeServerIndex !== null) {
        showServer(activeServerIndex); // Go back to the single server view
    } else {
        resetDashboard(); // Go back to main home
    }
}

// --- Data & Rendering ---
async function fetchData() {
    try {
        const res = await fetch(API_URL);
        globalData = await res.json();
        
        // If we are viewing a specific server, re-render just that server to update data
        if (activeServerIndex !== null && !document.getElementById('dashboard-view').classList.contains('hidden')) {
            renderDashboard([globalData[activeServerIndex]], true);
        } else if (activeServerIndex === null) {
            renderDashboard(globalData);
        }

        const ind = document.getElementById('status-indicator');
        ind.className = 'status-badge online';
        ind.innerText = '🟢 System Online';
    } catch (e) {
        document.getElementById('status-indicator').className = 'status-badge offline';
    }
}

function renderDashboard(serversToRender, isFiltered = false) {
    if (document.getElementById('dashboard-view').classList.contains('hidden')) return;

    const list = document.getElementById('server-list');
    if (!serversToRender.length) { list.innerHTML = '<div style="color:#666">Waiting for agents...</div>'; return; }

    list.innerHTML = serversToRender.map((server) => {
        // Find the REAL index of this server in the global array
        // This ensures clicking it works even if we are in a filtered list
        const realIndex = globalData.findIndex(s => s.hostname === server.hostname);

        const drivesHtml = (server.details.drives || []).map((d, dIdx) => {
            const passed = d.smart_status?.passed;
            return `
            <div class="drive-row" style="border-left: 4px solid ${passed ? '#10B981':'#EF4444'}">
                <div class="drive-link" onclick="showDriveDetails(${realIndex}, ${dIdx})">
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
            <div class="card" style="${isFiltered ? 'max-width: 800px; margin: 0 auto;' : ''}">
                <div class="card-header">
                    <span class="hostname-link" onclick="showServer(${realIndex})">${server.hostname}</span>
                    <span class="timestamp">${new Date(server.timestamp).toLocaleTimeString()}</span>
                </div>
                ${drivesHtml || '<div class="drive-row">No drives detected</div>'}
            </div>`;
    }).join('');
    
    // Adjust grid for filtered view
    list.style.display = isFiltered ? 'block' : 'grid';
}

setInterval(fetchData, 2000);
fetchData();