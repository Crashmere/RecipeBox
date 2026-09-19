# 自动检查与发布

main / PR 运行 Build and verify：锁定依赖构建、Go race/vet、前端单测、shell 语法、Linux 发布失败回退测试、Chromium/WebKit 业务交互、Linux amd64 产物。所有 GitHub Actions 固定提交版本。main 推送不会自动部署。

Deploy RecipeBox 是手动 workflow_dispatch，只允许 main，发送前核对 main 最新提交。production 环境保存 SSH_HOST、SSH_USER、SSH_PRIVATE_KEY、SSH_KNOWN_HOSTS。独立 Ed25519 发布密钥，严格主机校验；不使用管理员私钥。

`deploy/setup-ci.sh <public-key>` 建 recipebox-deploy，authorized_keys 限制为固定 `deploy <commit-sha> <sha256>` 命令、禁转发/交互 shell。sudo 仅允许 root 所有的固定发布脚本。上传候选仅以 recipebox 身份执行。

发布校验长度/SHA-256，保留旧程序，停 RecipeBox，备份数据库与照片，以运行身份执行候选 check，再替换启动。失败自动恢复旧程序并验证健康，不回滚数据库，不修改其他服务。所有版本维持兼容 schema v1；未来迁移必须明确设计备份与回退。

部署材料、配置和文档由管理员从已推送提交同步。普通发布只更新二进制/current-commit，不能改 unit、Nginx、部署脚本、文档。发布历史与发布前备份暂人工保留。

`deploy/test-release.sh` 在 Linux 隔离临时目录使用模拟 systemctl/curl/runuser 验证校验和失败、候选 check 失败、启动健康失败恢复、成功提交；不在生产实例制造故障。
