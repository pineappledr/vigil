package audit

import (
	"database/sql"
	"log"
	"net/http"

	"vigil/internal/middleware"
)

// LogEvent records an action in the audit_log table.
// userID and username should be extracted from the session by the caller
// (to avoid circular imports with the auth package).
// Pass userID=0 and username="" for unauthenticated actions (e.g. login attempts).
func LogEvent(db *sql.DB, r *http.Request, userID int, username, action, resource, resourceID, details, status string) {
	ip := middleware.ExtractIP(r)
	ua := r.Header.Get("User-Agent")

	var uid interface{} = userID
	if userID == 0 {
		uid = nil
	}

	_, err := db.Exec(`
		INSERT INTO audit_log (user_id, username, action, resource, resource_id, details, ip_address, user_agent, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uid, username, action, resource, resourceID, details, ip, ua, status)
	if err != nil {
		log.Printf("⚠️  audit.LogEvent: %v", err)
	}
}
