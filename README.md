# RecipeBox · 家的味道

一个打开即用的家庭菜谱本。把吃过还想吃的菜、厨房里的小窍门和每一次下厨留在一起。

<img src="web/public/recipebox.svg" width="88" alt="打开的菜谱与番茄餐盘图标">

- **菜谱记录**：照片、食材与用量、分步做法、分类、标签、时长、人数、拿手菜收藏。
- **省心录入**：只填菜名就能保存；批量粘贴食材、步骤排序、相册多选、拍照入口、上传重试与封面选择。
- **一起下厨**：食材打勾、大字做菜模式、下厨日期、评分与可修改的心得。
- **找回好味道**：按菜名、食材或标签搜索，分类筛选、常做/快手排序，以及“今天吃什么”随机选菜。
- **常备食品**：单独管理泡面、挂面、干货等，记录数量、单位、存放位置和日期，一键吃掉一份，过期与临期提醒。
- **安心编辑**：本机草稿、多设备冲突检查、幂等提交、30 天回收站、包含照片的 ZIP 导出。
- **手机与电脑**：暖白与番茄红界面，原创菜谱餐盘图标，适配小屏、触控和桌面。

## 本地运行

需要 Go（版本见 go.mod）、Node 24+、libvips；HEIC 还需 libheif 的 HEVC 解码插件。先使用合适的官方软件来源安装依赖。

```sh
npm --prefix web ci
make build
mkdir -p .local
bin/recipebox init --data .local/dev-data
bin/recipebox serve --data .local/dev-data --with-prefix
```

打开 `http://127.0.0.1:18083/recipebox/`。首次初始化仅用于新库，已有数据直接 serve。前端热更新用 `npm --prefix web run dev`，后端去掉 --with-prefix。

```sh
make test
npm --prefix web run test:e2e -- --project=chromium
```

项目默认使用本机 Chrome；CI 使用 Playwright Chromium。WebKit 的安装和完整检查见 [运维文档](docs/OPERATIONS.md)。

## 部署和数据

独立 Go 进程、SQLite 文件与本地照片；无需 Node 常驻或独立数据库服务。Nginx 从 `/recipebox/` 转发到 loopback，systemd 管理运行和每日备份。默认没有登录，知道网址的人拥有共同读写权限。

精确的安装、备份、恢复、发布与限制见 [项目文档](docs/README.md)。正式库从空白开始，测试数据不随发布上传。当前模式联网使用，暂无离线同步和网页导入。
