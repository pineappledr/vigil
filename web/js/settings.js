/**
 * Vigil Dashboard - Settings Management
 */

const Settings = {
    async loadRetention() {
        try {
            const resp = await API.get('/api/settings/retention');
            if (!resp.ok) return;
            const items = await resp.json();
            const map = {};
            for (const s of items) map[s.key] = s.value;

            const fields = {
                'retention-notification-days': 'notification_history_days',
                'retention-smart-days': 'smart_data_days',
                'retention-host-limit': 'host_history_limit',
                'retention-notify-limit': 'notification_display_limit',
            };
            for (const [id, key] of Object.entries(fields)) {
                const el = document.getElementById(id);
                if (el && map[key]) el.value = map[key];
            }
        } catch { /* settings page may not be visible */ }
    },

    async saveRetention(key, value) {
        try {
            const resp = await API.put(`/api/settings/retention/${key}`, { value: String(value) });
            if (resp.ok) {
                Utils.toast('Setting saved', 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                Utils.toast(data.error || 'Failed to save', 'error');
            }
        } catch {
            Utils.toast('Failed to save setting', 'error');
        }
    },

    async loadBackup() {
        try {
            const resp = await API.get('/api/settings/backup');
            if (!resp.ok) return;
            const items = await resp.json();
            const map = {};
            for (const s of items) map[s.key] = s.value;

            const el = (id) => document.getElementById(id);
            if (map.enabled) { const cb = el('backup-enabled'); if (cb) cb.checked = map.enabled === 'true'; }
            if (map.interval_hours) { const inp = el('backup-interval'); if (inp) inp.value = map.interval_hours; }
            if (map.max_backups) { const inp = el('backup-max'); if (inp) inp.value = map.max_backups; }
        } catch { /* settings page may not be visible */ }
    },

    async saveBackupSetting(key, value) {
        try {
            const resp = await API.put(`/api/settings/backup/${key}`, { value: String(value) });
            if (resp.ok) {
                Utils.toast('Setting saved', 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                Utils.toast(data.error || 'Failed to save', 'error');
            }
        } catch {
            Utils.toast('Failed to save setting', 'error');
        }
    },

    async triggerBackup() {
        const btn = document.getElementById('backup-now-btn');
        if (btn) { btn.disabled = true; btn.textContent = 'Backing up...'; }
        try {
            const resp = await API.post('/api/backup', {});
            if (resp.ok) {
                const info = await resp.json();
                Utils.toast(`Backup created: ${info.filename}`, 'success');
                this.loadBackupList();
            } else {
                const data = await resp.json().catch(() => ({}));
                Utils.toast(data.error || 'Backup failed', 'error');
            }
        } catch {
            Utils.toast('Backup failed', 'error');
        } finally {
            if (btn) { btn.disabled = false; btn.textContent = 'Backup Now'; }
        }
    },

    async loadBackupList() {
        const container = document.getElementById('backup-list');
        if (!container) return;
        try {
            const resp = await API.get('/api/backups');
            if (!resp.ok) return;
            const backups = await resp.json();
            if (!backups.length) {
                container.innerHTML = '<div class="settings-item"><div class="settings-item-info"><div class="settings-item-desc">No backups yet</div></div></div>';
                return;
            }
            container.innerHTML = backups.map(b => {
                const size = Utils.formatSize(b.size_bytes);
                const age = Utils.timeAgo(b.created_at);
                return `<div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">${Utils.escapeHtml(b.filename)}</div>
                        <div class="settings-item-desc">${size} &middot; ${age}</div>
                    </div>
                    <div class="backup-actions">
                        <a class="btn btn-secondary btn-sm" href="/api/backups/${encodeURIComponent(b.filename)}/download" download title="Download">Download</a>
                        <button class="btn btn-secondary btn-sm" onclick="Settings.restoreBackup('${Utils.escapeJSString(b.filename)}')" title="Restore">Restore</button>
                        <button class="btn btn-danger btn-sm" onclick="Settings.deleteBackup('${Utils.escapeJSString(b.filename)}')">Delete</button>
                    </div>
                </div>`;
            }).join('');
        } catch { /* ignore */ }
    },

    async deleteBackup(filename) {
        if (!await Utils.confirm(`Delete backup ${filename}?`)) return;
        try {
            const resp = await API.delete(`/api/backups/${encodeURIComponent(filename)}`);
            if (resp.ok) {
                Utils.toast('Backup deleted', 'success');
                this.loadBackupList();
            } else {
                Utils.toast('Failed to delete backup', 'error');
            }
        } catch {
            Utils.toast('Failed to delete backup', 'error');
        }
    },

    async restoreBackup(filename) {
        if (!confirm(`Restore database from "${filename}"?\n\nA safety backup of the current database will be created automatically.\n\nThe page will reload after restore.`)) return;
        try {
            // Download the backup file, then upload it to the restore endpoint
            const dlResp = await fetch(`/api/backups/${encodeURIComponent(filename)}/download`);
            if (!dlResp.ok) { Utils.toast('Failed to fetch backup file', 'error'); return; }
            const blob = await dlResp.blob();
            const form = new FormData();
            form.append('backup', blob, filename);
            const resp = await fetch('/api/backups/restore', { method: 'POST', headers: { 'X-Requested-With': 'XMLHttpRequest' }, body: form });
            if (resp.ok) {
                Utils.toast('Database restored. Reloading...', 'success');
                setTimeout(() => location.reload(), 1500);
            } else {
                const err = await resp.json().catch(() => ({}));
                Utils.toast(err.error || 'Restore failed', 'error');
            }
        } catch {
            Utils.toast('Restore failed', 'error');
        }
    },

    restoreFromFile() {
        const input = document.createElement('input');
        input.type = 'file';
        input.accept = '.db';
        input.onchange = async () => {
            const file = input.files[0];
            if (!file) return;
            if (!confirm(`Restore database from "${file.name}"?\n\nA safety backup of the current database will be created automatically.\n\nThe page will reload after restore.`)) return;
            const form = new FormData();
            form.append('backup', file);
            try {
                const resp = await fetch('/api/backups/restore', { method: 'POST', headers: { 'X-Requested-With': 'XMLHttpRequest' }, body: form });
                if (resp.ok) {
                    Utils.toast('Database restored. Reloading...', 'success');
                    setTimeout(() => location.reload(), 1500);
                } else {
                    const err = await resp.json().catch(() => ({}));
                    Utils.toast(err.error || 'Restore failed', 'error');
                }
            } catch {
                Utils.toast('Restore failed', 'error');
            }
        };
        input.click();
    },

    async loadStats() {
        const container = document.getElementById('system-stats');
        if (!container) return;
        try {
            const resp = await API.get('/api/stats');
            if (!resp.ok) return;
            const s = await resp.json();

            const uptime = this._formatUptime(s.uptime_seconds || 0);
            const dbSize = Utils.formatSize(s.db_size_bytes);
            const latency = s.report_processing_ms;
            const latencyStr = latency
                ? `${latency.avg}ms avg / ${latency.p95}ms p95 (${latency.samples} samples)`
                : 'No data yet';

            container.innerHTML = `
                <div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">Uptime</div>
                        <div class="settings-item-desc">${uptime}</div>
                    </div>
                </div>
                <div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">Report Queue</div>
                        <div class="settings-item-desc">${s.report_queue_depth} pending</div>
                    </div>
                </div>
                <div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">Reports Processed</div>
                        <div class="settings-item-desc">${s.reports_processed_total} total, ${s.reports_dropped_total} dropped</div>
                    </div>
                </div>
                <div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">Report Processing</div>
                        <div class="settings-item-desc">${latencyStr}</div>
                    </div>
                </div>
                <div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">Active Agents</div>
                        <div class="settings-item-desc">${s.active_agents}</div>
                    </div>
                </div>
                <div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">Notifications</div>
                        <div class="settings-item-desc">${s.notifications_sent_total} sent, ${s.notifications_failed_total} failed</div>
                    </div>
                </div>
                <div class="settings-item">
                    <div class="settings-item-info">
                        <div class="settings-item-title">Database Size</div>
                        <div class="settings-item-desc">${dbSize}</div>
                    </div>
                </div>`;
        } catch { /* ignore */ }
    },

    _formatUptime(seconds) {
        const d = Math.floor(seconds / 86400);
        const h = Math.floor((seconds % 86400) / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        if (d > 0) return `${d}d ${h}h ${m}m`;
        if (h > 0) return `${h}h ${m}m`;
        return `${m}m`;
    },

    async loadAll() {
        await Promise.all([
            this.loadRetention(),
            this.loadBackup(),
            this.loadBackupList(),
            this.loadStats(),
            typeof DriveGroups !== 'undefined' ? DriveGroups.load() : Promise.resolve()
        ]);
    }
};
