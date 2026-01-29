/**
 * Vigil Dashboard - SMART Attributes Module
 * Phase 1.1: Enhanced S.M.A.R.T. Monitoring
 */

const SmartAttributes = {
    // Critical attribute IDs that should be highlighted
    criticalIDs: [5, 10, 187, 188, 196, 197, 198, 181, 182, 183, 184, 232],

    // Fetch SMART attributes for a specific drive
    async fetchAttributes(hostname, serialNumber) {
        try {
            const response = await fetch(`/api/smart/attributes?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}`);
            if (!response.ok) throw new Error('Failed to fetch SMART attributes');
            return await response.json();
        } catch (error) {
            console.error('Error fetching SMART attributes:', error);
            return null;
        }
    },

    // Fetch attribute history for charting
    async fetchAttributeHistory(hostname, serialNumber, attributeID, limit = 100) {
        try {
            const response = await fetch(
                `/api/smart/attributes/history?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}&attribute_id=${attributeID}&limit=${limit}`
            );
            if (!response.ok) throw new Error('Failed to fetch attribute history');
            return await response.json();
        } catch (error) {
            console.error('Error fetching attribute history:', error);
            return null;
        }
    },

    // Fetch trend analysis
    async fetchAttributeTrend(hostname, serialNumber, attributeID, days = 30) {
        try {
            const response = await fetch(
                `/api/smart/attributes/trend?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}&attribute_id=${attributeID}&days=${days}`
            );
            if (!response.ok) throw new Error('Failed to fetch attribute trend');
            return await response.json();
        } catch (error) {
            console.error('Error fetching attribute trend:', error);
            return null;
        }
    },

    // Fetch health summary for a drive
    async fetchHealthSummary(hostname, serialNumber) {
        try {
            const response = await fetch(`/api/smart/health/summary?hostname=${encodeURIComponent(hostname)}&serial=${encodeURIComponent(serialNumber)}`);
            if (!response.ok) throw new Error('Failed to fetch health summary');
            return await response.json();
        } catch (error) {
            console.error('Error fetching health summary:', error);
            return null;
        }
    },

    // Fetch all drives health summaries
    async fetchAllHealthSummaries() {
        try {
            const response = await fetch('/api/smart/health/all');
            if (!response.ok) throw new Error('Failed to fetch all health summaries');
            return await response.json();
        } catch (error) {
            console.error('Error fetching all health summaries:', error);
            return null;
        }
    },

    // Determine severity based on attribute value
    getAttributeSeverity(attr) {
        // Check if value is below threshold
        if (attr.threshold > 0 && attr.value <= attr.threshold) {
            return 'critical';
        }

        // Check specific critical attributes by raw value
        if (this.criticalIDs.includes(attr.id)) {
            // Reallocated sectors, pending sectors, errors
            if ([5, 196, 197, 198, 187, 188, 181, 182, 183, 184].includes(attr.id)) {
                if (attr.raw_value > 0) return 'critical';
            }
            
            // Temperature
            if (attr.id === 194) {
                if (attr.raw_value > 60) return 'warning';
                if (attr.raw_value > 50) return 'info';
            }

            // Available Reserved Space (SSD)
            if (attr.id === 232) {
                if (attr.raw_value < 10) return 'critical';
                if (attr.raw_value < 20) return 'warning';
            }

            // CRC Errors
            if (attr.id === 199 && attr.raw_value > 0) {
                return 'warning';
            }
        }

        return 'healthy';
    },

    // Get severity badge HTML
    getSeverityBadge(severity) {
        const badges = {
            critical: '<span class="severity-badge critical">CRITICAL</span>',
            warning: '<span class="severity-badge warning">WARNING</span>',
            info: '<span class="severity-badge info">INFO</span>',
            healthy: '<span class="severity-badge healthy">OK</span>'
        };
        return badges[severity] || badges.healthy;
    },

    // Render SMART attributes table
    renderAttributesTable(attributes, containerId) {
        const container = document.getElementById(containerId);
        if (!container) return;

        if (!attributes || attributes.length === 0) {
            container.innerHTML = '<p class="no-data">No SMART attributes available</p>';
            return;
        }

        let html = `
            <div class="smart-table-container">
                <table class="smart-table">
                    <thead>
                        <tr>
                            <th>Status</th>
                            <th>ID</th>
                            <th>Attribute Name</th>
                            <th>Value</th>
                            <th>Worst</th>
                            <th>Threshold</th>
                            <th>Raw Value</th>
                            <th>Flags</th>
                        </tr>
                    </thead>
                    <tbody>
        `;

        // Sort: critical attributes first
        const sorted = [...attributes].sort((a, b) => {
            const aSev = this.getAttributeSeverity(a);
            const bSev = this.getAttributeSeverity(b);
            const severityOrder = { critical: 0, warning: 1, info: 2, healthy: 3 };
            return (severityOrder[aSev] || 3) - (severityOrder[bSev] || 3);
        });

        for (const attr of sorted) {
            const severity = this.getAttributeSeverity(attr);
            const isCritical = this.criticalIDs.includes(attr.id);
            const rowClass = isCritical ? 'critical-attribute' : '';

            html += `
                <tr class="${rowClass}" data-attr-id="${attr.id}" onclick="SmartAttributes.showAttributeDetails(${attr.id}, this)">
                    <td>${this.getSeverityBadge(severity)}</td>
                    <td><strong>${attr.id}</strong></td>
                    <td>${attr.name}</td>
                    <td>${attr.value || '—'}</td>
                    <td>${attr.worst || '—'}</td>
                    <td>${attr.threshold || '—'}</td>
                    <td><strong>${attr.raw_value !== undefined ? attr.raw_value : '—'}</strong></td>
                    <td><code>${attr.flags || '—'}</code></td>
                </tr>
            `;
        }

        html += `
                    </tbody>
                </table>
            </div>
        `;

        container.innerHTML = html;
    },

    // Render health summary
    renderHealthSummary(summary, containerId) {
        const container = document.getElementById(containerId);
        if (!container) return;

        if (!summary) {
            container.innerHTML = '<p class="no-data">Health summary unavailable</p>';
            return;
        }

        const healthClass = summary.overall_health.toLowerCase();
        const healthIcon = {
            healthy: '✓',
            warning: '⚠',
            critical: '✗'
        }[healthClass] || '?';

        let html = `
            <div class="health-summary ${healthClass}">
                <div class="health-header">
                    <span class="health-icon">${healthIcon}</span>
                    <h3>Overall Health: ${summary.overall_health}</h3>
                </div>
                <div class="health-stats">
                    <div class="stat-item">
                        <span class="stat-label">Critical Issues</span>
                        <span class="stat-value ${summary.critical_count > 0 ? 'critical' : ''}">${summary.critical_count}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Warnings</span>
                        <span class="stat-value ${summary.warning_count > 0 ? 'warning' : ''}">${summary.warning_count}</span>
                    </div>
                </div>
        `;

        if (summary.issues && summary.issues.length > 0) {
            html += '<div class="health-issues"><h4>Issues Detected</h4><ul>';
            for (const issue of summary.issues) {
                const issueClass = issue.severity.toLowerCase();
                html += `<li class="${issueClass}">${issue.message}</li>`;
            }
            html += '</ul></div>';
        }

        html += '</div>';
        container.innerHTML = html;
    },

    // Show detailed attribute information (placeholder for modal/expansion)
    showAttributeDetails(attributeID, rowElement) {
        console.log(`Show details for attribute ${attributeID}`);
        // TODO: Implement modal or expansion panel with historical chart
        // This would fetch history and render a trend chart
    },

    // Initialize SMART monitoring for current drive
    async initializeDriveMonitoring(hostname, serialNumber) {
        // Fetch and render SMART attributes
        const attrData = await this.fetchAttributes(hostname, serialNumber);
        if (attrData) {
            this.renderAttributesTable(attrData.attributes, 'smart-attributes-table');
        }

        // Fetch and render health summary
        const healthData = await this.fetchHealthSummary(hostname, serialNumber);
        if (healthData) {
            this.renderHealthSummary(healthData, 'smart-health-summary');
        }
    }
};

// Add to global scope for onclick handlers
window.SmartAttributes = SmartAttributes;