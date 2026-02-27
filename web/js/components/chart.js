/**
 * Vigil Dashboard - Chart Component (Task 4.5)
 *
 * Time-series charts using Chart.js with dual Y-axis support.
 * Receives metric telemetry updates via SSE and appends data points.
 */

const ChartComponent = {
    _charts: {},  // keyed by compId → { chart, config, datasets }

    /**
     * Render a chart component from manifest config.
     * @param {string} compId - Manifest component ID
     * @param {Object} config - Chart config from manifest
     *   { datasets: [{ label, color, yAxis }], maxPoints, yAxes: { left: {}, right: {} } }
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

        const datasets = (config.datasets || []).map((ds, i) => ({
            label: ds.label || `Series ${i + 1}`,
            data: [],
            borderColor: ds.color || this._defaultColors[i % this._defaultColors.length],
            backgroundColor: (ds.color || this._defaultColors[i % this._defaultColors.length]) + '20',
            borderWidth: 2,
            pointRadius: 0,
            tension: 0.3,
            fill: ds.fill !== false,
            yAxisID: ds.yAxis === 'right' ? 'y1' : 'y'
        }));

        const scales = { x: this._timeAxis() };
        scales.y = this._yAxis(config.yAxes?.left, 'left');
        if ((config.datasets || []).some(ds => ds.yAxis === 'right')) {
            scales.y1 = this._yAxis(config.yAxes?.right, 'right');
        }

        const chart = new Chart(canvas, {
            type: 'line',
            data: { datasets },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 300 },
                interaction: { mode: 'index', intersect: false },
                plugins: {
                    legend: {
                        labels: { color: '#8892b0', font: { family: 'Outfit', size: 12 } }
                    },
                    tooltip: {
                        backgroundColor: '#1a1f2e',
                        titleColor: '#ccd6f6',
                        bodyColor: '#8892b0',
                        borderColor: 'rgba(100, 255, 218, 0.2)',
                        borderWidth: 1
                    }
                },
                scales
            }
        });

        this._charts[compId] = {
            chart,
            config,
            maxPoints: config.maxPoints || 200,
            datasetMap: {}
        };

        // Map dataset labels for fast lookup
        (config.datasets || []).forEach((ds, i) => {
            this._charts[compId].datasetMap[ds.key || ds.label] = i;
        });

        if (typeof ManifestRenderer !== 'undefined') {
            ManifestRenderer.registerChart(chart);
        }
    },

    /**
     * Handle an incoming metric telemetry event.
     * @param {Object} payload - { key: string, value: number, timestamp?: string }
     */
    handleUpdate(payload) {
        if (!payload?.key) return;

        const timestamp = payload.timestamp ? new Date(payload.timestamp) : new Date();

        for (const [compId, entry] of Object.entries(this._charts)) {
            const idx = entry.datasetMap[payload.key];
            if (idx === undefined) continue;

            const dataset = entry.chart.data.datasets[idx];
            if (!dataset) continue;

            dataset.data.push({ x: timestamp, y: payload.value });

            // Trim to maxPoints
            while (dataset.data.length > entry.maxPoints) {
                dataset.data.shift();
            }

            entry.chart.update('none');
        }
    },

    // ─── Axis helpers ─────────────────────────────────────────────────────

    _timeAxis() {
        return {
            type: 'time',
            time: { unit: 'minute', displayFormats: { minute: 'HH:mm', hour: 'HH:mm' } },
            ticks: { color: '#8892b0', font: { family: 'Outfit', size: 11 }, maxTicksLimit: 8 },
            grid: { color: 'rgba(136, 146, 176, 0.1)' }
        };
    },

    _yAxis(config, position) {
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
                callback: config?.unit
                    ? function(value) { return value + config.unit; }
                    : undefined
            },
            grid: { color: position === 'left' ? 'rgba(136, 146, 176, 0.1)' : 'transparent' }
        };
    },

    _defaultColors: ['#64ffda', '#f78166', '#7ee787', '#d2a8ff', '#79c0ff', '#ffa657']
};
