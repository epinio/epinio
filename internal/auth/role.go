package auth

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	RolesDelimiter         = ","
	RoleNamespaceDelimiter = ":"
)

var (
	// AdminRole is a special role. It permits all actions.
	AdminRole = Role{
		ID:   "admin",
		Name: "Admin Role",
	}

	// EpinioRoles are all the available Epinio roles.
	// It is initialized with the AdminRole, and then it will load the other available Roles
	// with the auth.InitRoles function.
	EpinioRoles Roles = Roles{AdminRole}
)

type Roles []Role

// Role define an Epinio role, loaded from ConfigMaps
type Role struct {
	ID string
	// Name is a friendly name for the Role
	Name string
	// Namespace is the namespace where this role is applied to
	Namespace string
	Actions   []Action
	// Default is set to true if the Role id the default one
	Default bool
}

// Default return the default role, if found
func (roles Roles) Default() (Role, bool) {
	for _, role := range roles {
		if role.Default {
			return role, true
		}
	}
	return Role{}, false
}

// IDs return the IDs of the roles (namescoped)
func (roles Roles) IDs() []string {
	ids := make([]string, 0)

	for _, role := range roles {
		id := role.ID
		if role.Namespace != "" {
			id = role.ID + RoleNamespaceDelimiter + role.Namespace
		}
		ids = append(ids, id)
	}

	return ids
}

// FindByID return the role matching the id (not namescoped)
func (roles Roles) FindByID(id string) (Role, bool) {
	return roles.FindByIDAndNamespace(id, "")
}

// FindByIDAndNamespace return the role matching the id and namescoped
func (roles Roles) FindByIDAndNamespace(id, namespace string) (Role, bool) {
	for _, role := range roles {
		if role.ID == id && role.Namespace == namespace {
			return role, true
		}
	}
	return Role{}, false
}

func (roles Roles) IsAllowed(method, fullpath string) bool {
	for _, r := range roles {
		if r.IsAllowed(method, fullpath) {
			return true
		}
	}
	return false
}

func NewRole(id, name, defaultVal string, actionIDs []string) (Role, error) {
	var role Role
	var err error

	// init the actions with the default action
	actions := []Action{ActionsMap["default"]}

	for _, actionID := range actionIDs {
		action, found := ActionsMap[actionID]
		if !found {
			return role, fmt.Errorf("action '%s' in role '%s' does not exists", actionID, id)
		}

		actions = append(actions, action)
	}

	var defaultBool bool
	if defaultVal != "" {
		defaultBool, err = strconv.ParseBool(defaultVal)
		if err != nil {
			return role, err
		}
	}

	return Role{
		ID:      id,
		Name:    name,
		Default: defaultBool,
		Actions: actions,
	}, nil
}

func (r *Role) IsAllowed(method, fullpath string) bool {
	if r.ID == "admin" {
		return true
	}

	for _, a := range r.Actions {
		if a.IsAllowed(method, fullpath) {
			return true
		}
	}

	return false
}

func newRoleFromConfigMap(config corev1.ConfigMap) (Role, error) {
	actionIDs := []string{}

	if actionsData, found := config.Data["actions"]; found {
		actionsData = strings.TrimSpace(actionsData)
		actionIDs = strings.Split(actionsData, "\n")
	}

	return NewRole(
		config.Data["id"],
		config.Data["name"],
		config.Data["default"],
		actionIDs,
	)
}

type RolesGetter interface {
	GetRoles(context.Context) (Roles, error)
}

func InitRoles(rolesGetter RolesGetter) error {
	roles, err := rolesGetter.GetRoles(context.Background())
	if err != nil {
		return err
	}
	EpinioRoles = append(EpinioRoles, roles...)

	return nil
}

// ParseRoleID parses the "full" roleID, returning the roleID without the namespace, and the namespace
//
// i.e.:
//
//	"admin" will return "admin" and ""
//	"admin:workspace" will return "admin" and "workspace"
func ParseRoleID(roleID string) (string, string) {
	roleIDAndNamespace := strings.Split(roleID, RoleNamespaceDelimiter)
	if len(roleIDAndNamespace) > 1 {
		return roleIDAndNamespace[0], roleIDAndNamespace[1]
	}
	return roleIDAndNamespace[0], ""
}
