const API_URL = '/api/history';

// Helper: Format Bytes to TB/GB
function formatSize(bytes) {
    if (!bytes) return 'N/A';
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    if (bytes === 0) return '0 Byte';
    const i = parseInt(Math.floor(Math.log(bytes) / Math.log(1024)));
    return Math.round(bytes / Math.pow(1024, i), 2) + ' ' + sizes[i];
}

// Helper: Format Hours to Years
function formatAge(hours) {
    if (!hours) return 'N/A';
    const years = (hours / 8760).toFixed(1);
    return `${years} years`;
}

async function fetchData() {
    try {
        const res = await fetch(API_URL);
        const data = await res.json();
        render(data);
        document.getElementById('status-indicator').className = 'status-badge online';
        document.getElementById('status-indicator').innerHTML = '🟢 System Online';
    } catch (e) {
        document.getElementById('status-indicator').className = 'status-badge offline';
        document.getElementById('status-indicator').innerHTML = '🔴 Disconnected';
    }
}

function render(servers) {
    const list = document.getElementById('server-list');
    if (!servers || !servers.length) {
        list.innerHTML = '<div style="color:#aaa">Waiting for agents...</div>';
        return;
    }

    list.innerHTML = servers.map(server => {
        const drives = server.details.drives || [];
        
        // Build the HTML for each drive in this server
        const drivesHtml = drives.map(d => {
            // Safe access to nested properties
            const model = d.model_name || 'Unknown Device';
            const serial = d.serial_number || '';
            const capacity = formatSize(d.user_capacity?.bytes);
            const temp = d.temperature?.current ? `${d.temperature.current}°C` : 'N/A';
            const age = formatAge(d.power_on_time?.hours);
            const passed = d.smart_status?.passed;
            
            return `
            <div class="drive-row" style="border-left: 4px solid ${passed ? '#10B981' : '#EF4444'}">
                <div class="drive-header">
                    <div class="drive-model">${model} <span>${serial}</span></div>
                    </div>
                <div class="metrics">
                    <div class="metric">
                        <span class="label">Status</span>
                        <span class="value ${passed ? 'passed' : 'failed'}">${passed ? 'Passed' : 'Failed'}</span>
                    </div>
                    <div class="metric">
                        <span class="label">Temp</span>
                        <span class="value">${temp}</span>
                    </div>
                    <div class="metric">
                        <span class="label">Capacity</span>
                        <span class="value">${capacity}</span>
                    </div>
                    <div class="metric">
                        <span class="label">Powered On</span>
                        <span class="value">${age}</span>
                    </div>
                </div>
            </div>`;
        }).join('');

        return `
            <div class="card">
                <div class="card-header">
                    <span class="hostname">${server.hostname}</span>
                    <span class="timestamp">Updated: ${new Date(server.timestamp).toLocaleTimeString()}</span>
                </div>
                ${drivesHtml || '<div class="drive-row">No drives detected</div>'}
            </div>
        `;
    }).join('');
}

setInterval(fetchData, 2000);
fetchData();