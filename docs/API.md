# HTTP API

外部 `/recipebox/`，Nginx 去前缀；以下为内部路径。JSON camelCase。所有 POST 使用 32 位小写十六进制 Idempotency-Key，同键必须保持请求路径与 body 字节一致；修改已有记录必须带当前 revision。

| 方法与路径 | 用途 |
| --- | --- |
| GET /healthz | 数据库连接健康 |
| GET /api/records | 当前记录数组；trash=1 取回收站 |
| GET /api/records/:id | 单个记录，包含 deletedAt |
| POST /api/records | 创建，kind=recipe/pantry、name 必填 |
| POST /api/records/:id | 默认 save，全量资料（服务端保留下厨日记） |
| POST /api/records/:id action=favorite | 更新 favorite 布尔值 |
| POST /api/records/:id action=consume | 食品数量减一 |
| POST /api/records/:id action=log | log={id?,date,note,rating}；无 id 新增，有 id 修改 |
| POST /api/records/:id action=remove_log | log.id 指定删除的日记 |
| POST /api/records/:id action=delete/restore | 回收站动作 |
| GET /api/operations/:key | 取得已提交的原结果，未知返回 404 |
| POST /api/uploads | 原始文件 body，返回 media 元信息 |
| GET /api/media/:id/main 或 thumb | 标准 JPEG 或 WebP，核对归属/过期/删除 |
| GET /api/export | ZIP，recipes-and-pantry.json 与 photos/:id.jpg |

记录包含 id/kind/revision/name/category/notes/tags/ingredients/steps/minutes/servings/favorite/logs/photoIds/quantity/unit/location/purchaseDate/expiryDate/createdAt/updatedAt/deletedAt。标准定义见 internal/app/model.go 与 web/src/types.ts。

错误 JSON={code,message}：400 格式/提交号；403 来源；404 缺失；409 版本冲突、复用键或无库存；410 回收期过期；413 超量；415 格式；422 内容验证/图片解码；429 繁忙/限速；507 容量。每 IP 每分钟写操作 120，上传单独 30（仅接受 loopback 代理传来的 X-Real-IP）。未返回响应不表示未提交，必须查询原 key 或原字节重试。操作结果最多保留 7 天，不应自动重试更早的未知提交。
