# 安装与运行

操作 ali 前加载 server-operations 并显式读取 `/opt/AGENTS.md`。公开仓库不写正式公网地址；通过受信 SSH 别名取得现场信息。

## 布局

`/opt/recipebox/bin/recipebox` 为内嵌页面的 Linux amd64 程序。config 放本项目 unit/location，data 放 recipebox.db、media、tmp，backups 放快照，docs 与 AGENTS.md 是受控文档副本，releases 留发布历史，current-commit 为程序来源。

运行身份 recipebox，发布 recipebox-deploy；data/backups 0700，文件 0600，程序/配置 root 管理。监听 127.0.0.1:18083，经共享 Nginx `/recipebox/` 访问，不增加公网监听端口。systemd 限制写入 data，内存 640 MiB，CPU 100%。

## 首次安装

先核对端口、身份、目录、现有全部服务健康。Go 在本地/CI 构建；服务器只需 libvips-tools、libheif-plugin-libde265（同机现有官方 Ubuntu 包），不安装 Go/Node/Docker 服务。软件安装遵循 software-installation。

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
npm --prefix web ci
npm --prefix web run build
go test -race ./...
go vet ./...
npm --prefix web test
bin/recipebox init --data .local/e2e-data
bin/recipebox serve --data .local/e2e-data --with-prefix
# 另一终端：
npm --prefix web run test:e2e -- --project=chromium
```

完整双引擎测试需 Playwright WebKit（官方来源，项目内存放）：

```sh
PLAYWRIGHT_BROWSERS_PATH="$PWD/.local/playwright" node web/node_modules/playwright/cli.js install webkit
PLAYWRIGHT_BROWSERS_PATH="$PWD/.local/playwright" npm --prefix web run test:e2e
```

测试仅写 .local；CI 使用其隔离 runner。若要测试空库，另建唯一目录，不清空正式数据。

## 备份与恢复

每天 Asia/Shanghai 03:45 加 0–5 分钟随机延迟，保留 14 份 daily。备份 SQLite 一致性快照、图片硬链接和 SHA-256 清单。before-deploy / manual 不自动轮换；尚无异机备份。

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

缺库/损坏时查备份和权限，不能通过 init 创建空库掩盖错误。507 核对根盘/照片配额；429 核对备份锁与并发上传；422 核对图片格式与 vips 日志。libvips 8.15/8.18 共用 `--export-profile=srgb`。

文档提交推送后运行 `~/agent-config/skills/server-operations/scripts/sync-docs.sh RecipeBox`，它负责漂移检查、安装到 /opt/recipebox、逐文件校验、docs/SOURCE 和清理（用法见 server-operations 的 maintenance）。共享清单改动后不带参数运行同一脚本。
