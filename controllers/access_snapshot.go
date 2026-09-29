package controllers

import (
	"net/http"

	"github.com/beego/beego/v2/client/orm"
	"github.com/d3vilh/openvpn-ui/lib"
	"github.com/d3vilh/openvpn-ui/models"
)

type networkView struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	Source    string `json:"source"`
	Interface string `json:"interface"`
	Gateway   string `json:"gateway"`
	Enabled   bool   `json:"enabled"`
	Reachable bool   `json:"reachable"`
}

type memberView struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type groupRefView struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type groupView struct {
	ID              int64         `json:"id"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	Enabled         bool          `json:"enabled"`
	Members         []memberView  `json:"members"`
	AllowedNetworks []networkView `json:"allowedNetworks"`
}

type userView struct {
	ID              int64         `json:"id"`
	Name            string        `json:"name"`
	CertificateCN   string        `json:"certificateCN"`
	StaticIP        string        `json:"staticIP"`
	Enabled         bool          `json:"enabled"`
	Group           groupRefView  `json:"group"`
	AllowedNetworks []networkView `json:"allowedNetworks"`
}

type accessSnapshot struct {
	PolicyMode string        `json:"policyMode"`
	VPNPool    string        `json:"vpnPool"`
	XSRFToken  string        `json:"xsrfToken"`
	Groups     []groupView   `json:"groups"`
	Networks   []networkView `json:"networks"`
	Users      []userView    `json:"users"`
}

func toNetworkView(network *models.NetworkResource) networkView {
	return networkView{
		ID: network.Id, Name: network.Name, CIDR: network.CIDR, Source: network.Source,
		Interface: network.Interface, Gateway: network.Gateway, Enabled: network.Enabled, Reachable: network.Reachable,
	}
}

func buildAccessSnapshot(groups []*models.VPNGroup, networks []*models.NetworkResource, users []*models.VPNUser, links []*models.GroupNetwork, pool, token string) accessSnapshot {
	result := accessSnapshot{
		PolicyMode: "default-deny", VPNPool: pool, XSRFToken: token,
		Groups: make([]groupView, 0, len(groups)), Networks: make([]networkView, 0, len(networks)), Users: make([]userView, 0, len(users)),
	}
	groupNetworks := make(map[int64][]networkView)
	for _, network := range networks {
		result.Networks = append(result.Networks, toNetworkView(network))
	}
	for _, link := range links {
		if link.Group != nil && link.Network != nil {
			groupNetworks[link.Group.Id] = append(groupNetworks[link.Group.Id], toNetworkView(link.Network))
		}
	}
	groupMembers := make(map[int64][]memberView)
	for _, user := range users {
		if user.Group != nil {
			groupMembers[user.Group.Id] = append(groupMembers[user.Group.Id], memberView{ID: user.Id, Name: user.Name, Enabled: user.Enabled})
		}
	}
	for _, group := range groups {
		result.Groups = append(result.Groups, groupView{
			ID: group.Id, Name: group.Name, Description: group.Description, Enabled: group.Enabled,
			Members:         append([]memberView{}, groupMembers[group.Id]...),
			AllowedNetworks: append([]networkView{}, groupNetworks[group.Id]...),
		})
	}
	for _, user := range users {
		view := userView{ID: user.Id, Name: user.Name, CertificateCN: user.CertificateCN, StaticIP: user.StaticIP, Enabled: user.Enabled, AllowedNetworks: []networkView{}}
		if user.Group != nil {
			view.Group = groupRefView{ID: user.Group.Id, Name: user.Group.Name, Enabled: user.Group.Enabled}
			if user.Enabled && user.Group.Enabled {
				for _, network := range groupNetworks[user.Group.Id] {
					if network.Enabled {
						view.AllowedNetworks = append(view.AllowedNetworks, network)
					}
				}
			}
		}
		result.Users = append(result.Users, view)
	}
	return result
}

type AccessSnapshotController struct{ BaseController }

func (c *AccessSnapshotController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]string{"error": "authentication required"}
		_ = c.ServeJSON()
		return
	}
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.Abort("403")
	}
}

func (c *AccessSnapshotController) Get() {
	if !c.IsLogin || c.Userinfo == nil || !c.Userinfo.IsAdmin {
		return
	}
	o := orm.NewOrm()
	var groups []*models.VPNGroup
	var networks []*models.NetworkResource
	var users []*models.VPNUser
	var links []*models.GroupNetwork
	_, _ = o.QueryTable(new(models.VPNGroup)).OrderBy("Name").All(&groups)
	_, _ = o.QueryTable(new(models.NetworkResource)).OrderBy("CIDR").All(&networks)
	_, _ = o.QueryTable(new(models.VPNUser)).RelatedSel("Group").OrderBy("Name").All(&users)
	_, _ = o.QueryTable(new(models.GroupNetwork)).RelatedSel("Group", "Network").All(&links)
	pool, _ := lib.VPNPoolCIDR()
	c.Data["json"] = buildAccessSnapshot(groups, networks, users, links, pool, c.XSRFToken())
	_ = c.ServeJSON()
}

type ModernAppController struct{ BaseController }

func (c *ModernAppController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(http.StatusFound, c.LoginPath())
		return
	}
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.Abort("403")
	}
}

func (c *ModernAppController) Get() {
	if !c.IsLogin || c.Userinfo == nil || !c.Userinfo.IsAdmin {
		return
	}
	c.Ctx.Redirect(http.StatusMovedPermanently, "/access")
}
