# Linux Docker 试用部署

这份部署从**当前源码**构建，包含中英文切换和按用户组控制网段访问。需要一台能访问目标内网的 Linux 主机、Docker Engine 和 Docker Compose v2。`openvpn` 与 `openvpn-ui` 使用 `network_mode: host`，适用于 Linux 和基于 Linux 内核的 NAS。无需在宿主机安装 Go、OpenVPN 或数据库。

## 1. 上传和填写配置

把整个当前项目目录（包括 `vendor`、`views`、`static` 和新部署文件）上传至 Linux，例如 `/opt/openvpn-ui`。只重新克隆上游仓库不会包含本地改动。

```bash
cd /opt/openvpn-ui
cp .env.example .env
chmod 600 .env
nano .env
```

| 配置项 | 填写说明 |
| --- | --- |
| `OPENVPN_PUBLIC_HOST` | 客户端能访问的 Linux IP 或域名；必须替换示例，不带协议或端口 |
| `OPENVPN_PUBLIC_PORT` | OpenVPN 监听的 UDP 端口，默认 `1194` |
| `OPENVPN_VPN_CIDR` | 分配给 VPN 客户端的地址池，默认 `10.8.0.0/24`；当前试用限定 `/24` |
| `OPENVPN_LAN_CIDR` | 首次初始化使用的示例内网；正式权限在 Web 后台维护，可配置多个 IPv4 网段 |
| `OPENVPN_ADMIN_USERNAME` | 管理网页登录名，默认 `admin` |
| `OPENVPN_ADMIN_PASSWORD` | 管理网页密码，必须替换，至少 12 字符；不是 VPN 客户端密码 |
| `UI_BIND_IP` | 默认 `127.0.0.1`；要从内网直接打开网页，改为 Linux 的内网 IP |
| `UI_PORT` | 管理网页端口，默认 `8080` |

VPN 地址池不能与目标内网、客户端所在地网络或 Docker 网络重叠。`.env` 密码若含 `$` 或 `#` 等字符，请使用单引号包住整个值，例如 `OPENVPN_ADMIN_PASSWORD='your-long-random-password'`。

## 2. 启动

```bash
test -c /dev/net/tun || sudo modprobe tun
sudo sysctl -w net.ipv4.ip_forward=1
docker compose config --quiet
docker compose up -d --build
docker compose logs -f init
```

首次初始化会生成 CA、服务端证书和 DH 参数，可能耗时几分钟。看到 `Initialization complete` 后按 Ctrl+C 退出日志查看，不会停止容器。

```bash
docker compose ps -a
docker compose logs --tail=100 openvpn openvpn-ui
```

`init` 正常状态是 `Exited (0)`，其余两个服务应为 `Up` / `healthy`。主机需允许所选 UDP 端口；主机在路由器后时，还需将该 UDP 端口转发至 Linux。云服务器需设置对应安全组。管理端口和 OpenVPN 管理协议端口 `2080` 无需向公网开放。

默认通过 SSH 隧道打开网页：在自己的电脑运行以下命令，保持 SSH 连接，然后访问 `http://127.0.0.1:8080`。

```bash
ssh -L 8080:127.0.0.1:8080 your-user@your-linux-host
```

如果已把 `UI_BIND_IP` 配成 Linux 内网 IP，则直接访问 `http://Linux内网IP:8080`（端口以 `UI_PORT` 为准）。网页使用 `.env` 中的管理员账号登录，右上角可切换中文/English。

## 3. 配置访问权限并生成客户端

1. 登录后检查“服务端配置”和“客户端配置”：公网地址、端口和 VPN 地址池应与首次 `.env` 配置一致。
2. 打开“配置 → VPN 访问控制”，点击“发现宿主机网络”。也可以手工新增任意有效 IPv4 CIDR。自动发现的网段默认禁用，检查名称和范围后再启用。
3. 创建用户组，勾选该组允许访问的网段并保存。一个组可访问多个网段，一个网段也可分配给多个组。
4. 创建 VPN 用户并选择唯一用户组。固定 VPN IP 留空时自动分配，也可以手工指定地址池内未使用的地址。
5. 下载该用户的 `.ovpn` 并导入 OpenVPN 客户端。每个文件对应一个证书用户，连接时无需输入 VPN 用户名密码。
6. 连接后访问获准的内网设备。Linux 本身必须能访问目标网络。

CCD 只向客户端推送所属用户组获准的路由，宿主机防火墙还会按固定 VPN IP 强制过滤。没有明确授权的流量默认拒绝，客户端手工添加路由也不能绕过。VPN 出站使用 NAT，通常不需要给内网设备添加回程路由。

## 4. 后续修改与持久化

`.env` 中的地址、初始网段和管理员账号用于**第一次初始化**。数据库存在后，不会覆盖网页里保存的设置。后续修改公网地址和导出端口使用“客户端配置”页面；用户、组和授权网段统一在“VPN 访问控制”页面维护。

改变 `UI_BIND_IP` 或 `UI_PORT` 后执行 `docker compose up -d`。`host` 网络模式没有 Docker 端口映射；端口由进程直接在 Linux 主机上监听。管理员密码在网页账户设置中修改；单改 `.env` 不会重置已有密码。

数据保存在项目目录下：

| 目录 | 内容 |
| --- | --- |
| `data/db` | 管理账号、网页设置 SQLite 数据库 |
| `data/openvpn` | CA 和私钥、用户证书、服务端与客户端配置、固定 IP 设置 |
| `data/log` | OpenVPN 日志和连接状态 |

停止/重启/更新：

```bash
docker compose down
docker compose up -d
# 更新源码（含中英文改动）后重新构建：
docker compose up -d --build
```

备份时先停止服务，完整打包数据，随后启动：

```bash
docker compose stop
sudo tar -czf "openvpn-backup-$(date +%Y%m%d-%H%M%S).tar.gz" .env data
docker compose up -d
```

备份含 CA 私钥、客户端私钥和管理员数据，请保存在受控位置。不要删除 `data` 来升级；删除它会丢失 CA 和用户配置。

## 5. 已知边界和排错

- 本套文件只面向 Linux Docker Engine 或基于 Linux 内核的 NAS，不支持 Windows 生产部署。
- 为应用宿主机访问控制，OpenVPN 与管理界面都拥有 `NET_ADMIN`；管理界面还挂载 Docker socket。仅向可信管理员开放网页。
- 固定容器名为 `openvpn` 和 `openvpn-ui`，与现有重启脚本匹配；同一宿主机不要同时部署第二套同名实例。
- 当前访问控制只管理 IPv4 CIDR；IPv6 和全流量代理未纳入这套策略。
- 若网页中的 Docker 重启功能报 API 版本错误，先用 `docker compose restart openvpn`；上游脚本使用固定 Docker API 版本，较新引擎可能不兼容。
- `init` 失败先看 `docker compose logs init`；不要反复删除数据。`openvpn` 不健康时查看 `docker compose logs openvpn` 和 `sudo tail -n 100 data/log/openvpn.log`。
- VPN 能连接但内网不通：先确认 Linux 到目标 IP 可达、目标设备防火墙允许访问、用户组已勾选并启用目标网段。再检查 `docker compose exec openvpn-ui iptables -S OPENVPN_UI_FORWARD` 和 NAT 规则。
