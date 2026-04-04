package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"vigil/internal/db"
	"vigil/internal/drivegroups"
	"vigil/internal/validate"
)

// ── Group CRUD ──────────────────────────────────────────────────────────

// ListDriveGroups returns all groups with member counts.
func ListDriveGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := drivegroups.ListGroups(db.DB)
	if err != nil {
		log.Printf("❌ List drive groups: %v", err)
		JSONError(w, "Failed to list groups", http.StatusInternalServerError)
		return
	}
	if groups == nil {
		groups = []drivegroups.DriveGroup{}
	}
	JSONResponse(w, groups)
}

// CreateDriveGroup creates a new group.
func CreateDriveGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if err := validate.Name(req.Name, 64); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	g := &drivegroups.DriveGroup{Name: req.Name, Color: req.Color}
	id, err := drivegroups.CreateGroup(db.DB, g)
	if err != nil {
		JSONError(w, "Group name already exists", http.StatusConflict)
		return
	}
	g.ID = id
	log.Printf("🏷️ Drive group created: %s", req.Name)
	w.WriteHeader(http.StatusCreated)
	JSONResponse(w, g)
}

// GetDriveGroup returns a group with its members.
func GetDriveGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	g, err := drivegroups.GetGroup(db.DB, id)
	if err != nil {
		JSONError(w, "Failed to get group", http.StatusInternalServerError)
		return
	}
	if g == nil {
		JSONError(w, "Group not found", http.StatusNotFound)
		return
	}

	members, _ := drivegroups.ListGroupMembers(db.DB, id)
	if members == nil {
		members = []drivegroups.DriveGroupMember{}
	}

	JSONResponse(w, map[string]interface{}{
		"group":   g,
		"members": members,
	})
}

// UpdateDriveGroup updates a group's name and color.
func UpdateDriveGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if err := validate.Name(req.Name, 64); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := drivegroups.UpdateGroup(db.DB, &drivegroups.DriveGroup{ID: id, Name: req.Name, Color: req.Color}); err != nil {
		JSONError(w, "Failed to update group", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]string{"status": "updated"})
}

// DeleteDriveGroup removes a group (cascade deletes members and rules).
func DeleteDriveGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	if err := drivegroups.DeleteGroup(db.DB, id); err != nil {
		JSONError(w, "Failed to delete group", http.StatusInternalServerError)
		return
	}
	log.Printf("🗑️ Drive group deleted: id=%d", id)
	JSONResponse(w, map[string]string{"status": "deleted"})
}

// ── Member Management ───────────────────────────────────────────────────

// AssignDriveToGroup adds a drive to a group.
func AssignDriveToGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Hostname     string `json:"hostname"`
		SerialNumber string `json:"serial_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Hostname == "" || req.SerialNumber == "" {
		JSONError(w, "hostname and serial_number required", http.StatusBadRequest)
		return
	}

	if err := drivegroups.AssignDrive(db.DB, id, req.Hostname, req.SerialNumber); err != nil {
		JSONError(w, "Failed to assign drive", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]string{"status": "assigned"})
}

// UnassignDriveFromGroup removes a drive from any group.
func UnassignDriveFromGroup(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	serial := r.PathValue("serial")
	if hostname == "" || serial == "" {
		JSONError(w, "hostname and serial required", http.StatusBadRequest)
		return
	}
	if err := drivegroups.UnassignDrive(db.DB, hostname, serial); err != nil {
		JSONError(w, "Failed to unassign drive", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]string{"status": "unassigned"})
}

// GetDriveGroupAssignments returns all assignments as a map.
func GetDriveGroupAssignments(w http.ResponseWriter, r *http.Request) {
	m, err := drivegroups.ListAllAssignments(db.DB)
	if err != nil {
		JSONError(w, "Failed to list assignments", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, m)
}

// ── Group Event Rules ───────────────────────────────────────────────────

// GetGroupRulesForService returns all group-specific rules for a notification service.
func GetGroupRulesForService(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	all, err := drivegroups.GetAllGroupRulesForService(db.DB, serviceID)
	if err != nil {
		JSONError(w, "Failed to get group rules", http.StatusInternalServerError)
		return
	}
	if all == nil {
		all = make(map[int64][]drivegroups.GroupEventRule)
	}
	JSONResponse(w, all)
}

// UpdateGroupRules sets group-specific event rules for a service + group.
func UpdateGroupRules(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}
	groupID, err := strconv.ParseInt(r.PathValue("groupId"), 10, 64)
	if err != nil {
		JSONError(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	var rules []drivegroups.GroupEventRule
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	for i := range rules {
		rules[i].ServiceID = serviceID
		rules[i].GroupID = groupID
		if err := drivegroups.UpsertGroupEventRule(db.DB, &rules[i]); err != nil {
			log.Printf("❌ Upsert group rule: %v", err)
			JSONError(w, "Failed to update rules", http.StatusInternalServerError)
			return
		}
	}
	JSONResponse(w, map[string]string{"status": "updated"})
}

// DeleteGroupRulesHandler removes all group-specific rules for a service + group.
func DeleteGroupRulesHandler(w http.ResponseWriter, r *http.Request) {
	serviceID, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}
	groupID, err := strconv.ParseInt(r.PathValue("groupId"), 10, 64)
	if err != nil {
		JSONError(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	if err := drivegroups.DeleteGroupRules(db.DB, serviceID, groupID); err != nil {
		JSONError(w, "Failed to delete group rules", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]string{"status": "deleted"})
}

// ── Route Registration ──────────────────────────────────────────────────

// RegisterDriveGroupRoutes registers drive group API routes.
func RegisterDriveGroupRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/drive-groups", protect(ListDriveGroups))
	mux.HandleFunc("POST /api/drive-groups", protect(CreateDriveGroup))
	mux.HandleFunc("GET /api/drive-groups/{id}", protect(GetDriveGroup))
	mux.HandleFunc("PUT /api/drive-groups/{id}", protect(UpdateDriveGroup))
	mux.HandleFunc("DELETE /api/drive-groups/{id}", protect(DeleteDriveGroup))
	mux.HandleFunc("POST /api/drive-groups/{id}/members", protect(AssignDriveToGroup))
	mux.HandleFunc("DELETE /api/drive-groups/members/{hostname}/{serial}", protect(UnassignDriveFromGroup))
	mux.HandleFunc("GET /api/drive-groups/assignments", protect(GetDriveGroupAssignments))

	// Group-specific notification rules
	mux.HandleFunc("GET /api/notifications/services/{id}/group-rules", protect(GetGroupRulesForService))
	mux.HandleFunc("PUT /api/notifications/services/{id}/group-rules/{groupId}", protect(UpdateGroupRules))
	mux.HandleFunc("DELETE /api/notifications/services/{id}/group-rules/{groupId}", protect(DeleteGroupRulesHandler))
}
