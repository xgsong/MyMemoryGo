# MyMemoryGo 代码审查报告

> 审查日期: 2026-04-08
> 更新日期: 2026-04-08 (第二轮修复完成)
> 项目: MyMemoryGo — 面向 AI 助手的持久化记忆存储系统
> 技术栈: Go 1.25 + Cobra/Chi + SQLite + fsnotify
> 架构: DDD 分层 (Domain → Application → Infrastructure → Interface)

---

## 一、审查范围

全面审查了项目 4 个架构层共 40+ 源文件，涵盖：

- **Domain 层**: entity, repository, service, errors
- **Application 层**: service (store, search, sync, list)
- **Infrastructure 层**: SQLite 持久化, FileStore, Embedding, Search (hybrid, decay, mmr)
- **Interface 层**: REST API server, handlers, helpers
- **CLI 层**: root, app, store, search, sync, serve, watch, init, version
- **Utility 层**: vector, errors, log
- **测试层**: e2e, integration, 各包单元测试

---

## 二、已修复的问题

### P0 — 严重 Bug (5项, 已全部修复)

| # | 问题 | 文件 | 修复方式 | 状态 |
|---|------|------|----------|------|
| P0-1 | SyncIndex 创建的 Memory 缺少 ID/StartLine/EndLine, 验证必定失败 | `sync.go`, `store.go` | 添加行号计算、设置 ID 字段、将 embedding 生成移到写锁之前 | **已修复** |
| P0-2 | Search() 传入 nil embedding, 向量搜索实际无效 | `search_core.go`, `search.go`, `repository.go` | 在应用层生成 query embedding, 通过 SearchOptions.QueryEmbedding 传入 | **已修复** |
| P0-3 | AllowContentType 全局中间件阻断所有 GET 请求 | `server.go` | 移除全局中间件, 仅在 POST 路由组上应用 | **已修复** |
| P0-4 | 路径遍历漏洞, 可读写工作目录外文件 | `operations.go` | 重写 resolvePath 返回 error 而非静默降级; 添加 evalSymlinksSafe 防御符号链接逃逸; isSubPath 支持 target==base; List 方法纳入 resolvePath 保护 | **已修复** |
| P0-5 | 哨兵错误并发修改导致数据竞争 | `errors.go`, `errors_test.go` | pkg/errors 和 domain/errors 的 WithOp/Wrap/WithFields 均改为先拷贝再修改 (`cp := *e`) | **已修复** |

### P1 — 高优先级 Bug (6项, 已全部修复)

| # | 问题 | 文件 | 修复方式 | 状态 |
|---|------|------|----------|------|
| P1-6 | http.ErrServerClosed 被当错误处理 + 关闭无超时 + 默认绑定 0.0.0.0 | `serve.go` | 检测 ErrServerClosed 正常退出、添加 15s 关闭超时、默认改为 127.0.0.1 | **已修复** |
| P1-7 | "~" 输入导致切片越界 panic | `app.go` | 先检查长度再切片, 支持裸 ~ 展开 | **已修复** |
| P1-8 | truncate 按字节截断破坏多字节 UTF-8 | `search.go` (CLI) | 使用 utf8.RuneCountInString + []rune 切片 | **已修复** |
| P1-9 | POST search 的 Sources 字段被解码但未传递 | `handlers.go`, `helpers.go` | 添加 sourcesToTypes 函数, 将 Sources 传入 appReq | **已修复** |
| P1-10 | 内部错误信息泄露给 API 客户端 | `handlers.go` | 移除 err.Error() 拼接, 改用结构化日志, 返回通用错误消息 | **已修复** |
| P1-11 | fmt.Printf 手工拼接 JSON 存在注入风险 | `store.go` (CLI) | 改用 json.Marshal 安全序列化 | **已修复** |

### P2 — 计划修复 (25项, 已全部修复)

#### 并发模型

