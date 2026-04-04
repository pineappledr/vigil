/**
 * Vigil Dashboard - Utility Functions
 */

const Utils = {
    formatSize(bytes) {
        if (!bytes) return 'N/A';
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${sizes[i]}`;
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

    // Parse a timestamp string, treating bare datetimes (no Z or offset) as UTC.
    // This is needed because the server stores all times in UTC without a suffix.
    parseUTC(timestamp) {
        if (!timestamp) return null;
        // If it already has timezone info (Z, +, or T...±), parse as-is
        if (/[Z+]/.test(timestamp) || /T\d{2}:\d{2}:\d{2}-/.test(timestamp)) {
            return new Date(timestamp);
        }
        // Bare datetime like "2026-02-25 23:41:24" — treat as UTC
        return new Date(timestamp + 'Z');
    },

    formatTime(timestamp) {
        const date = this.parseUTC(timestamp);
        if (!date || isNaN(date)) return 'N/A';
        return date.toLocaleTimeString([], {
            hour: '2-digit',
            minute: '2-digit'
        });
    },

    getHealthStatus(drive) {
        // SMART self-test failed → critical
        if (!drive.smart_status?.passed) return 'critical';

        const attrs = drive.ata_smart_attributes?.table || [];
        const attrMap = {};
        for (const a of attrs) attrMap[a.id] = a.raw?.value || 0;

        const reallocated = attrMap[5] || 0;     // Reallocated Sectors
        const pending = attrMap[197] || 0;        // Current Pending Sectors
        const uncorrectable = attrMap[198] || 0;  // Offline Uncorrectable
        const reported = attrMap[187] || 0;       // Reported Uncorrectable Errors

        // Critical: drive is actively failing
        if (reallocated > 100 || pending > 0 || uncorrectable > 10) return 'critical';

        // Warning: worth monitoring, not urgent
        if (reallocated > 0 || reported > 0 || uncorrectable > 0) {
            // Also check wearout if available
            return 'warning';
        }

        // Check wearout percentage from State
        if (typeof State !== 'undefined') {
            const serial = drive.serial_number;
            const hostname = drive._hostname || '';
            const w = State.getWearoutForDrive(hostname, serial);
            if (w && w.percentage > 80) return 'warning';
        }

        return 'healthy';
    },

    getDriveName(drive) {
        if (drive._alias) return drive._alias;
        if (drive.model_name) return drive.model_name;
        if (drive.model_number) return drive.model_number;
        if (drive.scsi_model_name) return drive.scsi_model_name;
        if (drive.model_family) return drive.model_family;
        if (drive.device?.model) return drive.device.model;
        
        if (drive.device?.name) {
            const name = drive.device.name.replace('/dev/', '');
            if (drive.serial_number) {
                return `${name} (${drive.serial_number.slice(-8)})`;
            }
            return name;
        }
        
        if (drive.serial_number) {
            return `Drive ${drive.serial_number.slice(-8)}`;
        }
        
        return 'Unknown Drive';
    },

    getDriveType(drive) {
        const isNvme = drive.device?.type?.toLowerCase() === 'nvme' || 
                       drive.device?.protocol === 'NVMe';
        if (isNvme) return 'NVMe';
        
        const isSsd = drive.rotation_rate === 0;
        if (isSsd) return 'SSD';
        
        if (drive.rotation_rate) return `${drive.rotation_rate} RPM`;
        return 'HDD';
    },

    // ── CSS variable helper ───────────────────────────────────────────
    getCSSVar(name) {
        return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    },

    escapeHtml(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    },

    escapeJSString(str) {
        if (!str) return '';
        return str
            .replace(/\\/g, '\\\\')
            .replace(/'/g, "\\'")
            .replace(/"/g, '\\"')
            .replace(/\n/g, '\\n')
            .replace(/\r/g, '\\r');
    },

    // ── Time-ago formatting ─────────────────────────────────────────────
    timeAgo(dateStr) {
        if (!dateStr) return 'never';
        const date = this.parseUTC(dateStr);
        if (!date || isNaN(date)) return 'never';
        if (date.getFullYear() < 2000) return 'never';
        const mins = Math.floor((Date.now() - date.getTime()) / 60000);
        if (mins < 1) return 'just now';
        if (mins < 60) return `${mins}m ago`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}h ago`;
        return `${Math.floor(hours / 24)}d ago`;
    },

    // ── Toast notifications ─────────────────────────────────────────────
    toast(message, type = 'info') {
        let container = document.getElementById('toast-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'toast-container';
            document.body.appendChild(container);
        }
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.textContent = message;
        container.appendChild(toast);
        requestAnimationFrame(() => toast.classList.add('toast-visible'));
        setTimeout(() => {
            toast.classList.remove('toast-visible');
            setTimeout(() => toast.remove(), 300);
        }, 4000);
    },

    // ── Keyed DOM reconciliation ─────────────────────────────────────────
    // Updates a container's children by data-key attribute, only touching
    // elements that actually changed. Preserves scroll position and focus.
    reconcileChildren(parent, newHTMLs, keys) {
        if (!parent || !newHTMLs) return;
        if (newHTMLs.length !== keys.length) return;

        // Build map of existing children by key; collect unkeyed children
        const existingByKey = new Map();
        const unkeyed = [];
        for (const child of parent.children) {
            const key = child.getAttribute('data-key');
            if (key) existingByKey.set(key, child);
            else unkeyed.push(child);
        }

        // Remove unkeyed children (e.g. leftover from a different view)
        for (const child of unkeyed) child.remove();

        // Track which keys are in the new set
        const newKeySet = new Set(keys);

        // Remove children whose keys are no longer present
        for (const [key, child] of existingByKey) {
            if (!newKeySet.has(key)) {
                child.remove();
                existingByKey.delete(key);
            }
        }

        // Create a temporary container to parse new HTML
        const temp = document.createElement('div');

        // Process each new item in order
        let prevSibling = null;
        for (let i = 0; i < newHTMLs.length; i++) {
            const key = keys[i];
            const existing = existingByKey.get(key);

            if (existing) {
                // Compare innerHTML (cheaper than outerHTML for large elements)
                temp.innerHTML = newHTMLs[i];
                const newEl = temp.firstElementChild;
                if (newEl && existing.outerHTML !== newEl.outerHTML) {
                    // Content changed — replace
                    existing.replaceWith(newEl);
                    existingByKey.set(key, newEl);
                    prevSibling = newEl;
                } else {
                    // No change — just ensure correct position
                    if (prevSibling) {
                        if (existing.previousElementSibling !== prevSibling) {
                            prevSibling.after(existing);
                        }
                    } else if (existing !== parent.firstElementChild) {
                        parent.prepend(existing);
                    }
                    prevSibling = existing;
                }
            } else {
                // New key — insert
                temp.innerHTML = newHTMLs[i];
                const newEl = temp.firstElementChild;
                if (newEl) {
                    newEl.setAttribute('data-key', key);
                    if (prevSibling) {
                        prevSibling.after(newEl);
                    } else {
                        parent.prepend(newEl);
                    }
                    existingByKey.set(key, newEl);
                    prevSibling = newEl;
                }
            }
        }
    },

    // ── Confirmation dialog ─────────────────────────────────────────────
    confirm(message) {
        return new Promise(resolve => {
            const overlay = document.createElement('div');
            overlay.className = 'confirm-overlay';
            overlay.innerHTML = `
                <div class="confirm-dialog">
                    <p>${this.escapeHtml(message)}</p>
                    <div class="confirm-actions">
                        <button class="btn btn-secondary confirm-cancel">Cancel</button>
                        <button class="btn btn-danger confirm-ok">Confirm</button>
                    </div>
                </div>`;
            document.body.appendChild(overlay);
            requestAnimationFrame(() => overlay.classList.add('confirm-visible'));
            const close = (result) => {
                overlay.classList.remove('confirm-visible');
                setTimeout(() => overlay.remove(), 200);
                resolve(result);
            };
            overlay.querySelector('.confirm-cancel').onclick = () => close(false);
            overlay.querySelector('.confirm-ok').onclick = () => close(true);
            overlay.addEventListener('click', (e) => { if (e.target === overlay) close(false); });
        });
    }
};