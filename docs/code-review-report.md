# MyMemoryGo 代码审查报告

> 审查日期: 2026-04-08
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

| # | 问题 | 文件 | 修复方式 | Commit |
|---|------|------|----------|--------|
| P0-1 | SyncIndex 创建的 Memory 缺少 ID/StartLine/EndLine, 验证必定失败 | `sync.go`, `store.go` | 添加行号计算、设置 ID 字段、将 embedding 生成移到写锁之前 | ✅ |
| P0-2 | Search() 传入 nil embedding, 向量搜索实际无效 | `search_core.go`, `search.go`, `repository.go` | 在应用层生成 query embedding, 通过 SearchOptions.QueryEmbedding 传入 | ✅ |
| P0-3 | AllowContentType 全局中间件阻断所有 GET 请求 | `server.go` | 移除全局中间件, 仅在 POST 路由组上应用 | ✅ |
| P0-4 | 路径遍历漏洞, 可读写工作目录外文件 | `operations.go` | 重写 resolvePath, 添加 isSubPath 边界检查 | ✅ |
| P0-5 | 哨兵错误并发修改导致数据竞争 | `errors.go`, `errors_test.go` | WithOp/Wrap/WithFields 改为先拷贝再修改 | ✅ |

### P1 — 高优先级 Bug (6项, 已全部修复)

| # | 问题 | 文件 | 修复方式 | Commit |
|---|------|------|----------|--------|
| P1-6 | http.ErrServerClosed 被当错误处理 + 关闭无超时 + 默认绑定 0.0.0.0 | `serve.go` | 检测 ErrServerClosed 正常退出、添加 15s 关闭超时、默认改为 127.0.0.1 | ✅ |
| P1-7 | "~" 输入导致切片越界 panic | `app.go` | 先检查长度再切片, 支持裸 ~ 展开 | ✅ |
| P1-8 | truncate 按字节截断破坏多字节 UTF-8 | `search.go` (CLI) | 使用 utf8.RuneCountInString + []rune 切片 | ✅ |
| P1-9 | POST search 的 Sources 字段被解码但未传递 | `handlers.go`, `helpers.go` | 添加 sourcesToTypes 函数, 将 Sources 传入 appReq | ✅ |
| P1-10 | 内部错误信息泄露给 API 客户端 | `handlers.go` | 移除 err.Error() 拼接, 改用结构化日志, 返回通用错误消息 | ✅ |
| P1-11 | fmt.Printf 手工拼接 JSON 存在注入风险 | `store.go` (CLI) | 改用 json.Marshal 安全序列化 | ✅ |

### 测试层 Bug (3项, 已全部修复)

| # | 问题 | 文件 | 修复方式 | Commit |
|---|------|------|----------|--------|
| T-1 | mockEmbeddingProvider 数据竞争 (e2e ConcurrentStores 失败) | `e2e/e2e_test.go` | 添加 sync.Mutex 保护 embeddings map | ✅ |
| T-2 | mockEmbeddingProvider 数据竞争 (integration) | `integration/integration_test.go` | 同上 | ✅ |
| T-3 | mockEmbeddingProvider 潜在数据竞争 | `server_test.go` | 同上 (主动修复) | ✅ |

### Go Vet 警告 (3项, 已全部修复)

| # | 问题 | 文件 | 修复方式 | Commit |
|---|------|------|----------|--------|
| V-1 | Benchmark 中 Error() 返回值未使用 | `domain/errors/errors_test.go` | 改为 `_ = memErr.Error()` | ✅ |
| V-2 | Benchmark 中 Error() 返回值未使用 | `pkg/errors/errors_test.go` | 改为 `_ = err.Error()` | ✅ |
| V-3 | assert.NotNil 复制含 sync.noCopy 的 sync.Map | `search/mmr_test.go` | 改为通过 Load 操作验证 map 可用 | ✅ |

---

## 三、待修复的问题

以下问题按优先级分类, 建议在后续迭代中修复。

### P2 — 计划修复

#### 并发模型

| # | 文件 | 问题 |
|---|------|------|
| 1 | `service.go` | sync.RWMutex 放在应用层保护 SQLite 是 DDD 抽象泄漏, 应封装在仓储实现中 |
| 2 | `search.go` | SearchMemories 获取 RLock 对纯读操作不必要, 序列化了所有搜索 |
| 3 | `hybrid.go:80-82` | SetReranker/SetDecayCalculator 无同步保护, 并发调用存在数据竞争 |

#### 错误处理

| # | 文件 | 问题 |
|---|------|------|
| 4 | `sync.go` | 使用 fmt.Errorf 而非 errors.WrapOp, 与其他文件不一致 |
| 5 | `search.go` | 搜索错误直接返回原始 error, 未包装 |
| 6 | `domain/errors/errors.go:121-128` | ValidationError 缺少 Unwrap(), 无法参与 errors.Is/As 链 |
| 7 | `search_core.go:166-168` | Fulltext 搜索错误返回 nil,nil, 无法区分"无结果"和"查询失败" |