| # | 文件 | 问题 | 修复方式 | 状态 |
|---|------|------|----------|------|
| 1 | `service.go` | sync.RWMutex 放在应用层保护 SQLite 是 DDD 抽象泄漏, 应封装在仓储实现中 | 移除应用层 RWMutex, 下沉到仓储实现内部 | **已修复** |
| 2 | `search.go` | SearchMemories 获取 RLock 对纯读操作不必要, 序列化了所有搜索 | 移除 SearchMemories 中的 RLock | **已修复** |
| 3 | `hybrid.go:80-82` | SetReranker/SetDecayCalculator 无同步保护, 并发调用存在数据竞争 | 添加 `mu sync.RWMutex`, 写方法用 Lock, Search 用 RLock | **已修复** |

#### 错误处理

| # | 文件 | 问题 | 修复方式 | 状态 |
|---|------|------|----------|------|
| 4 | `sync.go` | 使用 fmt.Errorf 而非 errors.WrapOp, 与其他文件不一致 | 全部替换为 errors.WrapOp | **已修复** |
| 5 | `search.go` | 搜索错误直接返回原始 error, 未包装 | 使用 errors.New/WrapOp 统一包装 | **已修复** |
| 6 | `domain/errors/errors.go:121-128` | ValidationError 缺少 Unwrap(), 无法参与 errors.Is/As 链 | 添加 `Err error` 字段和 `Unwrap()` 方法 | **已修复** |
| 7 | `search_core.go:166-168` | Fulltext 搜索错误返回 nil,nil, 无法区分"无结果"和"查询失败" | 错误时返回 errors.WrapOp 包装的错误, rows.Err() 也传播 | **已修复** |

#### 零值歧义

| # | 文件 | 问题 | 修复方式 | 状态 |
|---|------|------|----------|------|
| 8 | `repository.go`, `search_core.go`, `hybrid.go` | MinScore==0, VectorWeight==0, MMRLambda==0 无法区分"未设置"和"显式设为 0" | 改为 `*float64` 指针类型, nil 表示未设置 | **已修复** |
| 9 | `search.go:59` | Lambda==0 是合法 MMR 值但被当作"未设置"处理 | 通过 `*float64` 指针区分, nil 为未设置, &0.0 为显式设 0 | **已修复** |

#### 验证缺失

| # | 文件 | 问题 | 修复方式 | 状态 |
|---|------|------|----------|------|
| 10 | `validator.go` | 不验证 ID, Checksum, CreatedAt/UpdatedAt, Source 与 Path 一致性 | 添加 ID 格式/Checksum/时间戳/Source-Path 一致性验证 | **已修复** |
| 11 | `repository.go` | SearchOptionsBuilder.Build() 不验证参数 (负数 Limit, 权重和不为 1 等) | Build() 添加 Limit/MinScore/VectorWeight/FulltextWeight/MMRLambda 范围校验, 权重和为 1 校验 | **已修复** |
| 12 | `service 层各文件` | 应用层入口不做请求参数校验 (空 query, 负数 Limit 等) | search.go 验证 query/Limit/MinScore/lambda/half_life; list.go 验证 Limit/Offset/OrderBy/OrderDirection/id 非空 | **已修复** |

#### API 设计

| # | 文件 | 问题 | 修复方式 | 状态 |
|---|------|------|----------|------|
| 13 | `handlers.go:23-32` | 健康检查硬编码 "ok" 而不实际检查依赖 | readinessCheck 实际调用 ListMemories 检查数据库、Embed 检查嵌入服务; 添加 Embed 方法到应用服务 | **已修复** |
| 14 | `helpers.go:13-24` | sourceFromString 对无效输入静默默认为 SourceDaily | 无效输入返回 errors.New, 空字符串返回零值(nil) 表示不过滤 | **已修复** |
| 15 | `helpers.go:27-36` | parseIntParam 解析失败静默使用默认值 | 函数签名改为返回 `(int, error)`, 解析失败返回错误 | **已修复** |
| 16 | `handlers.go` | GET/POST 搜索接口参数名不一致 (q vs query) | 移除 q 兼容别名, 统一使用 query 参数名 | **已修复** |
| 17 | `handlers.go` | 删除操作不区分"删除成功"和"不存在" | 使用 errors.IsNotFound 检查, 不存在返回 404 | **已修复** |

