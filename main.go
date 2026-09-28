package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/beego/beego/v2/server/web"
	"github.com/d3vilh/openvpn-ui/internal/deployconfig"
	"github.com/d3vilh/openvpn-ui/lib"
	"github.com/d3vilh/openvpn-ui/models"
	"github.com/d3vilh/openvpn-ui/routers"
	"github.com/d3vilh/openvpn-ui/state"
)

func main() {
	configDir := flag.String("config", "conf", "Path to config dir")
	initOnly := flag.Bool("init-only", false, "Initialize database and configuration, then exit")
	flag.Parse()
	deployment, err := deployconfig.Read(os.Getenv)
	if err != nil {
		panic(err)
	}

	configFile := filepath.Join(*configDir, "app.conf")
	fmt.Println("Config file:", configFile)

	if err := web.LoadAppConfig("ini", configFile); err != nil {
		panic(err)
	}
	if bindAddress := os.Getenv("UI_BIND_IP"); bindAddress != "" {
		web.BConfig.Listen.HTTPAddr = bindAddress
	}
	if port := os.Getenv("UI_PORT"); port != "" {
		var parsed int
		if _, err := fmt.Sscanf(port, "%d", &parsed); err != nil || parsed < 1 || parsed > 65535 {
			panic("invalid UI_PORT")
		}
		web.BConfig.Listen.HTTPPort = parsed
	}

	models.InitDB()
	models.CreateDefaultUsers()
	defaultSettings, err := models.CreateDefaultSettings()
	if err != nil {
		panic(err)
	}

	models.CreateDefaultOVConfig(*configDir, defaultSettings.OVConfigPath, defaultSettings.MIAddress, defaultSettings.MINetwork, deployment)
	models.CreateDefaultOVClientConfig(*configDir, defaultSettings.OVConfigPath, defaultSettings.MIAddress, defaultSettings.MINetwork, deployment)
	models.CreateDefaultEasyRSAConfig(*configDir, defaultSettings.EasyRSAPath, defaultSettings.MIAddress, defaultSettings.MINetwork)
	state.GlobalCfg = *defaultSettings
	if *initOnly {
		return
	}
	if err := lib.ReconcileAccessPolicy(); err != nil {
		panic(fmt.Errorf("initialize VPN access policy: %w", err))
	}

	routers.Init(*configDir)

	lib.AddFuncMaps()
	web.Run()
}
