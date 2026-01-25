const API_URL = '/api/history';
let globalData = [];
let activeServerIndex = null;

// Formatters
const formatSize = (b) => {
    if (!b) return '-';
    const s = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(b) / Math.log(1024));
    return `${(b / Math.pow(1024, i)).toFixed(2)} ${s[i]}`;
};
const formatAge = (h) => h ? `${(h / 8760).toFixed(1)}y` : '-';

// --- Navigation ---
function resetDashboard() {
    activeServerIndex = null;
    document.getElementById('breadcrumbs').classList.add('hidden');
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    renderDashboard(globalData);
}

function showServer(serverIdx) {
    activeServerIndex = serverIdx;
    const server = globalData[serverIdx];
    
    // Update Breadcrumbs
    document.getElementById('crumb-current').innerText = server.hostname;
    document.getElementById('breadcrumbs').classList.remove('hidden');

    // Show Dashboard View filtered
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    
    renderDashboard([server], true);
}

function showDriveDetails(serverIdx, driveIdx) {
    const server = globalData[serverIdx];
    const drive = server.details.drives[driveIdx];
    
    // Sidebar
    let rotation = drive.rotation_rate || 'N/A';
    if (rotation === 0) rotation = 'SSD';
    if (typeof rotation === 'number') rotation += ' RPM';

    const sb = document.getElementById('detail-sidebar');
    sb.innerHTML = `
        <div class="panel-header"><h2>Device Specs</h2></div>
        <div class="spec-item"><span class="spec-label">Model</span><span class="spec-value">${drive.model_name || 'N/A'}</span></div>
        <div class="spec-item"><span class="spec-label">Serial</span><span class="spec-value">${drive.serial_number || 'N/A'}</span></div>
        <div class="spec-item"><span class="spec-label">Capacity</span><span class="spec-value">${formatSize(drive.user_capacity?.bytes)}</span></div>
        <div class="spec-item"><span class="spec-label">Firmware</span><span class="spec-value">${drive.firmware_version || 'N/A'}</span></div>
        <div class="spec-item"><span class="spec-label">Rotation</span><span class="spec-value">${rotation}</span></div>
        <br>
        <div class="panel-header"><h2>Health</h2></div>
        <div class="spec-item">
            <span class="spec-label">Status</span>
            <span class="spec-value" style="color:${drive.smart_status?.passed ? '#22c55e':'#ef4444'}">
                ${drive.smart_status?.passed ? 'HEALTHY':'FAILING'}
            </span>
        </div>
        <div class="spec-item"><span class="spec-label">Temp</span><span class="spec-value">${drive.temperature?.current ?? '-'}°C</span></div>
        <div class="spec-item"><span class="spec-label">Power On</span><span class="spec-value">${formatAge(drive.power_on_time?.hours)}</span></div>
    `;

    // Table
    const tbody = document.getElementById('detail-table');
    const attributes = drive.ata_smart_attributes?.table || [];
    
    if (!attributes.length) {
        tbody.innerHTML = '<tr><td colspan="6" style="padding:24px; text-align:center; color:#52525b">No legacy ATA attributes (NVMe)</td></tr>';
    } else {
        tbody.innerHTML = `
            <thead><tr><th>ID</th><th>Attribute</th><th>Value</th><th>Thresh</th><th>Raw</th></tr></thead>
            <tbody>
            ${attributes.map(attr => {
                const isCrit = [5, 187, 197, 198].includes(attr.id);
                const isFail = (attr.raw?.value > 0 && isCrit) || (attr.thresh > 0 && attr.value <= attr.thresh);
                return `
                <tr style="${isFail ? 'color:#ef4444':''}">
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

function goBackToContext() {
    if (activeServerIndex !== null) showServer(activeServerIndex);
    else resetDashboard();
}

// --- Fetch ---
async function fetchData() {
    try {
        const res = await fetch(API_URL);
        globalData = await res.json();
        
        if (activeServerIndex !== null && !document.getElementById('dashboard-view').classList.contains('hidden')) {
            renderDashboard([globalData[activeServerIndex]], true);
        } else if (activeServerIndex === null) {
            renderDashboard(globalData);
        }

        const ind = document.getElementById('status-indicator');
        ind.className = 'status-pill online';
        ind.innerHTML = '<span class="status-dot"></span><span>System Online</span>';
    } catch (e) {
        document.getElementById('status-indicator').className = 'status-pill offline';
    }
}

function renderDashboard(servers, isFiltered = false) {
    if (document.getElementById('dashboard-view').classList.contains('hidden')) return;
    const list = document.getElementById('server-list');
    
    if (!servers.length) { 
        list.innerHTML = '<div style="color:#52525b; grid-column:1/-1; text-align:center; padding-top:40px">Waiting for agents...</div>'; 
        return; 
    }

    list.innerHTML = servers.map((server) => {
        const realIndex = globalData.findIndex(s => s.hostname === server.hostname);
        
        const drivesHtml = (server.details.drives || []).map((d, dIdx) => {
            const passed = d.smart_status?.passed;
            return `
            <div class="mini-drive" onclick="showDriveDetails(${realIndex}, ${dIdx})">
                <div class="mini-info">
                    <span class="mini-name">${d.model_name || 'Unknown Drive'}</span>
                    <span class="mini-meta">${formatSize(d.user_capacity?.bytes)} • ${d.temperature?.current ?? '-'}°C</span>
                </div>
                <div class="mini-status ${passed ? 'st-pass':'st-fail'}">${passed ? 'OK':'FAIL'}</div>
            </div>`;
        }).join('');

        return `
            <div class="server-card">
                <div class="card-header" onclick="showServer(${realIndex})">
                    <div class="server-name">
                        <span class="server-status-dot"></span>
                        ${server.hostname}
                    </div>
                    <span class="last-seen">${new Date(server.timestamp).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})}</span>
                </div>
                <div class="drive-list">
                    ${drivesHtml || '<div style="color:#52525b; font-size:0.9rem">No drives detected</div>'}
                </div>
            </div>`;
    }).join('');
    
    list.style.display = isFiltered ? 'block' : 'grid';
    if(isFiltered) list.children[0].style.maxWidth = '600px';
}

setInterval(fetchData, 2000);
fetchData();