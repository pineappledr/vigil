/**
 * Vigil Dashboard - Application Initialization
 */

document.addEventListener('DOMContentLoaded', async () => {
    const isAuth = await Auth.checkStatus();
    if (!isAuth) return;
    
    Data.fetchVersion();
    Data.fetch();
    State.refreshTimer = setInterval(() => Data.fetch(), State.REFRESH_INTERVAL);
});

window.addEventListener('beforeunload', () => {
    if (State.refreshTimer) {
        clearInterval(State.refreshTimer);
    }
});
