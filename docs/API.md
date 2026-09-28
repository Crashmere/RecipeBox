# HTTP API

外部 `/recipebox/`，Nginx 去前缀；以下为 Go 内部路径，`{id}`、`{key}` 为路径参数。公网 API 与媒体由 ServerPortal 统一设备认证保护，未授权返回 401；本机回环调用保留。JSON camelCase。所有 POST 使用 32 位小写十六进制 Idempotency-Key；记录写入同键必须保持 HTTP 方法、内部路径与 body 字节一致，上传同键须保持原文件字节一致。修改已有记录必须带当前 revision。

| 方法与路径 | 用途 |
| --- | --- |
| GET /healthz | 数据库连接健康 |
| GET /api/records | 全部未删除记录数组，不分页；查询参数 trash=1 取回收站，按 updatedAt、id 倒序 |
| GET /api/records/{id} | 单个记录，包含 deletedAt，也可读取尚未清理的回收站记录 |
| POST /api/records | 创建，kind=recipe/pantry、name 必填，action 省略或为 save；返回 200 和 Record |
| POST /api/records/{id} | body 包含当前 revision；action 为 save/favorite/consume/log/remove_log/delete/restore，成功返回 200 和完整 Record |
| GET /api/operations/{key} | 取得原记录写入或上传结果，未知返回 404，不代表资源当前最新状态 |
| POST /api/uploads | 原始图片文件 body（非 multipart），最大 25 MiB；返回 201 和 Media |
| GET /api/media/{id}/main | 标准 JPEG，仅有效暂存或未删除记录仍引用的照片 |
| GET /api/media/{id}/thumb | WebP 缩略图，使用与 main 相同的归属和有效期检查 |
| GET /api/export | 全部未删除记录的 ZIP，含 recipes-and-pantry.json 与 photos/{id}.jpg，不含回收站 |

本表覆盖当前业务、健康和媒体接口；门户的 `deploy/portal.json.apis` 与本表保持一致。每个方法和路径只登记一次；下列 action 是同一个记录写入接口的 JSON 字段，不是独立路径或查询参数。 核对与同步流程见[共享排障说明](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#门户-api-列表与服务不一致)。

## 记录写入动作

`POST /api/records/{id}` 的 JSON body 始终包含当前 revision，例如 `{"revision":3,"action":"favorite","favorite":true}`。

| action | 输入与行为 |
| --- | --- |
| save（默认） | 全量资料；保留原 kind、创建时间与下厨日记，省略可编辑字段会使用其零值，不是局部更新 |
| favorite | favorite 布尔值，仅菜谱可收藏 |
| consume | 常备食品 quantity 减一，无库存或类型不符返回 409 |
| log | log={id?,date,note,rating}；仅菜谱，无 id 新增，有 id 修改已有日记 |
| remove_log | log.id 指定删除的日记 |
| delete | 软删除到回收站 |
| restore | 恢复 30 天内删除记录；未删除返回 409，过期返回 410 |

所有动作成功后 revision 增加并返回完整 Record；首次创建只能使用 save，已有记录不能更换 kind。JSON 表单上限 256 KiB，拒绝未知字段和尾随 JSON。上传支持 JPEG、PNG、WebP、HEIC/HEIF；Media 为 {id,width,height,bytes,createdAt}。暂存照片须在 24 小时内绑定到记录；同一照片只能属于一个记录。main/thumb 之外的变体、已移除照片、已删除记录的照片和过期暂存照片都返回 404。

记录包含 id/kind/revision/name/category/notes/tags/ingredients/steps/minutes/servings/favorite/logs/photoIds/quantity/unit/location/purchaseDate/expiryDate/createdAt/updatedAt/deletedAt。标准定义见 internal/app/model.go 与 web/src/types.ts。

应用错误 JSON={code,message}：400 格式/提交号；403 来源；404 缺失；409 版本冲突、复用键或无库存；410 恢复期限已过；413 超量；415 格式；422 内容验证/图片解码；429 繁忙/限速；500 内部错误；507 容量。公网认证失败由共享入口返回 401。每 IP 每分钟写操作 120，上传单独 30（仅接受 loopback 代理传来的 X-Real-IP）。未返回响应不表示未提交，必须查询原 key 或原字节重试；网页在写入 3 秒无回复后自动查询原 key。操作结果由维护清理保留约 7 天，不应自动重试更早的未知提交。
