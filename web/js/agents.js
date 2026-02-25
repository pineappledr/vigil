/**
 * Vigil Dashboard - Agent Management
 */

const Agents = {
    async render() {
        const container = document.getElementById('dashboard-view');
        if (!container) return;

        container.innerHTML = '<div class="loading-spinner"><div class="spinner"></div>Loading agents...</div>';

        let agents = [];
        let tokens = [];

        try {
            const [agentsResp, tokensResp] = await Promise.all([
                API.getAgents(),
                API.getRegistrationTokens()
            ]);

            if (agentsResp.ok) {
                const data = await agentsResp.json();
                agents = data.agents || [];
            }
            if (tokensResp.ok) {
                const data = await tokensResp.json();
                tokens = data.tokens || [];
            }
        } catch (e) {
            console.error('Failed to load agents:', e);
        }

        container.innerHTML = this._buildView(agents, tokens);
    },

    _buildView(agents, tokens) {
        const agentCards = agents.length > 0
            ? agents.map(a => this._agentCard(a)).join('')
            : this._emptyState();

        const tokenRows = tokens.length > 0
            ? tokens.map(t => this._tokenRow(t)).join('')
            : '<div class="token-row" style="justify-content:center; color:var(--text-muted);">No registration tokens</div>';

        return `
            <div class="agents-header">
                <h2>Systems</h2>
                <button class="btn-add-agent" onclick="Modals.showAddAgent()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="12" y1="5" x2="12" y2="19"/>
                        <line x1="5" y1="12" x2="19" y2="12"/>
                    </svg>
                    Add System
                </button>
            </div>
            <div class="agents-grid">${agentCards}</div>
            <div class="tokens-section">
                <h3>Registration Tokens</h3>
                <div class="tokens-grid">${tokenRows}</div>
            </div>
        `;
    },

    _agentCard(agent) {
        // Use the most recent activity timestamp (report or auth)
        const lastActivity = this._mostRecent(agent.last_seen_at, agent.last_auth_at);
        const lastSeen = lastActivity ? this._timeAgo(lastActivity) : null;
        const lastDate = lastActivity ? Utils.parseUTC(lastActivity) : null;
        const isOnline = lastDate && (Date.now() - lastDate.getTime()) < 5 * 60 * 1000;
        const statusClass = isOnline ? 'online' : 'not-reporting';
        const statusLabel = isOnline ? 'Online' : 'Not Reporting';
        const fp = agent.fingerprint ? agent.fingerprint.substring(0, 16) + '...' : '';
        const displayName = agent.name || agent.hostname;
        const showHostname = agent.name && agent.name !== agent.hostname;

        let statusHint = '';
        if (!isOnline) {
            statusHint = lastActivity
                ? 'Agent has not sent data in over 5 minutes. Check if the agent service is running or re-register it.'
                : 'Agent was registered but has never connected. Verify the agent service is running on this system.';
        }

        return `
            <div class="agent-card ${isOnline ? 'agent-online' : 'agent-not-reporting'}">
                <div class="agent-card-top">
                    <div class="agent-card-left">
                        <div class="agent-icon">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <rect x="2" y="3" width="20" height="7" rx="1"/>
                                <rect x="2" y="14" width="20" height="7" rx="1"/>
                                <circle cx="6" cy="6.5" r="1.5" fill="currentColor"/>
                                <circle cx="6" cy="17.5" r="1.5" fill="currentColor"/>
                            </svg>
                        </div>
                        <div class="agent-info">
                            <h4>${this._escape(displayName)}</h4>
                            <div class="agent-info-meta">
                                ${showHostname ? `<span>${this._escape(agent.hostname)}</span><span class="dot"></span>` : ''}
                                <span>${fp}</span>
                                <span class="dot"></span>
                                <span>${lastSeen ? 'Last seen ' + lastSeen : 'Never connected'}</span>
                            </div>
                        </div>
                    </div>
                    <div class="agent-card-right">
                        <span class="agent-status-wrap">
                            <span class="agent-status ${statusClass}">${statusLabel}</span>
                            ${statusHint ? `<span class="agent-status-tooltip">${statusHint}</span>` : ''}
                        </span>
                        <button class="btn-agent-delete" onclick="Agents.deleteAgent(${agent.id}, '${this._escape(agent.hostname)}')" title="Remove agent">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polyline points="3 6 5 6 21 6"/>
                                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                            </svg>
                        </button>
                    </div>
                </div>
            </div>
        `;
    },

    _tokenRow(token) {
        const truncated = token.token.substring(0, 16) + '...';
        const now = new Date();
        const isUsed = !!token.used_at;
        const isExpired = token.expires_at
            ? new Date(token.expires_at + 'Z') < now  // stored as UTC, append Z so JS parses as UTC
            : false;  // null expires_at = never expires

        let badgeClass = 'available';
        let badgeLabel = 'Available';
        if (isUsed) { badgeClass = 'used'; badgeLabel = 'Used'; }
        else if (isExpired) { badgeClass = 'expired'; badgeLabel = 'Expired'; }

        return `
            <div class="token-row">
                <span class="token-value">${truncated}</span>
                <div class="token-meta">
                    <span class="token-badge ${badgeClass}">${badgeLabel}</span>
                    <span>${this._timeAgo(token.created_at)}</span>
                    <button class="btn-agent-delete" onclick="Agents.deleteToken(${token.id})" title="Delete token">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
            </div>
        `;
    },

    _emptyState() {
        return `
            <div class="agents-empty">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <rect x="2" y="2" width="20" height="8" rx="2"/>
                    <rect x="2" y="14" width="20" height="8" rx="2"/>
                    <circle cx="6" cy="6" r="1"/><circle cx="6" cy="18" r="1"/>
                </svg>
                <p>No agents registered yet</p>
                <span class="hint">Click "Add System" to register your first agent</span>
            </div>
        `;
    },

    async deleteAgent(id, hostname) {
        if (!confirm(`Remove agent "${hostname}"? It will need to re-register.`)) return;
        try {
            await API.deleteAgent(id);
            this.render();
        } catch (e) {
            console.error('Failed to delete agent:', e);
        }
    },

    async deleteToken(id) {
        try {
            await API.deleteRegistrationToken(id);
            this.render();
        } catch (e) {
            console.error('Failed to delete token:', e);
        }
    },

    _mostRecent(...dates) {
        let best = null;
        for (const d of dates) {
            if (!d) continue;
            const t = Utils.parseUTC(d);
            if (!t || isNaN(t)) continue;
            // Guard against Go zero-time (year 1 or earlier)
            if (t.getFullYear() < 2000) continue;
            if (!best || t > best) best = t;
        }
        return best ? best.toISOString() : null;
    },

    _timeAgo(dateStr) {
        if (!dateStr) return 'never';
        const date = Utils.parseUTC(dateStr);
        if (!date || isNaN(date)) return 'never';
        // Guard against Go zero-time
        if (date.getFullYear() < 2000) return 'never';
        const diff = Date.now() - date.getTime();
        const mins = Math.floor(diff / 60000);
        if (mins < 1) return 'just now';
        if (mins < 60) return `${mins}m ago`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}h ago`;
        const days = Math.floor(hours / 24);
        return `${days}d ago`;
    },

    _escape(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};
