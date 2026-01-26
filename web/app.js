/**
 * Vigil Dashboard - Infrastructure Monitor
 * Modern, real-time server monitoring interface
 */

const API_URL = '/api/history';
const REFRESH_INTERVAL = 5000;

// Application State
let globalData = [];
let activeServerIndex = null;
let activeFilter = null; // 'attention' or null
let refreshTimer = null;

// ============================================
// UTILITIES
// ============================================

const formatSize = (bytes) => {
    if (!bytes) return 'N/A';
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${sizes[i]}`;
};

const formatAge = (hours) => {
    if (!hours) return 'N/A';
    const years = hours / 8760;
    if (years >= 1) return `${years.toFixed(1)}y`;
    const months = hours / 730;
    if (months >= 1) return `${months.toFixed(1)}mo`;
    const days = hours / 24;
    if (days >= 1) return `${days.toFixed(0)}d`;
    return `${hours}h`;
};

const formatTime = (timestamp) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

const getHealthStatus = (drive) => {
    if (!drive.smart_status?.passed) return 'critical';
    
    // Check critical SMART attributes
    const attrs = drive.ata_smart_attributes?.table || [];
    const criticalIds = [5, 187, 197, 198]; // Reallocated, Uncorrectable, Pending, Offline
    
    for (const attr of attrs) {
        if (criticalIds.includes(attr.id) && attr.raw?.value > 0) {
            return 'warning';
        }
    }
    
    return 'healthy';
};

const getRotationType = (rate) => {
    if (rate === 0) return 'SSD (Solid State)';
    if (rate === undefined || rate === null) return 'Unknown';
    return `HDD (${rate} RPM)`;
};

// ============================================
// NAVIGATION
// ============================================

function resetDashboard() {
    activeServerIndex = null;
    activeFilter = null;
    
    document.getElementById('breadcrumbs').classList.add('hidden');
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    document.getElementById('page-title').textContent = 'Infrastructure Overview';
    
    // Hide settings view if it exists
    const settingsView = document.getElementById('settings-view');
    if (settingsView) settingsView.classList.add('hidden');
    
    // Reset nav active states
    document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
    document.querySelector('.nav-item')?.classList.add('active');
    
    // Remove active state from summary cards
    document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    
    renderDashboard(globalData);
}

function showNeedsAttention() {
    activeServerIndex = null;
    activeFilter = 'attention';
    
    // Update UI
    document.getElementById('breadcrumbs').classList.remove('hidden');
    document.getElementById('crumb-server').textContent = 'Needs Attention';
    document.getElementById('page-title').textContent = 'Drives Needing Attention';
    
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    
    // Update nav
    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    
    // Filter to only show drives that need attention
    const filterFn = (drive) => getHealthStatus(drive) !== 'healthy';
    renderFilteredDrivesView(filterFn, 'attention');
}

function showHealthyDrives() {
    activeServerIndex = null;
    activeFilter = 'healthy';
    
    // Update UI
    document.getElementById('breadcrumbs').classList.remove('hidden');
    document.getElementById('crumb-server').textContent = 'Healthy Drives';
    document.getElementById('page-title').textContent = 'Healthy Drives';
    
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    
    // Update nav
    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    
    // Filter to only show healthy drives
    const filterFn = (drive) => getHealthStatus(drive) === 'healthy';
    renderFilteredDrivesView(filterFn, 'healthy');
}

function showAllDrives() {
    activeServerIndex = null;
    activeFilter = 'all';
    
    // Update UI
    document.getElementById('breadcrumbs').classList.remove('hidden');
    document.getElementById('crumb-server').textContent = 'All Drives';
    document.getElementById('page-title').textContent = 'All Drives';
    
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    
    // Update nav
    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.summary-card').forEach(el => el.classList.remove('active'));
    
    // Show all drives
    const filterFn = () => true;
    renderFilteredDrivesView(filterFn, 'all');
}

// Render drives grouped by server with card layout
function renderFilteredDrivesView(filterFn, filterType) {
    const container = document.getElementById('server-list');
    const summaryContainer = document.getElementById('summary-cards');
    
    // Calculate global stats
    let totalServers = globalData.length;
    let totalDrives = 0;
    let healthyDrives = 0;
    let attentionDrives = 0;
    
    globalData.forEach(server => {
        const drives = server.details?.drives || [];
        totalDrives += drives.length;
        drives.forEach(drive => {
            const status = getHealthStatus(drive);
            if (status === 'healthy') healthyDrives++;
            else attentionDrives++;
        });
    });
    
    // Render summary cards
    summaryContainer.innerHTML = `
        <div class="summary-card clickable" onclick="resetDashboard()" title="View all servers">
            <div class="icon blue">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="2" width="20" height="8" rx="2"/>
                    <rect x="2" y="14" width="20" height="8" rx="2"/>
                    <circle cx="6" cy="6" r="1"/>
                    <circle cx="6" cy="18" r="1"/>
                </svg>
            </div>
            <div class="value">${totalServers}</div>
            <div class="label">Servers</div>
        </div>
        <div class="summary-card clickable ${filterType === 'all' ? 'active' : ''}" onclick="showAllDrives()" title="View all drives">
            <div class="icon blue">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="4" width="20" height="16" rx="2"/>
                    <circle cx="8" cy="12" r="2"/>
                </svg>
            </div>
            <div class="value">${totalDrives}</div>
            <div class="label">Total Drives</div>
        </div>
        <div class="summary-card clickable ${filterType === 'healthy' ? 'active' : ''}" onclick="showHealthyDrives()" title="View healthy drives">
            <div class="icon green">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                    <polyline points="22 4 12 14.01 9 11.01"/>
                </svg>
            </div>
            <div class="value">${healthyDrives}</div>
            <div class="label">Healthy</div>
        </div>
        <div class="summary-card ${attentionDrives > 0 ? 'clickable' : ''} ${filterType === 'attention' ? 'active' : ''}" ${attentionDrives > 0 ? 'onclick="showNeedsAttention()"' : ''} title="${attentionDrives > 0 ? 'View drives needing attention' : 'All drives healthy'}">
            <div class="icon ${attentionDrives > 0 ? 'red' : 'green'}">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                    <line x1="12" y1="9" x2="12" y2="13"/>
                    <line x1="12" y1="17" x2="12.01" y2="17"/>
                </svg>
            </div>
            <div class="value">${attentionDrives}</div>
            <div class="label">Needs Attention</div>
        </div>
    `;
    
    // Build sections for each server that has matching drives
    let sectionsHtml = '';
    
    globalData.forEach((server, serverIdx) => {
        const drives = server.details?.drives || [];
        const filteredDrives = drives.map((drive, idx) => ({ ...drive, _idx: idx }))
                                     .filter(filterFn);
        
        if (filteredDrives.length === 0) return;
        
        // Server icon
        const serverIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon server"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1"/><circle cx="6" cy="18" r="1"/></svg>`;
        
        sectionsHtml += `
            <div class="drive-section">
                <div class="drive-section-header clickable" onclick="showServer(${serverIdx})">
                    <div class="drive-section-title">
                        ${serverIcon}
                        <span>${server.hostname}</span>
                        <span class="drive-section-count">${filteredDrives.length} drive${filteredDrives.length !== 1 ? 's' : ''}</span>
                    </div>
                    <div class="drive-section-arrow">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="9 18 15 12 9 6"/>
                        </svg>
                    </div>
                </div>
                <div class="drive-grid">
                    ${filteredDrives.map(drive => renderDriveCard(drive, serverIdx)).join('')}
                </div>
            </div>
        `;
    });
    
    // Handle empty state
    if (!sectionsHtml) {
        if (filterType === 'attention') {
            container.innerHTML = `
                <div class="empty-state success-state">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                        <polyline points="22 4 12 14.01 9 11.01"/>
                    </svg>
                    <p>All drives are healthy!</p>
                    <span class="hint">No drives currently need attention</span>
                </div>
            `;
        } else {
            container.innerHTML = `
                <div class="empty-state">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <rect x="2" y="4" width="20" height="16" rx="2"/>
                        <circle cx="8" cy="12" r="2"/>
                    </svg>
                    <p>No drives found</p>
                    <span class="hint">No drives match the current filter</span>
                </div>
            `;
        }
    } else {
        container.innerHTML = `<div class="server-detail-view">${sectionsHtml}</div>`;
    }
    
    container.style.display = 'block';
}

