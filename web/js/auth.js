/**
 * Vigil Dashboard - Authentication
 */

const Auth = {
    async checkStatus() {
        try {
            const response = await API.getAuthStatus();
            const data = await response.json();

            if (data.auth_enabled && !data.authenticated) {
                window.location.href = '/login.html';
                return false;
            }

            State.currentUser = data.username || null;
            State.mustChangePassword = data.must_change_password || false;
            this.updateUserUI();

            if (State.mustChangePassword) {
                setTimeout(() => Modals.showForcePasswordChange(), 500);
            }

            return true;
        } catch (e) {
            console.error('Auth check failed:', e);
            return true;
        }
    },

    updateUserUI() {
        const userMenuEl = document.getElementById('user-menu');
        if (!userMenuEl) return;

        if (!State.currentUser) {
            userMenuEl.innerHTML = '';
            return;
        }

        userMenuEl.innerHTML = `
            <div class="user-dropdown">
                <button class="user-btn" onclick="Auth.toggleMenu()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                        <circle cx="12" cy="7" r="4"/>
                    </svg>
                    <span>${State.currentUser}</span>
                    <svg class="chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="6 9 12 15 18 9"/>
                    </svg>
                </button>
                <div class="dropdown-menu" id="dropdown-menu">
                    <button onclick="Navigation.showSettings()">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="3"/>
                            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
                        </svg>
                        Settings
                    </button>
                    <button onclick="Auth.logout()">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
                            <polyline points="16 17 21 12 16 7"/>
                            <line x1="21" y1="12" x2="9" y2="12"/>
                        </svg>
                        Sign Out
                    </button>
                </div>
            </div>
        `;
    },

    toggleMenu() {
        const menu = document.getElementById('dropdown-menu');
        menu?.classList.toggle('show');
    },

    async logout() {
        try {
            await API.logout();
        } catch (e) {
            console.error('Logout error:', e);
        }
        window.location.href = '/login.html';
    }
};

// Close dropdown when clicking outside
document.addEventListener('click', (e) => {
    if (!e.target.closest('.user-dropdown')) {
        document.getElementById('dropdown-menu')?.classList.remove('show');
    }
});
