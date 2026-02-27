/**
 * Vigil Dashboard - Progress Component (Task 4.4)
 *
 * Renders per-job progress cards with phase trackers, progress bars,
 * speed display, and ETA. Updated in real-time via SSE telemetry.
 */

const ProgressComponent = {
    _jobs: {},  // keyed by job_id

    /**
     * Render initial progress container.
     * @param {string} compId - Manifest component ID
     * @param {Object} config - Optional config (e.g., { columns: 2 })
     * @returns {string} HTML
     */
    render(compId, config) {
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
        let job = this._jobs[jobId];

        if (!job) {
            job = { phases: {}, currentPhase: null, startTime: Date.now() };
            this._jobs[jobId] = job;
        }

        job.currentPhase = payload.phase;
        job.phases[payload.phase] = {
            percent: payload.percent || 0,
            message: payload.message || '',
            eta: payload.eta || '',
            bytesDone: payload.bytes_done || 0,
            bytesTotal: payload.bytes_total || 0,
            updatedAt: Date.now()
        };

        this._renderJob(jobId);
    },

    _renderJob(jobId) {
        const job = this._jobs[jobId];
        if (!job) return;

        // Find or create the job card in any progress container
        let card = document.getElementById(`job-${jobId}`);
        if (!card) {
            const container = document.querySelector('.progress-container');
            if (!container) return;
            // Remove empty placeholder
            const empty = container.querySelector('.progress-empty');
            if (empty) empty.remove();

            card = document.createElement('div');
            card.id = `job-${jobId}`;
            card.className = 'progress-job-card';
            container.appendChild(card);
        }

        const phases = Object.entries(job.phases);
        const current = job.phases[job.currentPhase] || {};
        const overallPercent = this._overallPercent(job);
        const isComplete = overallPercent >= 100;

        card.innerHTML = `
            <div class="progress-job-header">
                <span class="progress-job-id">${this._escape(jobId)}</span>
                <span class="progress-job-status ${isComplete ? 'complete' : 'running'}">
                    ${isComplete ? 'Complete' : 'Running'}
                </span>
            </div>
            <div class="progress-bar-container">
                <div class="progress-bar">
                    <div class="progress-bar-fill ${isComplete ? 'complete' : ''}" style="width: ${overallPercent}%"></div>
                </div>
                <span class="progress-percent">${overallPercent.toFixed(1)}%</span>
            </div>
            ${current.message ? `<div class="progress-message">${this._escape(current.message)}</div>` : ''}
            <div class="progress-meta">
                ${this._speedDisplay(current)}
                ${current.eta ? `<span class="progress-eta">ETA: ${this._escape(current.eta)}</span>` : ''}
                <span class="progress-elapsed">${this._elapsed(job.startTime)}</span>
            </div>
            <div class="progress-phases">
                ${phases.map(([name, p]) => this._phaseChip(name, p, job.currentPhase)).join('')}
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

    _speedDisplay(phase) {
        if (!phase.bytesDone || !phase.bytesTotal) return '';
        const done = this._formatBytes(phase.bytesDone);
        const total = this._formatBytes(phase.bytesTotal);
        return `<span class="progress-speed">${done} / ${total}</span>`;
    },

    _elapsed(startTime) {
        const diff = Math.floor((Date.now() - startTime) / 1000);
        const h = Math.floor(diff / 3600);
        const m = Math.floor((diff % 3600) / 60);
        const s = diff % 60;
        if (h > 0) return `${h}h ${m}m`;
        if (m > 0) return `${m}m ${s}s`;
        return `${s}s`;
    },

    _formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};
