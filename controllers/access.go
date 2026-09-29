package controllers

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/server/web"
	"github.com/d3vilh/openvpn-ui/internal/networkscope"
	"github.com/d3vilh/openvpn-ui/lib"
	"github.com/d3vilh/openvpn-ui/models"
	"github.com/d3vilh/openvpn-ui/state"
)

type AccessController struct {
	BaseController
	accessScope networkscope.Scope
}

func (c *AccessController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}
	scope, allowed, err := resolveAccessScope(c.Userinfo)
	if err != nil {
		c.Abort("500")
		return
	}
	if !allowed {
		c.Abort("403")
		return
	}
	c.accessScope = scope
	c.Data["breadcrumbs"] = &BreadCrumbs{Title: "VPN Access Control"}
}

func (c *AccessController) Get() {
	c.TplName = "access.html"
}

func (c *AccessController) Post() {
	flash := web.NewFlash()
	err := c.perform(c.GetString("action"))
	if strings.Contains(c.Ctx.Input.Header("Accept"), "application/json") {
		if err != nil {
			c.Ctx.Output.SetStatus(http.StatusBadRequest)
			c.Data["json"] = map[string]string{"error": err.Error()}
		} else {
			c.Data["json"] = map[string]bool{"ok": true}
		}
		_ = c.ServeJSON()
		return
	}
	if err != nil {
		flash.Error("%s", err.Error())
	} else {
		flash.Success("Access policy has been updated")
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("AccessController.Get"), 303)
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid record id")
	}
	return id, nil
}