// Get drive display name from multiple possible fields
function getDriveName(drive) {
    // Check for user-defined alias first (future feature)
    if (drive._alias) return drive._alias;
    
    // Try various model name fields that smartctl might use
    if (drive.model_name && drive.model_name !== '') return drive.model_name;
    if (drive.scsi_model_name && drive.scsi_model_name !== '') return drive.scsi_model_name;
    if (drive.model_family && drive.model_family !== '') return drive.model_family;
    
    // Try device info
    if (drive.device?.model && drive.device.model !== '') return drive.device.model;
    if (drive.device?.name && drive.device.name !== '') {
        // Extract just the device name like "sda" from "/dev/sda"
        const name = drive.device.name.replace('/dev/', '');
        // If we have a serial, show both
        if (drive.serial_number) {
            return `${name} (${drive.serial_number.slice(-8)})`;
        }
        return name;
    }
    
    // Last resort - use serial number if available
    if (drive.serial_number && drive.serial_number !== '') {
        return `Drive ${drive.serial_number.slice(-8)}`;
    }
    
    return 'Unknown Drive';
}

// Render a single drive card (shared helper)
function renderDriveCard(drive, serverIdx) {
    const status = getHealthStatus(drive);
    const isNvme = drive.device?.type?.toLowerCase() === 'nvme' || drive.device?.protocol === 'NVMe';
    const isSsd = drive.rotation_rate === 0;
    const driveType = isNvme ? 'NVMe' : isSsd ? 'SSD' : drive.rotation_rate ? `${drive.rotation_rate} RPM` : 'HDD';
    const driveName = getDriveName(drive);
    const hostname = globalData[serverIdx]?.hostname || '';
    const serial = drive.serial_number || '';
    const alias = drive._alias || '';
    
    return `
        <div class="drive-card ${status}" onclick="showDriveDetails(${serverIdx}, ${drive._idx})">
            <div class="drive-card-header">
                <div class="drive-card-icon ${status}">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="2" y="4" width="20" height="16" rx="2"/>
                        <circle cx="8" cy="12" r="2"/>
                        <line x1="14" y1="9" x2="18" y2="9"/>
                        <line x1="14" y1="12" x2="18" y2="12"/>
                    </svg>
                </div>
                <button class="alias-btn" onclick="event.stopPropagation(); showAliasModal('${hostname}', '${serial}', '${alias.replace(/'/g, "\\'")}', '${driveName.replace(/'/g, "\\'")}')" title="Set alias">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                        <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                    </svg>
                </button>
                <span class="status-badge ${drive.smart_status?.passed ? 'passed' : 'failed'}">
                    ${drive.smart_status?.passed ? 'Passed' : 'Failed'}
                </span>
            </div>
            <div class="drive-card-body">
                <div class="drive-card-model">${driveName}</div>
                <div class="drive-card-serial">${drive.serial_number || 'N/A'}</div>
            </div>
            <div class="drive-card-stats">
                <div class="drive-card-stat">
                    <span class="stat-value">${formatSize(drive.user_capacity?.bytes)}</span>
                    <span class="stat-label">Capacity</span>
                </div>
                <div class="drive-card-stat">
                    <span class="stat-value">${drive.temperature?.current ?? '--'}°C</span>
                    <span class="stat-label">Temp</span>
                </div>
                <div class="drive-card-stat">
                    <span class="stat-value">${formatAge(drive.power_on_time?.hours)}</span>
                    <span class="stat-label">Age</span>
                </div>
            </div>
            <div class="drive-card-footer">
                <span class="drive-type-badge">${driveType}</span>
            </div>
        </div>
    `;
}