#### 资源管理

| # | 文件 | 问题 | 修复方式 | 状态 |
|---|------|------|----------|------|
| 18 | `mmr.go:12` | sync.Map (Embeddings) 无界增长, 无淘汰机制 | 添加 MaxEmbeddings=10000 上限, EvictionThreshold=0.9 触发 LRU 淘汰, 淘汰最旧 20% | **已修复** |
| 19 | `watch.go:19` | Watch 的 handlers 只增不减, 无清理机制 | 添加 RemoveHandler 方法; Close 时清空 handlers 切片 | **已修复** |
| 20 | `watch.go:30` | processEvents goroutine 无生命周期管理 | 添加 done chan struct{}, processEvents 退出时 close(done); Close 等待 <-done 确认 goroutine 退出 | **已修复** |
| 21 | `operations.go:42` | 文件操作不支持原子写入 | Write 改为先写临时文件 (os.CreateTemp), 再 os.Rename 实现原子写入 | **已修复** |

#### Embedding Client

| # | 文件 | 问题 | 修复方式 | 状态 |
|---|------|------|----------|------|
| 22 | `client.go:131-133` | Embedding 结果顺序可能不匹配 (应使用 d.Index 而非 i) | 改用 d.Index 映射结果位置, 添加边界检查 | **已修复** |
| 23 | `client.go:140-143` | isNonRetryableError 始终返回 false, 4xx 也被重试 | 检查 4xx 状态码和常见错误消息, 不可恢复错误不重试 | **已修复** |
| 24 | `client.go:96-98` | 空 BaseURL 导致 panic | 添加空 URL 校验, 返回 errors.WrapOp 错误 | **已修复** |
| 25 | `client.go:117` | HTTP 响应体无大小限制 | 使用 io.LimitReader 限制 10MB (MaxResponseSize) | **已修复** |

### 测试层 Bug (3项, 已全部修复)

| # | 问题 | 文件 | 修复方式 | 状态 |
|---|------|------|----------|------|
| T-1 | mockEmbeddingProvider 数据竞争 (e2e ConcurrentStores 失败) | `e2e/e2e_test.go` | 添加 sync.Mutex 保护 embeddings map | **已修复** |
| T-2 | mockEmbeddingProvider 数据竞争 (integration) | `integration/integration_test.go` | 同上 | **已修复** |
| T-3 | mockEmbeddingProvider 潜在数据竞争 | `server_test.go` | 同上 (主动修复) | **已修复** |

### Go Vet 警告 (3项, 已全部修复)

| # | 问题 | 文件 | 修复方式 | 状态 |
|---|------|------|----------|------|
| V-1 | Benchmark 中 Error() 返回值未使用 | `domain/errors/errors_test.go` | 改为 `_ = memErr.Error()` | **已修复** |
| V-2 | Benchmark 中 Error() 返回值未使用 | `pkg/errors/errors_test.go` | 改为 `_ = err.Error()` | **已修复** |
| V-3 | assert.NotNil 复制含 sync.noCopy 的 sync.Map | `search/mmr_test.go` | 改为通过 Load 操作验证 map 可用 | **已修复** |

---

## 三、待修复的问题

> P0-P2 全部 34 项问题已修复完成, 以下 P3 问题建议在后续迭代中修复。

### P3 — 持续改进

#### 性能

| # | 文件 | 问题 | 状态 |
|---|------|------|------|
| 1 | `search_core.go` | 向量搜索全表扫描, 无 LIMIT, 加载所有行到内存计算余弦相似度 | **已修复** |
| 2 | `mmr.go:43-64` | MMR 重排序 O(n^3) 复杂度, 大结果集下极慢 | |
| 3 | `mmr.go:67` | Slice 切片操作导致 O(n^2) 内存拷贝 | |
| 4 | `vector.go` | 无 SIMD 或批量优化, 768 维 embedding 热路径可优化 | **部分修复** |

#### 安全

