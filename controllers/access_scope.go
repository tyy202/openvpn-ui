package controllers

import (
	"os"

	"github.com/d3vilh/openvpn-ui/internal/networkscope"
	"github.com/d3vilh/openvpn-ui/models"
)

const networkScopesEnv = "OPENVPN_UI_NETWORK_SCOPES"

func resolveAccessScope(user *models.User) (networkscope.Scope, bool, error) {
	if user == nil {
		return networkscope.Scope{}, false, nil
	}
	return networkscope.Resolve(os.Getenv(networkScopesEnv), user.Login, user.IsAdmin)
}

func filterAccessData(
	scope networkscope.Scope,
	groups []*models.VPNGroup,
	networks []*models.NetworkResource,
	users []*models.VPNUser,
	links []*models.GroupNetwork,
) ([]*models.VPNGroup, []*models.NetworkResource, []*models.VPNUser, []*models.GroupNetwork) {
	if scope.All {
		return groups, networks, users, links
	}

	visibleNetworkIDs := make(map[int64]struct{})
	filteredNetworks := make([]*models.NetworkResource, 0, len(networks))
	for _, network := range networks {
		if network != nil && scope.Allows(network.CIDR) {
			visibleNetworkIDs[network.Id] = struct{}{}
			filteredNetworks = append(filteredNetworks, network)
		}
	}

	totalLinks := make(map[int64]int)
	visibleLinks := make(map[int64]int)
	for _, link := range links {
		if link == nil || link.Group == nil || link.Network == nil {
			continue
		}
		totalLinks[link.Group.Id]++
		if _, ok := visibleNetworkIDs[link.Network.Id]; ok {
			visibleLinks[link.Group.Id]++
		}
	}

	visibleGroupIDs := make(map[int64]struct{})
	filteredGroups := make([]*models.VPNGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil || totalLinks[group.Id] == 0 || totalLinks[group.Id] != visibleLinks[group.Id] {
			continue
		}
		visibleGroupIDs[group.Id] = struct{}{}
		filteredGroups = append(filteredGroups, group)
	}

	filteredLinks := make([]*models.GroupNetwork, 0, len(links))
	for _, link := range links {
		if link == nil || link.Group == nil || link.Network == nil {
			continue
		}
		_, groupVisible := visibleGroupIDs[link.Group.Id]
		_, networkVisible := visibleNetworkIDs[link.Network.Id]
		if groupVisible && networkVisible {
			filteredLinks = append(filteredLinks, link)
		}
	}

	filteredUsers := make([]*models.VPNUser, 0, len(users))
	for _, user := range users {
		if user == nil || user.Group == nil {
			continue
		}
		if _, ok := visibleGroupIDs[user.Group.Id]; ok {
			filteredUsers = append(filteredUsers, user)
		}
	}

	return filteredGroups, filteredNetworks, filteredUsers, filteredLinks
}
