# OpenVPN UI

OpenVPN UI 是一个面向小型 OpenVPN 环境的 Web 管理界面。它可以管理 OpenVPN 服务端配置、EasyRSA PKI、客户端证书和连接状态，并生成可直接导入客户端的 `.ovpn` 配置文件。

当前分支增加了简体中文界面和一套从源码构建的 Docker Compose 部署方案。

<img src="https://raw.githubusercontent.com/d3vilh/openvpn-ui/main/docs/images/OpenVPN-UI-Home.png" alt="OpenVPN UI 首页"/>

[![最新版本](https://img.shields.io/github/v/release/d3vilh/openvpn-ui?color=%2344cc11&label=LATEST%20RELEASE&style=flat-square&logo=Github)](https://github.com/d3vilh/openvpn-ui/releases/latest)
[![Docker 镜像版本](https://img.shields.io/docker/v/d3vilh/openvpn-ui/latest?logo=docker&label=DOCKER%20IMAGE&color=%2344cc11&style=flat-square&logoColor=white)](https://hub.docker.com/r/d3vilh/openvpn-ui)
![Docker 镜像大小](https://img.shields.io/docker/image-size/d3vilh/openvpn-ui/latest?logo=Docker&color=%2344cc11&label=IMAGE%20SIZE&style=flat-square&logoColor=white)

## 主要功能

- 查看 OpenVPN 服务状态、运行统计和已连接客户端。
- 管理 OpenVPN 服务端和客户端配置。
- 生成、下载、续期、吊销、删除和查看客户端证书。
- 生成包含 CA、客户端证书、私钥和 TLS 密钥的 `.ovpn` 文件。
- 支持为客户端证书设置私钥密码和固定 VPN IP。
- 支持基于 `oath-toolkit` 的双因素认证（2FA）。
- 管理 EasyRSA 参数、CA、DH、CRL 和 TLS 密钥。
- 在网页中查看 OpenVPN 日志并重启相关容器。
- 管理 OpenVPN UI 用户及管理员权限。
- 支持英文和简体中文界面，切换语言不会刷新页面或清空未保存的表单。
- 支持 AMD64、ARM64 等 Docker 可用架构，由目标机器在本地构建镜像。

## 推荐部署方式

当前仓库已经包含完整的 Docker Compose 部署文件，会从当前源码构建镜像，因此包含本分支的中文界面。部署包含三个服务：

| 服务 | 作用 |
| --- | --- |
| `init` | 首次创建数据库、OpenVPN 配置、CA 和服务端证书，完成后正常退出 |
| `openvpn` | 运行 OpenVPN 服务端，使用 TUN 设备和 `NET_ADMIN` 权限 |
| `openvpn-ui` | 提供管理网页，管理配置、证书和 OpenVPN 容器 |

### 环境要求

- Linux 主机。
- Docker Engine 和 Docker Compose v2。
- 主机存在 `/dev/net/tun`，并且能够访问需要通过 VPN 连接的目标内网。
- 首次构建时能够访问 Docker Hub 和 Alpine 软件仓库。

### 快速开始

```bash
git clone <当前仓库地址> openvpn-ui
cd openvpn-ui

cp .env.example .env
chmod 600 .env
nano .env

test -c /dev/net/tun || sudo modprobe tun
docker compose config --quiet
docker compose up -d --build
```

首次初始化需要生成 CA、服务端证书和 DH 参数，可能耗时几分钟。查看启动状态：

```bash
docker compose ps -a
docker compose logs -f init
docker compose logs --tail=100 openvpn openvpn-ui
```

`init` 服务正常完成后应显示 `Exited (0)`，`openvpn` 和 `openvpn-ui` 应显示为 `Up` 或 `healthy`。

完整的部署、端口转发、备份和排错说明请阅读：[Linux Docker 部署说明](docs/docker-deployment.zh-CN.md)。

> 使用上游预编译镜像 `d3vilh/openvpn-ui:latest` 不会包含本分支的中文界面。要使用当前修改，请执行 `docker compose up -d --build` 从源码构建。

## 环境变量

复制 `.env.example` 为 `.env` 后，至少需要修改服务器地址和管理员密码。

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `OPENVPN_PUBLIC_HOST` | `vpn.example.com` | 客户端能访问的服务器 IP 或域名，必须修改；不要填写协议或端口 |
| `OPENVPN_PUBLIC_PORT` | `1194` | 对外提供服务的 UDP 端口 |
| `OPENVPN_VPN_CIDR` | `10.8.0.0/24` | 分配给 VPN 客户端的地址池；当前固定 IP 功能要求使用 `/24` |
| `OPENVPN_LAN_CIDR` | `192.168.18.0/24` | VPN 客户端需要访问的目标内网 |
| `OPENVPN_ADMIN_USERNAME` | `admin` | 管理网页的初始管理员账号 |
| `OPENVPN_ADMIN_PASSWORD` | 示例占位符 | 管理网页的初始密码，必须修改且至少 12 个字符 |
| `UI_BIND_IP` | `127.0.0.1` | 管理网页监听的宿主机地址；改为 Linux 内网 IP 后可从内网直接访问 |
| `UI_PORT` | `8080` | 管理网页映射到宿主机的端口 |

VPN 地址池不能与目标内网、客户端所在地网络或 Docker 网络重叠。如果密码包含 `$`、`#` 等字符，请使用单引号包住整个值：

```dotenv
OPENVPN_ADMIN_PASSWORD='your-long-random-password'
```

这些变量用于首次初始化。数据库创建后，重启容器不会覆盖网页中已经保存的设置，单独修改 `.env` 也不会重置已有管理员密码。

## 访问管理界面

默认只将管理网页绑定到宿主机的 `127.0.0.1`。可以通过 SSH 隧道访问：

```bash
ssh -L 8080:127.0.0.1:8080 your-user@your-linux-host
```

保持 SSH 连接，然后在本地打开 `http://127.0.0.1:8080`。

如果将 `UI_BIND_IP` 设置为 Linux 的内网 IP，也可以直接访问：

```text
http://Linux内网IP:8080
```

使用 `.env` 中的管理员账号登录。登录页和顶部导航栏提供 `English / 简体中文` 切换；首次访问会跟随浏览器语言，手动选择后会保存在当前浏览器中。

证书名称、用户名、OpenVPN 配置内容和原始日志属于用户数据或技术配置，不会被翻译。中文界面的维护方式和测试方法见：[中英文界面维护说明](docs/bilingual-ui.md)。

## 生成客户端配置

1. 登录管理界面。
2. 打开“配置 → OpenVPN 客户端”，确认连接地址和连接端口是客户端实际能够访问的公网 IP、内网 IP 或域名。
3. 打开“证书”，点击“创建证书”。
4. 输入客户端名称；普通证书认证不需要启用 2FA，也不需要 VPN 用户名和密码。
5. 创建完成后，点击客户端名称下载 `.ovpn` 文件。
6. 将文件导入 [OpenVPN Connect](https://openvpn.net/client/) 或其他兼容的 OpenVPN 客户端。

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-New_Client.png" alt="创建客户端证书" width="500" border="1" />

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-New_Client_download.png" alt="下载 OVPN 配置" width="500" border="1" />

当前默认部署只向客户端推送目标内网路由，不会修改客户端的默认网关。OpenVPN 容器通过 NAT 转发 VPN 流量，通常无需在内网设备上额外配置回程路由。

> 推送路由不等同于权限控制。当前版本还没有实现“按用户限制可访问网段”；客户端可能自行添加路由。真正的用户级访问控制需要配合服务端防火墙规则实现。

## 网络和防火墙

默认开放的外部服务：

| 端口 | 协议 | 用途 |
| --- | --- | --- |
| `${OPENVPN_PUBLIC_PORT}` | UDP | OpenVPN 客户端连接 |
| `${UI_PORT}` | TCP | 管理网页；默认只绑定到 `127.0.0.1` |

OpenVPN 管理接口使用容器网络中的 `2080/tcp`，没有映射到宿主机。

如果服务器位于路由器后，需要把所选 UDP 端口转发到 Linux 主机；云服务器还需要在安全组中允许这个 UDP 端口。管理网页不建议直接暴露到公网。

OpenVPN 容器启动时会：

- 开启 IPv4 转发。
- 为 VPN 地址池添加 `MASQUERADE` 规则。
- 允许 VPN 隧道的转发流量。

VPN 可以连接但无法访问内网时，依次检查：

1. Linux 主机本身是否能够访问目标内网 IP。
2. VPN 地址池是否与客户端本地网络、目标内网或 Docker 网络重叠。
3. 目标设备的防火墙是否允许来自 Linux 主机的访问。
4. 宿主机或云平台是否阻止转发流量。

查看容器内的 NAT 规则：

```bash
docker compose exec openvpn iptables -t nat -S
```

## 数据持久化

运行数据保存在项目目录的 `data/` 中：

```text
data/
├── db/                         # 管理账号和网页设置数据库
├── log/                        # OpenVPN 日志和连接状态
└── openvpn/
    ├── server.conf             # OpenVPN 服务端配置
    ├── clients/                # 生成的客户端配置
    ├── config/
    │   ├── client.conf         # 客户端配置模板
    │   └── easy-rsa.vars       # EasyRSA 参数副本
    ├── staticclients/          # 客户端固定 IP 配置
    └── pki/
        ├── ca.crt
        ├── crl.pem
        ├── dh.pem
        ├── index.txt
        ├── ipp.txt
        ├── issued/             # 已签发证书
        ├── private/            # CA、服务端和客户端私钥
        ├── reqs/               # 证书请求
        └── ta.key              # TLS 控制通道密钥
```

`data/openvpn/pki/private` 包含 CA 和客户端私钥，必须限制访问。不要通过删除 `data/` 的方式升级；删除后会丢失 CA、管理员账号和所有客户端配置。

## 备份、更新和恢复

备份前先停止服务，完整保存 `.env` 和 `data/`：

```bash
docker compose stop
sudo tar -czf "openvpn-backup-$(date +%Y%m%d-%H%M%S).tar.gz" .env data
docker compose up -d
```

备份文件包含管理员信息、CA 私钥和客户端私钥，请保存在受控位置。

更新源码后重新构建：

```bash
git pull
docker compose up -d --build
docker compose ps -a
docker compose logs --tail=100 openvpn openvpn-ui
```

升级前建议先完成备份。如果新版出现问题，恢复旧源码和完整的 `.env`、`data/` 备份，再重新构建启动。

## 证书续期、吊销和删除

在“证书”页面可以执行以下操作：

- **续期**：为同一客户端生成新证书。新旧证书会暂时同时存在，方便先将新配置交给客户端。
- **吊销**：将证书加入 CRL，阻止它再次连接。吊销不会自动断开已经建立的连接；如需立即断开，请重启 OpenVPN 服务。
- **删除**：删除已经吊销的证书及相关文件。删除属于不可恢复操作，执行前应确认备份可用。

续期证书下载后，应先在客户端确认新配置可以连接，再吊销旧证书。

## 双因素认证（2FA）

OpenVPN UI 从 `0.9.3` 开始支持基于 [oath-toolkit](https://savannah.nongnu.org/projects/oath-toolkit/) 的双因素认证，不依赖第三方 2FA 服务。

启用步骤：

1. 在“配置 → OpenVPN 客户端”中启用双因素认证，使证书页面可以创建带 2FA 信息的证书。
2. 创建证书时填写唯一的 `2FA Name`，建议使用类似邮箱地址的格式。
3. 创建完成后，在证书详情页使用 Google Authenticator、Microsoft Authenticator 等应用扫描二维码。
4. 下载 `.ovpn` 并导入客户端。连接时，用户名填写创建证书时的 `2FA Name`，密码填写认证器当前显示的一次性验证码。
5. 所有客户端准备完毕后，再在“配置 → OpenVPN 服务端”中启用 2FA。

> 服务端启用 2FA 后，只接受支持 2FA 的客户端配置，普通证书将无法连接。建议先分发并验证所有新配置。

创建证书时还可以设置 `Passphrase` 保护客户端私钥。这种情况下，`Passphrase` 是私钥密码，认证器生成的数字仍作为登录密码。

2FA 证书的续期、吊销和删除流程与普通证书相同。

## Google OAuth 2.0 登录

OpenVPN UI 从 `0.9.5.5` 开始支持通过 Google OAuth 2.0 登录。需要在 [Google Developer Console](https://console.developers.google.com/) 创建 OAuth 客户端，并配置以下环境变量：

```text
GOOGLE_CLIENT_ID
GOOGLE_CLIENT_SECRET
GOOGLE_REDIRECT_URL
ALLOWED_DOMAINS
```

`ALLOWED_DOMAINS` 用于限制允许登录的邮箱域名。OAuth 凭据属于敏感信息，不应提交到 Git 仓库或写入公开镜像。

此功能由 [opsnin](https://github.com/d3vilh/openvpn-ui/pull/89) 贡献。

## 用户管理

OpenVPN UI 从 `0.9.2` 开始支持管理网页用户：

- 管理员可以访问全部功能和配置页面。
- 普通用户只能访问首页、证书和日志页面，但仍可以创建、续期、吊销和删除证书。

用户管理入口位于右上角账号菜单中的“用户资料”。这里的账号用于登录管理网页，与普通证书模式下的 VPN 客户端身份不是同一概念。

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-ProfileManage.png" alt="OpenVPN UI 用户管理" width="700" border="1" />

## 从源码运行（不使用 Docker）

如果 OpenVPN 服务已经运行在同一台主机，也可以直接运行 OpenVPN UI。需要：

- Go 版本满足 `go.mod` 的要求。
- GCC 和 SQLite3 CGO 构建环境。
- OpenVPN、EasyRSA 及项目脚本依赖。
- 在 `conf/app.conf` 中正确配置 `OpenVpnPath` 和 `EasyRsaPath`。
- OpenVPN 管理接口可从 UI 进程访问。

构建示例：

```bash
go build -mod=vendor -o openvpn-ui .
mkdir -p db

export OPENVPN_ADMIN_USERNAME=admin
export OPENVPN_ADMIN_PASSWORD='your-long-random-password'
./openvpn-ui -config ./conf
```

首次登录成功后，应从运行环境中删除初始管理员密码变量：

```bash
unset OPENVPN_ADMIN_USERNAME
unset OPENVPN_ADMIN_PASSWORD
```

当前分支主要验证 Docker 部署流程。非 Docker 安装需要自行提供 OpenVPN 服务、PKI、文件权限、管理接口和进程重启机制。

## 常用维护命令

```bash
# 查看状态
docker compose ps -a

# 查看日志
docker compose logs -f openvpn openvpn-ui

# 重启服务端
docker compose restart openvpn

# 重启管理界面
docker compose restart openvpn-ui

# 停止并移除容器，保留 data/ 中的数据
docker compose down

# 重新构建并启动
docker compose up -d --build
```

管理界面为了调用现有的容器重启功能，会以只读挂载形式访问 `/var/run/docker.sock`。Docker socket 即使只读挂载也具有较高权限，因此管理网页只应向可信管理员开放。

## 已知范围

- 当前 Docker 试用方案支持一个 IPv4 目标内网网段。
- 客户端固定 VPN IP 功能当前按 `/24` 地址池生成配置。
- 当前版本尚未提供按用户限制访问网段的完整策略管理。
- 复杂多网段、站点到站点 VPN、全流量代理和精细访问控制需要额外配置。
- 本仓库的部署配置面向 Linux Docker Engine，不支持直接在 Windows 容器模式中运行。

## 页面截图

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-Login.png" alt="OpenVPN UI 登录页" width="1000" border="1" />

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-Certs.png" alt="OpenVPN UI 证书页面" width="1000" border="1" />

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-Server-config.png" alt="OpenVPN UI 服务端配置" width="1000" border="1" />

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-ClientConf.png" alt="OpenVPN UI 客户端配置" width="1000" border="1" />

<img src="https://github.com/d3vilh/openvpn-ui/blob/main/docs/images/OpenVPN-UI-Logs.png" alt="OpenVPN UI 日志页面" width="1000" border="1" />

## 相关项目

- [d3vilh/openvpn-server](https://github.com/d3vilh/openvpn-server)：与 OpenVPN UI 配套的 OpenVPN 服务端容器。
- [OpenVPN-AWS](https://github.com/d3vilh/openvpn-aws)：面向云服务器、虚拟机和 x86 裸机的 OpenVPN 部署项目。
- [Raspberry-Gateway](https://github.com/d3vilh/raspberry-gateway)：面向 Raspberry Pi 的家庭网关环境，集成 Pi-hole、Unbound、VPN、下载工具和网络监控。

## 致谢

感谢 [Adam Walach](https://github.com/adamwalach) 开发原始的 [OpenVPN-WEB-UI](https://github.com/adamwalach/openvpn-web-ui) 项目，为 OpenVPN UI 提供了扎实的基础。

感谢 OpenVPN UI 及其依赖项目的所有贡献者。

<a href="https://www.buymeacoffee.com/d3vilh" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="请作者喝杯咖啡" height="51" width="217"></a>

## 许可证

本项目使用 [MIT 许可证](LICENSE)。使用、修改或重新分发前，请同时检查项目所依赖组件和容器镜像各自的许可证。