function showServer(serverIdx) {
    activeServerIndex = serverIdx;
    activeFilter = null;
    const server = globalData[serverIdx];
    
    if (!server) {
        resetDashboard();
        return;
    }
    
    // Update navigation
    document.getElementById('crumb-server').textContent = server.hostname;
    document.getElementById('breadcrumbs').classList.remove('hidden');
    document.getElementById('page-title').textContent = server.hostname;
    
    // Update sidebar nav
    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.server-nav-item').forEach((el, idx) => {
        el.classList.toggle('active', idx === serverIdx);
    });
    
    // Show filtered view
    document.getElementById('details-view').classList.add('hidden');
    document.getElementById('dashboard-view').classList.remove('hidden');
    
    renderServerDetailView(server, serverIdx);
}

function renderServerDetailView(server, serverIdx) {
    const container = document.getElementById('server-list');
    const summaryContainer = document.getElementById('summary-cards');
    const drives = server.details?.drives || [];
    
    // Categorize drives
    const ssdDrives = [];
    const hddDrives = [];
    const nvmeDrives = [];
    
    drives.forEach((drive, idx) => {
        const driveWithIndex = { ...drive, _idx: idx };
        const deviceType = drive.device?.type?.toLowerCase() || '';
        
        if (deviceType === 'nvme' || drive.device?.protocol === 'NVMe') {
            nvmeDrives.push(driveWithIndex);
        } else if (drive.rotation_rate === 0) {
            ssdDrives.push(driveWithIndex);
        } else {
            hddDrives.push(driveWithIndex);
        }
    });
    
    // Calculate stats
    let healthyCount = 0;
    let warningCount = 0;
    let criticalCount = 0;
    let totalCapacity = 0;
    let avgTemp = 0;
    let tempCount = 0;
    
    drives.forEach(drive => {
        const status = getHealthStatus(drive);
        if (status === 'healthy') healthyCount++;
        else if (status === 'warning') warningCount++;
        else criticalCount++;
        
        if (drive.user_capacity?.bytes) {
            totalCapacity += drive.user_capacity.bytes;
        }
        if (drive.temperature?.current) {
            avgTemp += drive.temperature.current;
            tempCount++;
        }
    });
    
    avgTemp = tempCount > 0 ? Math.round(avgTemp / tempCount) : 0;
    
    // Render summary cards for this server
    summaryContainer.innerHTML = `
        <div class="summary-card">
            <div class="icon blue">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="4" width="20" height="16" rx="2"/>
                    <circle cx="8" cy="12" r="2"/>
                </svg>
            </div>
            <div class="value">${drives.length}</div>
            <div class="label">Total Drives</div>
        </div>
        <div class="summary-card">
            <div class="icon purple">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
                </svg>
            </div>
            <div class="value">${formatSize(totalCapacity)}</div>
            <div class="label">Total Capacity</div>
        </div>
        <div class="summary-card">
            <div class="icon ${avgTemp > 50 ? 'red' : avgTemp > 40 ? 'yellow' : 'green'}">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M14 14.76V3.5a2.5 2.5 0 0 0-5 0v11.26a4.5 4.5 0 1 0 5 0z"/>
                </svg>
            </div>
            <div class="value">${avgTemp}°C</div>
            <div class="label">Avg Temperature</div>
        </div>
        <div class="summary-card">
            <div class="icon ${criticalCount > 0 ? 'red' : warningCount > 0 ? 'yellow' : 'green'}">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                    <polyline points="22 4 12 14.01 9 11.01"/>
                </svg>
            </div>
            <div class="value">${healthyCount}/${drives.length}</div>
            <div class="label">Healthy</div>
        </div>
    `;
    
    // Helper function to render a section
    const renderSection = (title, icon, drivesArray) => {
        if (drivesArray.length === 0) return '';
        
        return `
            <div class="drive-section">
                <div class="drive-section-header">
                    <div class="drive-section-title">
                        ${icon}
                        <span>${title}</span>
                        <span class="drive-section-count">${drivesArray.length}</span>
                    </div>
                </div>
                <div class="drive-grid">
                    ${drivesArray.map(d => renderDriveCard(d, serverIdx)).join('')}
                </div>
            </div>
        `;
    };
    
    // Icons for each section
    const nvmeIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon nvme"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>`;
    const ssdIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon ssd"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M6 8h4v8H6z"/><path d="M14 8h4"/><path d="M14 12h4"/><path d="M14 16h4"/></svg>`;
    const hddIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon hdd"><rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="12" r="3"/><line x1="14" y1="9" x2="18" y2="9"/><line x1="14" y1="12" x2="18" y2="12"/><line x1="14" y1="15" x2="18" y2="15"/></svg>`;
    
    // Render the container
    container.innerHTML = `
        <div class="server-detail-view">
            ${renderSection('NVMe Drives', nvmeIcon, nvmeDrives)}
            ${renderSection('Solid State Drives', ssdIcon, ssdDrives)}
            ${renderSection('Hard Disk Drives', hddIcon, hddDrives)}
        </div>
    `;
    
    container.style.display = 'block';
}

