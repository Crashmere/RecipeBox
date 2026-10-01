# 安装与运行

公网入口使用可信 IP 证书的 HTTPS，原有 /recipebox/ 路径保持。公网 HTTP 返回 308；API 客户端直接使用 HTTPS。Nginx 覆盖 `X-Forwarded-Proto`；写入来源校验只信任来自回环地址的代理头，仍拒绝跨站来源。证书、续期、回退和整机验收见 [共享 HTTPS 运维](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/https.md)（服务器副本 /opt/server-context/references/https.md）。本项目的后端与发布检查保留本机 HTTP，127.0.0.1:80 的代理检查入口不能从公网访问。公网已接入 ServerPortal 统一设备认证：先在 /portal/login 输入口令授权设备，随后使用同源 Secure/HttpOnly Cookie 访问；未授权 API 返回 401。本机发布检查与服务间调用保留。

操作 ali 前加载 server-operations 并显式读取 `/opt/AGENTS.md`。公开仓库不写正式公网地址；通过受信 SSH 别名取得现场信息。

图片大小：Nginx 请求上限 26 MiB，程序单张图片上限 25 MiB。超过 1 MiB 的图片若在进入程序前返回 500，见[统一认证误拦大请求](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#统一认证误拦大请求)；共享修正保留本应用的上传限制。

## 布局

`/opt/recipebox/bin/recipebox` 为内嵌页面的 Linux amd64 程序。config 放本项目 unit/location，data 放 recipebox.db、media、tmp，backups 放快照，docs 与 AGENTS.md 是受控文档副本，releases 留发布历史，current-commit 为程序来源。

运行身份 recipebox，发布 recipebox-deploy；data/backups 0700，文件 0600，程序/配置 root 管理。监听 127.0.0.1:18083，经共享 Nginx `/recipebox/` 访问，不增加公网监听端口。systemd 限制写入 data，内存 640 MiB，CPU 100%。

## 首次安装

先核对端口、身份、目录、现有全部服务健康。Go 在本机构建；服务器只需 libvips-tools、libheif-plugin-libde265（同机现有官方 Ubuntu 包），不安装 Go/Node/Docker 服务。软件安装遵循 software-installation。

```sh
make linux
# 将已审阅提交中的 deploy 与校验后的二进制上传独立暂存目录。
bash deploy/install.sh /path/to/recipebox-linux-amd64
ln -s /opt/recipebox/config/nginx-location.conf /etc/nginx/app-locations/recipebox.conf
nginx -t
systemctl reload nginx
```

install 拒绝覆盖现有目录/身份，显式初始化正式空库，开启服务和备份 timer，取得首份备份。正式库不填入测试记录。安装后核对四个服务直连/反代、深链接和资源。

## 本地开发与浏览器验证

README 有启动步骤。当前 Mac 有 goenv 包装器时用 `GOENV_VERSION=1.24.0 go ...`，go.mod 选择 go1.26.8；不更改系统默认版本。仅首次 init，已有库直接 serve。

```sh
npm --prefix web run build
bin/recipebox init --data .local/e2e-data
bin/recipebox serve --data .local/e2e-data --with-prefix
# 另一终端：
```

若要测试空库，另建唯一目录，不清空正式数据。

## 备份与恢复

每天 Asia/Shanghai 03:45 加 0–5 分钟随机延迟，保留 14 份 daily。备份 unit 的可写目录必须是整个 /opt/recipebox，原因见共享的 [systemd 沙箱下照片硬链接备份失败](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#systemd-沙箱下照片硬链接备份失败)。备份 SQLite 一致性快照、图片硬链接和 SHA-256 清单。before-deploy 按共享发布保留策略轮换，manual 不自动清理；2026-09-27 已另取包含本项目数据库和照片的全应用归档，下载到维护电脑并校验，见 [共享备份说明](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/current-state.md#手工数据归档)。本应用脚本不主动异机同步；服务器另有阿里云文件备份，范围、30 天保留与恢复限制见[主机云备份](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/current-state.md#云备份)。

```sh
systemctl status recipebox-backup.timer
systemctl start recipebox-backup.service
journalctl -u recipebox-backup.service -n 50 --no-pager
runuser -u recipebox -- /opt/recipebox/bin/recipebox backup --data /opt/recipebox/data --out /opt/recipebox/backups/manual-UNIQUE
```

备份目的目录必须不存在；锁冲突需稍后重试。不要直接复制活跃 WAL 主文件或修改硬链接照片。只有完整 manifest.json 的目录才可恢复。

```sh
recipebox restore --source /path/to/backup --out /path/to/new-restore-directory
recipebox check --data /path/to/new-restore-directory
# 先检查 19083 空闲，再启动隔离恢复实例
recipebox serve --data /path/to/new-restore-directory --listen 127.0.0.1:19083 --with-prefix
```

restore 验证校验和与相对路径，拒绝已有目标，照片复制为独立文件。生产恢复前确认时点和可能丢失的后续修改，停本应用，保留当前目录，恢复到新目录并校验后切换身份/路径，重启验证。程序回退不等于数据库回退。

## 诊断

```sh
systemctl status recipebox recipebox-backup.timer
journalctl -u recipebox -n 100 --no-pager
curl --fail http://127.0.0.1:18083/healthz
curl --fail http://127.0.0.1/recipebox/healthz
systemctl show recipebox -p MemoryCurrent -p MemoryPeak -p NRestarts
df -h /opt/recipebox
du -sh /opt/recipebox/data /opt/recipebox/backups
vips --version
```

每次记录写入在 journal 记一行 `write <路径> action=… status=… replay=… <服务端耗时>`，不含表单内容；格式、JSON 错误在解析阶段拒绝，不记录。保存慢时，先按时间核对这一行：耗时仅几毫秒且随后出现 `replay=true`，说明服务端已写入，是响应在网络上丢失或延迟后同一请求再次到达；共享 Nginx 访问日志的时间是请求结束时间，不含耗时。

```sh
journalctl -u recipebox --since '-1h' --no-pager | grep ' write '
```

缺库/损坏时查备份和权限，不能通过 init 创建空库掩盖错误。507 核对根盘/照片配额；429 核对备份锁与并发上传；422 核对图片格式与 vips 日志。libvips 8.15/8.18 共用 `--export-profile=srgb`。

文档提交推送后运行 `~/agent-config/skills/server-operations/scripts/sync-docs.sh RecipeBox`，它负责漂移检查、安装到 /opt/recipebox、逐文件校验、docs/SOURCE 和清理（用法见 server-operations 的 maintenance）。共享清单改动后不带参数运行同一脚本。

## ServerPortal 接入材料

`deploy/portal.json` 是本应用资源说明的维护源。本机发布使用 server-operations 校验器检查，再将同一声明与二进制保存到同一本地版本目录；发布前执行 `portal-check`，发布后执行 `portal`，通过现有受限 SSH 安装到 `/opt/recipebox/config/portal.json` 并核对采集器实际加载的 SHA-256。`config/portal-source.json` 记录声明来源提交；它与程序的 current-commit 各自表示不同材料的版本。

门户从 `/opt/serverportal/registry.d/recipebox.json` 的受控链接发现本应用，声明成功更新后自动加载，无需重启。首次正常 本机发布也会建立链接，无需再编辑门户中央应用列表。普通发布可更新本应用的声明，其他 unit/env/Nginx/发布脚本仍由管理员安装。

源码或数据库行为变更不能使用该选项代替程序发布。

数据根、媒体、备份格式、unit、端口或访问路径变化时，同一提交维护声明及对应文档，更新共享清单并核对资源覆盖。文件、媒体、数据库表和 systemd 状态由门户自动读取；目录用途、API 说明和权限边界须由维护 agent 明确更新。共同协议、失败处置与新应用接入见 [门户维护](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/portal.md)。

门户 /portal/ 已统一保护公网访问，发布脚本通过回环检查应用健康，本机发布公网检查预期未授权返回 401。设备授权永久有效至主动撤销，Cookie 经共享 Nginx 随有效请求续期；本应用若新增 add_header，必须保留共享 Set-Cookie 转发，规则及验收见共享门户维护文档。门户备份使用本应用原生一致性快照；真实完整链恢复验收按用户要求暂缓，不因本次维护自动继续下载或恢复。

## 手机桌面图标

现有 /recipebox/apple-touch-icon.png 为 180×180；Nginx 规则仅放行它与 favicon.ico、recipebox.svg 的 GET/HEAD。页面、API 和用户媒体继续使用设备认证。共同原因、部署状态与手机验收见[共享排障记录](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#统一认证后-iphone-桌面图标缺失)。

## 当前发布入口

本项目为个人使用：在本地验证本次改动即可发布，不设全量回归门槛，不默认新增或保留永久测试。界面改动检查实际使用的电脑/手机场景；数据迁移、批量写入/删除和备份恢复先用隔离副本针对性验证。

完整流程见 [本机发布与回退](DEPLOYMENT.md)。GitHub 只保存源码；本机 `make release` 构建，`make deploy` 更新生产，文档单独同步。

## 发布材料自动清理

服务器每天北京时间 05:00 按[发布材料自动保留](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md)保留最近 3 次成功发布、最近 5 份完整发布前备份，并保护当前版本、对应备份和待核对失败批次。共享实现、锁、回执、预览及停用命令由 server-operations 维护；本项目 deploy 保留发布脚本的 recovery 标记和每日备份的 flock 入口。首次安装先按共享文档建立 /run/lock/ali-release-retention.lock，再启用 backup timer。daily、manual、业务数据、门户 exports 和维护电脑构建材料不在此自动清理范围。