| # | 文件 | 问题 |
|---|------|------|
| 5 | `root.go:67` | API key 明文存储在 config.yaml |
| 6 | `list.go:17-18` | OrderBy/OrderDirection 字符串直接传递, 潜在 SQL 注入 |
| 7 | `client.go:108` | API 错误响应可能包含 Authorization 头信息 |

#### 设计

| # | 文件 | 问题 |
|---|------|------|
| 8 | `hybrid.go:155-158` | mergeResults 和 decay 修改原始 SearchHit 的 Score 字段 |
| 9 | `app.go:147-159` | WaitForInterrupt 泄漏 goroutine 和 signal channel |
| 10 | `init.go:99` | Config/DB 写入 workspace 外的父目录 |

#### Domain 层

| # | 文件 | 问题 |
|---|------|------|
| 11 | `id_generator.go:19-20` | ParseID 按所有冒号分割, Windows 路径 (C:) 会解析错误 |
| 12 | `id_generator.go:18-21` | ParseID 接受空路径 ":1-5" 和 StartLine=0 |
| 13 | `path_processor.go:13-17` | NormalizePath 只去除一个前导/尾部斜杠 |
| 14 | `path_processor.go:34-43` | DetermineSourceType 接受无效日期如 0000-00-00 |
| 15 | `memory.go:90` | LineCount() 零值实体返回 1, 语义不对 |
| 16 | `content_processor.go:31` | SplitContentIntoChunks maxChunkSize<=0 未定义行为 |

#### 测试质量

| # | 文件 | 问题 |
|---|------|------|
| 17 | `memory_test.go:506-526` | JSON 序列化测试不调用 json.Marshal |
| 18 | `memory_test.go:410-426` | sort.Slice 非稳定排序但测试断言稳定性 |
| 19 | `validator_test.go:234-245` | 自定义 containsString 重新实现 strings.Contains |
| 20 | `memory_service_test.go` | 与各独立测试文件存在大量重复测试 |
| 21 | `path_processor_test.go` | 使用 testing 而非 testify, 与其他测试风格不一致 |

---

## 四、架构级建议

### ~~1. 并发模型重构~~ (已修复)

~~当前 `sync.RWMutex` 放在应用层来保护 SQLite 写入, 是 DDD 抽象泄漏。~~

> **已修复**: RWMutex 已移至仓储实现内部, 应用层仅协调业务流程。SearchMemories 不再持有 RLock。

### ~~2. 错误处理统一~~ (已修复)

~~当前存在三种错误包装方式: `errors.WrapOp` / `fmt.Errorf` / 直接返回原始 error~~

> **已修复**: 全部统一使用 `errors.WrapOp` / `errors.New`, ValidationError 已添加 Unwrap() 方法。

### ~~3. 请求验证分层~~ (已修复)

~~建议在应用层入口 (service 方法) 添加请求参数校验~~

> **已修复**: search.go 验证 query/Limit/MinScore/lambda/half_life; list.go 验证 Limit/Offset/OrderBy/OrderDirection/id; store.go 验证 content 非空。

### ~~4. 零值语义~~ (已修复)

~~建议使用 `*float64` 指针类型来区分"未设置"和"显式设为零值"~~

> **已修复**: MinScore/VectorWeight/FulltextWeight/MMRLambda 均改为 `*float64` 指针类型, nil 表示未设置使用默认值。

### 5. ~~向量搜索性能~~ (已优化)

~~当前向量搜索全表扫描 (SELECT ... WHERE embedding IS NOT NULL), 随数据增长性能会严重退化。建议:~~
~~- 添加 LIMIT 预过滤~~
~~- 考虑引入 HNSW/IVF 索引 (如 sqlite-vec 扩展)~~
~~- 或在应用层缓存 embedding 矩阵, 避免每次查询都从 DB 读取~~