function showDriveDetails(serverIdx, driveIdx) {
    const server = globalData[serverIdx];
    const drive = server?.details?.drives?.[driveIdx];
    
    if (!drive) return;
    
    const status = getHealthStatus(drive);
    
    // Render sidebar
    const sidebar = document.getElementById('detail-sidebar');
    
    // Get rotation rate with proper formatting
    const getRotationDisplay = (rate) => {
        if (rate === 0) return { type: 'SSD', detail: 'Solid State Drive' };
        if (rate === undefined || rate === null) return { type: 'Unknown', detail: 'Not reported' };
        return { type: 'HDD', detail: `${rate} RPM` };
    };
    
    const rotationInfo = getRotationDisplay(drive.rotation_rate);
    
    sidebar.innerHTML = `
        <div class="drive-header">
            <div class="icon ${status}">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="4" width="20" height="16" rx="2"/>
                    <circle cx="8" cy="12" r="2"/>
                    <line x1="14" y1="9" x2="18" y2="9"/>
                    <line x1="14" y1="12" x2="18" y2="12"/>
                    <line x1="14" y1="15" x2="18" y2="15"/>
                </svg>
            </div>
            <h3>${getDriveName(drive)}</h3>
            <span class="serial">${drive.serial_number || 'N/A'}</span>
        </div>
        
        <div class="info-group">
            <div class="info-group-label">Device Information</div>
            <div class="info-row">
                <span class="label">Capacity</span>
                <span class="value">${formatSize(drive.user_capacity?.bytes)}</span>
            </div>
            <div class="info-row">
                <span class="label">Firmware</span>
                <span class="value">${drive.firmware_version || 'N/A'}</span>
            </div>
            <div class="info-row">
                <span class="label">Drive Type</span>
                <span class="value">${rotationInfo.type}</span>
            </div>
            <div class="info-row">
                <span class="label">Rotation Rate</span>
                <span class="value">${rotationInfo.detail}</span>
            </div>
            <div class="info-row">
                <span class="label">Interface</span>
                <span class="value">${drive.device?.protocol || 'ATA'}</span>
            </div>
        </div>
        
        <div class="info-group">
            <div class="info-group-label">Health Status</div>
            <div class="info-row">
                <span class="label">SMART Status</span>
                <span class="value ${drive.smart_status?.passed ? 'success' : 'danger'}">
                    ${drive.smart_status?.passed ? 'PASSED' : 'FAILED'}
                </span>
            </div>
            <div class="info-row">
                <span class="label">Temperature</span>
                <span class="value ${(drive.temperature?.current > 50) ? 'warning' : ''}">${drive.temperature?.current ?? 'N/A'}°C</span>
            </div>
            <div class="info-row">
                <span class="label">Powered On</span>
                <span class="value">${formatAge(drive.power_on_time?.hours)}</span>
            </div>
            <div class="info-row">
                <span class="label">Power Cycles</span>
                <span class="value">${drive.power_cycle_count ?? 'N/A'}</span>
            </div>
        </div>
    `;
    
    // Render SMART attributes table
    const table = document.getElementById('detail-table');
    const attributes = drive.ata_smart_attributes?.table || [];
    
    if (attributes.length === 0) {
        table.innerHTML = `
            <div class="nvme-notice">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <circle cx="12" cy="12" r="10"/>
                    <line x1="12" y1="8" x2="12" y2="12"/>
                    <line x1="12" y1="16" x2="12.01" y2="16"/>
                </svg>
                <p>No standard ATA SMART attributes available</p>
                <span>NVMe drives use different health reporting</span>
            </div>
        `;
    } else {
        const criticalIds = [5, 187, 197, 198];
        
        table.innerHTML = `
            <thead>
                <tr>
                    <th class="status-cell">Status</th>
                    <th>ID</th>
                    <th>Attribute</th>
                    <th>Value</th>
                    <th>Worst</th>
                    <th>Thresh</th>
                    <th>Raw</th>
                </tr>
            </thead>
            <tbody>
                ${attributes.map(attr => {
                    const isCritical = criticalIds.includes(attr.id);
                    const isFailing = (isCritical && attr.raw?.value > 0) || 
                                     (attr.thresh > 0 && attr.value <= attr.thresh);
                    
                    return `
                        <tr>
                            <td class="status-cell">
                                <span class="attr-pill ${isFailing ? 'fail' : 'ok'}">
                                    ${isFailing ? 'FAIL' : 'OK'}
                                </span>
                            </td>
                            <td>${attr.id}</td>
                            <td style="font-family: var(--font-sans)">${attr.name}</td>
                            <td>${attr.value}</td>
                            <td>${attr.worst ?? '-'}</td>
                            <td>${attr.thresh}</td>
                            <td>${attr.raw?.value ?? '-'}</td>
                        </tr>
                    `;
                }).join('')}
            </tbody>
        `;
    }
    
    // Show details view
    document.getElementById('dashboard-view').classList.add('hidden');
    document.getElementById('details-view').classList.remove('hidden');
}

function goBackToContext() {
    if (activeFilter === 'attention') {
        showNeedsAttention();
    } else if (activeServerIndex !== null) {
        showServer(activeServerIndex);
    } else {
        resetDashboard();
    }
}

