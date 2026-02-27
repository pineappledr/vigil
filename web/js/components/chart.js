/**
 * Vigil Dashboard - Chart Component (Task 4.5)
 *
 * Time-series charts using Chart.js with dual Y-axis support.
 * Uses a linear X-axis with formatted time labels to avoid
 * requiring the Chart.js date adapter (luxon/date-fns).
 *
 * Receives metric telemetry updates via SSE and appends data points.
 */

const ChartComponent = {
    _charts: {},  // keyed by compId → { chart, config, datasetMap, maxPoints }

    /**
     * @param {string} compId - Manifest component ID
     * @param {Object} config - Chart config from manifest
     *   {
     *     datasets: [{ key, label, color, yAxis: 'left'|'right', fill }],
     *     maxPoints?: number,
     *     height?: number,
     *     yAxes?: { left: { title, min, max, unit }, right: { title, min, max, unit } }
     *   }
     * @returns {string} HTML
     */
    render(compId, config) {
        const height = config.height || 300;

        // Defer Chart.js initialization to after DOM insertion
        setTimeout(() => this._initChart(compId, config), 0);

        return `<div class="chart-wrapper" style="height: ${height}px">
                    <canvas id="chart-canvas-${compId}"></canvas>
                </div>`;
    },

    _initChart(compId, config) {
        const canvas = document.getElementById(`chart-canvas-${compId}`);
        if (!canvas || typeof Chart === 'undefined') return;

        const maxPoints = config.maxPoints || 200;
        const hasRight = (config.datasets || []).some(ds => ds.yAxis === 'right');

        const datasets = (config.datasets || []).map((ds, i) => ({
            label: ds.label || `Series ${i + 1}`,
            data: [],
            borderColor: ds.color || this._colors[i % this._colors.length],
            backgroundColor: this._alpha(ds.color || this._colors[i % this._colors.length], 0.12),
            borderWidth: 2,
            pointRadius: 0,
            pointHitRadius: 8,
            tension: 0.3,
            fill: ds.fill === true,
            yAxisID: ds.yAxis === 'right' ? 'y1' : 'y'
        }));

        const scales = {
            x: {
                type: 'category',
                ticks: {
                    color: '#8892b0',
                    font: { family: 'Outfit', size: 11 },
                    maxTicksLimit: 8,
                    maxRotation: 0
                },
                grid: { color: 'rgba(136, 146, 176, 0.1)' }
            },
            y: this._buildYAxis(config.yAxes?.left, 'left')
        };

        if (hasRight) {
            scales.y1 = this._buildYAxis(config.yAxes?.right, 'right');
        }

        const chart = new Chart(canvas, {
            type: 'line',
            data: { labels: [], datasets },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 300 },
                interaction: { mode: 'index', intersect: false },
                plugins: {
                    legend: {
                        labels: { color: '#8892b0', font: { family: 'Outfit', size: 12 }, boxWidth: 12 }
                    },
                    tooltip: {
                        backgroundColor: '#1a1f2e',
                        titleColor: '#ccd6f6',
                        bodyColor: '#8892b0',
                        borderColor: 'rgba(100, 255, 218, 0.2)',
                        borderWidth: 1,
                        padding: 10,
                        displayColors: true
                    }
                },
                scales
            }
        });

        const datasetMap = {};
        (config.datasets || []).forEach((ds, i) => {
            datasetMap[ds.key || ds.label] = i;
        });

        this._charts[compId] = { chart, maxPoints, datasetMap };

        if (typeof ManifestRenderer !== 'undefined') {
            ManifestRenderer.registerChart(chart);
        }
    },

    /**
     * Handle an incoming metric telemetry event.
     * @param {Object} payload - { key: string, value: number, timestamp?: string }
     *   or batch: { metrics: [{ key, value, timestamp }] }
     */
    handleUpdate(payload) {
        // Support batch updates
        if (payload.metrics && Array.isArray(payload.metrics)) {
            payload.metrics.forEach(m => this._pushMetric(m));
            // Batch update all charts once
            for (const entry of Object.values(this._charts)) {
                entry.chart.update('none');
            }
            return;
        }

        if (!payload?.key) return;
        this._pushMetric(payload);

        // Update all affected charts
        for (const entry of Object.values(this._charts)) {
            if (entry.datasetMap[payload.key] !== undefined) {
                entry.chart.update('none');
            }
        }
    },

    _pushMetric(metric) {
        if (!metric?.key) return;

        const timeLabel = this._formatTime(metric.timestamp ? new Date(metric.timestamp) : new Date());

        for (const [, entry] of Object.entries(this._charts)) {
            const idx = entry.datasetMap[metric.key];
            if (idx === undefined) continue;

            const dataset = entry.chart.data.datasets[idx];
            if (!dataset) continue;

            // Add label if this is a new time point
            const labels = entry.chart.data.labels;
            if (labels.length === 0 || labels[labels.length - 1] !== timeLabel) {
                labels.push(timeLabel);
                // Pad other datasets with null to keep alignment
                entry.chart.data.datasets.forEach((ds, i) => {
                    if (i !== idx) ds.data.push(null);
                });
            }

            dataset.data.push(metric.value);

            // Trim to maxPoints
            while (labels.length > entry.maxPoints) {
                labels.shift();
                entry.chart.data.datasets.forEach(ds => ds.data.shift());
            }
        }
    },

    // ─── Axis builders ────────────────────────────────────────────────────

    _buildYAxis(config, position) {
        const unitSuffix = config?.unit || '';
        return {
            position,
            display: true,
            title: {
                display: !!(config?.title),
                text: config?.title || '',
                color: '#8892b0',
                font: { family: 'Outfit', size: 12 }
            },
            min: config?.min,
            max: config?.max,
            ticks: {
                color: '#8892b0',
                font: { family: 'Outfit', size: 11 },
                callback: unitSuffix
                    ? function(value) { return value + unitSuffix; }
                    : undefined
            },
            grid: {
                color: position === 'left' ? 'rgba(136, 146, 176, 0.1)' : 'transparent'
            }
        };
    },

    // ─── Helpers ──────────────────────────────────────────────────────────

    _formatTime(d) {
        const hh = String(d.getHours()).padStart(2, '0');
        const mm = String(d.getMinutes()).padStart(2, '0');
        const ss = String(d.getSeconds()).padStart(2, '0');
        return `${hh}:${mm}:${ss}`;
    },

    _alpha(hex, a) {
        // Convert hex color to rgba
        const r = parseInt(hex.slice(1, 3), 16);
        const g = parseInt(hex.slice(3, 5), 16);
        const b = parseInt(hex.slice(5, 7), 16);
        if (isNaN(r)) return hex + '20'; // fallback
        return `rgba(${r}, ${g}, ${b}, ${a})`;
    },

    _colors: ['#64ffda', '#f78166', '#7ee787', '#d2a8ff', '#79c0ff', '#ffa657']
};
