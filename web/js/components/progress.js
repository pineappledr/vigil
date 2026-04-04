/**
 * Vigil Dashboard - Progress Component (Task 4.4)
 *
 * Renders per-job progress cards with:
 *  - Phase tracker chips (pending → active → complete)
 *  - Animated progress bar with percentage
 *  - Transfer speed calculation (bytes/sec rolling average)
 *  - ETA display (from server or locally computed)
 *  - Live elapsed time counter (ticks every second)
 */

const ProgressComponent = {
    _jobs: {},       // keyed by job_id → job state
    _compIds: {},    // keyed by compId → true (for container lookup)
    _addonId: null,  // parent add-on ID for proxy calls
    _tickTimer: null, // 1-second render ticker for live elapsed/ETA
    _pollTimer: null, // periodic poll for active job updates
    _pollCompId: null, // compId used for polling

    /**
     * @param {string} compId - Manifest component ID
     * @param {Object} config - Optional: { showSpeed: true }
     * @param {number} addonId - Parent add-on ID (for cancel proxy)
     * @returns {string} HTML
     */
    render(compId, config, addonId) {
        this._compIds[compId] = true;
        if (addonId) this._addonId = addonId;

        // After DOM insertion, restore cached jobs and fetch active jobs
        // from the hub so that page navigation doesn't lose progress cards.
        setTimeout(() => {
            this._restoreJobs(compId);
            if (config?.source && addonId) {
                this._fetchActiveJobs(compId);
            }
        }, 0);

        const emptyText = (config?.source && addonId)
            ? 'Loading active jobs...'
            : 'Waiting for job data...';

        return `<div class="progress-container" id="progress-${compId}" data-comp="${compId}">
                    <div class="progress-empty">${emptyText}</div>
                </div>`;
    },

    /** Reset all state when switching addons to prevent cross-addon job leaking. */
    clearAllJobs() {
        this._jobs = {};
        this._compIds = {};
        this._addonId = null;
        if (this._tickTimer) {
            clearInterval(this._tickTimer);
            this._tickTimer = null;
        }
        if (this._pollTimer) {
            clearInterval(this._pollTimer);
            this._pollTimer = null;
        }
        this._pollCompId = null;
    },

    /** Re-render all tracked jobs into the container (e.g. after a page switch). */
    _restoreJobs(compId) {
        const container = document.getElementById(`progress-${compId}`);
        if (!container) return;

        const jobIds = Object.keys(this._jobs);
        if (jobIds.length === 0) return;

        // Remove the "Waiting for job data..." placeholder
        const empty = container.querySelector('.progress-empty');
        if (empty) empty.remove();

        for (const jobId of jobIds) {
            this._renderJob(jobId);
        }
    },

    /** Fetch active jobs from the hub API and populate the progress cards. */
    async _fetchActiveJobs(compId) {
        if (!this._addonId) return;

        try {
            let path = '/api/jobs/active';
            if (typeof ManifestRenderer !== 'undefined' && ManifestRenderer.getSelectedAgentId) {
                const agentId = ManifestRenderer.getSelectedAgentId();
                if (agentId) path += `?agent_id=${encodeURIComponent(agentId)}`;
            }
            const resp = await fetch(`/api/addons/${this._addonId}/proxy?path=${encodeURIComponent(path)}`);
            if (!resp.ok) return;

            const jobs = await resp.json();
            if (!Array.isArray(jobs)) return;

            // Reconcile: remove tracked jobs that are no longer active on the hub.
            const activeIds = new Set(jobs.map(j => j.job_id).filter(Boolean));
            for (const trackedId of Object.keys(this._jobs)) {
                if (!activeIds.has(trackedId)) {
                    this._removeJob(trackedId);
                }
            }

            if (jobs.length === 0) {
                // Show empty placeholder if nothing is running
                const container = document.getElementById(`progress-${compId}`);
                if (container && !container.querySelector('.progress-empty')) {
                    const empty = document.createElement('div');
                    empty.className = 'progress-empty';
                    empty.textContent = 'No active jobs';
                    container.appendChild(empty);
                }
                return;
            }

            // The active jobs API returns ProgressPayload objects with the
            // same shape that handleUpdate expects (job_id, command, phase,
            // percent, elapsed_sec, etc.) — pass them through directly.
            for (const job of jobs) {
                this.handleUpdate(job);
            }

            // Start polling while jobs are active so progress updates even
            // when the addon doesn't push SSE progress frames.
            this._ensurePoll(compId);
        } catch (e) {
            console.error('[Progress] Failed to fetch active jobs:', e);
        }
    },

    /** Poll for active job updates every 3 seconds while jobs exist. */
    _ensurePoll(compId) {
        if (this._pollTimer) return;
        this._pollCompId = compId;
        this._pollTimer = setInterval(() => {
            if (Object.keys(this._jobs).length === 0) {
                clearInterval(this._pollTimer);
                this._pollTimer = null;
                this._pollCompId = null;
                return;
            }
            this._fetchActiveJobs(this._pollCompId);
        }, 3000);
    },

    /** Public refresh — re-fetches active jobs from the hub. */
    refresh(compId) {
        this._fetchActiveJobs(compId);
    },

    /**
     * Handle an incoming progress telemetry event.
     * @param {Object} payload - ProgressPayload from WebSocket/SSE
     */
    handleUpdate(payload) {
        if (!payload?.job_id) return;

        // Remove job on CANCELLED or COMPLETE phase from server.
        if (payload.phase === 'CANCELLED') {
            this._removeJob(payload.job_id);
            return;
        }

        const jobId = payload.job_id;
        const now = Date.now();
        let job = this._jobs[jobId];

        if (!job) {
            job = {
                phases: {},
                phaseOrder: [],
                currentPhase: null,
                command: payload.command || '',
                indeterminate: !!payload.indeterminate,
                startTime: now,
                lastBytesSample: 0,
                lastSampleTime: now,
                speedBps: 0,
                speedMbps: 0,
                tempC: 0,
                // Elapsed: anchor to server value + timestamp for client-side ticking
                elapsedBase: 0,
                elapsedAnchor: now,
                // ETA: anchor to server value + timestamp for client-side countdown
                etaBase: 0,
                etaAnchor: now
            };
            this._jobs[jobId] = job;
        }

        // Update indeterminate flag (may change if server starts reporting progress)
        if (payload.indeterminate !== undefined) {
            job.indeterminate = !!payload.indeterminate;
        }

        // Track phase ordering
        if (payload.phase && !job.phases[payload.phase]) {
            job.phaseOrder.push(payload.phase);
        }

        job.currentPhase = payload.phase;

        // Build message from available fields
        const message = payload.message || payload.phase_detail || '';

        // Store raw eta_sec for client-side countdown
        let etaSec = 0;
        if (payload.eta_sec > 0) {
            etaSec = payload.eta_sec;
        }

        const phaseData = {
            percent: payload.percent || 0,
            message,
            etaSec,
            bytesDone: payload.bytes_done || 0,
            bytesTotal: payload.bytes_total || 0,
            updatedAt: now
        };
        job.phases[payload.phase] = phaseData;

        // Update server-provided metrics
        if (payload.speed_mbps !== undefined) job.speedMbps = payload.speed_mbps;
        if (payload.temp_c !== undefined) job.tempC = payload.temp_c;
        if (payload.badblocks_errors !== undefined) job.badblockErrs = payload.badblocks_errors;

        // Anchor elapsed time: store the server's value and the local timestamp
        if (payload.elapsed_sec !== undefined && payload.elapsed_sec > 0) {
            job.elapsedBase = payload.elapsed_sec;
            job.elapsedAnchor = now;
        }

        // Anchor ETA: store the server's value and the local timestamp for countdown
        if (etaSec > 0) {
            job.etaBase = etaSec;
            job.etaAnchor = now;
        }

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

        // Compute local ETA if server didn't provide one and we have byte-level data
        if (etaSec === 0 && phaseData.bytesTotal > 0 && job.speedBps > 0) {
            const remaining = phaseData.bytesTotal - phaseData.bytesDone;
            if (remaining > 0) {
                job.etaBase = remaining / job.speedBps;
                job.etaAnchor = now;
            }
        }

        this._renderJob(jobId);
        this._ensureTicker();
    },

    /** Compute live elapsed seconds by adding client-side delta to server anchor. */
    _liveElapsed(job) {
        if (job.elapsedBase > 0) {
            const delta = (Date.now() - job.elapsedAnchor) / 1000;
            return job.elapsedBase + delta;
        }
        return (Date.now() - job.startTime) / 1000;
    },

    /** Compute live ETA by counting down from server anchor. */
    _liveETA(job) {
        if (job.etaBase > 0) {
            const elapsed = (Date.now() - job.etaAnchor) / 1000;
            const remaining = job.etaBase - elapsed;
            return remaining > 0 ? remaining : 0;
        }
        return 0;
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
        const isIndeterminate = job.indeterminate && !done;

        const cancelBtn = !done && this._addonId
            ? `<button class="btn-cancel-job" onclick="ProgressComponent.cancelJob('${Utils.escapeHtml(jobId)}')" title="Cancel job">
                   <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                   Cancel
               </button>`
            : '';

        // Live elapsed and ETA (tick every second via the ticker)
        const elapsedStr = this._formatDuration(this._liveElapsed(job));
        const liveEta = this._liveETA(job);
        const etaStr = liveEta > 0 ? this._formatDuration(liveEta) : '';

        // Indeterminate jobs show a pulsing bar with "Running..." instead of 0.0%
        const barFillClass = done ? 'complete' : (isIndeterminate ? 'indeterminate' : '');
        const pctDisplay = isIndeterminate ? 'Running...' : `${overallPct.toFixed(1)}%`;
        const barWidth = isIndeterminate ? '100' : String(overallPct);

        card.innerHTML = `
            <div class="progress-job-header">
                <span class="progress-job-id">${Utils.escapeHtml(jobId)}</span>
                <div class="progress-job-header-actions">
                    ${cancelBtn}
                    <span class="progress-job-status ${done ? 'complete' : 'running'}">
                        ${done ? 'Complete' : 'Running'}
                    </span>
                </div>
            </div>
            <div class="progress-bar-container">
                <div class="progress-bar">
                    <div class="progress-bar-fill ${barFillClass}" style="width: ${barWidth}%"></div>
                </div>
                <span class="progress-percent">${pctDisplay}</span>
            </div>
            ${current.message ? `<div class="progress-message">${Utils.escapeHtml(current.message)}</div>` : ''}
            <div class="progress-meta">
                ${this._speedDisplay(current, job)}
                ${etaStr ? `<span class="progress-eta">ETA: ${Utils.escapeHtml(etaStr)}</span>` : ''}
                ${job.tempC > 0 ? `<span class="progress-temp">${job.tempC}°C</span>` : ''}
                <span class="progress-elapsed">${elapsedStr}</span>
            </div>
            <div class="progress-phases">
                ${job.phaseOrder.map(name => this._phaseChip(name, job.phases[name], job.currentPhase, isIndeterminate)).join('')}
            </div>
        `;
    },

    /** Start a 1-second ticker to keep elapsed/ETA live on screen. */
    _ensureTicker() {
        if (this._tickTimer) return;
        this._tickTimer = setInterval(() => {
            const activeJobs = Object.entries(this._jobs).filter(([_, job]) => {
                const pct = this._overallPercent(job);
                return pct < 100;
            });
            if (activeJobs.length === 0) {
                clearInterval(this._tickTimer);
                this._tickTimer = null;
                return;
            }
            for (const [jobId] of activeJobs) {
                this._renderJob(jobId);
            }
        }, 1000);
    },

    async cancelJob(jobId) {
        if (!confirm(`Cancel job ${jobId}? This will abort the running operation.`)) return;

        try {
            const path = `/api/jobs/${encodeURIComponent(jobId)}`;
            const resp = await fetch(`/api/addons/${this._addonId}/proxy?path=${encodeURIComponent(path)}&method=DELETE`);
            if (resp.ok || resp.status === 404) {
                // Remove locally — 404 means the job doesn't exist on the
                // hub (stale/ghost entry), so just clean up the UI.
                this._removeJob(jobId);
            } else {
                const data = await resp.json().catch(() => ({}));
                alert(data.error || 'Failed to cancel job');
            }
        } catch {
            alert('Connection error while cancelling job');
        }
    },

    /** Remove a job card from the DOM and internal state. */
    _removeJob(jobId) {
        delete this._jobs[jobId];
        const card = document.getElementById(`job-${this._cssId(jobId)}`);
        if (card) card.remove();

        // If no active jobs remain, show the empty placeholder.
        if (Object.keys(this._jobs).length === 0) {
            const container = document.querySelector('.progress-container');
            if (container && !container.querySelector('.progress-empty')) {
                const empty = document.createElement('div');
                empty.className = 'progress-empty';
                empty.textContent = 'No active jobs';
                container.appendChild(empty);
            }
        }
    },

    _phaseChip(name, phase, currentPhase, indeterminate) {
        let status = 'pending';
        if (phase.percent >= 100) status = 'complete';
        else if (name === currentPhase) status = 'active';

        const pctLabel = (indeterminate && phase.percent === 0 && name === currentPhase)
            ? '' : `${phase.percent.toFixed(0)}%`;

        return `<div class="progress-phase ${status}">
                    <span class="progress-phase-dot"></span>
                    <span class="progress-phase-name">${Utils.escapeHtml(name)}</span>
                    <span class="progress-phase-pct">${pctLabel}</span>
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

    _formatDuration(totalSec) {
        const s = Math.floor(totalSec);
        const h = Math.floor(s / 3600);
        const m = Math.floor((s % 3600) / 60);
        const sec = s % 60;
        if (h > 0) return `${h}h ${m}m ${sec}s`;
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

};
