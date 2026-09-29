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
	"github.com/d3vilh/openvpn-ui/lib"
	"github.com/d3vilh/openvpn-ui/models"
	"github.com/d3vilh/openvpn-ui/state"
)

type AccessController struct{ BaseController }

func (c *AccessController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.Abort("403")
		return
	}
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
		_, err := lib.DiscoverNetworkResources()
		return err
	case "create-network":
		cidr := strings.TrimSpace(c.GetString("CIDR"))
		ip, n, err := net.ParseCIDR(cidr)
		if err != nil || ip.To4() == nil {
			return fmt.Errorf("invalid IPv4 CIDR")
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
		row := models.VPNGroup{Name: name, Description: strings.TrimSpace(c.GetString("Description")), Enabled: true}
		if _, err := o.Insert(&row); err != nil {
			return err
		}
		for _, value := range c.GetStrings("NetworkIDs") {
			networkID, err := parseID(value)
			if err != nil {
				_, _ = o.Delete(&row)
				return err
			}
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
		for _, value := range c.GetStrings("NetworkIDs") {
			networkID, parseErr := parseID(value)
			if parseErr != nil {
				return parseErr
			}
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
		if err = o.Read(&user); err != nil {
			return err
		}
		oldIP := user.StaticIP
		groupID, err := parseID(c.GetString("GroupId"))
		if err != nil {
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
		if err = o.Read(&user); err != nil {
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
		if err = o.Read(&user); err != nil {
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