// ============================================
// DATA FETCHING & RENDERING
// ============================================

async function fetchData() {
    try {
        const response = await fetch(API_URL);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        
        globalData = await response.json() || [];
        
        // Update UI based on current view
        if (!document.getElementById('dashboard-view').classList.contains('hidden')) {
            if (activeServerIndex !== null && globalData[activeServerIndex]) {
                // Server detail view - use new layout
                renderServerDetailView(globalData[activeServerIndex], activeServerIndex);
            } else if (activeFilter === 'attention') {
                // Re-apply attention filter with fresh data
                const filterFn = (drive) => getHealthStatus(drive) !== 'healthy';
                renderFilteredDrivesView(filterFn, 'attention');
            } else if (activeFilter === 'healthy') {
                // Re-apply healthy filter with fresh data
                const filterFn = (drive) => getHealthStatus(drive) === 'healthy';
                renderFilteredDrivesView(filterFn, 'healthy');
            } else if (activeFilter === 'all') {
                // Re-apply all drives filter with fresh data
                const filterFn = () => true;
                renderFilteredDrivesView(filterFn, 'all');
            } else {
                renderDashboard(globalData);
            }
        }
        
        updateSidebar();
        updateStats();
        setOnlineStatus(true);
        
        // Update last refresh time
        document.getElementById('last-update-time').textContent = 
            new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        
    } catch (error) {
        console.error('Fetch error:', error);
        setOnlineStatus(false);
    }
}

function setOnlineStatus(online) {
    const indicator = document.getElementById('status-indicator');
    indicator.classList.toggle('online', online);
    indicator.classList.toggle('offline', !online);
    indicator.title = online ? 'Connected' : 'Connection Lost';
}

function updateSidebar() {
    const serverNav = document.getElementById('server-nav-list');
    const serverCount = document.getElementById('server-count');
    
    serverCount.textContent = globalData.length;
    
    serverNav.innerHTML = globalData.map((server, idx) => {
        const drives = server.details?.drives || [];
        const hasWarning = drives.some(d => getHealthStatus(d) === 'warning');
        const hasCritical = drives.some(d => getHealthStatus(d) === 'critical');
        
        let statusClass = '';
        if (hasCritical) statusClass = 'critical';
        else if (hasWarning) statusClass = 'warning';
        
        return `
            <div class="server-nav-item ${activeServerIndex === idx ? 'active' : ''}" 
                 onclick="showServer(${idx})">
                <span class="status-indicator ${statusClass}"></span>
                ${server.hostname}
            </div>
        `;
    }).join('');
}

function updateStats() {
    let totalDrives = 0;
    let healthyDrives = 0;
    let warningDrives = 0;
    
    globalData.forEach(server => {
        const drives = server.details?.drives || [];
        totalDrives += drives.length;
        
        drives.forEach(drive => {
            const status = getHealthStatus(drive);
            if (status === 'healthy') healthyDrives++;
            else if (status === 'warning') warningDrives++;
        });
    });
    
    document.getElementById('total-drives').textContent = totalDrives;
    document.getElementById('healthy-count').textContent = healthyDrives;
    document.getElementById('warning-count').textContent = totalDrives - healthyDrives;
}

