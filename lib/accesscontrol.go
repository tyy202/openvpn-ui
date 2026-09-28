package lib

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/d3vilh/openvpn-server-config/server/mi"
	"github.com/d3vilh/openvpn-ui/internal/accesspolicy"
	"github.com/d3vilh/openvpn-ui/models"
	"github.com/d3vilh/openvpn-ui/state"
)

var certificateCNPattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

func ValidateCertificateCN(value string) error {
	if !certificateCNPattern.MatchString(value) {
		return fmt.Errorf("certificate CN may contain only letters, numbers, dot, underscore, @ and hyphen")
	}
	return nil
}

func VPNPoolCIDR() (string, error) {
	cfg := models.OVConfig{Profile: "default"}
	if err := cfg.Read("Profile"); err != nil {
		return "", err
	}
	parts := strings.Fields(cfg.Server)
	if len(parts) != 3 || parts[0] != "server" {
		return "", fmt.Errorf("unsupported OpenVPN server directive %q", cfg.Server)
	}
	ip := net.ParseIP(parts[1]).To4()
	maskIP := net.ParseIP(parts[2]).To4()
	if ip == nil || maskIP == nil {
		return "", fmt.Errorf("invalid OpenVPN server directive %q", cfg.Server)
	}
	mask := net.IPMask(maskIP)
	ones, bits := mask.Size()
	if bits != 32 || ones < 0 {
		return "", fmt.Errorf("invalid OpenVPN netmask %q", parts[2])
	}
	return (&net.IPNet{IP: ip.Mask(mask), Mask: mask}).String(), nil
}

func AllocateVPNAddress(requested string) (string, error) {
	cidr, err := VPNPoolCIDR()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(requested) != "" {
		address := strings.TrimSpace(requested)
		if err := accesspolicy.ValidateStaticIPv4(cidr, address); err != nil {
			return "", err
		}
		count, err := orm.NewOrm().QueryTable(new(models.VPNUser)).Filter("StaticIP", address).Count()
		if err != nil {
			return "", err
		}
		if count > 0 {
			return "", fmt.Errorf("fixed VPN IP %s is already assigned", address)
		}
		return address, nil
	}
	var users []*models.VPNUser
	if _, err := orm.NewOrm().QueryTable(new(models.VPNUser)).All(&users); err != nil {
		return "", err
	}
	used := make(map[string]bool, len(users))
	for _, user := range users {
		used[user.StaticIP] = true
	}
	return accesspolicy.AllocateIPv4(cidr, used)
}

func userNetworks(o orm.Ormer, user *models.VPNUser) ([]string, error) {
	if user.Group == nil || !user.Group.Enabled {
		return nil, nil
	}
	var links []*models.GroupNetwork
	_, err := o.QueryTable(new(models.GroupNetwork)).Filter("Group__Id", user.Group.Id).RelatedSel("Network").All(&links)
	if err != nil {
		return nil, err
	}
	var networks []string
	for _, link := range links {
		if link.Network != nil && link.Network.Enabled {
			networks = append(networks, link.Network.CIDR)
		}
	}
	sort.Strings(networks)
	return networks, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".openvpn-ui-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(mode)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func ReconcileAccessPolicy() error {
	o := orm.NewOrm()
	var users []*models.VPNUser
	if _, err := o.QueryTable(new(models.VPNUser)).RelatedSel("Group").All(&users); err != nil {
		return err
	}
	cidr, err := VPNPoolCIDR()
	if err != nil {
		return err
	}
	ccdDir := filepath.Join(state.GlobalCfg.OVConfigPath, "staticclients")
	if err := os.MkdirAll(ccdDir, 0750); err != nil {
		return err
	}
	policies := make([]accesspolicy.UserPolicy, 0, len(users))
	for _, user := range users {
		if err := ValidateCertificateCN(user.CertificateCN); err != nil {
			return err
		}
		networks, err := userNetworks(o, user)
		if err != nil {
			return err
		}
		enabled := user.Enabled && user.Group != nil && user.Group.Enabled
		content, err := accesspolicy.RenderCCD(user.StaticIP, enabled, networks)
		if err != nil {
			return err
		}
		if err := atomicWrite(filepath.Join(ccdDir, user.CertificateCN), []byte(content), 0640); err != nil {
			return err
		}
		policies = append(policies, accesspolicy.UserPolicy{IP: user.StaticIP, Enabled: enabled, Networks: networks})
	}
	script, err := accesspolicy.CompileFirewall(cidr, policies)
	if err != nil {
		return err
	}
	scriptPath := filepath.Join(state.GlobalCfg.OVConfigPath, "policy", "apply-acl.sh")
	if err := atomicWrite(scriptPath, []byte(script), 0750); err != nil {
		return err
	}
	if runtime.GOOS == "linux" {
		output, err := exec.Command("/bin/sh", scriptPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("apply firewall policy: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func DiscoverNetworkResources() ([]accesspolicy.DiscoveredRoute, error) {
	cidr, err := VPNPoolCIDR()
	if err != nil {
		return nil, err
	}
	output, err := exec.Command("ip", "-j", "-4", "route", "show", "table", "main").Output()
	if err != nil {
		return nil, err
	}
	routes, err := accesspolicy.ParseDiscoveredRoutes(output, cidr)
	if err != nil {
		return nil, err
	}
	o := orm.NewOrm()
	_, _ = o.QueryTable(new(models.NetworkResource)).Filter("Source", "discovered").Update(orm.Params{"Reachable": false})
	for _, route := range routes {
		row := models.NetworkResource{CIDR: route.CIDR}
		err := o.Read(&row, "CIDR")
		if err == orm.ErrNoRows {
			row.Name = route.CIDR
			row.Source = "discovered"
			row.Interface = route.Interface
			row.Gateway = route.Gateway
			row.Enabled = false
			row.Reachable = true
			row.LastSeen = time.Now()
			if _, err = o.Insert(&row); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		} else {
			row.Interface = route.Interface
			row.Gateway = route.Gateway
			row.Reachable = true
			row.LastSeen = time.Now()
			if _, err = o.Update(&row, "Interface", "Gateway", "Reachable", "LastSeen"); err != nil {
				return nil, err
			}
		}
	}
	return routes, nil
}

func KillVPNSession(cn string) error {
	_, err := mi.NewClient(state.GlobalCfg.MINetwork, state.GlobalCfg.MIAddress).KillSession(cn)
	return err
}
