const API_URL = '/api/history';
let globalData = [];
let activeServerIndex = null;

const formatSize = (b) => {
    if (!b) return '-';
    const s = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(b) / Math.log(1024));
    return `${(b / Math.pow(1024, i)).toFixed(2)} ${s[i]}`;
};
const formatAge = (h) => h ? `${(h / 8760).toFixed(1)}y` : '-';

// Navigation
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
    document.getElementById('crumb-current').innerText = server.hostname;
    document.getElementById('breadcrumbs').classList.remove('hidden');
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    renderDashboard([server], true);
}

function goBackToContext() {
    if (activeServerIndex !== null) showServer(activeServerIndex);
    else resetDashboard();
}

function showDriveDetails(serverIdx, driveIdx) {
    const server = globalData[serverIdx];
    const drive = server.details.drives[driveIdx];
    
    let rot = drive.rotation_rate || 'N/A';
    if (rot === 0) rot = 'SSD';
    if (typeof rot === 'number') rot += ' RPM';

    const sb = document.getElementById('detail-sidebar');
    sb.innerHTML = `
        <div class="panel-header"><h2>Specs</h2></div>
        <div class="spec-row"><span class="spec-key">Model</span><span class="spec-val">${drive.model_name || 'N/A'}</span></div>
        <div class="spec-row"><span class="spec-key">Serial</span><span class="spec-val">${drive.serial_number || 'N/A'}</span></div>
        <div class="spec-row"><span class="spec-key">Capacity</span><span class="spec-val">${formatSize(drive.user_capacity?.bytes)}</span></div>
        <div class="spec-row"><span class="spec-key">Rotation</span><span class="spec-val">${rot}</span></div>
        <br>
        <div class="panel-header"><h2>Status</h2></div>
        <div class="spec-row"><span class="spec-key">Health</span><span class="spec-val" style="color:${drive.smart_status?.passed?'#22c55e':'#ef4444'}">${drive.smart_status?.passed?'PASSED':'FAILED'}</span></div>
        <div class="spec-row"><span class="spec-key">Temp</span><span class="spec-val">${drive.temperature?.current ?? '-'}°C</span></div>
        <div class="spec-row"><span class="spec-key">Power On</span><span class="spec-val">${formatAge(drive.power_on_time?.hours)}</span></div>
    `;

    const tbody = document.getElementById('detail-table');
    const attr = drive.ata_smart_attributes?.table || [];
    if (!attr.length) {
        tbody.innerHTML = '<tr><td colspan="5" style="text-align:center; padding:24px">No Legacy Attributes (NVMe)</td></tr>';
    } else {
        tbody.innerHTML = `<thead><tr><th>ID</th><th>Attribute</th><th>Value</th><th>Thresh</th><th>Raw</th></tr></thead><tbody>` + 
        attr.map(a => {
            const fail = (a.raw?.value > 0 && [5,187,197,198].includes(a.id)) || (a.thresh > 0 && a.value <= a.thresh);
            return `<tr style="${fail?'color:#ef4444':''}"><td>${a.id}</td><td>${a.name}</td><td>${a.value}</td><td>${a.thresh}</td><td>${a.raw?.value}</td></tr>`;
        }).join('') + `</tbody>`;
    }

    document.getElementById('dashboard-view').classList.add('hidden');
    document.getElementById('details-view').classList.remove('hidden');
}

// Fetch
async function fetchData() {
    try {
        const res = await fetch(API_URL);
        globalData = await res.json();
        
        if (activeServerIndex !== null && !document.getElementById('dashboard-view').classList.contains('hidden')) {
            renderDashboard([globalData[activeServerIndex]], true);
        } else if (activeServerIndex === null) {
            renderDashboard(globalData);
        }

        // Update Version Pill Status (Green/Normal)
        const el = document.getElementById('connection-status');
        el.classList.remove('disconnected');
    } catch (e) {
        // Update Version Pill Status (Red/Disconnected)
        document.getElementById('connection-status').classList.add('disconnected');
    }
}

function renderDashboard(servers, isFiltered) {
    if (document.getElementById('dashboard-view').classList.contains('hidden')) return;
    const list = document.getElementById('server-list');

    if (!servers.length) { 
        list.innerHTML = '<div style="grid-column:1/-1; text-align:center; color:#52525b; margin-top:40px">Waiting for data...</div>';
        return; 
    }

    list.innerHTML = servers.map((server) => {
        const realIdx = globalData.findIndex(s => s.hostname === server.hostname);
        
        const drivesHtml = (server.details.drives || []).map((d, dIdx) => {
            const passed = d.smart_status?.passed;
            return `
            <div class="drive-module" onclick="showDriveDetails(${realIdx}, ${dIdx})">
                <div class="drive-info">
                    <span class="drive-model">${d.model_name || 'Unknown Drive'}</span>
                    <div class="drive-details">
                        <span>${formatSize(d.user_capacity?.bytes)}</span>
                        <span>${d.temperature?.current ?? '-'}°C</span>
                    </div>
                </div>
                <div class="health-badge ${passed ? 'hb-pass':'hb-fail'}">
                    ${passed ? 'Healthy' : 'Failing'}
                </div>
            </div>`;
        }).join('');

        return `
            <div class="server-card">
                <div class="card-header server-active" onclick="showServer(${realIdx})">
                    <div class="server-title">
                        <span class="status-indicator-dot"></span>
                        ${server.hostname}
                    </div>
                    <span class="server-meta">${new Date(server.timestamp).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'})}</span>
                </div>
                <div class="drive-grid">
                    ${drivesHtml || '<div style="color:#52525b; font-size:0.9rem">No drives detected</div>'}
                </div>
            </div>`;
    }).join('');

    list.style.display = isFiltered ? 'block' : 'grid';
    if(isFiltered) list.children[0].style.maxWidth = '600px';
}

setInterval(fetchData, 2000);
fetchData();