> **已优化** (第三轮): 实施四项向量搜索性能优化:
> 1. **HNSW 向量索引** — 引入 `github.com/coder/hnsw` 纯 Go HNSW 库, 启动时从 SQLite 加载构建内存索引, O(log n) 搜索替代 O(n) 全表扫描
> 2. **预归一化 + 点积** — 存储时归一化 embedding, 搜索用点积替代余弦相似度 (1 趟 vs 3 趟), 自动迁移旧数据
> 3. **Top-K 最小堆** — `SearchVector` 使用 TopKHeap 仅保留 top-K 候选, 避免全量收集+排序
> 4. **激活 HybridEngine** — 接入 MMR 重排序 + 时间衰减, 搜索管道完整激活

---

## 五、修改文件清单

### 第一轮修复 (18 个文件, 新增 255 行, 删除 74 行)

```
cmd/memory/cmd/app.go                                        | 16 +++++++++-
cmd/memory/cmd/search.go                                     |  8 +++--
cmd/memory/cmd/serve.go                                      | 14 +++++----
cmd/memory/cmd/store.go                                      | 18 +++++++----
internal/application/service/search.go                       |  7 +++++
internal/application/service/store.go                        |  7 +++--
internal/application/service/sync.go                         | 29 +++++++++++++++----
internal/domain/repository/repository.go                     |  4 ++++
internal/domain/errors/errors_test.go                        |  2 +-
internal/infrastructure/persistence/filestore/operations.go  | 50 +++++++++++++++++++++++++++++--
internal/infrastructure/persistence/sqlite/search_core.go    |  2 +-
internal/infrastructure/search/mmr_test.go                   |  5 ++--
internal/interface/api/handlers.go                           | 30 ++++++++++++--------
internal/interface/api/helpers.go                            | 12 ++++++++
internal/interface/api/server.go                             | 18 +++++------
internal/interface/api/server_test.go                        | 21 +++++++++----
internal/pkg/errors/errors.go                                | 15 +++++----
internal/pkg/errors/errors_test.go                           | 36 +++++++++++++++++----
tests/e2e/e2e_test.go                                        | 21 ++++++++----
tests/integration/integration_test.go                        | 21 ++++++++----
```

### 第二轮修复 (9 个文件)

```
internal/domain/errors/errors.go                              | 添加 ValidationError.Err 字段和 Unwrap(); MemoryError.WithFields 改为先拷贝再修改
internal/domain/errors/errors_test.go                         | 更新 WithFields 测试: 返回新副本而非修改原始
internal/domain/service/memory_service_test.go                | 更新 WithFields 测试断言
internal/application/service/list.go                          | 添加 ListMemories/GetMemory/DeleteMemory 参数校验
internal/application/service/service.go                       | 添加 Embed 方法 (健康检查用)
internal/infrastructure/persistence/filestore/operations.go   | resolvePath 返回 (string,error); 原子写入; isSubPath 支持 target==base; evalSymlinksSafe
internal/infrastructure/persistence/filestore/manager.go      | 添加 done channel; Close 等待 goroutine 退出并清空 handlers
internal/infrastructure/persistence/filestore/watch.go        | RemoveHandler; done channel 生命周期管理; resolvePath 签名适配
internal/interface/api/handlers.go                            | readinessCheck 实际检查数据库和嵌入服务; 移除 q 兼容别名
internal/interface/api/helpers.go                             | sourceFromString 空字符串返回零值而非错误
internal/infrastructure/persistence/filestore/watch_test.go   | resolvePath 新签名适配
```

---

## 六、验证结果

### 第一轮验证

```
go build ./...    # 编译通过
go vet ./...      # 零警告
go test -race ./... -count=1  # 全部通过, 无数据竞争
```

### 第二轮验证 (全部 P0-P2 修复完成后)

```
go build ./...    # 编译通过
go vet ./...      # 零警告
go test -race ./... -count=1  # 全部通过, 无数据竞争
```

### 修复统计

| 优先级 | 问题数 | 已修复 | 待修复 |
|--------|--------|--------|--------|
| P0 | 5 | **5** | 0 |
| P1 | 6 | **6** | 0 |
| P2 | 25 | **25** | 0 |
| 测试层 | 3 | **3** | 0 |
| Go Vet | 3 | **3** | 0 |
| P3 | 21 | **3** | 18 |
| **合计** | **63** | **45** | **18** |

