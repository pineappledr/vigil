/**
 * Vigil Dashboard - Progress Component (Task 4.4)
 *
 * Renders per-job progress cards with:
 *  - Phase tracker chips (pending → active → complete)
 *  - Animated progress bar with percentage
 *  - Transfer speed calculation (bytes/sec rolling average)
 *  - ETA display (from server or locally computed)
 *  - Elapsed time counter
 */

const ProgressComponent = {
    _jobs: {},       // keyed by job_id → job state
    _compIds: {},    // keyed by compId → true (for container lookup)

    /**
     * @param {string} compId - Manifest component ID
     * @param {Object} config - Optional: { showSpeed: true }
     * @returns {string} HTML
     */
    render(compId, config) {
        this._compIds[compId] = true;
        return `<div class="progress-container" id="progress-${compId}" data-comp="${compId}">
                    <div class="progress-empty">Waiting for job data...</div>
                </div>`;
    },

    /**
     * Handle an incoming progress telemetry event.
     * @param {Object} payload - ProgressPayload from WebSocket/SSE
     */
    handleUpdate(payload) {
        if (!payload?.job_id) return;

        const jobId = payload.job_id;
        const now = Date.now();
        let job = this._jobs[jobId];

        if (!job) {
            job = {
                phases: {},
                phaseOrder: [],
                currentPhase: null,
                startTime: now,
                lastBytesSample: 0,
                lastSampleTime: now,
                speedBps: 0
            };
            this._jobs[jobId] = job;
        }

        // Track phase ordering
        if (payload.phase && !job.phases[payload.phase]) {
            job.phaseOrder.push(payload.phase);
        }

        job.currentPhase = payload.phase;
        const phaseData = {
            percent: payload.percent || 0,
            message: payload.message || '',
            eta: payload.eta || '',
            bytesDone: payload.bytes_done || 0,
            bytesTotal: payload.bytes_total || 0,
            updatedAt: now
        };
        job.phases[payload.phase] = phaseData;

        // Speed calculation — rolling sample over the delta since last update
        if (phaseData.bytesDone > 0 && phaseData.bytesDone > job.lastBytesSample) {
            const dtSec = (now - job.lastSampleTime) / 1000;
            if (dtSec > 0.5) { // avoid spikes from sub-500ms bursts
                const dBytes = phaseData.bytesDone - job.lastBytesSample;
                const instantSpeed = dBytes / dtSec;
                // Exponential moving average (α = 0.3)
                job.speedBps = job.speedBps === 0
                    ? instantSpeed
                    : job.speedBps * 0.7 + instantSpeed * 0.3;
                job.lastBytesSample = phaseData.bytesDone;
                job.lastSampleTime = now;
            }
        }

        // Compute local ETA if server doesn't provide one
        if (!phaseData.eta && phaseData.bytesTotal > 0 && job.speedBps > 0) {
            const remaining = phaseData.bytesTotal - phaseData.bytesDone;
            if (remaining > 0) {
                const etaSec = remaining / job.speedBps;
                phaseData.eta = this._formatDuration(etaSec);
            }
        }

        this._renderJob(jobId);
    },

    _renderJob(jobId) {
        const job = this._jobs[jobId];
        if (!job) return;

        let card = document.getElementById(`job-${this._cssId(jobId)}`);
        if (!card) {
            const container = document.querySelector('.progress-container');
            if (!container) return;
            const empty = container.querySelector('.progress-empty');
            if (empty) empty.remove();

            card = document.createElement('div');
            card.id = `job-${this._cssId(jobId)}`;
            card.className = 'progress-job-card';
            container.appendChild(card);
        }

        const current = job.phases[job.currentPhase] || {};
        const overallPct = this._overallPercent(job);
        const done = overallPct >= 100;

        card.innerHTML = `
            <div class="progress-job-header">
                <span class="progress-job-id">${this._escape(jobId)}</span>
                <span class="progress-job-status ${done ? 'complete' : 'running'}">
                    ${done ? 'Complete' : 'Running'}
                </span>
            </div>
            <div class="progress-bar-container">
                <div class="progress-bar">
                    <div class="progress-bar-fill ${done ? 'complete' : ''}" style="width: ${overallPct}%"></div>
                </div>
                <span class="progress-percent">${overallPct.toFixed(1)}%</span>
            </div>
            ${current.message ? `<div class="progress-message">${this._escape(current.message)}</div>` : ''}
            <div class="progress-meta">
                ${this._speedDisplay(current, job)}
                ${current.eta ? `<span class="progress-eta">ETA: ${this._escape(current.eta)}</span>` : ''}
                <span class="progress-elapsed">${this._elapsed(job.startTime)}</span>
            </div>
            <div class="progress-phases">
                ${job.phaseOrder.map(name => this._phaseChip(name, job.phases[name], job.currentPhase)).join('')}
            </div>
        `;
    },

    _phaseChip(name, phase, currentPhase) {
        let status = 'pending';
        if (phase.percent >= 100) status = 'complete';
        else if (name === currentPhase) status = 'active';

        return `<div class="progress-phase ${status}">
                    <span class="progress-phase-dot"></span>
                    <span class="progress-phase-name">${this._escape(name)}</span>
                    <span class="progress-phase-pct">${phase.percent.toFixed(0)}%</span>
                </div>`;
    },

    _overallPercent(job) {
        const phases = Object.values(job.phases);
        if (phases.length === 0) return 0;
        const sum = phases.reduce((acc, p) => acc + p.percent, 0);
        return Math.min(100, sum / phases.length);
    },

    _speedDisplay(phase, job) {
        if (!phase.bytesDone || !phase.bytesTotal) return '';
        const done = this._formatBytes(phase.bytesDone);
        const total = this._formatBytes(phase.bytesTotal);
        const speed = job.speedBps > 0 ? `${this._formatBytes(job.speedBps)}/s` : '';

        return `<span class="progress-speed">${done} / ${total}${speed ? ' · ' + speed : ''}</span>`;
    },

    _elapsed(startTime) {
        return this._formatDuration((Date.now() - startTime) / 1000);
    },

    _formatDuration(totalSec) {
        const s = Math.floor(totalSec);
        const h = Math.floor(s / 3600);
        const m = Math.floor((s % 3600) / 60);
        const sec = s % 60;
        if (h > 0) return `${h}h ${m}m`;
        if (m > 0) return `${m}m ${sec}s`;
        return `${sec}s`;
    },

    _formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
    },

    _cssId(str) {
        return String(str).replace(/[^a-zA-Z0-9_-]/g, '_');
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = String(str);
        return div.innerHTML;
    }
};
