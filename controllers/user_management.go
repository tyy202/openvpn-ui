package controllers

import (
	"os"
	"strconv"
	"strings"

	"github.com/d3vilh/openvpn-ui/models"
)

const userManagementEnv = "ALLOW_USER_MANAGEMENT"

func userManagementEnabled() bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(userManagementEnv)))
	return err == nil && enabled
}

func canManageUsers(user *models.User) bool {
	return user != nil && user.IsAdmin && userManagementEnabled()
}