#### 零值歧义

| # | 文件 | 问题 |
|---|------|------|
| 8 | `repository.go`, `search_core.go`, `hybrid.go` | MinScore==0, VectorWeight==0, MMRLambda==0 无法区分"未设置"和"显式设为 0" |
| 9 | `search.go:59` | Lambda==0 是合法 MMR 值但被当作"未设置"处理 |

#### 验证缺失

| # | 文件 | 问题 |
|---|------|------|
| 10 | `validator.go` | 不验证 ID, Checksum, CreatedAt/UpdatedAt, Source 与 Path 一致性 |
| 11 | `repository.go` | SearchOptionsBuilder.Build() 不验证参数 (负数 Limit, 权重和不为 1 等) |
| 12 | `service 层各文件` | 应用层入口不做请求参数校验 (空 query, 负数 Limit 等) |

#### API 设计

| # | 文件 | 问题 |
|---|------|------|
| 13 | `handlers.go:23-32` | 健康检查硬编码 "ok" 而不实际检查依赖 |
| 14 | `helpers.go:13-24` | sourceFromString 对无效输入静默默认为 SourceDaily |
| 15 | `helpers.go:27-36` | parseIntParam 解析失败静默使用默认值 |
| 16 | `handlers.go` | GET/POST 搜索接口参数名不一致 (q vs query) |
| 17 | `handlers.go` | 删除操作不区分"删除成功"和"不存在" |

#### 资源管理

| # | 文件 | 问题 |
|---|------|------|
| 18 | `mmr.go:12` | sync.Map (Embeddings) 无界增长, 无淘汰机制 |
| 19 | `watch.go:19` | Watch 的 handlers 只增不减, 无清理机制 |
| 20 | `watch.go:30` | processEvents goroutine 无生命周期管理 |
| 21 | `operations.go:42` | 文件操作不支持原子写入 |

#### Embedding Client

| # | 文件 | 问题 |
|---|------|------|
| 22 | `client.go:131-133` | Embedding 结果顺序可能不匹配 (应使用 d.Index 而非 i) |
| 23 | `client.go:140-143` | isNonRetryableError 始终返回 false, 4xx 也被重试 |
| 24 | `client.go:96-98` | 空 BaseURL 导致 panic |
| 25 | `client.go:117` | HTTP 响应体无大小限制 |

### P3 — 持续改进

#### 性能

| # | 文件 | 问题 |
|---|------|------|
| 1 | `search_core.go:88-93` | 向量搜索全表扫描, 无 LIMIT, 加载所有行到内存计算余弦相似度 |
| 2 | `mmr.go:43-64` | MMR 重排序 O(n^3) 复杂度, 大结果集下极慢 |
| 3 | `mmr.go:67` | Slice 切片操作导致 O(n^2) 内存拷贝 |
| 4 | `vector.go` | 无 SIMD 或批量优化, 768 维 embedding 热路径可优化 |

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

### 1. 并发模型重构

当前 `sync.RWMutex` 放在应用层来保护 SQLite 写入, 是 DDD 抽象泄漏。建议:

- 将写锁下沉到 SQLite Store 实现内部
- 应用层仅协调业务流程, 不关心底层并发策略
- SyncIndex 和 StoreMemory 统一"先生成 embedding 再加锁写入"的模式

### 2. 错误处理统一

当前存在三种错误包装方式:
- `errors.WrapOp` (store.go, list.go)
- `fmt.Errorf` (sync.go)
- 直接返回原始 error (search.go)

建议: 全部统一使用 `errors.WrapOp`, 并在应用层入口统一将 domain error 映射为适当的 error code。

### 3. 请求验证分层

建议在应用层入口 (service 方法) 添加请求参数校验:
- 空 query、负数 Limit/Offset、无效 Source 等应在应用层拦截
- Domain validator 仅验证实体完整性
- 避免无效请求传播到基础设施层

### 4. 零值语义

建议使用 `*float64` 指针类型或引入 `Option[T]` 泛型来区分"未设置"和"显式设为零值", 特别是对 MinScore, MMRLambda, DecayHalfLife 等参数。

### 5. 向量搜索性能

当前向量搜索全表扫描 (SELECT ... WHERE embedding IS NOT NULL), 随数据增长性能会严重退化。建议:
- 添加 LIMIT 预过滤
- 考虑引入 HNSW/IVF 索引 (如 sqlite-vec 扩展)
- 或在应用层缓存 embedding 矩阵, 避免每次查询都从 DB 读取

---

## 五、修改文件清单

本次审查共修改 18 个文件, 新增 255 行, 删除 74 行:

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

---

## 六、验证结果

```
go build ./...    # ✅ 编译通过
go vet ./...      # ✅ 零警告
go test -race ./... -count=1  # ✅ 全部通过, 无数据竞争
```
