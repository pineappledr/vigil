package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"vigil/internal/db"
	"vigil/internal/models"
)

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword verifies a password against a stored hash.
// Supports both bcrypt and legacy SHA256 hashes (auto-upgrades on match).
func CheckPassword(userID int, storedHash, password string) bool {
	if isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}

	// Legacy SHA256 comparison
	legacyHash := sha256.Sum256([]byte(password))
	if hex.EncodeToString(legacyHash[:]) != storedHash {
		return false
	}

	// Auto-upgrade to bcrypt
	newHash, err := HashPassword(password)
	if err == nil {
		db.DB.Exec("UPDATE users SET password_hash = ? WHERE id = ?", newHash, userID)
		log.Printf("🔐 Upgraded password hash to bcrypt for user %d", userID)
	}
	return true
}

func isBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$")
}

// GenerateToken creates a secure random token
func GenerateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GetSession retrieves a session by token
func GetSession(token string) *models.Session {
	if token == "" {
		return nil
	}

	var session models.Session
	var expiresAt string

	err := db.DB.QueryRow(`
		SELECT s.token, s.user_id, u.username, s.expires_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > datetime('now')
	`, token).Scan(&session.Token, &session.UserID, &session.Username, &expiresAt)

	if err != nil {
		return nil
	}

	session.ExpiresAt, _ = time.Parse("2006-01-02 15:04:05", expiresAt)
	return &session
}

// CreateSession creates a new session for a user
func CreateSession(userID int) (string, time.Time, error) {
	token := GenerateToken()
	expiresAt := time.Now().Add(24 * time.Hour * 7)

	_, err := db.DB.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expiresAt.Format("2006-01-02 15:04:05"),
	)
	return token, expiresAt, err
}

// DeleteSession removes a session
func DeleteSession(token string) {
	db.DB.Exec("DELETE FROM sessions WHERE token = ?", token)
}

// CleanupExpiredSessions removes expired sessions from the database
func CleanupExpiredSessions() {
	db.DB.Exec("DELETE FROM sessions WHERE expires_at < datetime('now')")
}

// CreateDefaultAdmin creates the initial admin user if none exists
func CreateDefaultAdmin(config models.Config) {
	var count int
	db.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count > 0 {
		return
	}

	password := config.AdminPass
	mustChange := 1

	if password == "" {
		password = GenerateToken()[:12]
		log.Printf("🔑 Generated admin password: %s", password)
		log.Printf("   Set ADMIN_PASS environment variable to use a custom password")
	} else {
		mustChange = 0
	}

	hash, err := HashPassword(password)
	if err != nil {
		log.Printf("⚠️  Could not hash admin password: %v", err)
		return
	}

	_, err = db.DB.Exec(
		"INSERT INTO users (username, password_hash, must_change_password) VALUES (?, ?, ?)",
		config.AdminUser, hash, mustChange,
	)
	if err != nil {
		log.Printf("⚠️  Could not create admin user: %v", err)
	} else {
		log.Printf("✓ Created admin user: %s", config.AdminUser)
	}
}
