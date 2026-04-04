package drivegroups

import (
	"database/sql"
	"fmt"
	"time"
)

// ── Group CRUD ──────────────────────────────────────────────────────────

// CreateGroup inserts a new drive group and returns its ID.
func CreateGroup(db *sql.DB, g *DriveGroup) (int64, error) {
	color := g.Color
	if color == "" {
		color = "#6366f1"
	}
	res, err := db.Exec(
		`INSERT INTO drive_groups (name, color) VALUES (?, ?)`,
		g.Name, color,
	)
	if err != nil {
		return 0, fmt.Errorf("create group: %w", err)
	}
	return res.LastInsertId()
}

// UpdateGroup updates a group's name and color.
func UpdateGroup(db *sql.DB, g *DriveGroup) error {
	_, err := db.Exec(
		`UPDATE drive_groups SET name = ?, color = ? WHERE id = ?`,
		g.Name, g.Color, g.ID,
	)
	return err
}

// DeleteGroup removes a group. Members and group event rules are cascade-deleted.
func DeleteGroup(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM drive_groups WHERE id = ?`, id)
	return err
}

// ListGroups returns all groups with member counts.
func ListGroups(db *sql.DB) ([]DriveGroup, error) {
	rows, err := db.Query(`
		SELECT g.id, g.name, g.color, g.created_at, COUNT(m.id)
		FROM drive_groups g
		LEFT JOIN drive_group_members m ON m.group_id = g.id
		GROUP BY g.id
		ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []DriveGroup
	for rows.Next() {
		var g DriveGroup
		var ts string
		if err := rows.Scan(&g.ID, &g.Name, &g.Color, &ts, &g.MemberCount); err != nil {
			continue
		}
		g.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
		groups = append(groups, g)
	}
	return groups, nil
}

// GetGroup returns a single group by ID, or nil if not found.
func GetGroup(db *sql.DB, id int64) (*DriveGroup, error) {
	var g DriveGroup
	var ts string
	err := db.QueryRow(
		`SELECT id, name, color, created_at FROM drive_groups WHERE id = ?`, id,
	).Scan(&g.ID, &g.Name, &g.Color, &ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	g.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
	return &g, nil
}

// ── Member Management ───────────────────────────────────────────────────

// AssignDrive adds a drive to a group. If the drive is already in a different
// group, it's moved to the new group.
func AssignDrive(db *sql.DB, groupID int64, hostname, serial string) error {
	_, err := db.Exec(`
		INSERT INTO drive_group_members (group_id, hostname, serial_number)
		VALUES (?, ?, ?)
		ON CONFLICT(hostname, serial_number) DO UPDATE SET group_id = excluded.group_id`,
		groupID, hostname, serial,
	)
	return err
}

// UnassignDrive removes a drive from any group.
func UnassignDrive(db *sql.DB, hostname, serial string) error {
	_, err := db.Exec(
		`DELETE FROM drive_group_members WHERE hostname = ? AND serial_number = ?`,
		hostname, serial,
	)
	return err
}

// ListGroupMembers returns all drives in a group.
func ListGroupMembers(db *sql.DB, groupID int64) ([]DriveGroupMember, error) {
	rows, err := db.Query(
		`SELECT id, group_id, hostname, serial_number FROM drive_group_members WHERE group_id = ? ORDER BY hostname, serial_number`,
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []DriveGroupMember
	for rows.Next() {
		var m DriveGroupMember
		if err := rows.Scan(&m.ID, &m.GroupID, &m.Hostname, &m.SerialNumber); err != nil {
			continue
		}
		members = append(members, m)
	}
	return members, nil
}

// GetDriveGroup returns the group ID for a drive, or nil if unassigned.
func GetDriveGroup(db *sql.DB, hostname, serial string) (*int64, error) {
	var groupID int64
	err := db.QueryRow(
		`SELECT group_id FROM drive_group_members WHERE hostname = ? AND serial_number = ?`,
		hostname, serial,
	).Scan(&groupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &groupID, nil
}

// ListAllAssignments returns a map of "hostname:serial" -> groupID for all assigned drives.
func ListAllAssignments(db *sql.DB) (map[string]int64, error) {
	rows, err := db.Query(`SELECT hostname, serial_number, group_id FROM drive_group_members`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]int64)
	for rows.Next() {
		var host, serial string
		var gid int64
		if err := rows.Scan(&host, &serial, &gid); err != nil {
			continue
		}
		m[host+":"+serial] = gid
	}
	return m, nil
}

// ── Group Event Rules ───────────────────────────────────────────────────

// UpsertGroupEventRule creates or updates a group-specific event rule.
func UpsertGroupEventRule(db *sql.DB, r *GroupEventRule) error {
	_, err := db.Exec(`
		INSERT INTO notification_group_event_rules (service_id, group_id, event_type, enabled, cooldown_secs)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(service_id, group_id, event_type)
		DO UPDATE SET enabled = excluded.enabled, cooldown_secs = excluded.cooldown_secs`,
		r.ServiceID, r.GroupID, r.EventType, r.Enabled, r.Cooldown,
	)
	return err
}

// GetGroupEventRules returns event rules for a specific service + group combination.
func GetGroupEventRules(db *sql.DB, serviceID, groupID int64) ([]GroupEventRule, error) {
	rows, err := db.Query(`
		SELECT id, service_id, group_id, event_type, enabled, cooldown_secs
		FROM notification_group_event_rules
		WHERE service_id = ? AND group_id = ?`,
		serviceID, groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []GroupEventRule
	for rows.Next() {
		var r GroupEventRule
		if err := rows.Scan(&r.ID, &r.ServiceID, &r.GroupID, &r.EventType, &r.Enabled, &r.Cooldown); err != nil {
			continue
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// GetAllGroupRulesForService returns all group rules for a service, keyed by group ID.
func GetAllGroupRulesForService(db *sql.DB, serviceID int64) (map[int64][]GroupEventRule, error) {
	rows, err := db.Query(`
		SELECT id, service_id, group_id, event_type, enabled, cooldown_secs
		FROM notification_group_event_rules
		WHERE service_id = ?`,
		serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64][]GroupEventRule)
	for rows.Next() {
		var r GroupEventRule
		if err := rows.Scan(&r.ID, &r.ServiceID, &r.GroupID, &r.EventType, &r.Enabled, &r.Cooldown); err != nil {
			continue
		}
		m[r.GroupID] = append(m[r.GroupID], r)
	}
	return m, nil
}

// DeleteGroupRules removes all group-specific rules for a service + group.
func DeleteGroupRules(db *sql.DB, serviceID, groupID int64) error {
	_, err := db.Exec(
		`DELETE FROM notification_group_event_rules WHERE service_id = ? AND group_id = ?`,
		serviceID, groupID,
	)
	return err
}