func (c *AccessController) perform(action string) error {
	o := orm.NewOrm()
	switch action {
	case "discover-networks":
		if !c.accessScope.All {
			return fmt.Errorf("host network discovery requires full network scope")
		}
		_, err := lib.DiscoverNetworkResources()
		return err
	case "create-network":
		cidr := strings.TrimSpace(c.GetString("CIDR"))
		ip, n, err := net.ParseCIDR(cidr)
		if err != nil || ip.To4() == nil {
			return fmt.Errorf("invalid IPv4 CIDR")
		}
		if !c.accessScope.Allows(n.String()) {
			return fmt.Errorf("network is outside your configured scope")
		}
		row := models.NetworkResource{Name: strings.TrimSpace(c.GetString("Name")), CIDR: n.String(), Source: "manual", Enabled: true, Reachable: true}
		if row.Name == "" {
			row.Name = row.CIDR
		}
		if _, err = o.Insert(&row); err != nil {
			return err
		}
		return lib.ReconcileAccessPolicy()
	case "update-network":
		id, err := parseID(c.GetString("Id"))
		if err != nil {
			return err
		}
		row := models.NetworkResource{Id: id}
		if err = o.Read(&row); err != nil {
			return err
		}
		if err = c.requireNetwork(&row); err != nil {
			return err
		}
		row.Name = strings.TrimSpace(c.GetString("Name"))
		row.Enabled = c.GetString("Enabled") == "on"
		if row.Name == "" {
			row.Name = row.CIDR
		}
		if _, err = o.Update(&row, "Name", "Enabled"); err != nil {
			return err
		}
		return lib.ReconcileAccessPolicy()
	case "delete-network":
		id, err := parseID(c.GetString("Id"))
		if err != nil {
			return err
		}
		row := models.NetworkResource{Id: id}
		if err = o.Read(&row); err != nil {
			return err
		}
		if err = c.requireNetwork(&row); err != nil {
			return err
		}
		if _, err = o.QueryTable(new(models.GroupNetwork)).Filter("Network__Id", id).Delete(); err != nil {
			return err
		}
		if _, err = o.Delete(&models.NetworkResource{Id: id}); err != nil {
			return err
		}
		return lib.ReconcileAccessPolicy()
	case "create-group":
		name := strings.TrimSpace(c.GetString("Name"))
		if name == "" {
			return fmt.Errorf("group name is required")
		}
		networkIDs, err := c.allowedNetworkIDs(o, c.GetStrings("NetworkIDs"))
		if err != nil {
			return err
		}
		if !c.accessScope.All && len(networkIDs) == 0 {
			return fmt.Errorf("select at least one network within your scope")
		}
		row := models.VPNGroup{Name: name, Description: strings.TrimSpace(c.GetString("Description")), Enabled: true}
		if _, err = o.Insert(&row); err != nil {
			return err
		}
		for _, networkID := range networkIDs {
			link := models.GroupNetwork{Group: &row, Network: &models.NetworkResource{Id: networkID}}
			if _, err = o.Insert(&link); err != nil {
				_, _ = o.QueryTable(new(models.GroupNetwork)).Filter("Group__Id", row.Id).Delete()
				_, _ = o.Delete(&row)
				return err
			}
		}
		return lib.ReconcileAccessPolicy()
	case "update-group":
		id, err := parseID(c.GetString("Id"))
		if err != nil {
			return err
		}
		row := models.VPNGroup{Id: id}
		if err = o.Read(&row); err != nil {
			return err
		}
		if err = c.requireGroup(o, row.Id); err != nil {
			return err
		}
		networkIDs, err := c.allowedNetworkIDs(o, c.GetStrings("NetworkIDs"))
		if err != nil {
			return err
		}
		if !c.accessScope.All && len(networkIDs) == 0 {
			return fmt.Errorf("select at least one network within your scope")
		}
		row.Name = strings.TrimSpace(c.GetString("Name"))
		row.Description = strings.TrimSpace(c.GetString("Description"))
		row.Enabled = c.GetString("Enabled") == "on"
		if row.Name == "" {
			return fmt.Errorf("group name is required")
		}
		if _, err = o.Update(&row, "Name", "Description", "Enabled"); err != nil {
			return err
		}
		if _, err = o.QueryTable(new(models.GroupNetwork)).Filter("Group__Id", id).Delete(); err != nil {
			return err
		}
		for _, networkID := range networkIDs {
			link := models.GroupNetwork{Group: &row, Network: &models.NetworkResource{Id: networkID}}
			if _, err = o.Insert(&link); err != nil {
				return err
			}
		}
		if err = lib.ReconcileAccessPolicy(); err != nil {
			return err
		}
		var users []*models.VPNUser
		_, _ = o.QueryTable(new(models.VPNUser)).Filter("Group__Id", id).All(&users)
		for _, user := range users {
			_ = lib.KillVPNSession(user.CertificateCN)
		}
		return nil
	case "delete-group":
		id, err := parseID(c.GetString("Id"))
		if err != nil {
			return err
		}
		if err = c.requireGroup(o, id); err != nil {
			return err
		}
		count, err := o.QueryTable(new(models.VPNUser)).Filter("Group__Id", id).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("move or delete all users in this group first")
		}
		_, err = o.Delete(&models.VPNGroup{Id: id})
		return err
	case "create-user":
		return c.createUser(o)
	case "update-user":
		id, err := parseID(c.GetString("Id"))
		if err != nil {
			return err
		}
		user := models.VPNUser{Id: id}
		if err = o.QueryTable(new(models.VPNUser)).Filter("Id", id).RelatedSel("Group").One(&user); err != nil {
			return err
		}
		if user.Group == nil {
			return fmt.Errorf("user has no group")
		}
		if err = c.requireGroup(o, user.Group.Id); err != nil {
			return err
		}
		oldIP := user.StaticIP
		groupID, err := parseID(c.GetString("GroupId"))
		if err != nil {
			return err
		}
		if err = c.requireGroup(o, groupID); err != nil {
			return err
		}
		if requested := strings.TrimSpace(c.GetString("StaticIP")); requested != "" && requested != oldIP {
			if user.StaticIP, err = lib.AllocateVPNAddress(requested); err != nil {
				return err
			}
		}
		user.Name = strings.TrimSpace(c.GetString("Name"))
		user.Group = &models.VPNGroup{Id: groupID}
		user.Enabled = c.GetString("Enabled") == "on"
		if user.Name == "" {
			return fmt.Errorf("user name is required")
		}
		if _, err = o.Update(&user, "Name", "StaticIP", "Group", "Enabled"); err != nil {
			return err
		}
		if err = lib.ReconcileAccessPolicy(); err != nil {
			return err
		}
		_ = lib.KillVPNSession(user.CertificateCN)
		return nil
	case "renew-user":
		id, err := parseID(c.GetString("Id"))
		if err != nil {
			return err
		}
		user := models.VPNUser{Id: id}
		if err = o.QueryTable(new(models.VPNUser)).Filter("Id", id).RelatedSel("Group").One(&user); err != nil {
			return err
		}
		if user.Group == nil {
			return fmt.Errorf("user has no group")
		}
		if err = c.requireGroup(o, user.Group.Id); err != nil {
			return err
		}
		serial, err := validCertificateSerial(user.CertificateCN)
		if err != nil {
			return err
		}
		if err = lib.RenewCertificate(user.CertificateCN, user.StaticIP, serial, "none"); err != nil {
			return err
		}
		if err = lib.RevokeCertificate(user.CertificateCN, serial, "none"); err != nil {
			return err
		}
		_ = lib.KillVPNSession(user.CertificateCN)
		return lib.ReconcileAccessPolicy()
	case "delete-user":
		id, err := parseID(c.GetString("Id"))
		if err != nil {
			return err
		}
		user := models.VPNUser{Id: id}
		if err = o.QueryTable(new(models.VPNUser)).Filter("Id", id).RelatedSel("Group").One(&user); err != nil {
			return err
		}
		if user.Group == nil {
			return fmt.Errorf("user has no group")
		}
		if err = c.requireGroup(o, user.Group.Id); err != nil {
			return err
		}
		_ = lib.KillVPNSession(user.CertificateCN)
		serial, status, err := certificateSerial(user.CertificateCN)
		if err == nil {
			if status == "V" {
				if err = lib.RevokeCertificate(user.CertificateCN, serial, "none"); err != nil {
					return err
				}
			}
			if err = lib.BurnCertificate(user.CertificateCN, serial, "none"); err != nil {
				return err
			}
		}
		if _, err = o.Delete(&user); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(state.GlobalCfg.OVConfigPath, "staticclients", user.CertificateCN))
		return lib.ReconcileAccessPolicy()
	default:
		return fmt.Errorf("unsupported access policy action")
	}
}

