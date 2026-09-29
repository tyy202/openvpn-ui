package controllers

import (
	"testing"

	"github.com/d3vilh/openvpn-ui/models"
)

func TestCanManageUsersRequiresEnvironmentFlagAndAdministrator(t *testing.T) {
	admin := &models.User{IsAdmin: true}
	member := &models.User{IsAdmin: false}

	t.Setenv(userManagementEnv, "")
	if canManageUsers(admin) {
		t.Fatal("user management must be disabled by default")
	}

	t.Setenv(userManagementEnv, "true")
	if !canManageUsers(admin) {
		t.Fatal("administrator must be allowed when user management is enabled")
	}
	if canManageUsers(member) {
		t.Fatal("non-administrator must not manage users")
	}

	t.Setenv(userManagementEnv, "not-a-boolean")
	if canManageUsers(admin) {
		t.Fatal("invalid values must fail closed")
	}
}
