package controllers

import (
	"testing"

	"github.com/d3vilh/openvpn-ui/internal/appversion"
	"github.com/d3vilh/openvpn-ui/models"
)

func TestBuildAccessSnapshotExposesUserGroupAndInheritedNetworks(t *testing.T) {
	group := &models.VPNGroup{Id: 10, Name: "研发组", Enabled: true}
	lan := &models.NetworkResource{Id: 20, Name: "办公网", CIDR: "192.168.18.0/24", Enabled: true}
	user := &models.VPNUser{Id: 30, Name: "张三", CertificateCN: "zhangsan", StaticIP: "10.8.22.10", Group: group, Enabled: true}
	link := &models.GroupNetwork{Id: 40, Group: group, Network: lan}

	snapshot := buildAccessSnapshot(
		[]*models.VPNGroup{group},
		[]*models.NetworkResource{lan},
		[]*models.VPNUser{user},
		[]*models.GroupNetwork{link},
		"10.8.22.0/24",
		"csrf-token",
	)

	if snapshot.AppVersion != appversion.Current() {
		t.Fatalf("AppVersion = %q, want %q", snapshot.AppVersion, appversion.Current())
	}
	if snapshot.PolicyMode != "default-deny" {
		t.Fatalf("PolicyMode = %q, want default-deny", snapshot.PolicyMode)
	}
	if len(snapshot.Users) != 1 || snapshot.Users[0].Group.ID != group.Id {
		t.Fatalf("user group relationship was not included: %#v", snapshot.Users)
	}
	if got := snapshot.Users[0].AllowedNetworks; len(got) != 1 || got[0].ID != lan.Id {
		t.Fatalf("inherited user networks = %#v, want network %d", got, lan.Id)
	}
	if len(snapshot.Groups) != 1 || len(snapshot.Groups[0].Members) != 1 || snapshot.Groups[0].Members[0].ID != user.Id {
		t.Fatalf("group members were not included: %#v", snapshot.Groups)
	}
	if len(snapshot.Groups[0].AllowedNetworks) != 1 || snapshot.Groups[0].AllowedNetworks[0].ID != lan.Id {
		t.Fatalf("group network relationship was not included: %#v", snapshot.Groups[0].AllowedNetworks)
	}
}

func TestBuildAccessSnapshotDoesNotGrantDisabledRelationships(t *testing.T) {
	group := &models.VPNGroup{Id: 1, Name: "停用组", Enabled: false}
	network := &models.NetworkResource{Id: 2, Name: "停用网络", CIDR: "10.0.0.0/8", Enabled: false}
	user := &models.VPNUser{Id: 3, Name: "用户", Group: group, Enabled: true}

	snapshot := buildAccessSnapshot(
		[]*models.VPNGroup{group},
		[]*models.NetworkResource{network},
		[]*models.VPNUser{user},
		[]*models.GroupNetwork{{Group: group, Network: network}},
		"10.8.0.0/24",
		"token",
	)

	if len(snapshot.Users[0].AllowedNetworks) != 0 {
		t.Fatalf("disabled relationship must not become an effective permission: %#v", snapshot.Users[0].AllowedNetworks)
	}
	if len(snapshot.Groups[0].AllowedNetworks) != 1 {
		t.Fatalf("configured group relationship should remain visible for editing")
	}
}