function renderDashboard(servers, isFiltered = false, isAttentionView = false) {
    const container = document.getElementById('server-list');
    const summaryContainer = document.getElementById('summary-cards');
    
    // Calculate summary stats from GLOBAL data (not filtered)
    let totalServers = globalData.length;
    let totalDrives = 0;
    let healthyDrives = 0;
    let attentionDrives = 0;
    
    globalData.forEach(server => {
        const drives = server.details?.drives || [];
        totalDrives += drives.length;
        drives.forEach(drive => {
            const status = getHealthStatus(drive);
            if (status === 'healthy') healthyDrives++;
            else attentionDrives++;
        });
    });
    
    // Render summary cards - all clickable
    const attentionCardClass = attentionDrives > 0 ? 'clickable' : '';
    const attentionCardClick = attentionDrives > 0 ? 'onclick="showNeedsAttention()"' : '';
    
    summaryContainer.innerHTML = `
        <div class="summary-card clickable active" onclick="resetDashboard()" title="View all servers">
            <div class="icon blue">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="2" width="20" height="8" rx="2"/>
                    <rect x="2" y="14" width="20" height="8" rx="2"/>
                    <circle cx="6" cy="6" r="1"/>
                    <circle cx="6" cy="18" r="1"/>
                </svg>
            </div>
            <div class="value">${totalServers}</div>
            <div class="label">Servers</div>
        </div>
        <div class="summary-card clickable" onclick="showAllDrives()" title="View all drives">
            <div class="icon blue">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="4" width="20" height="16" rx="2"/>
                    <circle cx="8" cy="12" r="2"/>
                </svg>
            </div>
            <div class="value">${totalDrives}</div>
            <div class="label">Total Drives</div>
        </div>
        <div class="summary-card clickable" onclick="showHealthyDrives()" title="View healthy drives">
            <div class="icon green">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                    <polyline points="22 4 12 14.01 9 11.01"/>
                </svg>
            </div>
            <div class="value">${healthyDrives}</div>
            <div class="label">Healthy</div>
        </div>
        <div class="summary-card ${attentionCardClass}" ${attentionCardClick} title="${attentionDrives > 0 ? 'Click to view drives needing attention' : 'All drives healthy'}">
            <div class="icon ${attentionDrives > 0 ? 'red' : 'green'}">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                    <line x1="12" y1="9" x2="12" y2="13"/>
                    <line x1="12" y1="17" x2="12.01" y2="17"/>
                </svg>
            </div>
            <div class="value">${attentionDrives}</div>
            <div class="label">Needs Attention</div>
        </div>
    `;
    
    // Render server sections with card grid
    if (!servers || servers.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <rect x="2" y="2" width="20" height="8" rx="2"/>
                    <rect x="2" y="14" width="20" height="8" rx="2"/>
                    <circle cx="6" cy="6" r="1"/>
                    <circle cx="6" cy="18" r="1"/>
                </svg>
                <p>Waiting for agents to connect...</p>
                <span class="hint">Run vigil-agent on your servers to begin monitoring</span>
            </div>
        `;
        return;
    }
    
    // Build sections for each server
    const serverIcon = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="section-icon server"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1"/><circle cx="6" cy="18" r="1"/></svg>`;
    
    let sectionsHtml = servers.map(server => {
        const serverIdx = globalData.findIndex(s => s.hostname === server.hostname);
        const drives = server.details?.drives || [];
        const drivesWithIndex = drives.map((drive, idx) => ({ ...drive, _idx: idx }));
        
        if (drives.length === 0) {
            return `
                <div class="drive-section">
                    <div class="drive-section-header clickable" onclick="showServer(${serverIdx})">
                        <div class="drive-section-title">
                            ${serverIcon}
                            <span>${server.hostname}</span>
                            <span class="drive-section-count">0 drives</span>
                        </div>
                        <div class="drive-section-meta">
                            <span class="timestamp">${formatTime(server.timestamp)}</span>
                        </div>
                        <div class="drive-section-arrow">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polyline points="9 18 15 12 9 6"/>
                            </svg>
                        </div>
                    </div>
                    <div class="drive-grid-empty">
                        <p>No drives detected</p>
                    </div>
                </div>
            `;
        }
        
        return `
            <div class="drive-section">
                <div class="drive-section-header clickable" onclick="showServer(${serverIdx})">
                    <div class="drive-section-title">
                        ${serverIcon}
                        <span>${server.hostname}</span>
                        <span class="drive-section-count">${drives.length} drive${drives.length !== 1 ? 's' : ''}</span>
                    </div>
                    <div class="drive-section-meta">
                        <span class="timestamp">${formatTime(server.timestamp)}</span>
                    </div>
                    <div class="drive-section-arrow">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="9 18 15 12 9 6"/>
                        </svg>
                    </div>
                </div>
                <div class="drive-grid">
                    ${drivesWithIndex.map(drive => renderDriveCard(drive, serverIdx)).join('')}
                </div>
            </div>
        `;
    }).join('');
    
    container.innerHTML = `<div class="server-detail-view">${sectionsHtml}</div>`;
    container.style.display = 'block';
}

// ============================================
// AUTHENTICATION
// ============================================

let currentUser = null;
let mustChangePassword = false;

async function checkAuthStatus() {
    try {
        const response = await fetch('/api/auth/status');
        const data = await response.json();
        
        if (data.auth_enabled && !data.authenticated) {
            window.location.href = '/login.html';
            return false;
        }
        
        currentUser = data.username || null;
        mustChangePassword = data.must_change_password || false;
        updateUserUI();
        
        // Force password change modal if required
        if (mustChangePassword) {
            setTimeout(() => showForcePasswordChange(), 500);
        }
        
        return true;
    } catch (e) {
        console.error('Auth check failed:', e);
        return true; // Continue anyway
    }
}

function updateUserUI() {
    const userMenuEl = document.getElementById('user-menu');
    if (!userMenuEl) return;
    
    if (currentUser) {
        userMenuEl.innerHTML = `
            <div class="user-dropdown">
                <button class="user-btn" onclick="toggleUserMenu()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                        <circle cx="12" cy="7" r="4"/>
                    </svg>
                    <span>${currentUser}</span>
                    <svg class="chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="6 9 12 15 18 9"/>
                    </svg>
                </button>
                <div class="dropdown-menu" id="dropdown-menu">
                    <button onclick="showSettings()">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="3"/>
                            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
                        </svg>
                        Settings
                    </button>
                    <button onclick="logout()">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
                            <polyline points="16 17 21 12 16 7"/>
                            <line x1="21" y1="12" x2="9" y2="12"/>
                        </svg>
                        Sign Out
                    </button>
                </div>
            </div>
        `;
    } else {
        userMenuEl.innerHTML = '';
    }
}

function toggleUserMenu() {
    const menu = document.getElementById('dropdown-menu');
    if (menu) {
        menu.classList.toggle('show');
    }
}

// Close dropdown when clicking outside
document.addEventListener('click', (e) => {
    if (!e.target.closest('.user-dropdown')) {
        const menu = document.getElementById('dropdown-menu');
        if (menu) menu.classList.remove('show');
    }
});

async function logout() {
    try {
        await fetch('/api/auth/logout', { method: 'POST' });
    } catch (e) {
        console.error('Logout error:', e);
    }
    window.location.href = '/login.html';
}

// Settings Page
function showSettings() {
    // Hide dashboard, show settings view
    document.getElementById('dashboard-view').classList.add('hidden');
    document.getElementById('details-view').classList.add('hidden');
    
    // Update breadcrumbs
    document.getElementById('breadcrumbs').classList.remove('hidden');
    document.getElementById('crumb-server').textContent = 'Settings';
    document.getElementById('page-title').textContent = 'Settings';
    
    // Clear server selection
    document.querySelectorAll('.server-nav-item').forEach(el => el.classList.remove('active'));
    
    // Create settings view if not exists
    let settingsView = document.getElementById('settings-view');
    if (!settingsView) {
        settingsView = document.createElement('div');
        settingsView.id = 'settings-view';
        settingsView.className = 'view settings-view';
        document.querySelector('.main-content').appendChild(settingsView);
    }
    
    settingsView.innerHTML = `
        <div class="settings-container">
            <div class="settings-section">
                <div class="settings-section-header">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                        <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                    </svg>
                    <h3>Security</h3>
                </div>
                <div class="settings-card">
                    <div class="settings-item">
                        <div class="settings-item-info">
                            <div class="settings-item-title">Change Password</div>
                            <div class="settings-item-desc">Update your account password</div>
                        </div>
                        <button class="btn btn-secondary" onclick="showChangePasswordModal()">Change</button>
                    </div>
                </div>
            </div>
            
            <div class="settings-section">
                <div class="settings-section-header">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                        <circle cx="12" cy="7" r="4"/>
                    </svg>
                    <h3>Account</h3>
                </div>
                <div class="settings-card">
                    <div class="settings-item">
                        <div class="settings-item-info">
                            <div class="settings-item-title">Username</div>
                            <div class="settings-item-desc">${currentUser}</div>
                        </div>
                         <button class="btn btn-secondary" onclick="showChangeUsernameModal()">Change</button>
                    </div>
                    <div class="settings-item">
                        <div class="settings-item-info">
                            <div class="settings-item-title">Sign Out</div>
                            <div class="settings-item-desc">Log out of your account</div>
                        </div>
                        <button class="btn btn-danger" onclick="logout()">Sign Out</button>
                    </div>
                </div>
            </div>
            
            <div class="settings-section">
                <div class="settings-section-header">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"/>
                        <line x1="12" y1="16" x2="12" y2="12"/>
                        <line x1="12" y1="8" x2="12.01" y2="8"/>
                    </svg>
                    <h3>About</h3>
                </div>
                <div class="settings-card">
                    <div class="settings-item">
                        <div class="settings-item-info">
                            <div class="settings-item-title">Version</div>
                            <div class="settings-item-desc" id="settings-version">Loading...</div>
                        </div>
                    </div>
                    <div class="settings-item">
                        <div class="settings-item-info">
                            <div class="settings-item-title">GitHub</div>
                            <div class="settings-item-desc">
                                <a href="https://github.com/pineappledr/vigil" target="_blank">github.com/pineappledr/vigil</a>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `;
    
    settingsView.classList.remove('hidden');
    
    // Load version
    fetch('/api/version').then(r => r.json()).then(data => {
        document.getElementById('settings-version').textContent = data.version || 'Unknown';
    });
}

function showChangePasswordModal() {
    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.innerHTML = `
        <div class="modal">
            <div class="modal-header">
                <h3>Change Password</h3>
                <button class="modal-close" onclick="this.closest('.modal-overlay').remove()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="18" y1="6" x2="6" y2="18"/>
                        <line x1="6" y1="6" x2="18" y2="18"/>
                    </svg>
                </button>
            </div>
            <div class="modal-body">
                <div class="form-group">
                    <label>Current Password</label>
                    <input type="password" id="current-password" class="form-input">
                </div>
                <div class="form-group">
                    <label>New Password</label>
                    <input type="password" id="new-password" class="form-input">
                </div>
                <div class="form-group">
                    <label>Confirm New Password</label>
                    <input type="password" id="confirm-password" class="form-input">
                </div>
                <div id="password-error" class="form-error"></div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" onclick="submitPasswordChange()">Change Password</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    document.getElementById('current-password').focus();
}

// Force password change on first login
function showForcePasswordChange() {
    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.id = 'force-password-modal';
    modal.innerHTML = `
        <div class="modal">
            <div class="modal-header">
                <h3>🔐 Password Change Required</h3>
            </div>
            <div class="modal-body">
                <p class="modal-message">For security, you must change your password before continuing.</p>
                <div class="form-group">
                    <label>Current Password</label>
                    <input type="password" id="force-current-password" class="form-input">
                </div>
                <div class="form-group">
                    <label>New Password</label>
                    <input type="password" id="force-new-password" class="form-input">
                </div>
                <div class="form-group">
                    <label>Confirm New Password</label>
                    <input type="password" id="force-confirm-password" class="form-input">
                </div>
                <div id="force-password-error" class="form-error"></div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-primary" onclick="submitForcePasswordChange()">Set New Password</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    document.getElementById('force-current-password').focus();
}

async function submitForcePasswordChange() {
    const currentPassword = document.getElementById('force-current-password').value;
    const newPassword = document.getElementById('force-new-password').value;
    const confirmPassword = document.getElementById('force-confirm-password').value;
    const errorEl = document.getElementById('force-password-error');
    
    if (!currentPassword) {
        errorEl.textContent = 'Please enter your current password';
        return;
    }
    
    if (newPassword !== confirmPassword) {
        errorEl.textContent = 'New passwords do not match';
        return;
    }
    
    if (newPassword.length < 6) {
        errorEl.textContent = 'Password must be at least 6 characters';
        return;
    }
    
    if (currentPassword === newPassword) {
        errorEl.textContent = 'New password must be different from current password';
        return;
    }
    
    try {
        const response = await fetch('/api/users/password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                current_password: currentPassword,
                new_password: newPassword
            })
        });
        
        const data = await response.json();
        
        if (response.ok) {
            document.getElementById('force-password-modal').remove();
            mustChangePassword = false;
            alert('Password changed successfully! Welcome to Vigil.');
        } else {
            errorEl.textContent = data.error || 'Failed to change password';
        }
    } catch (e) {
        errorEl.textContent = 'Connection error';
    }
}

