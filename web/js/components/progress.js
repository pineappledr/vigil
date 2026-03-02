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
    _addonId: null,  // parent add-on ID for proxy calls

    /**
     * @param {string} compId - Manifest component ID
     * @param {Object} config - Optional: { showSpeed: true }
     * @param {number} addonId - Parent add-on ID (for cancel proxy)
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        this._compIds[compId] = true;
        if (addonId) this._addonId = addonId;
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
                command: payload.command || '',
                startTime: now,
                lastBytesSample: 0,
                lastSampleTime: now,
                speedBps: 0,
                speedMbps: 0,
                tempC: 0,
                elapsedSec: 0
            };
            this._jobs[jobId] = job;
        }

        // Track phase ordering
        if (payload.phase && !job.phases[payload.phase]) {
            job.phaseOrder.push(payload.phase);
        }

        job.currentPhase = payload.phase;

        // Build message from available fields
        const message = payload.message || payload.phase_detail || '';

        // Build ETA string: prefer server string, then convert eta_sec
        let eta = payload.eta || '';
        if (!eta && payload.eta_sec > 0) {
            eta = this._formatDuration(payload.eta_sec);
        }

        const phaseData = {
            percent: payload.percent || 0,
            message,
            eta,
            bytesDone: payload.bytes_done || 0,
            bytesTotal: payload.bytes_total || 0,
            updatedAt: now
        };
        job.phases[payload.phase] = phaseData;

        // Update server-provided metrics
        if (payload.speed_mbps !== undefined) job.speedMbps = payload.speed_mbps;
        if (payload.temp_c !== undefined) job.tempC = payload.temp_c;
        if (payload.elapsed_sec !== undefined) job.elapsedSec = payload.elapsed_sec;
        if (payload.badblocks_errors !== undefined) job.badblockErrs = payload.badblocks_errors;

        // Client-side speed calculation from bytes (fallback when speed_mbps not provided)
        if (phaseData.bytesDone > 0 && phaseData.bytesDone > job.lastBytesSample) {
            const dtSec = (now - job.lastSampleTime) / 1000;
            if (dtSec > 0.5) {
                const dBytes = phaseData.bytesDone - job.lastBytesSample;
                const instantSpeed = dBytes / dtSec;
                job.speedBps = job.speedBps === 0
                    ? instantSpeed
                    : job.speedBps * 0.7 + instantSpeed * 0.3;
                job.lastBytesSample = phaseData.bytesDone;
                job.lastSampleTime = now;
            }
        }

        // Compute local ETA if not provided and we have byte-level data
        if (!phaseData.eta && phaseData.bytesTotal > 0 && job.speedBps > 0) {
            const remaining = phaseData.bytesTotal - phaseData.bytesDone;
            if (remaining > 0) {
                phaseData.eta = this._formatDuration(remaining / job.speedBps);
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

        const cancelBtn = !done && this._addonId
            ? `<button class="btn-cancel-job" onclick="ProgressComponent.cancelJob('${this._escape(jobId)}')" title="Cancel job">
                   <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                   Cancel
               </button>`
            : '';

        card.innerHTML = `
            <div class="progress-job-header">
                <span class="progress-job-id">${this._escape(jobId)}</span>
                <div class="progress-job-header-actions">
                    ${cancelBtn}
                    <span class="progress-job-status ${done ? 'complete' : 'running'}">
                        ${done ? 'Complete' : 'Running'}
                    </span>
                </div>
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
                ${job.tempC > 0 ? `<span class="progress-temp">${job.tempC}°C</span>` : ''}
                <span class="progress-elapsed">${job.elapsedSec > 0 ? this._formatDuration(job.elapsedSec) : this._elapsed(job.startTime)}</span>
            </div>
            <div class="progress-phases">
                ${job.phaseOrder.map(name => this._phaseChip(name, job.phases[name], job.currentPhase)).join('')}
            </div>
        `;
    },

    async cancelJob(jobId) {
        if (!confirm(`Cancel job ${jobId}? This will abort the running operation.`)) return;

        try {
            const path = `/api/jobs/${encodeURIComponent(jobId)}`;
            const resp = await fetch(`/api/addons/${this._addonId}/proxy?path=${encodeURIComponent(path)}&method=DELETE`);
            if (resp.ok) {
                const job = this._jobs[jobId];
                if (job) {
                    job.currentPhase = 'CANCELLED';
                    job.phases['CANCELLED'] = { percent: 100, message: 'Job cancelled by user', updatedAt: Date.now() };
                    this._renderJob(jobId);
                }
            } else {
                const data = await resp.json().catch(() => ({}));
                alert(data.error || 'Failed to cancel job');
            }
        } catch {
            alert('Connection error while cancelling job');
        }
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
        // Byte-level progress (bytes_done / bytes_total)
        if (phase.bytesDone > 0 && phase.bytesTotal > 0) {
            const done = this._formatBytes(phase.bytesDone);
            const total = this._formatBytes(phase.bytesTotal);
            const speed = job.speedBps > 0 ? `${this._formatBytes(job.speedBps)}/s` : '';
            return `<span class="progress-speed">${done} / ${total}${speed ? ' · ' + speed : ''}</span>`;
        }

        // Server-provided speed in MB/s (e.g. burn-in agent)
        if (job.speedMbps > 0) {
            return `<span class="progress-speed">${job.speedMbps.toFixed(1)} MB/s</span>`;
        }

        return '';
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