func (c *AccessController) createUser(o orm.Ormer) error {
	name := strings.TrimSpace(c.GetString("Name"))
	cn := strings.TrimSpace(c.GetString("CertificateCN"))
	if name == "" {
		return fmt.Errorf("user name is required")
	}
	if err := lib.ValidateCertificateCN(cn); err != nil {
		return err
	}
	groupID, err := parseID(c.GetString("GroupId"))
	if err != nil {
		return err
	}
	group := models.VPNGroup{Id: groupID}
	if err = o.Read(&group); err != nil {
		return err
	}
	if err = c.requireGroup(o, groupID); err != nil {
		return err
	}
	address, err := lib.AllocateVPNAddress(c.GetString("StaticIP"))
	if err != nil {
		return err
	}
	user := models.VPNUser{Name: name, CertificateCN: cn, StaticIP: address, Group: &group, Enabled: true}
	if _, err = o.Insert(&user); err != nil {
		return err
	}
	easy := models.EasyRSAConfig{Profile: "default"}
	if err = easy.Read("Profile"); err != nil {
		_, _ = o.Delete(&user)
		return err
	}
	err = lib.CreateCertificate(cn, address, "", strconv.Itoa(easy.EasyRSACertExpire), easy.EasyRSAReqEmail, easy.EasyRSAReqCountry, easy.EasyRSAReqProvince, strconv.Quote(easy.EasyRSAReqCity), strconv.Quote(easy.EasyRSAReqOrg), strconv.Quote(easy.EasyRSAReqOu), "none", "")
	if err != nil {
		_, _ = o.Delete(&user)
		return err
	}
	return lib.ReconcileAccessPolicy()
}

func (c *AccessController) requireNetwork(network *models.NetworkResource) error {
	if network == nil || !c.accessScope.Allows(network.CIDR) {
		return fmt.Errorf("network is outside your configured scope")
	}
	return nil
}

func (c *AccessController) allowedNetworkIDs(o orm.Ormer, values []string) ([]int64, error) {
	ids := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		id, err := parseID(value)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			continue
		}
		network := models.NetworkResource{Id: id}
		if err = o.Read(&network); err != nil {
			return nil, err
		}
		if err = c.requireNetwork(&network); err != nil {
			return nil, err
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *AccessController) requireGroup(o orm.Ormer, groupID int64) error {
	if c.accessScope.All {
		return nil
	}
	var links []*models.GroupNetwork
	if _, err := o.QueryTable(new(models.GroupNetwork)).Filter("Group__Id", groupID).RelatedSel("Network").All(&links); err != nil {
		return err
	}
	if len(links) == 0 {
		return fmt.Errorf("group is outside your configured scope")
	}
	for _, link := range links {
		if link.Network == nil || !c.accessScope.Allows(link.Network.CIDR) {
			return fmt.Errorf("group is outside your configured scope")
		}
	}
	return nil
}

func validCertificateSerial(cn string) (string, error) {
	serial, status, err := certificateSerial(cn)
	if err != nil {
		return "", err
	}
	if status != "V" {
		return "", fmt.Errorf("no valid certificate found for %s", cn)
	}
	return serial, nil
}

func certificateSerial(cn string) (string, string, error) {
	certs, err := lib.ReadCerts(filepath.Join(state.GlobalCfg.OVConfigPath, "pki", "index.txt"))
	if err != nil {
		return "", "", err
	}
	var serial, status string
	for _, cert := range certs {
		if cert.Details != nil && cert.Details.CN == cn {
			serial, status = cert.Serial, cert.EntryType
			if status == "V" {
				return serial, status, nil
			}
		}
	}
	if serial != "" {
		return serial, status, nil
	}
	return "", "", fmt.Errorf("no certificate found for %s", cn)
}
