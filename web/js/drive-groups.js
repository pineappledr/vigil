/**
 * Vigil Dashboard - Drive Groups Management
 */

const DriveGroups = {
    _groups: [],
    _expanded: null, // group ID currently expanded

    async load() {
        try {
            const [gResp, aResp] = await Promise.all([
                API.getDriveGroups(),
                API.getDriveGroupAssignments()
            ]);
            this._groups = gResp.ok ? await gResp.json() : [];
            const assignments = aResp.ok ? await aResp.json() : {};
            State.driveGroups = this._groups;
            State.driveGroupAssignments = assignments;
            State.buildDriveGroupMap();
        } catch (_) {
            this._groups = [];
        }
        this.render();
    },

    render() {
        const listEl = document.getElementById('drive-groups-list');
        const createEl = document.getElementById('drive-groups-create');
        if (!listEl || !createEl) return;

        if (this._groups.length === 0) {
            listEl.innerHTML = '<div class="settings-item"><div class="settings-item-info"><div class="settings-item-desc" style="opacity:0.6">No groups yet</div></div></div>';
        } else {
            listEl.innerHTML = '<div class="group-list">' + this._groups.map(g => this._groupCard(g)).join('') + '</div>';
        }

        createEl.innerHTML = `
            <div class="create-group-form">
                <input type="text" id="new-group-name" placeholder="New group name" maxlength="64">
                <input type="color" id="new-group-color" value="#6366f1" title="Group color">
                <button class="btn btn-secondary" onclick="DriveGroups.create()">Create Group</button>
            </div>
        `;
    },

    _groupCard(g) {
        const expanded = this._expanded === g.id;
        return `
            <div class="group-card">
                <div class="group-card-header">
                    <span class="group-color-dot" style="background:${Utils.escapeHtml(g.color)}"></span>
                    <span class="group-card-name">${Utils.escapeHtml(g.name)}</span>
                    <span class="group-member-count">${g.member_count || 0} drive${(g.member_count || 0) !== 1 ? 's' : ''}</span>
                    <div class="group-card-actions">
                        <button onclick="DriveGroups.toggle(${g.id})" title="${expanded ? 'Collapse' : 'Expand'}">${expanded ? '▲' : '▼'}</button>
                        <button onclick="DriveGroups.edit(${g.id}, '${Utils.escapeJSString(g.name)}', '${Utils.escapeJSString(g.color)}')" title="Edit">✏️</button>
                        <button class="danger" onclick="DriveGroups.remove(${g.id}, '${Utils.escapeJSString(g.name)}')" title="Delete">🗑️</button>
                    </div>
                </div>
                ${expanded ? `<div class="group-members" id="group-members-${g.id}">Loading...</div>` : ''}
            </div>
        `;
    },

    async toggle(id) {
        this._expanded = this._expanded === id ? null : id;
        this.render();
        if (this._expanded === id) {
            await this._loadMembers(id);
        }
    },

    async _loadMembers(groupId) {
        const el = document.getElementById(`group-members-${groupId}`);
        if (!el) return;

        try {
            const resp = await API.getDriveGroup(groupId);
            if (!resp.ok) { el.innerHTML = 'Failed to load'; return; }
            const data = await resp.json();
            const members = data.members || [];

            let html = '';
            if (members.length === 0) {
                html = '<div class="group-member-item"><span class="member-drive" style="opacity:0.6">No drives assigned</span></div>';
            } else {
                html = members.map(m => {
                    const alias = this._driveAlias(m.hostname, m.serial_number);
                    const display = alias ? `${m.hostname} / ${alias} (${m.serial_number})` : `${m.hostname} / ${m.serial_number}`;
                    return `
                    <div class="group-member-item">
                        <span class="member-drive">${Utils.escapeHtml(display)}</span>
                        <button class="remove-member" onclick="DriveGroups.unassign('${Utils.escapeJSString(m.hostname)}', '${Utils.escapeJSString(m.serial_number)}')">Remove</button>
                    </div>`;
                }).join('');
            }

            // Add assign dropdown
            html += this._assignDropdown(groupId, members);
            el.innerHTML = html;
        } catch (_) {
            el.innerHTML = 'Failed to load';
        }
    },

    _assignDropdown(groupId, currentMembers) {
        // Build list of all drives not already in this group
        const assigned = new Set(currentMembers.map(m => `${m.hostname}:${m.serial_number}`));
        const drives = [];
        (State.data || []).forEach(server => {
            (server.details?.drives || []).forEach(d => {
                const serial = d.serial_number || '';
                if (serial && !assigned.has(`${server.hostname}:${serial}`)) {
                    const alias = d._alias || '';
                    const model = d.model_name || d.device?.name || serial;
                    const name = alias || model;
                    drives.push({ hostname: server.hostname, serial, label: `${server.hostname} / ${name} (${serial})` });
                }
            });
        });

        if (drives.length === 0) return '';

        const options = drives.map(d =>
            `<option value="${Utils.escapeHtml(d.hostname)}|${Utils.escapeHtml(d.serial)}">${Utils.escapeHtml(d.label)}</option>`
        ).join('');

        return `
            <select class="assign-drive-select" onchange="DriveGroups.assign(${groupId}, this.value); this.value=''">
                <option value="">+ Add drive to group...</option>
                ${options}
            </select>
        `;
    },

    async create() {
        const nameEl = document.getElementById('new-group-name');
        const colorEl = document.getElementById('new-group-color');
        const name = (nameEl?.value || '').trim();
        const color = colorEl?.value || '#6366f1';
        if (!name) { Utils.toast('Group name is required', 'error'); return; }

        try {
            const resp = await API.createDriveGroup(name, color);
            if (resp.ok) {
                Utils.toast(`Group "${name}" created`, 'success');
                if (nameEl) nameEl.value = '';
                await this.load();
            } else {
                const err = await resp.json().catch(() => ({}));
                Utils.toast(err.error || 'Failed to create group', 'error');
            }
        } catch (_) {
            Utils.toast('Failed to create group', 'error');
        }
    },

    async edit(id, currentName, currentColor) {
        const name = prompt('Group name:', currentName);
        if (name === null) return;
        const trimmed = name.trim();
        if (!trimmed) { Utils.toast('Name cannot be empty', 'error'); return; }

        const color = prompt('Color (hex):', currentColor) || currentColor;

        try {
            const resp = await API.updateDriveGroup(id, trimmed, color);
            if (resp.ok) {
                Utils.toast('Group updated', 'success');
                await this.load();
            } else {
                Utils.toast('Failed to update group', 'error');
            }
        } catch (_) {
            Utils.toast('Failed to update group', 'error');
        }
    },

    async remove(id, name) {
        if (!confirm(`Delete group "${name}"? Drives will be unassigned and group notification rules will be removed.`)) return;

        try {
            const resp = await API.deleteDriveGroup(id);
            if (resp.ok) {
                Utils.toast(`Group "${name}" deleted`, 'success');
                this._expanded = null;
                await this.load();
            } else {
                Utils.toast('Failed to delete group', 'error');
            }
        } catch (_) {
            Utils.toast('Failed to delete group', 'error');
        }
    },

    async assign(groupId, value) {
        if (!value) return;
        const [hostname, serial] = value.split('|');
        try {
            const resp = await API.assignDriveToGroup(groupId, hostname, serial);
            if (resp.ok) {
                Utils.toast('Drive assigned', 'success');
                await this.load();
                if (this._expanded === groupId) {
                    // Re-render will reload members
                }
            } else {
                Utils.toast('Failed to assign drive', 'error');
            }
        } catch (_) {
            Utils.toast('Failed to assign drive', 'error');
        }
    },

    async unassign(hostname, serial) {
        try {
            const resp = await API.unassignDrive(hostname, serial);
            if (resp.ok) {
                Utils.toast('Drive removed from group', 'success');
                await this.load();
            } else {
                Utils.toast('Failed to remove drive', 'error');
            }
        } catch (_) {
            Utils.toast('Failed to remove drive', 'error');
        }
    },

    _driveAlias(hostname, serial) {
        for (const server of (State.data || [])) {
            if (server.hostname !== hostname) continue;
            for (const d of (server.details?.drives || [])) {
                if (d.serial_number === serial) {
                    return d._alias || '';
                }
            }
        }
        return '';
    }
};
