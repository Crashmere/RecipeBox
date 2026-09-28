# 本机发布与回退

本项目为个人使用：在本地验证本次改动即可发布，不设全量回归门槛，不默认新增或保留永久测试。界面改动检查实际使用的电脑/手机场景；数据迁移、批量写入/删除和备份恢复先用隔离副本针对性验证。

GitHub 仅作源码和配置备份，Actions 已关闭。提交、推送不会更新生产。共享实现和环境准备见 [server-operations 本机发布](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/release.md)；本项目参数来自 `deploy/release.json`。

## 日常使用

```sh
# 在本地走通本次功能，再提交并推送源码。
git add <本次修改的文件>
git commit -m "描述改动"
git push origin main
make release   # 从已提交快照构建；不运行全量测试
make deploy    # 复用同提交产物，经 SSH 发布
```

本地材料位于 `.local/releases/<完整提交>/`，包含 program、manifest.json 和同提交 portal.json。部署会校验哈希并确认提交已备份到 origin/main；源码未变时复用产物。未跟踪文件不进入构建。`make releases` 列出本地版本；这些材料不提交 Git。

仅更新门户信息用 `make portal`；它预检并同步同提交声明，不构建/重启业务程序。声明同步失败不会自动撤回已成功更新的程序，修复后单独重试。

## 发布边界

使用 recipebox-deploy 受限身份，私钥仅在维护电脑。首次建立账号用 `deploy/setup-deploy.sh <本机发布公钥>`；已有账号不要重复初始化。强制命令只接受 deploy、portal-check、portal，sudo 只允许该应用固定发布脚本。

上传完成且校验通过后才进入切换。保留升级前备份和 previous，健康失败恢复旧程序；正常发布不携带生产数据。unit、Nginx、env、授权及发布脚本由管理员按源码显式同步，文档使用共享 sync-docs.sh。

## 回退与排障

```sh
make releases
make rollback COMMIT=<明确选择的完整旧提交>
```

回退复用本机保留的已校验产物，仍经过备份与健康检查；先确认旧程序兼容当前数据。程序回退不恢复数据库、照片或设备状态。需要恢复数据时先选择恢复时点并保留现状，按 OPERATIONS/RESTORE 执行。

服务器在 `/opt/recipebox/releases/` 保留程序、previous、metadata、result，`current-commit` 记录实际运行来源。`.local/releases/<commit>/last-deployment.json` 记录本机发布结果。上传失败通常尚未停服务；已切换后失败先看 result 和 journal。成功版本与真实备份不自动删除。