function showChangePassword() {
    showChangePasswordModal();
}

async function submitPasswordChange() {
    const currentPassword = document.getElementById('current-password').value;
    const newPassword = document.getElementById('new-password').value;
    const confirmPassword = document.getElementById('confirm-password').value;
    const errorEl = document.getElementById('password-error');
    
    if (newPassword !== confirmPassword) {
        errorEl.textContent = 'New passwords do not match';
        return;
    }
    
    if (newPassword.length < 6) {
        errorEl.textContent = 'Password must be at least 6 characters';
        return;
    }
    
    try {
        const response = await fetch('/api/users/password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                current_password: currentPassword,
                new_password: newPassword
            })
        });
        
        const data = await response.json();
        
        if (response.ok) {
            document.querySelector('.modal-overlay').remove();
            alert('Password changed successfully');
        } else {
            errorEl.textContent = data.error || 'Failed to change password';
        }
    } catch (e) {
        errorEl.textContent = 'Connection error';
    }
}

// [ADDED FUNCTION] Show Change Username Modal
function showChangeUsernameModal() {
    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.innerHTML = `
        <div class="modal">
            <div class="modal-header">
                <h3>Change Username</h3>
                <button class="modal-close" onclick="this.closest('.modal-overlay').remove()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="18" y1="6" x2="6" y2="18"/>
                        <line x1="6" y1="6" x2="18" y2="18"/>
                    </svg>
                </button>
            </div>
            <div class="modal-body">
                <p class="form-hint" style="margin-bottom: 16px;">
                    Changing your username will update it immediately. You will need to use the new username for future logins.
                </p>
                <div class="form-group">
                    <label>New Username</label>
                    <input type="text" id="new-username" class="form-input" placeholder="Enter new username">
                </div>
                <div class="form-group">
                    <label>Current Password</label>
                    <input type="password" id="current-password-for-user" class="form-input" placeholder="Verify your password">
                </div>
                <div id="username-error" class="form-error"></div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" onclick="submitUsernameChange()">Save Changes</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    document.getElementById('new-username').focus();
}

// [ADDED FUNCTION] Submit Username Change
async function submitUsernameChange() {
    const newUsername = document.getElementById('new-username').value.trim();
    const currentPassword = document.getElementById('current-password-for-user').value;
    const errorEl = document.getElementById('username-error');

    if (!newUsername) {
        errorEl.textContent = 'Username cannot be empty';
        return;
    }

    if (!currentPassword) {
        errorEl.textContent = 'Please enter your password to confirm';
        return;
    }

    try {
        const response = await fetch('/api/users/username', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                new_username: newUsername,
                current_password: currentPassword
            })
        });

        const data = await response.json();

        if (response.ok) {
            document.querySelector('.modal-overlay').remove();
            // Update global user state
            currentUser = newUsername;
            // Refresh settings view to show new name
            showSettings();
            // Update sidebar user menu
            updateUserUI();
            alert(`Username successfully changed to ${newUsername}`);
        } else {
            errorEl.textContent = data.error || 'Failed to change username';
        }
    } catch (e) {
        errorEl.textContent = 'Connection error';
    }
}

// ============================================
// DRIVE ALIASES
// ============================================

function showAliasModal(hostname, serialNumber, currentAlias, driveName) {
    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.innerHTML = `
        <div class="modal">
            <div class="modal-header">
                <h3>Set Drive Alias</h3>
                <button class="modal-close" onclick="this.closest('.modal-overlay').remove()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="18" y1="6" x2="6" y2="18"/>
                        <line x1="6" y1="6" x2="18" y2="18"/>
                    </svg>
                </button>
            </div>
            <div class="modal-body">
                <div class="alias-drive-info">
                    <div><strong>Drive:</strong> ${driveName}</div>
                    <div><strong>Serial:</strong> ${serialNumber}</div>
                    <div><strong>Server:</strong> ${hostname}</div>
                </div>
                <div class="form-group">
                    <label>Alias (friendly name)</label>
                    <input type="text" id="alias-input" class="form-input" 
                           value="${currentAlias || ''}" 
                           placeholder="e.g., Plex Media, VM Storage, Backup Drive">
                </div>
                <p class="form-hint">Leave empty to remove the alias</p>
                <div id="alias-error" class="form-error"></div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" onclick="submitAlias('${hostname}', '${serialNumber}')">Save Alias</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    document.getElementById('alias-input').focus();
}

async function submitAlias(hostname, serialNumber) {
    const alias = document.getElementById('alias-input').value.trim();
    const errorEl = document.getElementById('alias-error');
    
    try {
        const response = await fetch('/api/aliases', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                hostname: hostname,
                serial_number: serialNumber,
                alias: alias
            })
        });
        
        const data = await response.json();
        
        if (response.ok) {
            document.querySelector('.modal-overlay').remove();
            // Refresh data to show new alias
            fetchData();
        } else {
            errorEl.textContent = data.error || 'Failed to save alias';
        }
    } catch (e) {
        errorEl.textContent = 'Connection error';
    }
}

// ============================================
// INITIALIZATION
// ============================================

document.addEventListener('DOMContentLoaded', async () => {
    // Check auth first
    const isAuth = await checkAuthStatus();
    if (!isAuth) return;
    
    fetchVersion();
    fetchData();
    refreshTimer = setInterval(fetchData, REFRESH_INTERVAL);
});

// Fetch and display server version
async function fetchVersion() {
    try {
        const resp = await fetch('/api/version');
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

// Clean up on page unload
window.addEventListener('beforeunload', () => {
    if (refreshTimer) {
        clearInterval(refreshTimer);
    }
});