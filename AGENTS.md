# RecipeBox 维护入口

先读 [docs/README.md](docs/README.md)，再按任务读取架构、API、运维和验证文档。

- 家庭共享菜谱接入统一设备认证；授权设备可以查看、添加、修改、删除和导出。菜谱是主体，常备食品独立作为附属功能。
- Go 内嵌 Vue + SQLite + libvips；服务器为正式数据源，不把浏览器草稿当成备份。
- 记录写入使用 revision 和 Idempotency-Key。普通资料编辑不得清空下厨日记，消耗食品不得覆盖其他字段。
- 照片不可变，数据库与照片一起备份。启动缺库必须报错，不得隐式初始化空库。
- 修改后运行前端 build、Go race 测试/vet、前端单测及相关 Playwright Chromium/WebKit 流程；重点检查 320/375 px 与桌面、日期、表单、照片、弹窗和底部保存区。
- 测试使用 .local 隔离数据。生产只读验收，不导入虚构家庭菜谱。
- 部署先读 server-operations，并显式读取 ssh ali 'cat /opt/AGENTS.md'。源码、部署配置与当前文档保持一致，推送后运行 agent-config 的 `skills/server-operations/scripts/sync-docs.sh RecipeBox` 同步服务器副本。
- 哪些事直接做完再告知、哪些先确认，只看 server-operations SKILL.md 的授权表；文档维护与同步不需要事先确认。
- 仓库 public，不提交正式数据库、照片、备份、凭据或服务器公网地址。main 推送运行 CI and deploy，检查通过后自动发布到生产；纯文档提交加 `[skip ci]`。

## 门户资源同步

- 本项目的 `deploy/portal.json` 是 ServerPortal 资源声明的维护源，记录目录用途、数据库、运行用户、端口、unit、访问路径、API 与备份类型。新增/迁移/删除数据根、接口或运行材料时，必须同步修改声明、对应 docs 与共享应用清单。
- 声明部署在 `/opt/recipebox/config/portal.json`，root 管理；普通 CI 不自动安装配置。维护安装后重启门户采集进程并核对目录、只读数据表、备份覆盖与未知资源提示。
- 统一认证由共享 Nginx 与门户负责，不在本项目另存设备白名单；本机调用和发布健康检查按共享约定保留。生产已启用设备认证；变更后同步 server-operations current-state。
- 门户只读展示不替代本项目原生一致性备份；备份格式或媒体生命周期变化必须同时验证门户全量/增量与离线恢复。真实业务数据、凭据和备份仍不得进入 Git。
