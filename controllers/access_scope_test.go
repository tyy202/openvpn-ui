package controllers

import (
	"testing"

	"github.com/d3vilh/openvpn-ui/internal/networkscope"
	"github.com/d3vilh/openvpn-ui/models"
)

func TestFilterAccessDataHidesMixedScopeGroupsAndUsers(t *testing.T) {
	visibleNetwork := &models.NetworkResource{Id: 1, CIDR: "192.168.18.0/24"}
	hiddenNetwork := &models.NetworkResource{Id: 2, CIDR: "10.20.0.0/16"}
	visibleGroup := &models.VPNGroup{Id: 10, Name: "visible"}
	mixedGroup := &models.VPNGroup{Id: 11, Name: "mixed"}
	visibleUser := &models.VPNUser{Id: 20, Name: "visible-user", Group: visibleGroup}
	hiddenUser := &models.VPNUser{Id: 21, Name: "hidden-user", Group: mixedGroup}
	links := []*models.GroupNetwork{
		{Group: visibleGroup, Network: visibleNetwork},
		{Group: mixedGroup, Network: visibleNetwork},
		{Group: mixedGroup, Network: hiddenNetwork},
	}

	scope := networkscope.Scope{CIDRs: map[string]struct{}{"192.168.18.0/24": {}}}
	groups, networks, users, filteredLinks := filterAccessData(
		scope,
		[]*models.VPNGroup{visibleGroup, mixedGroup},
		[]*models.NetworkResource{visibleNetwork, hiddenNetwork},
		[]*models.VPNUser{visibleUser, hiddenUser},
		links,
	)

	if len(networks) != 1 || networks[0].Id != visibleNetwork.Id {
		t.Fatalf("networks = %#v; want only visible network", networks)
	}
	if len(groups) != 1 || groups[0].Id != visibleGroup.Id {
		t.Fatalf("groups = %#v; want only fully scoped group", groups)
	}
	if len(users) != 1 || users[0].Id != visibleUser.Id {
		t.Fatalf("users = %#v; want only user from fully scoped group", users)
	}
	if len(filteredLinks) != 1 || filteredLinks[0].Group.Id != visibleGroup.Id {
		t.Fatalf("links = %#v; want only visible group relationship", filteredLinks)
	}
}

func TestFilterAccessDataFullScopeKeepsEverything(t *testing.T) {
	group := &models.VPNGroup{Id: 1}
	network := &models.NetworkResource{Id: 2, CIDR: "10.0.0.0/8"}
	user := &models.VPNUser{Id: 3, Group: group}
	link := &models.GroupNetwork{Group: group, Network: network}

	groups, networks, users, links := filterAccessData(
		networkscope.Scope{All: true},
		[]*models.VPNGroup{group},
		[]*models.NetworkResource{network},
		[]*models.VPNUser{user},
		[]*models.GroupNetwork{link},
	)
	if len(groups) != 1 || len(networks) != 1 || len(users) != 1 || len(links) != 1 {
		t.Fatalf("full scope unexpectedly filtered data: %d %d %d %d", len(groups), len(networks), len(users), len(links))
	}
}