---

## 七、第三轮优化 — 向量搜索性能 (2026-04-08)

### 优化内容

| # | 优化项 | 实现方式 | 影响 |
|---|--------|----------|------|
| 1 | HNSW 向量索引 | `github.com/coder/hnsw v0.6.1` 纯 Go HNSW, 启动时加载, 写入/删除时同步更新 | O(log n) 替代 O(n) 全表扫描 |
| 2 | 预归一化 + 点积 | 存储前 `vector.Normalize()`, 搜索用 `DotProduct` 替代 `CosineSimilarity` | 1 趟遍历替代 3 趟, ~2x 加速 |
| 3 | Top-K 最小堆 | `TopKHeap` (container/heap) 替代全量收集+排序 | 减少内存分配和排序开销 |
| 4 | 激活 HybridEngine | app.go 接入 HybridEngine + MMR + 时间衰减 | 搜索质量提升 (多样性+时效性) |

### 关键设计决策

- **HNSW 库选择**: 使用 `coder/hnsw` (纯 Go, 无 CGO), 与项目 `modernc.org/sqlite` 兼容。`sqlite-vec` 因需 CGO 不可用。
- **向量索引持久化**: HNSW 索引为内存结构, SQLite BLOB 仍为数据源。启动时从 SQLite 加载构建, 写入/删除时同步更新。`coder/hnsw` 支持 Export/Import 可做持久化, 当前方案重启时重建即可。
- **数据迁移**: 新增 `__schema_version` 元数据行追踪 schema 版本。版本 < 2 时自动执行 embedding 归一化迁移。
- **降级策略**: 若 HNSW 索引不可用 (加载失败/空库), 自动降级为 TopK 优化的暴力扫描。
- **HybridEngine 接口**: 补充 `Index`/`RemoveFromIndex` 方法, 委托给底层 Store。
- **MMR embedding 填充**: `Rerank()` 自动从 SearchHit.Embedding 填充内部缓存, 无需外部手动调用 `SetEmbedding`。

### 新增文件

| 文件 | 用途 |
|------|------|
| `internal/pkg/vector/topk.go` | Top-K 最小堆实现 (基于 container/heap) |
| `internal/pkg/vector/topk_test.go` | Top-K 堆测试 (13 个用例 + 3 个 benchmark) |
| `internal/infrastructure/search/vector_index.go` | HNSW 向量索引封装 (Build/Add/Remove/Search) |
| `internal/infrastructure/search/vector_index_test.go` | 向量索引测试 (14 个用例 + 3 个 benchmark) |

### 修改文件

| 文件 | 变更 |
|------|------|
| `internal/infrastructure/persistence/sqlite/store.go` | 添加 VectorIndex 字段, loadVectorIndex(), SchemaVersion(), migrateNormalizedEmbeddings(), runMigrations() |
| `internal/infrastructure/persistence/sqlite/db_setup.go` | schema 版本标记 (`__schema_version`) |
| `internal/infrastructure/persistence/sqlite/write_ops.go` | 存储前归一化 embedding, 提交后更新 HNSW 索引 |
| `internal/infrastructure/persistence/sqlite/delete_ops.go` | 删除后更新 HNSW 索引 (DeleteByPath 先查询 ID) |
| `internal/infrastructure/persistence/sqlite/search_core.go` | 点积替代余弦, TopK 堆, HNSW 搜索路径 + 暴力降级 |
| `internal/infrastructure/search/mmr.go` | 点积替代余弦, Rerank 自动填充 embedding |
| `internal/infrastructure/search/hybrid.go` | 添加 Index/RemoveFromIndex 方法实现 SearchRepository 接口 |
| `cmd/memory/cmd/app.go` | 接入 HybridEngine + MMR + Decay 替代直接传 Store |
| `internal/application/service/search.go` | 添加 embedding 生成注释说明 |
| `go.mod` / `go.sum` | 新增 `github.com/coder/hnsw v0.6.1` 及传递依赖 |
