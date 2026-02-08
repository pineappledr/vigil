/**
 * Vigil Dashboard - SMART Attributes Module
 * Phase 1.3: Enhanced S.M.A.R.T. Monitoring UI with NVMe Health
 */

const SmartAttributes = {
    // Current state
    currentDrive: null,
    currentHostname: null,
    attributeFilter: 'all',
    detailPanelOpen: false,

    // Critical attribute IDs for ATA drives
    criticalATAIds: [5, 10, 187, 188, 196, 197, 198, 181, 182, 183, 184, 199, 232, 233],
    
    // NVMe attribute pseudo-IDs
    nvmeAttrIds: {
        temperature: 194,
        availableSpare: 232,
        percentageUsed: 233,
        dataUnitsWritten: 241,
        dataUnitsRead: 242,
        powerCycles: 12,
        powerOnHours: 9,
        mediaErrors: 187,
        criticalWarning: 1,
        unsafeShutdowns: 174,
        controllerBusyTime: 175,
        hostReads: 176,
        hostWrites: 178
    },

    // SMART Attribute explanations for tooltips
    attributeExplanations: {
        1: { name: 'Raw Read Error Rate', desc: 'Rate of hardware read errors. Higher raw values may indicate disk surface or read/write head issues.', critical: false },
        2: { name: 'Throughput Performance', desc: 'Overall throughput performance of the drive. Lower values indicate degraded performance.', critical: false },
        3: { name: 'Spin-Up Time', desc: 'Average time (ms) for the spindle to spin up. Increasing values may indicate motor issues.', critical: false },
        4: { name: 'Start/Stop Count', desc: 'Total count of spindle start/stop cycles. High counts indicate heavy usage.', critical: false },
        5: { name: 'Reallocated Sectors Count', desc: '⚠️ CRITICAL: Count of bad sectors that have been remapped. Non-zero values indicate drive degradation.', critical: true },
        7: { name: 'Seek Error Rate', desc: 'Rate of seek errors by the magnetic heads. High values may indicate mechanical issues.', critical: false },
        8: { name: 'Seek Time Performance', desc: 'Average performance of seek operations. Degradation indicates mechanical wear.', critical: false },
        9: { name: 'Power-On Hours', desc: 'Total hours the drive has been powered on. Used to estimate drive age and remaining lifespan.', critical: false },
        10: { name: 'Spin Retry Count', desc: '⚠️ CRITICAL: Count of retry attempts to spin up. Non-zero values indicate motor or power issues.', critical: true },
        11: { name: 'Calibration Retry Count', desc: 'Count of attempts to calibrate the drive. Increasing values may indicate mechanical problems.', critical: false },
        12: { name: 'Power Cycle Count', desc: 'Total number of complete power on/off cycles. High counts indicate frequent use.', critical: false },
        183: { name: 'SATA Downshift Error Count', desc: 'Count of errors resulting in SATA speed downgrade. May indicate cable or interface issues.', critical: false },
        184: { name: 'End-to-End Error', desc: '⚠️ CRITICAL: Parity errors in data path. Non-zero indicates potential data corruption.', critical: true },
        187: { name: 'Reported Uncorrectable Errors', desc: '⚠️ CRITICAL: Errors that could not be recovered using ECC. Indicates serious media problems.', critical: true },
        188: { name: 'Command Timeout', desc: '⚠️ CRITICAL: Count of aborted operations due to timeout. May indicate failing drive or connection.', critical: true },
        190: { name: 'Airflow Temperature', desc: 'Drive temperature from airflow sensor. Used for thermal monitoring.', critical: false },
        191: { name: 'G-Sense Error Rate', desc: 'Count of errors due to shock/vibration. High values indicate physical stress on the drive.', critical: false },
        192: { name: 'Power-Off Retract Count', desc: 'Count of emergency head retracts due to power loss. High values may indicate power issues.', critical: false },
        193: { name: 'Load Cycle Count', desc: 'Count of load/unload cycles of the heads. Each drive has a rated maximum.', critical: false },
        194: { name: 'Temperature Celsius', desc: 'Current internal drive temperature. Operating range typically 0-60°C.', critical: false },
        195: { name: 'Hardware ECC Recovered', desc: 'Count of errors corrected by hardware ECC. Increasing values may indicate media wear.', critical: false },
        196: { name: 'Reallocated Event Count', desc: '⚠️ CRITICAL: Count of remap operations. Non-zero indicates sectors have been reallocated.', critical: true },
        197: { name: 'Current Pending Sector Count', desc: '⚠️ CRITICAL: Sectors waiting to be remapped. These may cause read errors until resolved.', critical: true },
        198: { name: 'Offline Uncorrectable', desc: '⚠️ CRITICAL: Sectors that cannot be read or written. Data loss has likely occurred.', critical: true },
        199: { name: 'UDMA CRC Error Count', desc: 'Count of data transfer CRC errors. Often caused by bad cables or connections.', critical: false },
        200: { name: 'Multi-Zone Error Rate', desc: 'Rate of errors found when writing to sectors. May indicate surface degradation.', critical: false },
        220: { name: 'Disk Shift', desc: 'Distance the disk has shifted from its original position. Indicates mechanical issues.', critical: false },
        222: { name: 'Loaded Hours', desc: 'Time spent with heads loaded (spinning). Subset of Power-On Hours.', critical: false },
        223: { name: 'Load Retry Count', desc: 'Count of load retries. Increasing values indicate head loading mechanism issues.', critical: false },
        224: { name: 'Load Friction', desc: 'Resistance encountered when loading heads. Indicates mechanical wear.', critical: false },
        226: { name: 'Load-in Time', desc: 'Total time spent loading the heads. Used for wear analysis.', critical: false },
        240: { name: 'Head Flying Hours', desc: 'Time heads have spent over the platters. Key wear indicator for HDDs.', critical: false },
        241: { name: 'Total LBAs Written', desc: 'Total data written to the drive in logical blocks. Used to calculate total bytes written.', critical: false },
        242: { name: 'Total LBAs Read', desc: 'Total data read from the drive in logical blocks. Indicates overall read activity.', critical: false },
        // SSD specific
        170: { name: 'Available Reserved Space', desc: 'Percentage of reserved blocks remaining for wear leveling.', critical: false },
        171: { name: 'SSD Program Fail Count', desc: 'Count of flash program failures. Indicates NAND wear.', critical: false },
        172: { name: 'SSD Erase Fail Count', desc: 'Count of flash erase failures. Indicates NAND degradation.', critical: false },
        173: { name: 'SSD Wear Leveling Count', desc: 'Count of wear leveling operations. Higher is better for even wear.', critical: false },
        174: { name: 'Unexpected Power Loss', desc: 'Count of unexpected power losses. May affect data integrity.', critical: false },
        175: { name: 'Power Loss Protection Failure', desc: 'Count of failures in power loss protection circuits.', critical: false },
        176: { name: 'Erase Fail Count', desc: 'Total count of erase operation failures.', critical: false },
        177: { name: 'Wear Range Delta', desc: 'Difference between most and least worn blocks. Lower is better.', critical: false },
        181: { name: 'Program Fail Count Total', desc: '⚠️ CRITICAL: Total program failures across all flash. Indicates NAND wear.', critical: true },
        182: { name: 'Erase Fail Count Total', desc: '⚠️ CRITICAL: Total erase failures across all flash. Indicates NAND degradation.', critical: true },
        231: { name: 'SSD Life Left', desc: 'Estimated remaining drive life as percentage. Plan replacement when low.', critical: false },
        232: { name: 'Available Reserved Space', desc: 'SSD/NVMe: Percentage of spare blocks remaining for bad block replacement.', critical: false },
        233: { name: 'Media Wearout Indicator', desc: 'SSD/NVMe: Percentage of rated write endurance used. 100 means end of rated life.', critical: false },
        234: { name: 'Thermal Throttle Status', desc: 'Indicates if drive is throttling due to temperature.', critical: false },
        235: { name: 'Good Block Count', desc: 'Number of good/usable NAND blocks remaining.', critical: false },
        241: { name: 'Total Host Writes', desc: 'Total data written by host system. Key metric for SSD/NVMe wear.', critical: false },
        242: { name: 'Total Host Reads', desc: 'Total data read by host system.', critical: false },
        249: { name: 'NAND Writes', desc: 'Total data written to NAND (including write amplification).', critical: false }
    },

    // Info icon SVG
    infoIcon: `<svg class="info-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>`,

    // Icons for different attribute types
    icons: {
        health: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>`,
        temp: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 14.76V3.5a2.5 2.5 0 0 0-5 0v11.26a4.5 4.5 0 1 0 5 0z"/></svg>`,
        storage: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>`,
        power: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18.36 6.64a9 9 0 1 1-12.73 0"/><line x1="12" y1="2" x2="12" y2="12"/></svg>`,
        error: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`,
        clock: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>`,
        activity: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>`,
        check: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>`,
        warning: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
        alert: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`,
        nvme: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>`,
        close: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`,
        table: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/></svg>`,
        trendUp: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/><polyline points="17 6 23 6 23 12"/></svg>`,
        trendDown: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 18 13.5 8.5 8.5 13.5 1 6"/><polyline points="17 18 23 18 23 12"/></svg>`,
        minus: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/></svg>`
    },

    async init(hostname, serialNumber, driveData) {
        this.currentHostname = hostname;
        this.currentDrive = driveData;
        this.currentSerial = serialNumber;
        this.attributeFilter = 'all';
        const isNvme = this.isNvmeDrive(driveData);
        await this.render(hostname, serialNumber, isNvme);
    },

    isNvmeDrive(drive) {
        if (!drive) return false;
        const deviceType = drive.device?.type?.toLowerCase() || '';
        const protocol = drive.device?.protocol || '';
        // Also check for nvme_smart_health_information_log
        const hasNvmeLog = !!drive.nvme_smart_health_information_log;
        return deviceType === 'nvme' || protocol === 'NVMe' || hasNvmeLog;
    },

    async render(hostname, serialNumber, isNvme) {
        const container = document.getElementById('smart-view-container');
        if (!container) return;

        // If we have local drive data, render immediately without waiting for API
        if (this.currentDrive) {
            const localHealth = this.buildHealthFromDrive(this.currentDrive);
            const localAttrs = this.buildAttrsFromDrive(this.currentDrive);
            
            let html = '';
            html += this.renderTabs(isNvme);
            html += '<div id="smart-tab-contents">';
            html += `<div id="tab-health" class="smart-tab-content active">`;
            html += this.renderHealthSummary(localHealth);
            if (localHealth?.issues?.length > 0) {
                html += this.renderIssuesList(localHealth.issues);
            }
            html += `</div>`;

            if (isNvme) {
                html += `<div id="tab-nvme" class="smart-tab-content">`;
                html += this.renderNvmeHealth(localAttrs, this.currentDrive);
                html += `</div>`;
            }

            html += `<div id="tab-attributes" class="smart-tab-content">`;
            html += this.renderAttributesTable(localAttrs?.attributes || [], isNvme);
            html += `</div>`;

            html += `<div id="tab-temperature" class="smart-tab-content">`;
            html += this.renderTemperatureChart(null); // Will be updated async
            html += `</div>`;

            html += '</div>';
            container.innerHTML = html;
            this.initTabs();
            
            // Fetch temperature history in background and update chart
            this.fetchTemperatureHistory(hostname, serialNumber, 24).then(tempData => {
                if (tempData) {
                    const tempContainer = document.getElementById('tab-temperature');
                    if (tempContainer) {
                        tempContainer.innerHTML = this.renderTemperatureChart(tempData);
                    }
                }
            }).catch(err => console.error('Error fetching temperature:', err));
            
            return;
        }

        // Fallback: No local data, show loading and fetch from API
        container.innerHTML = this.renderLoading();

        try {
            // Fetch all API calls in parallel for faster loading
            const [healthData, attrData, tempData] = await Promise.all([
                this.fetchHealthSummary(hostname, serialNumber),
                this.fetchAttributes(hostname, serialNumber),
                this.fetchTemperatureHistory(hostname, serialNumber, 24)
            ]);

            let finalHealthData = healthData;
            let finalAttrData = attrData;

            // Use API data or empty defaults
            if (!finalHealthData || Object.keys(finalHealthData).length === 0) {
                finalHealthData = { overall_health: 'Unknown', smart_passed: true, critical_count: 0, warning_count: 0 };
            }
            if (!finalAttrData || !finalAttrData.attributes) {
                finalAttrData = { attributes: [] };
            }

            let html = '';
            html += this.renderTabs(isNvme);
            html += '<div id="smart-tab-contents">';
            html += `<div id="tab-health" class="smart-tab-content active">`;
            html += this.renderHealthSummary(finalHealthData);
            if (finalHealthData?.issues?.length > 0) {
                html += this.renderIssuesList(finalHealthData.issues);
            }
            html += `</div>`;

            if (isNvme) {
                html += `<div id="tab-nvme" class="smart-tab-content">`;
                html += this.renderNvmeHealth(finalAttrData, this.currentDrive);
                html += `</div>`;
            }

            html += `<div id="tab-attributes" class="smart-tab-content">`;
            html += this.renderAttributesTable(finalAttrData?.attributes || [], isNvme);
            html += `</div>`;

            html += `<div id="tab-temperature" class="smart-tab-content">`;
            html += this.renderTemperatureChart(tempData);
            html += `</div>`;

            html += '</div>';
            container.innerHTML = html;
            this.initTabs();
        } catch (error) {
            console.error('Error rendering SMART view:', error);
            container.innerHTML = this.renderError('Failed to load SMART data');
        }
    },

    // Build health summary from local drive data
    buildHealthFromDrive(drive) {
        const smartPassed = drive.smart_status?.passed !== false;
        const issues = [];
        
        // Check ATA attributes for issues
        const attrs = drive.ata_smart_attributes?.table || [];
        
        // Critical IDs where ANY non-zero raw value indicates a problem
        const criticalIds = [5, 10, 181, 182, 183, 184, 187, 188, 196, 197, 198];
        // Warning IDs (high values are concerning but not critical)
        const warningIds = [199]; // CRC errors - often cable issues
        
        attrs.forEach(attr => {
            const rawValue = attr.raw?.value || 0;
            const value = attr.value || 0;
            const thresh = attr.thresh || 0;
            
            // Check if normalized value hit threshold (SMART failure imminent)
            if (thresh > 0 && value > 0 && value <= thresh) {
                issues.push({
                    attribute_id: attr.id,
                    attribute_name: attr.name,
                    severity: 'CRITICAL',
                    message: `${attr.name} reached failure threshold (value: ${value}, threshold: ${thresh})`
                });
                return;
            }
            
            // Check critical IDs where raw > 0 is bad
            if (criticalIds.includes(attr.id) && rawValue > 0) {
                issues.push({
                    attribute_id: attr.id,
                    attribute_name: attr.name,
                    severity: 'CRITICAL',
                    message: `${attr.name} has non-zero value: ${rawValue}`
                });
                return;
            }
            
            // Check warning IDs
            if (warningIds.includes(attr.id) && rawValue > 0) {
                issues.push({
                    attribute_id: attr.id,
                    attribute_name: attr.name,
                    severity: 'WARNING',
                    message: `${attr.name}: ${rawValue} (check cables)`
                });
                return;
            }
            
            // Check temperature (ID 194, 190)
            if ((attr.id === 194 || attr.id === 190) && rawValue > 0) {
                if (rawValue > 65) {
                    issues.push({
                        attribute_id: attr.id,
                        attribute_name: attr.name,
                        severity: 'CRITICAL',
                        message: `Temperature critical: ${rawValue}°C`
                    });
                } else if (rawValue > 55) {
                    issues.push({
                        attribute_id: attr.id,
                        attribute_name: attr.name,
                        severity: 'WARNING',
                        message: `Temperature warning: ${rawValue}°C`
                    });
                }
            }
        });

        // Calculate counts
        const criticalCount = issues.filter(i => i.severity === 'CRITICAL').length;
        const warningCount = issues.filter(i => i.severity === 'WARNING').length;
        
        // Determine overall health - Critical takes precedence
        let overallHealth = 'Healthy';
        if (!smartPassed || criticalCount > 0) {
            overallHealth = 'Critical';
        } else if (warningCount > 0) {
            overallHealth = 'Warning';
        }

        return {
            overall_health: overallHealth,
            smart_passed: smartPassed,
            critical_count: criticalCount,
            warning_count: warningCount,
            model_name: drive.model_name || drive.scsi_model_name || 'Unknown',
            drive_type: Utils.getDriveType(drive),
            issues: issues
        };
    },

    // Build attributes from local drive data
    buildAttrsFromDrive(drive) {
        const attrs = [];
        
        // ATA SMART attributes (primary source)
        if (drive.ata_smart_attributes?.table) {
            drive.ata_smart_attributes.table.forEach(attr => {
                attrs.push({
                    id: attr.id,
                    name: attr.name,
                    value: attr.value,
                    worst: attr.worst,
                    threshold: attr.thresh,
                    raw_value: attr.raw?.value,
                    flags: attr.flags?.string
                });
            });
        }
        
        // Fallback: ATA device statistics (alternative source)
        if (attrs.length === 0 && drive.ata_device_statistics?.pages) {
            drive.ata_device_statistics.pages.forEach(page => {
                if (page.table) {
                    page.table.forEach(stat => {
                        if (stat.name && stat.value !== undefined) {
                            attrs.push({
                                id: stat.offset || attrs.length + 1,
                                name: stat.name.replace(/_/g, ' '),
                                value: stat.flags?.normalized ? stat.value : null,
                                worst: null,
                                threshold: null,
                                raw_value: stat.value,
                                flags: stat.flags?.string
                            });
                        }
                    });
                }
            });
        }
        
        // NVMe attributes from nvme_smart_health_information_log
        if (drive.nvme_smart_health_information_log) {
            const log = drive.nvme_smart_health_information_log;
            if (log.temperature !== undefined) {
                attrs.push({ id: 194, name: 'Temperature', raw_value: log.temperature });
            }
            if (log.available_spare !== undefined) {
                attrs.push({ id: 232, name: 'Available Spare', raw_value: log.available_spare, value: log.available_spare });
            }
            if (log.percentage_used !== undefined) {
                attrs.push({ id: 233, name: 'Percentage Used', raw_value: log.percentage_used, value: 100 - log.percentage_used });
            }
            if (log.power_on_hours !== undefined) {
                attrs.push({ id: 9, name: 'Power On Hours', raw_value: log.power_on_hours });
            }
            if (log.power_cycles !== undefined) {
                attrs.push({ id: 12, name: 'Power Cycles', raw_value: log.power_cycles });
            }
            if (log.media_errors !== undefined) {
                attrs.push({ id: 187, name: 'Media Errors', raw_value: log.media_errors });
            }
            if (log.unsafe_shutdowns !== undefined) {
                attrs.push({ id: 174, name: 'Unsafe Shutdowns', raw_value: log.unsafe_shutdowns });
            }
        }
        
        return { attributes: attrs };
    },

    // Render from local data when API fails
    renderFromLocalData(container, isNvme) {
        const healthData = this.buildHealthFromDrive(this.currentDrive);
        const attrData = this.buildAttrsFromDrive(this.currentDrive);
        
        let html = '';
        html += this.renderTabs(isNvme);
        html += '<div id="smart-tab-contents">';
        html += `<div id="tab-health" class="smart-tab-content active">`;
        html += this.renderHealthSummary(healthData);
        html += `</div>`;

        if (isNvme) {
            html += `<div id="tab-nvme" class="smart-tab-content">`;
            html += this.renderNvmeHealth(attrData, this.currentDrive);
            html += `</div>`;
        }

        html += `<div id="tab-attributes" class="smart-tab-content">`;
        html += this.renderAttributesTable(attrData?.attributes || [], isNvme);
        html += `</div>`;

        html += `<div id="tab-temperature" class="smart-tab-content">`;
        html += this.renderTemperatureChart(null);
        html += `</div>`;

        html += '</div>';
        container.innerHTML = html;
        this.initTabs();
    },

    renderTabs(isNvme) {
        let tabs = `
            <div class="smart-tabs">
                <button class="smart-tab active" data-tab="health">
                    ${this.icons.health}
                    <span>Health</span>
                </button>
        `;
        if (isNvme) {
            tabs += `
                <button class="smart-tab" data-tab="nvme">
                    ${this.icons.nvme}
                    <span>NVMe Info</span>
                </button>
            `;
        } else {
            // Only show Attributes tab for non-NVMe drives (ATA/SATA)
            tabs += `
                <button class="smart-tab" data-tab="attributes">
                    ${this.icons.table}
                    <span>Attributes</span>
                </button>
            `;
        }
        tabs += `
                <button class="smart-tab" data-tab="temperature">
                    ${this.icons.temp}
                    <span>Temperature</span>
                </button>
            </div>
        `;
        return tabs;
    },

    initTabs() {
        const tabs = document.querySelectorAll('.smart-tab');
        tabs.forEach(tab => {
            tab.addEventListener('click', () => {
                const tabId = tab.dataset.tab;
                this.switchTab(tabId);
            });
        });
    },

    switchTab(tabId) {
        document.querySelectorAll('.smart-tab').forEach(t => t.classList.remove('active'));
        document.querySelector(`.smart-tab[data-tab="${tabId}"]`)?.classList.add('active');
        document.querySelectorAll('.smart-tab-content').forEach(c => c.classList.remove('active'));
        document.getElementById(`tab-${tabId}`)?.classList.add('active');
    },

    renderHealthSummary(healthData) {
        if (!healthData) {
            return `<div class="smart-empty">
                ${this.icons.alert}
                <p>Health data unavailable</p>
                <span class="hint">No SMART data has been collected yet</span>
            </div>`;
        }

        const healthClass = healthData.overall_health?.toLowerCase() || 'healthy';
        const healthIcon = healthClass === 'healthy' ? this.icons.check :
                          healthClass === 'warning' ? this.icons.warning : this.icons.error;

        // Truncate model name if too long to prevent layout issues
        let modelName = healthData.model_name || 'N/A';
        if (modelName.length > 20) {
            modelName = modelName.substring(0, 18) + '…';
        }

        return `
            <div class="health-summary-panel">
                <div class="health-summary-header">
                    <div class="health-summary-icon ${healthClass}">
                        ${healthIcon}
                    </div>
                    <div class="health-summary-title">
                        <h3>${healthData.overall_health || 'Unknown'}</h3>
                        <span class="subtitle">${healthData.smart_passed ? 'SMART Self-Test Passed' : 'SMART Self-Test Failed'}</span>
                    </div>
                </div>
                <div class="health-summary-stats">
                    <div class="health-stat-card">
                        <div class="health-stat-value ${healthData.critical_count > 0 ? 'critical' : 'healthy'}">
                            ${healthData.critical_count || 0}
                        </div>
                        <div class="health-stat-label">CRITICAL ISSUES</div>
                    </div>
                    <div class="health-stat-card">
                        <div class="health-stat-value ${healthData.warning_count > 0 ? 'warning' : 'healthy'}">
                            ${healthData.warning_count || 0}
                        </div>
                        <div class="health-stat-label">WARNINGS</div>
                    </div>
                    <div class="health-stat-card">
                        <div class="health-stat-value model-name" title="${healthData.model_name || 'N/A'}">${modelName}</div>
                        <div class="health-stat-label">MODEL</div>
                    </div>
                    <div class="health-stat-card">
                        <div class="health-stat-value">${healthData.drive_type || 'Unknown'}</div>
                        <div class="health-stat-label">TYPE</div>
                    </div>
                </div>
            </div>
        `;
    },

    renderIssuesList(issues) {
        if (!issues || issues.length === 0) return '';

        let html = `
            <div class="health-issues-panel">
                <div class="health-issues-header">
                    ${this.icons.warning}
                    <span>Issues Detected (${issues.length})</span>
                </div>
                <div class="health-issues-list">
        `;

        for (const issue of issues) {
            const severityClass = issue.severity?.toLowerCase() || 'warning';
            html += `
                <div class="health-issue-item">
                    <div class="issue-severity-dot ${severityClass}"></div>
                    <div class="issue-content">
                        <div class="issue-message">${this.escapeHtml(issue.message)}</div>
                        <div class="issue-meta">
                            Attribute #${issue.attribute_id} · ${issue.attribute_name}
                        </div>
                    </div>
                </div>
            `;
        }

        html += '</div></div>';
        return html;
    },

    renderNvmeHealth(attrData, driveData) {
        const attrs = attrData?.attributes || [];
        const nvmeLog = driveData?.nvme_smart_health_information_log || {};
        const metrics = this.extractNvmeMetrics(attrs, nvmeLog);

        return `
            <div class="nvme-health-panel">
                <div class="nvme-health-header">
                    ${this.icons.nvme}
                    <h3>NVMe Health Information</h3>
                    <span class="badge">NVMe</span>
                </div>
                <div class="nvme-health-grid">
                    ${this.renderNvmeMetric('Available Spare', metrics.availableSpare, '%', this.icons.storage, 
                        this.getNvmeSpareClass(metrics.availableSpare), metrics.availableSpare)}
                    ${this.renderNvmeMetric('Percentage Used', metrics.percentageUsed, '%', this.icons.activity,
                        this.getNvmeWearClass(metrics.percentageUsed), 100 - metrics.percentageUsed)}
                    ${this.renderNvmeMetric('Temperature', metrics.temperature, '°C', this.icons.temp,
                        this.getTempClass(metrics.temperature))}
                    ${this.renderNvmeMetric('Power On Hours', this.formatAge(metrics.powerOnHours), '', this.icons.clock)}
                    ${this.renderNvmeMetric('Power Cycles', this.formatNumber(metrics.powerCycles), '', this.icons.power)}
                    ${this.renderNvmeMetric('Data Written', this.formatNvmeBytes(metrics.dataUnitsWritten), '', this.icons.storage)}
                    ${this.renderNvmeMetric('Data Read', this.formatNvmeBytes(metrics.dataUnitsRead), '', this.icons.storage)}
                    ${this.renderNvmeMetric('Media Errors', metrics.mediaErrors, '', this.icons.error,
                        metrics.mediaErrors > 0 ? 'critical' : 'healthy')}
                    ${this.renderNvmeMetric('Unsafe Shutdowns', this.formatNumber(metrics.unsafeShutdowns), '', this.icons.warning,
                        metrics.unsafeShutdowns > 10 ? 'warning' : '')}
                    ${this.renderNvmeMetric('Controller Busy', this.formatNumber(metrics.controllerBusyTime), 'min', this.icons.clock)}
                    ${this.renderNvmeMetric('Critical Warning', metrics.criticalWarning, '', this.icons.alert,
                        metrics.criticalWarning > 0 ? 'critical' : 'healthy')}
                    ${this.renderNvmeMetric('Host Reads', this.formatNumber(metrics.hostReads), '', this.icons.activity)}
                </div>
            </div>
        `;
    },

    renderNvmeMetric(label, value, unit, icon, valueClass = '', progressValue = null) {
        let progressHtml = '';
        if (progressValue !== null && progressValue !== undefined) {
            const progressClass = valueClass || 'healthy';
            progressHtml = `
                <div class="nvme-health-progress">
                    <div class="nvme-health-progress-bar ${progressClass}" style="width: ${Math.max(0, Math.min(100, progressValue))}%"></div>
                </div>
            `;
        }

        return `
            <div class="nvme-health-item">
                <div class="nvme-health-label">
                    ${icon}
                    ${label}
                </div>
                <div class="nvme-health-value ${valueClass}">
                    ${value !== null && value !== undefined ? value : 'N/A'}
                    ${unit ? `<span class="unit">${unit}</span>` : ''}
                </div>
                ${progressHtml}
            </div>
        `;
    },

    extractNvmeMetrics(attrs, nvmeLog) {
        const findAttr = (id) => attrs.find(a => a.id === id);
        const getVal = (id, fallback = null) => {
            const attr = findAttr(id);
            return attr?.raw_value ?? fallback;
        };

        return {
            temperature: nvmeLog.temperature ?? getVal(194),
            availableSpare: nvmeLog.available_spare ?? getVal(232),
            percentageUsed: nvmeLog.percentage_used ?? getVal(233),
            dataUnitsWritten: nvmeLog.data_units_written ?? getVal(241),
            dataUnitsRead: nvmeLog.data_units_read ?? getVal(242),
            powerCycles: nvmeLog.power_cycles ?? getVal(12),
            powerOnHours: nvmeLog.power_on_hours ?? getVal(9),
            mediaErrors: nvmeLog.media_errors ?? getVal(187),
            unsafeShutdowns: nvmeLog.unsafe_shutdowns ?? getVal(174),
            controllerBusyTime: nvmeLog.controller_busy_time ?? getVal(175),
            criticalWarning: nvmeLog.critical_warning ?? getVal(1),
            hostReads: nvmeLog.host_reads ?? getVal(176),
            hostWrites: nvmeLog.host_writes ?? getVal(178)
        };
    },

    renderAttributesTable(attributes, isNvme) {
        if (!attributes || attributes.length === 0) {
            return `
                <div class="smart-empty">
                    ${this.icons.table}
                    <p>No SMART attributes available</p>
                    <span class="hint">${isNvme ? 'NVMe drives report health differently' : 'Waiting for data collection'}</span>
                </div>
            `;
        }

        const sorted = [...attributes].sort((a, b) => {
            const aSev = this.getAttributeSeverity(a);
            const bSev = this.getAttributeSeverity(b);
            const sevOrder = { critical: 0, warning: 1, info: 2, healthy: 3 };
            const sevDiff = (sevOrder[aSev] || 3) - (sevOrder[bSev] || 3);
            return sevDiff !== 0 ? sevDiff : a.id - b.id;
        });

        let html = `
            <div class="smart-table-container">
                <div class="smart-table-header">
                    <div class="smart-table-title">
                        ${this.icons.table}
                        S.M.A.R.T. Attributes
                    </div>
                    <div class="smart-table-filter">
                        <button class="filter-btn ${this.attributeFilter === 'all' ? 'active' : ''}" 
                                onclick="SmartAttributes.setFilter('all')">All</button>
                        <button class="filter-btn ${this.attributeFilter === 'critical' ? 'active' : ''}"
                                onclick="SmartAttributes.setFilter('critical')">Critical</button>
                        <button class="filter-btn ${this.attributeFilter === 'issues' ? 'active' : ''}"
                                onclick="SmartAttributes.setFilter('issues')">Issues Only</button>
                    </div>
                </div>
                <div class="smart-table-wrapper">
                    <table class="smart-table">
                        <thead>
                            <tr>
                                <th style="width: 90px">Status</th>
                                <th style="width: 50px">ID</th>
                                <th>Attribute</th>
                                <th style="width: 70px">Value</th>
                                <th style="width: 70px">Worst</th>
                                <th style="width: 70px">Thresh</th>
                                <th style="width: 100px">Raw</th>
                            </tr>
                        </thead>
                        <tbody>
        `;

        for (const attr of sorted) {
            const severity = this.getAttributeSeverity(attr);
            const isCritical = this.criticalATAIds.includes(attr.id);

            if (this.attributeFilter === 'critical' && !isCritical) continue;
            if (this.attributeFilter === 'issues' && severity === 'healthy') continue;

            const rowClass = severity === 'critical' ? 'critical-row' : 
                            severity === 'warning' ? 'warning-row' : '';

            // Get attribute explanation for tooltip
            const explanation = this.attributeExplanations[attr.id];
            const tooltipContent = explanation ? this.escapeHtml(explanation.desc) : '';
            const hasTooltip = tooltipContent.length > 0;

            html += `
                <tr class="${rowClass}" onclick="SmartAttributes.showAttributeDetail(${attr.id})">
                    <td>${this.getSeverityBadge(severity)}</td>
                    <td><span class="attr-id ${isCritical ? 'critical' : ''}">${attr.id}</span></td>
                    <td class="attr-name-cell">
                        <span class="attr-name-wrapper">
                            ${this.escapeHtml(attr.name)}
                            ${hasTooltip ? `<span class="attr-tooltip-trigger" data-tooltip="${tooltipContent}">${this.infoIcon}</span>` : ''}
                        </span>
                    </td>
                    <td>${attr.value ?? '—'}</td>
                    <td>${attr.worst ?? '—'}</td>
                    <td>${attr.threshold ?? '—'}</td>
                    <td><strong>${attr.raw_value !== undefined ? attr.raw_value : '—'}</strong></td>
                </tr>
            `;
        }

        html += `
                        </tbody>
                    </table>
                </div>
            </div>
        `;

        return html;
    },

    setFilter(filter) {
        this.attributeFilter = filter;
        const tabContent = document.getElementById('tab-attributes');
        if (tabContent && this.currentDrive) {
            // Use local data if available, otherwise try API
            const attrData = this.buildAttrsFromDrive(this.currentDrive);
            tabContent.innerHTML = this.renderAttributesTable(attrData?.attributes || [], this.isNvmeDrive(this.currentDrive));
        }
    },

    renderTemperatureChart(tempData) {
        if (!tempData || !tempData.history || tempData.history.length === 0) {
            return `
                <div class="smart-empty">
                    ${this.icons.temp}
                    <p>No temperature history available</p>
                    <span class="hint">Temperature data will appear after collection</span>
                </div>
            `;
        }

        const temps = tempData.history.map(h => h.temperature);
        const minTemp = tempData.min_temp || Math.min(...temps);
        const maxTemp = tempData.max_temp || Math.max(...temps);
        const avgTemp = tempData.avg_temp || Math.round(temps.reduce((a, b) => a + b, 0) / temps.length);

        return `
            <div class="temp-chart-panel">
                <div class="temp-chart-header">
                    <div class="temp-chart-title">
                        ${this.icons.temp}
                        Temperature History (${tempData.hours || 24}h)
                    </div>
                    <div class="temp-chart-stats">
                        <div class="temp-chart-stat">
                            <span class="label">Min:</span>
                            <span class="value min">${minTemp}°C</span>
                        </div>
                        <div class="temp-chart-stat">
                            <span class="label">Avg:</span>
                            <span class="value avg">${avgTemp}°C</span>
                        </div>
                        <div class="temp-chart-stat">
                            <span class="label">Max:</span>
                            <span class="value max">${maxTemp}°C</span>
                        </div>
                    </div>
                </div>
                <div class="temp-chart-body">
                    <canvas id="temp-chart-canvas" class="temp-chart-canvas"></canvas>
                </div>
            </div>
        `;
    },

    async showAttributeDetail(attributeId) {
        const trend = await this.fetchAttributeTrend(
            this.currentHostname, 
            this.currentDrive?.serial_number, 
            attributeId, 
            30
        );

        const attrDef = this.getAttributeDefinition(attributeId);

        let panel = document.getElementById('attr-detail-panel');
        if (!panel) {
            panel = document.createElement('div');
            panel.id = 'attr-detail-panel';
            panel.className = 'attr-detail-panel';
            document.body.appendChild(panel);
        }

        const trendIcon = trend?.trend === 'degrading' ? this.icons.trendUp :
                         trend?.trend === 'improving' ? this.icons.trendDown :
                         this.icons.minus;

        panel.innerHTML = `
            <div class="attr-detail-header">
                <h3>Attribute #${attributeId}</h3>
                <button class="attr-detail-close" onclick="SmartAttributes.closeDetailPanel()">
                    ${this.icons.close}
                </button>
            </div>
            <div class="attr-detail-body">
                <div class="attr-detail-section">
                    <h4>Overview</h4>
                    <div class="attr-detail-row">
                        <span class="attr-detail-label">Name</span>
                        <span class="attr-detail-value">${attrDef?.name || 'Unknown'}</span>
                    </div>
                    <div class="attr-detail-row">
                        <span class="attr-detail-label">Severity</span>
                        <span class="attr-detail-value">${attrDef?.severity || 'Unknown'}</span>
                    </div>
                    <div class="attr-detail-row">
                        <span class="attr-detail-label">Drive Type</span>
                        <span class="attr-detail-value">${attrDef?.drive_type || 'All'}</span>
                    </div>
                </div>
                ${attrDef?.description ? `
                    <div class="attr-detail-section">
                        <h4>Description</h4>
                        <div class="attr-description">${attrDef.description}</div>
                    </div>
                ` : ''}
                ${trend ? `
                    <div class="attr-detail-section">
                        <h4>30-Day Trend</h4>
                        <div class="attr-detail-row">
                            <span class="attr-detail-label">Trend</span>
                            <span class="attr-detail-value">
                                <span class="trend-indicator ${trend.trend}">
                                    ${trendIcon}
                                    ${this.capitalize(trend.trend)}
                                </span>
                            </span>
                        </div>
                        <div class="attr-detail-row">
                            <span class="attr-detail-label">First Value</span>
                            <span class="attr-detail-value">${trend.first_raw_value}</span>
                        </div>
                        <div class="attr-detail-row">
                            <span class="attr-detail-label">Current Value</span>
                            <span class="attr-detail-value">${trend.last_raw_value}</span>
                        </div>
                        <div class="attr-detail-row">
                            <span class="attr-detail-label">Change</span>
                            <span class="attr-detail-value">${trend.raw_change >= 0 ? '+' : ''}${trend.raw_change}</span>
                        </div>
                        <div class="attr-detail-row">
                            <span class="attr-detail-label">Data Points</span>
                            <span class="attr-detail-value">${trend.point_count}</span>
                        </div>
                    </div>
                ` : ''}
            </div>
        `;

        setTimeout(() => panel.classList.add('open'), 10);
        this.detailPanelOpen = true;
    },

    closeDetailPanel() {
        const panel = document.getElementById('attr-detail-panel');
        if (panel) {
            panel.classList.remove('open');
            setTimeout(() => panel.remove(), 300);
        }
        this.detailPanelOpen = false;
    },

    // API Methods
    async fetchAttributes(hostname, serialNumber) {
        try {
            const response = await fetch(
                `/api/smart/attributes?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}`
            );
            if (!response.ok) throw new Error('Failed to fetch');
            return await response.json();
        } catch (error) {
            console.error('Error fetching SMART attributes:', error);
            return null;
        }
    },

    async fetchHealthSummary(hostname, serialNumber) {
        try {
            const response = await fetch(
                `/api/smart/health/summary?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}`
            );
            if (!response.ok) throw new Error('Failed to fetch');
            return await response.json();
        } catch (error) {
            console.error('Error fetching health summary:', error);
            return null;
        }
    },

    async fetchTemperatureHistory(hostname, serialNumber, hours = 24) {
        try {
            const response = await fetch(
                `/api/smart/temperature/history?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}&hours=${hours}`
            );
            if (!response.ok) throw new Error('Failed to fetch');
            return await response.json();
        } catch (error) {
            console.error('Error fetching temperature history:', error);
            return null;
        }
    },

    async fetchAttributeTrend(hostname, serialNumber, attributeId, days = 30) {
        try {
            const response = await fetch(
                `/api/smart/attributes/trend?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}&attribute_id=${attributeId}&days=${days}`
            );
            if (!response.ok) throw new Error('Failed to fetch');
            return await response.json();
        } catch (error) {
            console.error('Error fetching attribute trend:', error);
            return null;
        }
    },

    // Helper Methods
    getAttributeSeverity(attr) {
        if (attr.threshold > 0 && attr.value > 0 && attr.value <= attr.threshold) {
            return 'critical';
        }

        const id = attr.id;
        const raw = attr.raw_value || 0;

        if ([5, 10, 196, 197, 198, 187, 188, 181, 182, 183, 184].includes(id)) {
            if (raw > 0) return 'critical';
        }

        if (id === 194 || id === 190) {
            if (raw > 65) return 'critical';
            if (raw > 55) return 'warning';
            if (raw > 45) return 'info';
        }

        if (id === 199 && raw > 0) {
            return raw > 100 ? 'critical' : 'warning';
        }

        if (id === 232) {
            if (raw < 10) return 'critical';
            if (raw < 20) return 'warning';
        }

        if (id === 233) {
            if (raw > 95) return 'critical';
            if (raw > 80) return 'warning';
            if (raw > 50) return 'info';
        }

        if (id === 177) {
            if (attr.value < 10) return 'critical';
            if (attr.value < 20) return 'warning';
        }

        return 'healthy';
    },

    getSeverityBadge(severity) {
        const icons = {
            healthy: this.icons.check,
            info: this.icons.alert,
            warning: this.icons.warning,
            critical: this.icons.error
        };
        const labels = {
            healthy: 'OK',
            info: 'INFO',
            warning: 'WARN',
            critical: 'CRIT'
        };
        return `<span class="severity-badge ${severity}">${icons[severity] || ''} ${labels[severity] || severity}</span>`;
    },

    getAttributeDefinition(id) {
        const defs = {
            5: { name: 'Reallocated Sectors Count', severity: 'CRITICAL', drive_type: 'BOTH', description: 'Count of reallocated sectors. When a sector is found bad, it\'s remapped to a spare area. Any non-zero value indicates potential drive degradation.' },
            9: { name: 'Power-On Hours', severity: 'INFO', drive_type: 'BOTH', description: 'Total hours the drive has been powered on.' },
            10: { name: 'Spin Retry Count', severity: 'CRITICAL', drive_type: 'HDD', description: 'Count of retry attempts to spin up the drive. May indicate motor or bearing issues.' },
            12: { name: 'Power Cycle Count', severity: 'INFO', drive_type: 'BOTH', description: 'Count of full power on/off cycles.' },
            177: { name: 'Wear Leveling Count', severity: 'WARNING', drive_type: 'SSD', description: 'SSD wear leveling status. Lower values indicate more wear.' },
            187: { name: 'Reported Uncorrectable Errors', severity: 'CRITICAL', drive_type: 'BOTH', description: 'Count of uncorrectable errors reported to the host.' },
            188: { name: 'Command Timeout', severity: 'CRITICAL', drive_type: 'BOTH', description: 'Count of aborted operations due to timeout.' },
            190: { name: 'Airflow Temperature', severity: 'WARNING', drive_type: 'BOTH', description: 'Temperature of air flowing across the drive.' },
            194: { name: 'Temperature Celsius', severity: 'WARNING', drive_type: 'BOTH', description: 'Current internal temperature in Celsius.' },
            196: { name: 'Reallocation Event Count', severity: 'CRITICAL', drive_type: 'BOTH', description: 'Count of remap operations from bad to spare sectors.' },
            197: { name: 'Current Pending Sector Count', severity: 'CRITICAL', drive_type: 'BOTH', description: 'Count of unstable sectors waiting to be remapped.' },
            198: { name: 'Offline Uncorrectable Sector Count', severity: 'CRITICAL', drive_type: 'BOTH', description: 'Count of uncorrectable errors found during offline scan.' },
            199: { name: 'UltraDMA CRC Error Count', severity: 'WARNING', drive_type: 'BOTH', description: 'Count of CRC errors during data transfer. Often indicates cable issues.' },
            232: { name: 'Available Reserved Space', severity: 'CRITICAL', drive_type: 'SSD', description: 'Percentage of reserved space remaining for bad block replacement.' },
            233: { name: 'Media Wearout Indicator', severity: 'WARNING', drive_type: 'SSD', description: 'SSD wear indicator showing percentage of rated write cycles used.' },
            241: { name: 'Total LBAs Written', severity: 'INFO', drive_type: 'SSD', description: 'Total count of logical block addresses written.' },
            242: { name: 'Total LBAs Read', severity: 'INFO', drive_type: 'SSD', description: 'Total count of logical block addresses read.' }
        };
        return defs[id] || null;
    },

    getNvmeSpareClass(value) {
        if (value === null || value === undefined) return '';
        if (value < 10) return 'critical';
        if (value < 20) return 'warning';
        return 'healthy';
    },

    getNvmeWearClass(value) {
        if (value === null || value === undefined) return '';
        if (value > 95) return 'critical';
        if (value > 80) return 'warning';
        return 'healthy';
    },

    getTempClass(value) {
        if (value === null || value === undefined) return '';
        if (value > 70) return 'critical';
        if (value > 55) return 'warning';
        return '';
    },

    formatAge(hours) {
        if (!hours) return 'N/A';
        const years = hours / 8760;
        if (years >= 1) return `${years.toFixed(1)}y`;
        const months = hours / 730;
        if (months >= 1) return `${months.toFixed(1)}mo`;
        const days = hours / 24;
        if (days >= 1) return `${days.toFixed(0)}d`;
        return `${hours}h`;
    },

    formatNumber(num) {
        if (num === null || num === undefined) return 'N/A';
        return num.toLocaleString();
    },

    formatNvmeBytes(dataUnits) {
        if (!dataUnits) return 'N/A';
        const bytes = dataUnits * 512 * 1000;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
    },

    capitalize(str) {
        if (!str) return '';
        return str.charAt(0).toUpperCase() + str.slice(1);
    },

    escapeHtml(str) {
        if (!str) return '';
        return str.replace(/[&<>"']/g, m => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
        }[m]));
    },

    renderLoading() {
        return `
            <div class="smart-loading">
                <div class="smart-loading-spinner"></div>
                <span>Loading SMART data...</span>
            </div>
        `;
    },

    renderError(message) {
        return `
            <div class="smart-empty">
                ${this.icons.error}
                <p>${message}</p>
                <span class="hint">Please try again later</span>
            </div>
        `;
    }
};

window.SmartAttributes = SmartAttributes;