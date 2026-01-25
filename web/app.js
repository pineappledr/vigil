const API_URL = '/api/history';
const statusEl = document.getElementById('status-indicator');
const listEl = document.getElementById('server-list');

async function fetchData() {
    try {
        const response = await fetch(API_URL);
        if (!response.ok) throw new Error("Network error");
        const data = await response.json();
        render(data);
        statusEl.textContent = "System Online";
        statusEl.className = "status online";
    } catch (error) {
        statusEl.textContent = "Connection Lost";
        statusEl.className = "status offline";
    }
}

function render(servers) {
    if (!servers || servers.length === 0) {
        listEl.innerHTML = '<div class="loading">No agents reporting yet...</div>';
        return;
    }
    listEl.innerHTML = servers.map(server => {
        const drives = server.details.drives || [];
        const driveHtml = drives.map(d => `
            <li class="drive-item">
                <span class="drive-name">${d.model_name || 'Unknown Drive'}</span>
                <span class="drive-status ${d.smart_status?.passed ? 'passed' : 'failed'}">
                    ${d.smart_status?.passed ? 'HEALTHY' : 'FAILING'}
                </span>
            </li>
        `).join('');

        return `
            <div class="card">
                <div class="card-header">
                    <span class="hostname">${server.hostname}</span>
                    <span class="time">${new Date(server.timestamp).toLocaleTimeString()}</span>
                </div>
                <ul class="drive-list">
                    ${driveHtml || '<li class="drive-item">No drives detected</li>'}
                </ul>
            </div>
        `;
    }).join('');
}

setInterval(fetchData, 2000);
fetchData();