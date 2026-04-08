# MyMemoryGo ContextEngine 适配方案

> 文档版本：v2.0.0
> 日期：2026-04-09
> 状态：草稿
> 替代：v1.0.0（2026-04-08）

## 0. v1 → v2 变更摘要

基于对 OpenClaw ContextEngine 实际接口（`src/context-engine/types.ts`）的校验，v1 存在以下关键问题，v2 予以修正：

| # | v1 问题 | v2 修正 | 严重程度 |
|---|--------|--------|---------|
| 1 | 缺失 `afterTurn` 生命周期方法 | 完整实现所有生命周期方法 | 高 |
| 2 | 缺失 `maintain` 实现（proto 有定义但应用层未实现） | 实现并返回 `MaintenanceResult` | 高 |
| 3 | 缺失 `onSubagentEnded` / `prepareSubagentSpawn` | 实现子代理生命周期钩子 | 中 |
| 4 | `ContextEngineService` 直接依赖 `*sqlite.XxxRepository` 具体类型 | 改为依赖领域接口，遵循 DDD 依赖倒置 | 高 |
| 5 | 给 `Memory` 实体加 `MemoryType` 字段统一知识/对话 | 保持 `Memory` 不变，新增独立 `ConversationMessage` 实体和表 | 高 |
| 6 | Proto `AgentMessage` 缺少 `is_error`/`api`/`provider`/`model`/`stop_reason` 字段，`CompactResult.result` 结构与 OpenClaw 不一致 | 完全对齐 OpenClaw TypeScript 类型 | 中 |
| 7 | TS 插件 `assemble` 方法重复定义 | 修正，所有方法统一使用降级模式 | 低 |

---

## 1. 背景与目标

### 1.1 集成目标

| 目标 | 描述 |
|------|------|
| **完整实现 ContextEngine 接口** | 实现 OpenClaw `ContextEngine` 接口的所有必需方法 + 所有可选方法 |
| **处理 OpenClaw 会话消息** | 直接接收和处理 OpenClaw 的 `AgentMessage`（含 UserMessage / AssistantMessage / ToolResultMessage），独立存储 |
| **三层混合上下文组装** | Assemble 阶段合并历史摘要 + 远期对话检索 + 知识记忆交叉检索 |
| **深度复用现有能力** | 共享 `EmbeddingRepository`、`HybridEngine`（知识检索）、SQLite 连接 |
| **独立服务部署** | MyMemoryGo 作为独立 gRPC 服务运行，OpenClaw 通过插件桥接调用 |

### 1.2 OpenClaw ContextEngine 生命周期

ContextEngine 是 OpenClaw 中控制模型上下文构建的可插拔模块，参与以下生命周期节点：

| 方法 | 必需/可选 | 调用时机 |
|------|---------|---------|
| `ingest` | 必需 | 新消息添加到会话时调用 |
| `assemble` | 必需 | 每次模型运行前调用，返回在 token 预算内的有序消息 |
| `compact` | 必需 | 上下文窗口满或用户执行 `/compact` 时调用 |
| `bootstrap` | 可选 | 会话首次出现时调用（可导入历史） |
| `maintain` | 可选 | bootstrap、成功 turn 或 compact 后调用，执行 transcript 维护 |
| `ingestBatch` | 可选 | 一次 turn 完成后批量导入消息 |
| `afterTurn` | 可选 | 运行完成后调用（持久化状态、触发后台压缩） |
| `prepareSubagentSpawn` | 可选 | 子代理启动前准备共享状态 |
| `onSubagentEnded` | 可选 | 子代理结束后清理 |
| `dispose` | 可选 | 网关关闭或插件重载时释放资源 |

详见：[OpenClaw ContextEngine 文档](file:///home/xgsong/Projects/openclaw/docs/concepts/context-engine.md)

---

## 2. 整体架构

### 2.1 架构图

```
┌──────────────────────────────────────────────────────────────────────┐
│                            OpenClaw                                   │
│  ┌──────────────────────────────────────────────────────────────────┐│
│  │              ContextEngine Plugin (JS/TS)                        ││
│  │  ┌────────┐ ┌──────────┐ ┌────────┐ ┌───────────┐ ┌─────────┐ ││
│  │  │ ingest │ │ assemble │ │compact │ │ afterTurn │ │ maintain│ ││
│  │  └───┬────┘ └────┬─────┘ └───┬────┘ └─────┬─────┘ └────┬────┘ ││
│  └──────┼───────────┼───────────┼────────────┼──────────────┼──────┘│
└─────────┼───────────┼───────────┼────────────┼──────────────┼───────┘
          │           │           │            │              │
          │ gRPC      │ gRPC      │ gRPC       │ gRPC         │ gRPC
          ▼           ▼           ▼            ▼              ▼
┌──────────────────────────────────────────────────────────────────────┐
│                     MyMemoryGo (Go Service)                           │
│  ┌──────────────────────────────────────────────────────────────────┐│
│  │                    ContextEngine gRPC Server                      ││
│  │  ┌──────────┐ ┌───────────┐ ┌──────────┐ ┌───────────┐         ││
│  │  │IngestMsg │ │AssembleCtx│ │CompactCtx│ │AfterTurn  │ ...     ││
│  │  └────┬─────┘ └─────┬─────┘ └────┬─────┘ └─────┬─────┘         ││
│  └───────┼─────────────┼────────────┼──────────────┼───────────────┘│
│  ┌───────┼─────────────┼────────────┼──────────────┼───────────────┐│
│  │       ▼             ▼            ▼              ▼               ││
│  │  ┌──────────────────────────────────────────────────────────┐    ││
│  │  │            ContextEngine Application Service              │    ││
│  │  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐ │    ││
│  │  │  │ Conversation │ │  Summary     │ │ Knowledge Memory │ │    ││
│  │  │  │   Message    │ │  Cache       │ │  (cross-search)  │ │    ││
│  │  │  └──────────────┘ └──────────────┘ └──────────────────┘ │    ││
│  │  └──────────────────────────────────────────────────────────┘    ││
│  └──────────────────────────────────────────────────────────────────┘│
│  ┌──────────────────────────────────────────────────────────────────┐│
│  │                    Shared Infrastructure                          ││
│  │  ┌──────────────┐ ┌───────────────────┐ ┌────────────────────┐ ││
│  │  │  Embedding   │ │  SQLite Store      │ │  HybridEngine      │ ││
│  │  │  Provider    │ │  - memories (现有)  │ │  (知识记忆检索)     │ ││
│  │  │  (共享缓存)   │ │  - conversation_*  │ │                    │ ││
│  │  │              │ │  - message_emb     │ │  MessageVectorIdx  │ ││
│  │  │              │ │                    │ │  (对话向量检索)     │ ││
│  │  └──────────────┘ └───────────────────┘ └────────────────────┘ ││
│  └──────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流

1. **Ingest 流程**：
   ```
   OpenClaw → ingest(sessionId, message) → gRPC → MyMemoryGo
   → 哈希去重 → 存储 ConversationMessage → 生成 Embedding
   → 写入 message_embeddings + conversation_messages_fts + MessageVectorIndex
   ```

2. **Assemble 流程**（三层检索）：
   ```
   OpenClaw → assemble(sessionId, messages, tokenBudget, prompt) → gRPC → MyMemoryGo
   → 加载历史摘要（conversation_summaries）
   → 远期对话检索（MessageVectorIndex + sessionID 过滤）
   → 知识记忆交叉检索（复用 HybridEngine）
   → 近期消息顺序保留（70% budget）
   → 合并：摘要 → 远期对话 → 知识记忆 → 近期消息
   → ensureConversationIntegrity → truncateToTokenBudget → 返回 AgentMessage[]
   ```

3. **Compact 流程**：
   ```
   OpenClaw → compact(sessionId, tokenBudget, force) → gRPC → MyMemoryGo
   → 获取远期未压缩消息 → LLM 生成摘要（重试 + 降级）
   → 保存 ConversationSummary → 标记已压缩 → 清理旧向量
   ```

4. **AfterTurn 流程**：
   ```
   OpenClaw → afterTurn(sessionId, messages, ...) → gRPC → MyMemoryGo
   → 刷新 MessageVectorIndex → 检查 token 阈值 → 可选主动压缩
   ```

5. **Maintain 流程**：
   ```
   OpenClaw → maintain(sessionId, sessionFile) → gRPC → MyMemoryGo
   → 清理已压缩消息的冗余数据 → 返回 MaintenanceResult
   ```

---

## 3. gRPC 接口定义

### 3.1 Proto 文件

新建文件：`proto/context_engine.proto`

> 对齐 OpenClaw `src/context-engine/types.ts` 中的完整类型定义。

```protobuf
syntax = "proto3";

package contextengine;

option go_package = "github.com/xgsong/MyMemoryGo/proto/gen";

// ---- 消息类型（对齐 OpenClaw AgentMessage 联合类型）----

// AgentMessage 对应 OpenClaw 的 UserMessage | AssistantMessage | ToolResultMessage
message AgentMessage {
  string role = 1;           // "user" | "assistant" | "system" | "tool" | "toolResult"
  string content = 2;
  int64 timestamp = 3;
  string id = 4;

  // assistant 专有字段
  string api = 5;            // "openai" | "anthropic" 等
  string provider = 6;
  string model = 7;
  string stop_reason = 8;

  // toolResult 专有字段
  string tool_call_id = 9;
  string tool_name = 10;
  bool is_error = 11;

  // 向后兼容（deprecated，保留字段号）
  ToolCall tool_call = 12;
  string tool_use_id = 13;
  string tool_args = 14;
}

message ToolCall {
  string id = 1;
  string name = 2;
  string arguments = 3;      // JSON string
}

// ---- 返回结果 ----

// AssembleResult 对齐 OpenClaw AssembleResult
message AssembleResult {
  repeated AgentMessage messages = 1;
  int32 estimated_tokens = 2;          // required
  string system_prompt_addition = 3;    // optional
}

// CompactResult 对齐 OpenClaw CompactResult
message CompactResult {
  bool ok = 1;
  bool compacted = 2;
  string reason = 3;
  CompactDetails result = 4;           // optional
}

// CompactDetails 对齐 OpenClaw CompactResult.result
message CompactDetails {
  string summary = 1;                  // optional
  string first_kept_entry_id = 2;      // optional
  int32 tokens_before = 3;             // required
  int32 tokens_after = 4;              // optional
}

message IngestResult {
  bool ingested = 1;
}

message IngestBatchResult {
  int32 ingested_count = 1;
}

// BootstrapResult 对齐 OpenClaw BootstrapResult
message BootstrapResult {
  bool bootstrapped = 1;
  int32 imported_messages = 2;         // optional
  string reason = 3;                   // optional
}

// MaintenanceResult 对齐 OpenClaw ContextEngineMaintenanceResult
message MaintenanceResult {
  bool changed = 1;
  int64 bytes_freed = 2;
  int32 rewritten_entries = 3;
  string reason = 4;                   // optional
}

// EngineInfo 对齐 OpenClaw ContextEngineInfo
message EngineInfo {
  string id = 1;
  string name = 2;
  string version = 3;                  // optional
  bool owns_compaction = 4;            // optional
}

message Empty {}

// ---- 请求消息 ----

message BootstrapRequest {
  string session_id = 1;
  string session_key = 2;
  string session_file = 3;
}

message MaintainRequest {
  string session_id = 1;
  string session_key = 2;
  string session_file = 3;
}

message IngestRequest {
  string session_id = 1;
  string session_key = 2;
  AgentMessage message = 3;
  bool is_heartbeat = 4;
}

message IngestBatchRequest {
  string session_id = 1;
  string session_key = 2;
  repeated AgentMessage messages = 3;
  bool is_heartbeat = 4;
}

message AssembleRequest {
  string session_id = 1;
  string session_key = 2;
  repeated AgentMessage messages = 3;
  int32 token_budget = 4;
  repeated string available_tools = 5;
  string citations_mode = 6;
  string model = 7;
  string prompt = 8;
}

message CompactRequest {
  string session_id = 1;
  string session_key = 2;
  string session_file = 3;
  int32 token_budget = 4;
  bool force = 5;
  int32 current_token_count = 6;
  string compaction_target = 7;      // "budget" | "threshold"
  string custom_instructions = 8;
}

// AfterTurnRequest 对齐 OpenClaw afterTurn 参数
message AfterTurnRequest {
  string session_id = 1;
  string session_key = 2;
  string session_file = 3;
  repeated AgentMessage messages = 4;
  int32 pre_prompt_message_count = 5;
  string auto_compaction_summary = 6;  // optional
  bool is_heartbeat = 7;
  int32 token_budget = 8;              // optional
}

// SubagentSpawnRequest 对齐 OpenClaw prepareSubagentSpawn 参数
message SubagentSpawnRequest {
  string parent_session_key = 1;
  string child_session_key = 2;
  int64 ttl_ms = 3;                    // optional
}

message SubagentSpawnResult {
  // 空表示成功；未来可扩展 rollback token 等
}

// SubagentEndedRequest 对齐 OpenClaw onSubagentEnded 参数
message SubagentEndedRequest {
  string child_session_key = 1;
  string reason = 2;                   // "deleted" | "completed" | "swept" | "released"
}

// ---- 服务定义 ----

service ContextEngineService {
  // 生命周期
  rpc Bootstrap(BootstrapRequest) returns (BootstrapResult);
  rpc Maintain(MaintainRequest) returns (MaintenanceResult);
  rpc Dispose(Empty) returns (Empty);

  // 核心方法
  rpc Ingest(IngestRequest) returns (IngestResult);
  rpc IngestBatch(IngestBatchRequest) returns (IngestBatchResult);
  rpc Assemble(AssembleRequest) returns (AssembleResult);
  rpc Compact(CompactRequest) returns (CompactResult);

  // Turn 后生命周期
  rpc AfterTurn(AfterTurnRequest) returns (Empty);

  // 子代理生命周期
  rpc PrepareSubagentSpawn(SubagentSpawnRequest) returns (SubagentSpawnResult);
  rpc OnSubagentEnded(SubagentEndedRequest) returns (Empty);

  // 元数据
  rpc GetInfo(Empty) returns (EngineInfo);
}
```

---

## 4. 领域模型设计

### 4.1 设计原则：对话消息与知识记忆分离

> **v1 方案错误地给 Memory 加了 `MemoryType` 字段来统一处理。v2 采用分离策略。**

理由：
- 生命周期不同：知识记忆持久且永不压缩；对话消息会被 Compact 清理
- 查询模式不同：知识检索全局搜索；对话检索需 `sessionID` 过滤 + 时间排序
- 索引策略不同：知识记忆用现有 `memories` FTS5 + HNSW；对话消息需独立索引
- 实体字段差异大：对话消息有 `Role`/`SessionID`/`IsCompacted` 等，知识记忆有 `Path`/`StartLine`/`SourceType` 等

### 4.2 新增实体（不修改现有 Memory）

```go
// internal/domain/entity/conversation.go（新文件）

package entity

// ConversationMessage 对话消息实体
// 对齐 OpenClaw AgentMessage 联合类型（UserMessage | AssistantMessage | ToolResultMessage）
type ConversationMessage struct {
    ID          string `json:"id"`
    SessionID   string `json:"session_id"`
    SessionKey  string `json:"session_key,omitempty"`
    Role        string `json:"role"`           // "user" | "assistant" | "system" | "tool" | "toolResult"
    Content     string `json:"content"`
    Timestamp   int64  `json:"timestamp"`

    // assistant 专有
    API         string `json:"api,omitempty"`
    Provider    string `json:"provider,omitempty"`
    Model       string `json:"model,omitempty"`
    StopReason  string `json:"stop_reason,omitempty"`

    // toolResult 专有
    ToolCallID  string `json:"tool_call_id,omitempty"`
    ToolName    string `json:"tool_name,omitempty"`
    IsError     bool   `json:"is_error,omitempty"`

    // 通用
    MessageHash string `json:"message_hash"`             // SHA-256 去重
    IsHeartbeat bool   `json:"is_heartbeat,omitempty"`
    IsCompacted bool   `json:"is_compacted"`              // Compact 标记
    CreatedAt   int64  `json:"created_at"`
}

// MessageEmbedding 对话消息向量
type MessageEmbedding struct {
    ID        string    `json:"id"`
    MessageID string    `json:"message_id"`
    SessionID string    `json:"session_id"`
    Provider  string    `json:"provider"`
    Vector    []float32 `json:"vector"`
    Dimension int       `json:"dimension"`
    CreatedAt int64     `json:"created_at"`
}
```

```go
// internal/domain/entity/summary.go（新文件）

package entity

// ConversationSummary 对话摘要
type ConversationSummary struct {
    ID          string `json:"id"`
    SessionID   string `json:"session_id"`
    SessionKey  string `json:"session_key,omitempty"`
    StartMsgID  string `json:"start_msg_id"`
    EndMsgID    string `json:"end_msg_id"`
    SummaryText string `json:"summary_text"`
    TokenCount  int    `json:"token_count"`
    Version     int    `json:"version"`
    CreatedAt   int64  `json:"created_at"`
}
```

### 4.3 现有 Memory 实体 — 保持不变

`internal/domain/entity/memory.go` 不做任何修改。知识记忆与对话消息是完全独立的概念。

---

## 5. 仓库接口设计

### 5.1 新增接口（Interface Segregation Principle）

```go
// internal/domain/repository/conversation_repo.go（新文件）

package repository

import (
    "context"

    "github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// ConversationListOptions 对话消息列表查询参数
type ConversationListOptions struct {
    SessionID string
    Limit     int
    Offset    int
    BeforeTS  int64  // 只返回早于此时间戳的消息（用于 Compact）
    UncompactedOnly bool  // 只返回未压缩的消息
}

// ConversationRepository 对话消息持久化接口
type ConversationRepository interface {
    // Save 存储单条对话消息
    Save(ctx context.Context, msg *entity.ConversationMessage) error

    // SaveBatch 批量存储对话消息
    SaveBatch(ctx context.Context, msgs []*entity.ConversationMessage) error

    // GetBySession 获取会话消息（支持分页和过滤）
    GetBySession(ctx context.Context, opts *ConversationListOptions) ([]*entity.ConversationMessage, error)

    // GetRecent 获取会话最近 N 条消息（按时间正序）
    GetRecent(ctx context.Context, sessionID string, limit int) ([]*entity.ConversationMessage, error)

    // GetOlderForCompaction 获取早于指定时间戳的未压缩消息（用于 Compact）
    GetOlderForCompaction(ctx context.Context, sessionID string, beforeTimestamp int64) ([]*entity.ConversationMessage, error)

    // MarkCompacted 标记消息为已压缩
    MarkCompacted(ctx context.Context, sessionID string, ids []string) error

    // ExistsByHash 检查消息哈希是否已存在（去重）
    ExistsByHash(ctx context.Context, hash string) (bool, error)

    // ExistsByHashes 批量检查消息哈希
    ExistsByHashes(ctx context.Context, hashes []string) (map[string]bool, error)

    // CountBySession 统计会话消息数
    CountBySession(ctx context.Context, sessionID string) (int, error)

    // DeleteBySession 删除会话所有消息
    DeleteBySession(ctx context.Context, sessionID string) error
}
```

```go
// internal/domain/repository/summary_repo.go（新文件）

package repository

import (
    "context"

    "github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// SummaryRepository 对话摘要持久化接口
type SummaryRepository interface {
    // Save 保存对话摘要
    Save(ctx context.Context, summary *entity.ConversationSummary) error

    // GetBySession 获取会话所有摘要（按创建时间正序）
    GetBySession(ctx context.Context, sessionID string) ([]*entity.ConversationSummary, error)

    // GetLatest 获取会话最新摘要
    GetLatest(ctx context.Context, sessionID string) (*entity.ConversationSummary, error)
}
```

```go
// internal/domain/repository/message_embedding_repo.go（新文件）

package repository

import (
    "context"

    "github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// MessageEmbeddingRepository 对话消息向量索引接口
type MessageEmbeddingRepository interface {
    // Save 存储单条消息向量
    Save(ctx context.Context, emb *entity.MessageEmbedding) error

    // SaveBatch 批量存储消息向量
    SaveBatch(ctx context.Context, embeddings []*entity.MessageEmbedding) error

    // SearchVector 在指定会话内进行向量检索
    SearchVector(ctx context.Context, embedding []float32, sessionID string, opts *SearchOptions) ([]*entity.ConversationMessage, error)

    // SearchFulltext 在指定会话内进行全文检索
    SearchFulltext(ctx context.Context, query string, sessionID string, opts *SearchOptions) ([]*entity.ConversationMessage, error)

    // DeleteByMessageIDs 删除指定消息的向量
    DeleteByMessageIDs(ctx context.Context, messageIDs []string) error

    // RebuildVectorIndex 重建会话向量索引
    RebuildVectorIndex(ctx context.Context, sessionID string) error
}
```

```go
// internal/domain/repository/llm_repo.go（新文件）

package repository

import (
    "context"
)

// LLMResponse LLM 生成结果
type LLMResponse struct {
    Text  string
    Tokens int
}

// LLMRepository LLM 文本生成接口（用于摘要等）
type LLMRepository interface {
    // Generate 生成文本
    Generate(ctx context.Context, prompt string) (*LLMResponse, error)
}
```

### 5.2 接口复用关系

| 接口 | 实现位置 | 用途 |
|------|---------|------|
| `ConversationRepository` | `sqlite/conversation_ops.go` | 对话消息 CRUD |
| `SummaryRepository` | `sqlite/summary_ops.go` | 摘要 CRUD |
| `MessageEmbeddingRepository` | `sqlite/message_embedding_ops.go` | 对话向量索引 |
| `LLMRepository` | `llm/provider.go` | 摘要生成 |
| `EmbeddingRepository`（现有） | `embedding/cached_provider.go` | 共享 embedding |
| `SearchRepository`（现有） | `search/hybrid.go` | 知识记忆检索 |
| `MemoryRepository`（现有） | `sqlite/store.go` | 知识记忆 CRUD |

---

## 6. 数据库设计

### 6.1 新增表（与现有 `memories` / `memories_fts` 完全隔离）

```sql
-- conversation_messages：对话消息（对应 OpenClaw AgentMessage）
CREATE TABLE IF NOT EXISTS conversation_messages (
    id            TEXT PRIMARY KEY,
    session_id    TEXT NOT NULL,
    session_key   TEXT,
    role          TEXT NOT NULL,              -- user/assistant/system/tool/toolResult
    content       TEXT NOT NULL,
    timestamp     INTEGER NOT NULL,
    -- assistant 专有
    api           TEXT,
    provider      TEXT,
    model         TEXT,
    stop_reason   TEXT,
    -- toolResult 专有
    tool_call_id  TEXT,
    tool_name     TEXT,
    is_error      INTEGER DEFAULT 0,
    -- 通用
    message_hash  TEXT UNIQUE NOT NULL,       -- SHA-256 去重
    is_heartbeat  INTEGER DEFAULT 0,
    is_compacted  INTEGER DEFAULT 0,          -- Compact 标记
    created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conv_session_ts ON conversation_messages(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_conv_hash ON conversation_messages(message_hash);
CREATE INDEX IF NOT EXISTS idx_conv_compacted ON conversation_messages(session_id, is_compacted);

-- conversation_messages_fts：对话消息全文索引
CREATE VIRTUAL TABLE IF NOT EXISTS conversation_messages_fts USING fts5(
    id, session_id, role, content,
    content=conversation_messages,
    content_rowid=rowid,
    tokenize='porter unicode61'
);

-- 触发器：保持 FTS 索引同步
CREATE TRIGGER IF NOT EXISTS conv_msg_fts_insert AFTER INSERT ON conversation_messages BEGIN
    INSERT INTO conversation_messages_fts(rowid, id, session_id, role, content)
    VALUES (new.rowid, new.id, new.session_id, new.role, new.content);
END;

CREATE TRIGGER IF NOT EXISTS conv_msg_fts_delete AFTER DELETE ON conversation_messages BEGIN
    INSERT INTO conversation_messages_fts(conversation_messages_fts, rowid, id, session_id, role, content)
    VALUES ('delete', old.rowid, old.id, old.session_id, old.role, old.content);
END;

-- message_embeddings：对话消息向量
CREATE TABLE IF NOT EXISTS message_embeddings (
    id         TEXT PRIMARY KEY,
    message_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    provider   TEXT NOT NULL,
    vector     BLOB NOT NULL,                -- 序列化向量（与现有 memories.embedding 格式一致）
    dimension  INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (message_id) REFERENCES conversation_messages(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_msg_emb_session ON message_embeddings(session_id);
CREATE INDEX IF NOT EXISTS idx_msg_emb_message ON message_embeddings(message_id);

-- conversation_summaries：对话摘要缓存
CREATE TABLE IF NOT EXISTS conversation_summaries (
    id           TEXT PRIMARY KEY,
    session_id   TEXT NOT NULL,
    session_key  TEXT,
    start_msg_id TEXT NOT NULL,
    end_msg_id   TEXT NOT NULL,
    summary_text TEXT NOT NULL,
    token_count  INTEGER NOT NULL,
    version      INTEGER DEFAULT 1,
    created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_summary_session ON conversation_summaries(session_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_summary_range ON conversation_summaries(session_id, start_msg_id, end_msg_id);
```

### 6.2 独立向量索引

对话消息使用独立的 HNSW 向量索引（`MessageVectorIndex`），与知识记忆的 `VectorIndex` 分开：

```go
// internal/infrastructure/search/message_vector_index.go（新文件）

// MessageVectorIndex 对话消息的 HNSW 向量索引
// 与 VectorIndex 共享底层 coder/hnsw，但作为独立实例
// 支持按 sessionID 过滤的检索
type MessageVectorIndex struct {
    mu       sync.RWMutex
    index    *hnsw.Index[string]
    sessionMap map[string][]string  // sessionID -> []messageID
    embeddings map[string][]float32 // messageID -> embedding
    config   *VectorIndexConfig
}
```

分离原因：
- 查询模式不同：对话检索需 `sessionID` 过滤 + 时间排序，知识检索全局搜索
- 生命周期不同：对话消息会被 Compact 清理，知识记忆持久
- 避免向量空间噪声：对话消息和知识记忆的语义空间不同

---

## 7. 项目结构

### 7.1 变更总览

```
MyMemoryGo/
├── proto/                                  # 新增：gRPC proto 文件
│   ├── context_engine.proto
│   └── gen/                               # 生成的 Go 代码
│       ├── context_engine.pb.go
│       └── context_engine_grpc.pb.go
│
├── cmd/
│   ├── memory/                             # 保持不变（知识记忆服务）
│   │   └── cmd/
│   │       ├── app.go
│   │       ├── root.go
│   │       └── ...
│   └── context-engine/                     # 新增：ContextEngine gRPC 服务
│       └── main.go
│
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── memory.go                  # 保持不变
│   │   │   ├── memory_test.go             # 保持不变
│   │   │   ├── conversation.go            # 新增：ConversationMessage + MessageEmbedding
│   │   │   └── summary.go                 # 新增：ConversationSummary
│   │   ├── repository/
│   │   │   ├── repository.go             # 保持不变
│   │   │   ├── repository_test.go        # 保持不变
│   │   │   ├── conversation_repo.go       # 新增：ConversationRepository
│   │   │   ├── summary_repo.go            # 新增：SummaryRepository
│   │   │   ├── message_embedding_repo.go  # 新增：MessageEmbeddingRepository
│   │   │   └── llm_repo.go               # 新增：LLMRepository
│   │   ├── service/
│   │   │   ├── validator.go              # 保持不变
│   │   │   ├── id_generator.go           # 保持不变
│   │   │   └── ...                        # 保持不变
│   │   └── errors/
│   │       └── errors.go                 # 保持不变
│   │
│   ├── application/
│   │   └── service/
│   │       ├── service.go                # 保持不变
│   │       ├── store.go                  # 保持不变
│   │       ├── search.go                 # 保持不变
│   │       ├── list.go                   # 保持不变
│   │       ├── sync.go                   # 保持不变
│   │       └── context_engine.go         # 新增：ContextEngine 应用服务
│   │
│   ├── infrastructure/
│   │   ├── persistence/
│   │   │   ├── sqlite/
│   │   │   │   ├── store.go              # 保持不变
│   │   │   │   ├── db_setup.go           # 扩展：新增表的 schema
│   │   │   │   ├── write_ops.go          # 保持不变
│   │   │   │   ├── read_ops.go           # 保持不变
│   │   │   │   ├── delete_ops.go         # 保持不变
│   │   │   │   ├── search_core.go        # 保持不变
│   │   │   │   ├── conversation_ops.go   # 新增：ConversationRepository 实现
│   │   │   │   ├── summary_ops.go        # 新增：SummaryRepository 实现
│   │   │   │   └── message_embedding_ops.go  # 新增：MessageEmbeddingRepository 实现
│   │   │   └── filestore/               # 保持不变
│   │   ├── embedding/                    # 保持不变（共享）
│   │   ├── search/
│   │   │   ├── hybrid.go                # 保持不变（知识记忆检索复用）
│   │   │   ├── vector_index.go          # 保持不变
│   │   │   └── message_vector_index.go  # 新增：对话消息独立向量索引
│   │   └── llm/                          # 新增：LLM 文本生成
│   │       └── provider.go
│   │
│   ├── interface/
│   │   ├── api/                          # 保持不变（REST API）
│   │   └── grpc/                         # 新增：gRPC 服务端
│   │       ├── server.go
│   │       └── context_engine_handler.go
│   │
│   └── pkg/                              # 保持不变
│
└── configs/
    └── config.yaml                       # 扩展：新增 context_engine 和 llm 配置段
```

---

## 8. 核心实现

### 8.1 ContextEngine 应用服务

```go
// internal/application/service/context_engine.go

package service

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "math"
    "strings"
    "time"

    "github.com/xgsong/MyMemoryGo/internal/domain/entity"
    "github.com/xgsong/MyMemoryGo/internal/domain/repository"
    "github.com/xgsong/MyMemoryGo/internal/pkg/log"
)

// ContextEngineConfig ContextEngine 行为配置
type ContextEngineConfig struct {
    RecentMessageBudgetRatio float64 // 近期消息占 token budget 的比例，默认 0.7
    SummaryTriggerTokens     int     // 触发主动压缩的 token 阈值
    MaxSummaryRetries        int     // 摘要生成重试次数
    FallbackSummary          bool    // 生成失败时是否使用降级摘要
}

// DefaultContextEngineConfig 默认配置
func DefaultContextEngineConfig() *ContextEngineConfig {
    return &ContextEngineConfig{
        RecentMessageBudgetRatio: 0.7,
        SummaryTriggerTokens:     3000,
        MaxSummaryRetries:        3,
        FallbackSummary:          true,
    }
}

// ContextEngineService ContextEngine 应用服务
// 依赖领域接口，不依赖具体实现
type ContextEngineService struct {
    config          *ContextEngineConfig
    convRepo        repository.ConversationRepository
    summaryRepo     repository.SummaryRepository
    msgEmbedRepo    repository.MessageEmbeddingRepository
    searchRepo      repository.SearchRepository      // 复用 HybridEngine（知识记忆检索）
    embeddingRepo   repository.EmbeddingRepository    // 共享 embedding
    llmRepo         repository.LLMRepository          // 摘要生成
}

// NewContextEngineService 创建 ContextEngine 应用服务
func NewContextEngineService(
    config *ContextEngineConfig,
    convRepo repository.ConversationRepository,
    summaryRepo repository.SummaryRepository,
    msgEmbedRepo repository.MessageEmbeddingRepository,
    searchRepo repository.SearchRepository,
    embeddingRepo repository.EmbeddingRepository,
    llmRepo repository.LLMRepository,
) *ContextEngineService {
    return &ContextEngineService{
        config:        config,
        convRepo:      convRepo,
        summaryRepo:   summaryRepo,
        msgEmbedRepo:  msgEmbedRepo,
        searchRepo:    searchRepo,
        embeddingRepo: embeddingRepo,
        llmRepo:       llmRepo,
    }
}
```

#### 8.1.1 Ingest — 消息摄入

```go
// Ingest 处理单条消息
func (s *ContextEngineService) Ingest(ctx context.Context, sessionID, sessionKey string, msg *entity.ConversationMessage, isHeartbeat bool) (bool, error) {
    // 1. 哈希去重
    hash := s.hashMessage(msg)
    exists, err := s.convRepo.ExistsByHash(ctx, hash)
    if err != nil {
        return false, fmt.Errorf("check hash: %w", err)
    }
    if exists {
        return false, nil
    }
    msg.MessageHash = hash
    msg.IsHeartbeat = isHeartbeat

    // 2. 存储对话消息
    if err := s.convRepo.Save(ctx, msg); err != nil {
        return false, fmt.Errorf("save message: %w", err)
    }

    // 3. 生成并存储 embedding（失败不影响 ingest 成功）
    if err := s.generateAndStoreEmbedding(ctx, msg); err != nil {
        log.Warn("failed to generate embedding", "error", err, "message_id", msg.ID)
    }

    return true, nil
}

// IngestBatch 批量处理消息
func (s *ContextEngineService) IngestBatch(ctx context.Context, sessionID, sessionKey string, msgs []*entity.ConversationMessage, isHeartbeat bool) (int, error) {
    // 1. 计算哈希
    hashes := make([]string, len(msgs))
    for i, msg := range msgs {
        hashes[i] = s.hashMessage(msg)
        msg.MessageHash = hashes[i]
        msg.IsHeartbeat = isHeartbeat
    }

    // 2. 批量去重
    existingHashes, err := s.convRepo.ExistsByHashes(ctx, hashes)
    if err != nil {
        return 0, fmt.Errorf("batch check hashes: %w", err)
    }

    // 3. 过滤新消息
    var newMsgs []*entity.ConversationMessage
    for i, msg := range msgs {
        if !existingHashes[msgs[i].MessageHash] {
            newMsgs = append(newMsgs, msg)
        }
    }
    if len(newMsgs) == 0 {
        return 0, nil
    }

    // 4. 批量插入
    if err := s.convRepo.SaveBatch(ctx, newMsgs); err != nil {
        return 0, fmt.Errorf("batch save messages: %w", err)
    }

    // 5. 批量生成 embedding
    texts := make([]string, len(newMsgs))
    for i, msg := range newMsgs {
        texts[i] = formatMessageForEmbedding(msg)
    }
    vectors, err := s.embeddingRepo.EmbedBatch(ctx, texts)
    if err != nil {
        log.Warn("failed to batch generate embeddings", "error", err)
        return len(newMsgs), nil
    }

    // 6. 批量存储 embedding
    var embeddings []*entity.MessageEmbedding
    for i, msg := range newMsgs {
        if i >= len(vectors) {
            break
        }
        embeddings = append(embeddings, &entity.MessageEmbedding{
            ID:        fmt.Sprintf("emb_%s", msg.ID),
            MessageID: msg.ID,
            SessionID: msg.SessionID,
            Provider:  s.embeddingRepo.Model(),
            Vector:    vectors[i],
            Dimension: len(vectors[i]),
            CreatedAt: time.Now().Unix(),
        })
    }
    if err := s.msgEmbedRepo.SaveBatch(ctx, embeddings); err != nil {
        log.Warn("failed to batch save embeddings", "error", err)
    }

    return len(newMsgs), nil
}
```

#### 8.1.2 Assemble — 三层上下文组装

```go
// AssemblePipeline 组装上下文（三层检索策略）
func (s *ContextEngineService) Assemble(ctx context.Context, sessionID, sessionKey string, tokenBudget int, prompt string, availableTools []string, citationsMode string) (*AssembleResult, error) {
    // 1. 加载历史摘要
    summaries, _ := s.summaryRepo.GetBySession(ctx, sessionID)
    summaryTokens := estimateSummaryTokens(summaries)

    // 2. 获取近期消息（顺序保留，占 budget 的 70%）
    recentBudget := int(float64(tokenBudget) * s.config.RecentMessageBudgetRatio)
    recentMessages, _ := s.convRepo.GetRecent(ctx, sessionID, recentBudget*2) // 多取一些用于估算
    recentTokens := estimateMessageTokens(recentMessages)

    // 截断近期消息到 budget 内
    if recentTokens > recentBudget {
        recentMessages = truncateMessagesByBudget(recentMessages, recentBudget)
        recentTokens = estimateMessageTokens(recentMessages)
    }

    // 3. 远期对话检索（MessageVectorIndex + FTS5 混合）
    budgetForOld := tokenBudget - recentTokens - summaryTokens
    var retrievedMessages []*entity.ConversationMessage
    if budgetForOld > 0 && prompt != "" {
        retrievedMessages = s.retrieveOldConversationMessages(ctx, sessionID, prompt, budgetForOld)
    }

    // 4. 知识记忆交叉检索（复用 HybridEngine）
    var knowledgeHits []*entity.SearchHit
    if budgetForOld > 0 && prompt != "" {
        opts, _ := repository.NewSearchOptionsBuilder().
            WithLimit(5).
            WithMinScore(0.6).
            Build()
        result, err := s.searchRepo.Search(ctx, prompt, opts)
        if err == nil && result != nil {
            knowledgeHits = result.Hits
        }
    }

    // 5. 合并消息（摘要 → 远期对话 → 知识记忆 → 近期消息）
    finalMessages := s.mergeContextLayers(summaries, retrievedMessages, knowledgeHits, recentMessages, tokenBudget)

    // 6. 保证对话完整性
    finalMessages = ensureConversationIntegrity(finalMessages)

    // 7. 最终截断到 token budget
    finalMessages = truncateMessagesByBudget(finalMessages, tokenBudget)

    totalTokens := estimateMessageTokens(finalMessages)

    return &AssembleResult{
        Messages:            finalMessages,
        EstimatedTokens:     totalTokens,
        SystemPromptAddition: s.buildSystemPromptAddition(availableTools, citationsMode),
    }, nil
}

// retrieveOldConversationMessages 远期对话检索（向量 + 全文混合）
func (s *ContextEngineService) retrieveOldConversationMessages(ctx context.Context, sessionID, query string, budget int) []*entity.ConversationMessage {
    // 生成查询 embedding
    queryEmb, err := s.embeddingRepo.Embed(ctx, query)
    if err != nil {
        log.Warn("failed to embed query for conversation retrieval", "error", err)
        return nil
    }

    opts, _ := repository.NewSearchOptionsBuilder().
        WithLimit(20).
        WithMinScore(0.4).
        Build()

    // 向量检索
    vecResults, err := s.msgEmbedRepo.SearchVector(ctx, queryEmb, sessionID, opts)
    if err != nil {
        log.Warn("vector search for conversation failed", "error", err)
    }

    // 全文检索
    ftsResults, err := s.msgEmbedRepo.SearchFulltext(ctx, query, sessionID, opts)
    if err != nil {
        log.Warn("fulltext search for conversation failed", "error", err)
    }

    // 合并去重，按时间排序
    merged := mergeAndDeduplicate(vecResults, ftsResults)

    // 按 budget 截断
    return truncateMessagesByBudget(merged, budget)
}

// mergeContextLayers 合并三层上下文
// 顺序：摘要(虚拟system消息) → 远期对话 → 知识记忆(虚拟system消息) → 近期消息
func (s *ContextEngineService) mergeContextLayers(
    summaries []*entity.ConversationSummary,
    retrieved []*entity.ConversationMessage,
    knowledge []*entity.SearchHit,
    recent []*entity.ConversationMessage,
    budget int,
) []*entity.ConversationMessage {
    var result []*entity.ConversationMessage

    // 摘要层 → 转为虚拟 system 消息
    for _, sum := range summaries {
        result = append(result, &entity.ConversationMessage{
            ID:        fmt.Sprintf("summary_%s", sum.ID),
            SessionID: sum.SessionID,
            Role:      "system",
            Content:   fmt.Sprintf("[历史对话摘要] %s", sum.SummaryText),
            Timestamp: sum.CreatedAt,
        })
    }

    // 远期对话层（按时间正序）
    result = append(result, retrieved...)

    // 知识记忆层 → 转为虚拟 system 消息
    for _, hit := range knowledge {
        result = append(result, &entity.ConversationMessage{
            ID:        fmt.Sprintf("knowledge_%s", hit.ID),
            SessionID: recent[0].SessionID,
            Role:      "system",
            Content:   fmt.Sprintf("[相关记忆] %s", hit.Snippet),
            Timestamp: hit.Timestamp.Unix(),
        })
    }

    // 近期消息层（顺序保留）
    result = append(result, recent...)

    return result
}

// ensureConversationIntegrity 确保对话完整性，保留完整的 user-assistant 配对
func ensureConversationIntegrity(messages []*entity.ConversationMessage) []*entity.ConversationMessage {
    if len(messages) == 0 {
        return nil
    }

    result := make([]*entity.ConversationMessage, 0, len(messages))
    var lastUserMsg *entity.ConversationMessage

    for _, msg := range messages {
        switch msg.Role {
        case "user":
            if lastUserMsg != nil {
                // 未配对的 user 消息，丢弃
                lastUserMsg = nil
            }
            lastUserMsg = msg
        case "assistant", "toolResult":
            if lastUserMsg != nil {
                result = append(result, lastUserMsg)
                result = append(result, msg)
                lastUserMsg = nil
            }
        case "system":
            // system 消息可以单独保留
            result = append(result, msg)
        }
    }

    return result
}
```

#### 8.1.3 Compact — 上下文压缩

```go
// Compact 压缩上下文
func (s *ContextEngineService) Compact(ctx context.Context, sessionID, sessionKey string, tokenBudget int, force bool, customInstructions string) (*CompactResult, error) {
    // 1. 确定压缩范围：获取远期未压缩消息
    //    近期消息阈值 = 保留最近 N 条（N 基于预算估算）
    recentCount := estimateMessageCountForBudget(tokenBudget)
    recentMsgs, _ := s.convRepo.GetRecent(ctx, sessionID, recentCount)
    if len(recentMsgs) == 0 {
        return &CompactResult{Ok: true, Compacted: false, Reason: "no messages to compact"}, nil
    }

    // 远期消息 = 早于近期消息最早时间戳的未压缩消息
    cutoffTS := recentMsgs[0].Timestamp
    oldMessages, err := s.convRepo.GetOlderForCompaction(ctx, sessionID, cutoffTS)
    if err != nil {
        return &CompactResult{Ok: false, Compacted: false, Reason: err.Error()}, nil
    }
    if len(oldMessages) == 0 {
        return &CompactResult{Ok: true, Compacted: false, Reason: "no old messages to compact"}, nil
    }

    // 2. 生成摘要（重试 + 降级）
    summaryText, tokenCount, err := s.generateSummary(ctx, oldMessages, customInstructions)
    if err != nil {
        return &CompactResult{Ok: false, Compacted: false, Reason: err.Error()}, nil
    }

    // 3. 保存摘要
    summary := &entity.ConversationSummary{
        ID:          fmt.Sprintf("summary_%d", time.Now().UnixNano()),
        SessionID:   sessionID,
        SessionKey:  sessionKey,
        StartMsgID:  oldMessages[0].ID,
        EndMsgID:    oldMessages[len(oldMessages)-1].ID,
        SummaryText: summaryText,
        TokenCount:  tokenCount,
        Version:     1,
        CreatedAt:   time.Now().Unix(),
    }
    if err := s.summaryRepo.Save(ctx, summary); err != nil {
        return nil, fmt.Errorf("save summary: %w", err)
    }

    // 4. 标记旧消息为已压缩
    ids := make([]string, len(oldMessages))
    for i, msg := range oldMessages {
        ids[i] = msg.ID
    }
    if err := s.convRepo.MarkCompacted(ctx, sessionID, ids); err != nil {
        log.Warn("failed to mark messages compacted", "error", err)
    }

    // 5. 删除旧消息的 embedding
    if err := s.msgEmbedRepo.DeleteByMessageIDs(ctx, ids); err != nil {
        log.Warn("failed to delete old embeddings", "error", err)
    }

    tokensBefore := estimateMessageTokens(oldMessages)
    var firstKeptID string
    if len(recentMsgs) > 0 {
        firstKeptID = recentMsgs[0].ID
    }

    return &CompactResult{
        Ok:        true,
        Compacted: true,
        Result: &CompactDetails{
            Summary:         summaryText,
            FirstKeptEntryId: firstKeptID,
            TokensBefore:    tokensBefore,
            TokensAfter:     tokenCount,
        },
    }, nil
}

// generateSummary 生成摘要（重试 + 降级策略）
func (s *ContextEngineService) generateSummary(ctx context.Context, messages []*entity.ConversationMessage, instructions string) (string, int, error) {
    prompt := s.buildSummaryPrompt(messages, instructions)
    var lastErr error

    for i := 0; i < s.config.MaxSummaryRetries; i++ {
        select {
        case <-ctx.Done():
            return "", 0, ctx.Err()
        default:
        }

        resp, err := s.llmRepo.Generate(ctx, prompt)
        if err == nil {
            return resp.Text, resp.Tokens, nil
        }
        lastErr = err
        log.Warn("summary generation failed, retrying", "attempt", i+1, "error", err)

        if i < s.config.MaxSummaryRetries-1 {
            time.Sleep(time.Duration(math.Pow(2, float64(i))) * time.Second)
        }
    }

    // 降级策略：生成不含 LLM 的粗略摘要
    if s.config.FallbackSummary {
        log.Warn("all retries failed, using fallback summary", "error", lastErr)
        return s.generateFallbackSummary(messages), estimateMessageTokens(messages) / 10, nil
    }

    return "", 0, lastErr
}

// generateFallbackSummary 降级摘要（无需 LLM）
func (s *ContextEngineService) generateFallbackSummary(messages []*entity.ConversationMessage) string {
    var buf bytes.Buffer
    buf.WriteString(fmt.Sprintf("对话摘要（共%d条消息，时间范围：%s ~ %s）\n",
        len(messages),
        time.Unix(messages[0].Timestamp, 0).Format(time.RFC3339),
        time.Unix(messages[len(messages)-1].Timestamp, 0).Format(time.RFC3339),
    ))

    userCount, assistantCount, toolCount := 0, 0, 0
    for _, msg := range messages {
        switch msg.Role {
        case "user":
            userCount++
        case "assistant":
            assistantCount++
        case "toolResult", "tool":
            toolCount++
        }
    }
    buf.WriteString(fmt.Sprintf("用户消息：%d条，助手回复：%d条，工具调用：%d次\n", userCount, assistantCount, toolCount))

    if len(messages) > 6 {
        buf.WriteString("\n消息预览：\n")
        for i := 0; i < 3; i++ {
            buf.WriteString(fmt.Sprintf("- %s: %s\n", messages[i].Role, truncateString(messages[i].Content, 200)))
        }
        buf.WriteString("...\n")
        for i := len(messages) - 3; i < len(messages); i++ {
            buf.WriteString(fmt.Sprintf("- %s: %s\n", messages[i].Role, truncateString(messages[i].Content, 200)))
        }
    }

    return buf.String()
}
```

#### 8.1.4 AfterTurn / Bootstrap / Maintain

```go
// AfterTurn 运行后生命周期（可选主动压缩）
func (s *ContextEngineService) AfterTurn(ctx context.Context, sessionID, sessionKey string, tokenBudget int) error {
    // 1. 刷新向量索引
    if err := s.msgEmbedRepo.RebuildVectorIndex(ctx, sessionID); err != nil {
        log.Warn("failed to rebuild vector index after turn", "error", err)
    }

    // 2. 检查是否需要主动压缩
    if s.config.SummaryTriggerTokens > 0 && tokenBudget > 0 {
        count, _ := s.convRepo.CountBySession(ctx, sessionID)
        estimatedTokens := count * 50 // 粗略估算
        if estimatedTokens > s.config.SummaryTriggerTokens {
            log.Info("proactive compaction triggered", "session_id", sessionID)
            _, err := s.Compact(ctx, sessionID, sessionKey, tokenBudget, false, "")
            if err != nil {
                log.Warn("proactive compaction failed", "error", err)
            }
        }
    }

    return nil
}

// Bootstrap 初始化会话（导入历史消息）
func (s *ContextEngineService) Bootstrap(ctx context.Context, sessionID, sessionKey, sessionFile string) (bool, int, string, error) {
    // 检查会话是否已有消息
    count, err := s.convRepo.CountBySession(ctx, sessionID)
    if err != nil {
        return false, 0, "", fmt.Errorf("count messages: %w", err)
    }
    if count > 0 {
        return false, 0, "session already has messages", nil
    }

    // TODO: 从 sessionFile 解析历史消息并批量导入
    // 取决于 OpenClaw 的 sessionFile 格式

    return true, 0, "bootstrap complete, no history to import", nil
}

// Maintain 执行 transcript 维护（清理已压缩消息的冗余数据）
func (s *ContextEngineService) Maintain(ctx context.Context, sessionID, sessionKey, sessionFile string) (bool, int64, int, string, error) {
    // 清理已压缩消息的冗余向量数据
    // 实际清理逻辑取决于业务需求
    return false, 0, 0, "no maintenance needed", nil
}

// GetInfo 返回引擎信息
func (s *ContextEngineService) GetInfo() *EngineInfo {
    return &EngineInfo{
        ID:              "mymemory-go",
        Name:            "MyMemoryGo Context Engine",
        Version:         "1.0.0",
        OwnsCompaction:  true,
    }
}

// Dispose 释放资源
func (s *ContextEngineService) Dispose(ctx context.Context) error {
    // 清理资源（如关闭连接池等）
    return nil
}

// PrepareSubagentSpawn 准备子代理（目前为空实现）
func (s *ContextEngineService) PrepareSubagentSpawn(ctx context.Context, parentSessionKey, childSessionKey string, ttlMs int64) error {
    // 未来可扩展：复制父会话上下文到子会话
    return nil
}

// OnSubagentEnded 子代理结束（目前为空实现）
func (s *ContextEngineService) OnSubagentEnded(ctx context.Context, childSessionKey, reason string) error {
    // 未来可扩展：合并子会话摘要到父会话
    return nil
}
```

#### 8.1.5 私有辅助方法

```go
func (s *ContextEngineService) hashMessage(msg *entity.ConversationMessage) string {
    data := fmt.Sprintf("%s:%s:%d:%s:%s:%s", msg.Role, msg.Content, msg.Timestamp, msg.ToolName, msg.ToolCallID, msg.Model)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

func (s *ContextEngineService) generateAndStoreEmbedding(ctx context.Context, msg *entity.ConversationMessage) error {
    text := formatMessageForEmbedding(msg)
    vec, err := s.embeddingRepo.Embed(ctx, text)
    if err != nil {
        return err
    }
    emb := &entity.MessageEmbedding{
        ID:        fmt.Sprintf("emb_%s", msg.ID),
        MessageID: msg.ID,
        SessionID: msg.SessionID,
        Provider:  s.embeddingRepo.Model(),
        Vector:    vec,
        Dimension: len(vec),
        CreatedAt: time.Now().Unix(),
    }
    return s.msgEmbedRepo.Save(ctx, emb)
}

func formatMessageForEmbedding(msg *entity.ConversationMessage) string {
    switch msg.Role {
    case "user":
        return fmt.Sprintf("User: %s", msg.Content)
    case "assistant":
        return fmt.Sprintf("Assistant: %s", msg.Content)
    case "toolResult":
        return fmt.Sprintf("Tool[%s] Result: %s", msg.ToolName, msg.Content)
    default:
        return msg.Content
    }
}

func (s *ContextEngineService) buildSummaryPrompt(messages []*entity.ConversationMessage, instructions string) string {
    var buf bytes.Buffer
    buf.WriteString("请总结以下对话内容，保留关键信息、决策和重要细节。\n\n")
    if instructions != "" {
        buf.WriteString(fmt.Sprintf("特别关注：%s\n\n", instructions))
    }
    buf.WriteString("对话内容：\n")
    for _, msg := range messages {
        buf.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, truncateString(msg.Content, 500)))
    }
    return buf.String()
}

func (s *ContextEngineService) buildSystemPromptAddition(tools []string, citationsMode string) string {
    var lines []string
    lines = append(lines, "## Memory Recall")
    if citationsMode != "" {
        lines = append(lines, fmt.Sprintf("citations=%s", citationsMode))
    }
    if len(tools) > 0 {
        lines = append(lines, fmt.Sprintf("available_tools=%s", strings.Join(tools, ",")))
    }
    return strings.Join(lines, "\n")
}

// --- Token 估算工具函数 ---

func estimateMessageTokens(messages []*entity.ConversationMessage) int {
    total := 0
    for _, msg := range messages {
        // 粗略估算：1 个中文字符 ≈ 2 tokens，1 个英文单词 ≈ 1.3 tokens
        total += len(msg.Content)/2 + 10 // +10 为角色/元数据开销
    }
    return total
}

func estimateSummaryTokens(summaries []*entity.ConversationSummary) int {
    total := 0
    for _, sum := range summaries {
        total += sum.TokenCount
    }
    return total
}

func estimateMessageCountForBudget(tokenBudget int) int {
    return tokenBudget / 60 // 平均每条消息约 60 tokens
}

func truncateMessagesByBudget(messages []*entity.ConversationMessage, budget int) []*entity.ConversationMessage {
    totalTokens := 0
    for i, msg := range messages {
        msgTokens := len(msg.Content)/2 + 10
        if totalTokens+msgTokens > budget {
            return messages[:i]
        }
        totalTokens += msgTokens
    }
    return messages
}

func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}

func mergeAndDeduplicate(vecResults, ftsResults []*entity.ConversationMessage) []*entity.ConversationMessage {
    seen := make(map[string]bool)
    var result []*entity.ConversationMessage
    for _, msg := range append(vecResults, ftsResults...) {
        if !seen[msg.ID] {
            seen[msg.ID] = true
            result = append(result, msg)
        }
    }
    // 按时间正序
    sort.Slice(result, func(i, j int) bool {
        return result[i].Timestamp < result[j].Timestamp
    })
    return result
}
```

---

## 9. gRPC 服务端

### 9.1 服务端实现

```go
// internal/interface/grpc/server.go

package grpc

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/reflection"

    pb "github.com/xgsong/MyMemoryGo/proto/gen"
    "github.com/xgsong/MyMemoryGo/internal/application/service"
    "github.com/xgsong/MyMemoryGo/internal/pkg/log"
)

// GRPCServerConfig gRPC 服务配置
type GRPCServerConfig struct {
    Address               string
    MaxReceiveMessageSize int
    EnableHealthCheck     bool
    EnableReflection      bool
    MetricsEnabled        bool
    MetricsPath           string
    MetricsPort           int
}

// Server gRPC 服务端
type Server struct {
    config        *GRPCServerConfig
    grpcServer    *grpc.Server
    svc           *service.ContextEngineService
    metricsServer *http.Server
}

// NewServer 创建 gRPC 服务端
func NewServer(cfg *GRPCServerConfig, svc *service.ContextEngineService) *Server {
    opts := []grpc.ServerOption{}
    if cfg.MaxReceiveMessageSize > 0 {
        opts = append(opts, grpc.MaxRecvMsgSize(cfg.MaxReceiveMessageSize))
    }

    grpcServer := grpc.NewServer(opts...)

    // 健康检查
    if cfg.EnableHealthCheck {
        healthServer := health.NewServer()
        healthpb.RegisterHealthServer(grpcServer, healthServer)
    }

    // 反射
    if cfg.EnableReflection {
        reflection.Register(grpcServer)
    }

    // 注册 ContextEngine 服务
    pb.RegisterContextEngineServiceServer(grpcServer, &contextEngineHandler{svc: svc})

    return &Server{
        config:     cfg,
        grpcServer: grpcServer,
        svc:        svc,
    }
}

// Serve 启动服务
func (s *Server) Serve(lis net.Listener) error {
    if s.config.MetricsEnabled {
        go s.startMetricsServer()
    }
    return s.grpcServer.Serve(lis)
}

// GracefulShutdown 优雅关闭
func (s *Server) GracefulShutdown() {
    s.grpcServer.GracefulStop()
    if s.metricsServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        s.metricsServer.Shutdown(ctx)
    }
}

func (s *Server) startMetricsServer() {
    mux := http.NewServeMux()
    // mux.Handle(s.config.MetricsPath, promhttp.Handler()) // 需要 prometheus 依赖
    s.metricsServer = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.config.MetricsPort),
        Handler: mux,
    }
    if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Error("metrics server failed", "error", err)
    }
}
```

### 9.2 Handler 实现

```go
// internal/interface/grpc/context_engine_handler.go

package grpc

import (
    "context"

    pb "github.com/xgsong/MyMemoryGo/proto/gen"
    "github.com/xgsong/MyMemoryGo/internal/application/service"
    "github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// contextEngineHandler gRPC 请求处理器
type contextEngineHandler struct {
    pb.UnimplementedContextEngineServiceServer
    svc *service.ContextEngineService
}

func (h *contextEngineHandler) Bootstrap(ctx context.Context, req *pb.BootstrapRequest) (*pb.BootstrapResult, error) {
    bootstrapped, imported, reason, err := h.svc.Bootstrap(ctx, req.SessionId, req.SessionKey, req.SessionFile)
    if err != nil {
        return nil, err
    }
    return &pb.BootstrapResult{Bootstrapped: bootstrapped, ImportedMessages: int32(imported), Reason: reason}, nil
}

func (h *contextEngineHandler) Maintain(ctx context.Context, req *pb.MaintainRequest) (*pb.MaintenanceResult, error) {
    changed, bytesFreed, rewritten, reason, err := h.svc.Maintain(ctx, req.SessionId, req.SessionKey, req.SessionFile)
    if err != nil {
        return nil, err
    }
    return &pb.MaintenanceResult{Changed: changed, BytesFreed: bytesFreed, RewrittenEntries: int32(rewritten), Reason: reason}, nil
}

func (h *contextEngineHandler) Dispose(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
    return &pb.Empty{}, h.svc.Dispose(ctx)
}

func (h *contextEngineHandler) Ingest(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResult, error) {
    msg := protoToConversationMessage(req.Message, req.SessionId, req.SessionKey)
    ingested, err := h.svc.Ingest(ctx, req.SessionId, req.SessionKey, msg, req.IsHeartbeat)
    if err != nil {
        return nil, err
    }
    return &pb.IngestResult{Ingested: ingested}, nil
}

func (h *contextEngineHandler) IngestBatch(ctx context.Context, req *pb.IngestBatchRequest) (*pb.IngestBatchResult, error) {
    msgs := make([]*entity.ConversationMessage, len(req.Messages))
    for i, m := range req.Messages {
        msgs[i] = protoToConversationMessage(m, req.SessionId, req.SessionKey)
    }
    count, err := h.svc.IngestBatch(ctx, req.SessionId, req.SessionKey, msgs, req.IsHeartbeat)
    if err != nil {
        return nil, err
    }
    return &pb.IngestBatchResult{IngestedCount: int32(count)}, nil
}

func (h *contextEngineHandler) Assemble(ctx context.Context, req *pb.AssembleRequest) (*pb.AssembleResult, error) {
    result, err := h.svc.Assemble(ctx, req.SessionId, req.SessionKey, int(req.TokenBudget), req.Prompt, req.AvailableTools, req.CitationsMode)
    if err != nil {
        return nil, err
    }
    messages := make([]*pb.AgentMessage, len(result.Messages))
    for i, m := range result.Messages {
        messages[i] = conversationMessageToProto(m)
    }
    return &pb.AssembleResult{
        Messages:            messages,
        EstimatedTokens:     int32(result.EstimatedTokens),
        SystemPromptAddition: result.SystemPromptAddition,
    }, nil
}

func (h *contextEngineHandler) Compact(ctx context.Context, req *pb.CompactRequest) (*pb.CompactResult, error) {
    result, err := h.svc.Compact(ctx, req.SessionId, req.SessionKey, int(req.TokenBudget), req.Force, req.CustomInstructions)
    if err != nil {
        return nil, err
    }
    pbResult := &pb.CompactResult{Ok: result.Ok, Compacted: result.Compacted, Reason: result.Reason}
    if result.Result != nil {
        pbResult.Result = &pb.CompactDetails{
            Summary:          result.Result.Summary,
            FirstKeptEntryId: result.Result.FirstKeptEntryId,
            TokensBefore:     result.Result.TokensBefore,
            TokensAfter:      result.Result.TokensAfter,
        }
    }
    return pbResult, nil
}

func (h *contextEngineHandler) AfterTurn(ctx context.Context, req *pb.AfterTurnRequest) (*pb.Empty, error) {
    return &pb.Empty{}, h.svc.AfterTurn(ctx, req.SessionId, req.SessionKey, int(req.TokenBudget))
}

func (h *contextEngineHandler) PrepareSubagentSpawn(ctx context.Context, req *pb.SubagentSpawnRequest) (*pb.SubagentSpawnResult, error) {
    return &pb.SubagentSpawnResult{}, h.svc.PrepareSubagentSpawn(ctx, req.ParentSessionKey, req.ChildSessionKey, req.TtlMs)
}

func (h *contextEngineHandler) OnSubagentEnded(ctx context.Context, req *pb.SubagentEndedRequest) (*pb.Empty, error) {
    return &pb.Empty{}, h.svc.OnSubagentEnded(ctx, req.ChildSessionKey, req.Reason)
}

func (h *contextEngineHandler) GetInfo(ctx context.Context, _ *pb.Empty) (*pb.EngineInfo, error) {
    info := h.svc.GetInfo()
    return &pb.EngineInfo{Id: info.ID, Name: info.Name, Version: info.Version, OwnsCompaction: info.OwnsCompaction}, nil
}

// --- Proto ↔ Entity 转换 ---

func protoToConversationMessage(m *pb.AgentMessage, sessionID, sessionKey string) *entity.ConversationMessage {
    return &entity.ConversationMessage{
        ID:         m.Id,
        SessionID:  sessionID,
        SessionKey: sessionKey,
        Role:       m.Role,
        Content:    m.Content,
        Timestamp:  m.Timestamp,
        API:        m.Api,
        Provider:   m.Provider,
        Model:      m.Model,
        StopReason: m.StopReason,
        ToolCallID: m.ToolCallId,
        ToolName:   m.ToolName,
        IsError:    m.IsError,
        CreatedAt:  m.Timestamp,
    }
}

func conversationMessageToProto(m *entity.ConversationMessage) *pb.AgentMessage {
    return &pb.AgentMessage{
        Id:         m.ID,
        Role:       m.Role,
        Content:    m.Content,
        Timestamp:  m.Timestamp,
        Api:        m.API,
        Provider:   m.Provider,
        Model:      m.Model,
        StopReason: m.StopReason,
        ToolCallId: m.ToolCallID,
        ToolName:   m.ToolName,
        IsError:    m.IsError,
    }
}
```

---

## 10. 服务入口（Composition Root）

```go
// cmd/context-engine/main.go

package main

import (
    "flag"
    "fmt"
    "net"
    "os"

    "github.com/xgsong/MyMemoryGo/internal/application/service"
    "github.com/xgsong/MyMemoryGo/internal/infrastructure/embedding"
    "github.com/xgsong/MyMemoryGo/internal/infrastructure/llm"
    "github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/sqlite"
    "github.com/xgsong/MyMemoryGo/internal/infrastructure/search"
    grpcserver "github.com/xgsong/MyMemoryGo/internal/interface/grpc"
    "github.com/xgsong/MyMemoryGo/internal/pkg/log"
)

var (
    grpcAddr  = flag.String("grpc-addr", "localhost:50051", "gRPC server address")
    configDir = flag.String("config", "", "config directory (default: ~/.memory)")
)

func main() {
    flag.Parse()

    // 1. 加载配置（复用 Viper 配置体系，扩展 context_engine 和 llm 配置段）
    cfg := loadConfig(*configDir)

    // 2. 初始化 Embedding Provider（共享缓存实例）
    embedProvider, err := embedding.NewProvider(cfg.Embedding)
    if err != nil {
        log.Fatal("failed to create embedding provider", "error", err)
    }
    cachedProvider := embedding.NewCachedProvider(embedProvider, 10000)

    // 3. 初始化 SQLite Store（共享同一个 *sql.DB）
    store, err := sqlite.New(cfg.SQLite)
    if err != nil {
        log.Fatal("failed to create sqlite store", "error", err)
    }
    defer store.Close()

    // 4. 初始化对话消息仓库（在同一 Store 上扩展）
    convOps := sqlite.NewConversationOps(store)
    summaryOps := sqlite.NewSummaryOps(store)
    msgEmbedOps := sqlite.NewMessageEmbeddingOps(store, search.NewMessageVectorIndex(cfg.VectorIndex))

    // 5. 初始化 HybridEngine（复用，用于知识记忆检索）
    hybridEngine := search.NewHybridEngine(cfg.Search, store, store, cachedProvider)

    // 6. 初始化 LLM Provider（新增）
    llmProvider, err := llm.NewProvider(cfg.LLM)
    if err != nil {
        log.Fatal("failed to create LLM provider", "error", err)
    }

    // 7. 创建 ContextEngine 应用服务（依赖接口，不依赖具体类型）
    contextEngineSvc := service.NewContextEngineService(
        cfg.ContextEngine,
        convOps,        // ConversationRepository
        summaryOps,     // SummaryRepository
        msgEmbedOps,    // MessageEmbeddingRepository
        hybridEngine,   // SearchRepository
        cachedProvider, // EmbeddingRepository
        llmProvider,    // LLMRepository
    )

    // 8. 启动 gRPC 服务
    lis, err := net.Listen("tcp", *grpcAddr)
    if err != nil {
        log.Fatal("failed to listen", "error", err)
    }

    server := grpcserver.NewServer(cfg.GRPC, contextEngineSvc)
    log.Info("starting ContextEngine gRPC server", "address", *grpcAddr)
    if err := server.Serve(lis); err != nil {
        log.Fatal("failed to serve", "error", err)
    }
}

func loadConfig(configDir string) *Config {
    // 使用 Viper 加载配置，与现有 cmd/memory/cmd/app.go 类似
    // ...
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

---

## 11. OpenClaw 插件桥接

### 11.1 插件结构

在 OpenClaw 项目中创建插件：

```
extensions/
└── mymemory-context-engine/
    ├── openclaw.plugin.json
    ├── package.json
    └── src/
        ├── index.ts           # 插件入口（registerContextEngine）
        ├── grpc-client.ts     # gRPC 客户端封装
        └── context-engine.ts  # ContextEngine 接口实现
```

### 11.2 插件清单

```json
{
  "id": "mymemory-context-engine",
  "kind": "context-engine",
  "name": "MyMemoryGo Context Engine",
  "version": "1.0.0",
  "description": "Context engine backed by MyMemoryGo memory service with hybrid vector+fulltext retrieval and LLM summarization",
  "configSchema": {
    "type": "object",
    "additionalProperties": false,
    "properties": {
      "grpcAddress": {
        "type": "string",
        "default": "localhost:50051",
        "description": "MyMemoryGo gRPC server address"
      },
      "requestTimeout": {
        "type": "number",
        "default": 30000,
        "description": "gRPC request timeout in milliseconds"
      },
      "maxRetries": {
        "type": "number",
        "default": 2,
        "description": "Maximum retry attempts for failed requests"
      },
      "circuitBreakerEnabled": {
        "type": "boolean",
        "default": true,
        "description": "Enable circuit breaker for gRPC calls"
      },
      "fallbackToDefaultEngine": {
        "type": "boolean",
        "default": true,
        "description": "Fallback to default context engine when MyMemoryGo is unavailable"
      }
    }
  }
}
```

### 11.3 插件入口

```typescript
// src/index.ts

import type { PluginApi } from "openclaw/plugin-sdk";
import { MyMemoryGoContextEngine } from "./context-engine.js";

export default function register(api: PluginApi) {
  api.registerContextEngine("mymemory-go", () => {
    const config = api.getConfig() as MyMemoryGoConfig;
    const defaultEngine = api.getDefaultContextEngine?.();
    return new MyMemoryGoContextEngine(config, defaultEngine);
  });
}
```

### 11.4 ContextEngine 实现

```typescript
// src/context-engine.ts

import type {
  ContextEngine,
  ContextEngineInfo,
  AssembleResult,
  CompactResult,
  IngestResult,
  IngestBatchResult,
  BootstrapResult,
  ContextEngineMaintenanceResult,
  ContextEngineRuntimeContext,
  AgentMessage,
} from "openclaw/plugin-sdk";

import { GrpcClient } from "./grpc-client.js";

export interface MyMemoryGoConfig {
  grpcAddress: string;
  requestTimeout: number;
  maxRetries: number;
  circuitBreakerEnabled: boolean;
  fallbackToDefaultEngine: boolean;
}

export class MyMemoryGoContextEngine implements ContextEngine {
  readonly info: ContextEngineInfo = {
    id: "mymemory-go",
    name: "MyMemoryGo Context Engine",
    version: "1.0.0",
    ownsCompaction: true,
  };

  private client: GrpcClient;
  private defaultEngine: ContextEngine | undefined;
  private circuitBreaker: CircuitBreaker | undefined;
  private config: MyMemoryGoConfig;

  constructor(config: MyMemoryGoConfig, defaultEngine?: ContextEngine) {
    this.config = config;
    this.defaultEngine = defaultEngine;
    this.client = new GrpcClient(config.grpcAddress, {
      timeout: config.requestTimeout,
      maxRetries: config.maxRetries,
    });

    if (config.circuitBreakerEnabled) {
      this.circuitBreaker = new CircuitBreaker({
        failureThreshold: 5,
        recoveryTimeout: 30000,
        requestVolumeThreshold: 10,
      });
    }
  }

  // ---- 必需方法 ----

  async ingest(params: {
    sessionId: string;
    sessionKey?: string;
    message: AgentMessage;
    isHeartbeat?: boolean;
  }): Promise<IngestResult> {
    try {
      return await this.runWithFallback(
        () => this.client.ingest(params),
        () => this.defaultEngine!.ingest(params),
      );
    } catch {
      // ingest 失败不影响主流程
      return { ingested: false };
    }
  }

  async assemble(params: {
    sessionId: string;
    sessionKey?: string;
    messages: AgentMessage[];
    tokenBudget?: number;
    availableTools?: Set<string>;
    citationsMode?: string;
    model?: string;
    prompt?: string;
  }): Promise<AssembleResult> {
    return this.runWithFallback(
      () => this.client.assemble(params),
      () => this.defaultEngine!.assemble(params),
    );
  }

  async compact(params: {
    sessionId: string;
    sessionKey?: string;
    sessionFile: string;
    tokenBudget?: number;
    force?: boolean;
    currentTokenCount?: number;
    compactionTarget?: "budget" | "threshold";
    customInstructions?: string;
    runtimeContext?: ContextEngineRuntimeContext;
  }): Promise<CompactResult> {
    return this.runWithFallback(
      () => this.client.compact(params),
      () => this.defaultEngine!.compact(params),
    );
  }

  // ---- 可选方法 ----

  async bootstrap(params: {
    sessionId: string;
    sessionKey?: string;
    sessionFile: string;
  }): Promise<BootstrapResult> {
    return this.runWithFallback(
      () => this.client.bootstrap(params),
      () => ({ bootstrapped: false, reason: "fallback" }),
    );
  }

  async ingestBatch(params: {
    sessionId: string;
    sessionKey?: string;
    messages: AgentMessage[];
    isHeartbeat?: boolean;
  }): Promise<IngestBatchResult> {
    return this.runWithFallback(
      () => this.client.ingestBatch(params),
      () => ({ ingestedCount: 0 }),
    );
  }

  async afterTurn(params: {
    sessionId: string;
    sessionKey?: string;
    sessionFile: string;
    messages: AgentMessage[];
    prePromptMessageCount: number;
    autoCompactionSummary?: string;
    isHeartbeat?: boolean;
    tokenBudget?: number;
    runtimeContext?: ContextEngineRuntimeContext;
  }): Promise<void> {
    try {
      await this.client.afterTurn(params);
    } catch {
      // afterTurn 失败不阻塞
    }
  }

  async maintain(params: {
    sessionId: string;
    sessionKey?: string;
    sessionFile: string;
    runtimeContext?: ContextEngineRuntimeContext;
  }): Promise<ContextEngineMaintenanceResult> {
    return this.runWithFallback(
      () => this.client.maintain(params),
      () => ({ changed: false, bytesFreed: 0, rewrittenEntries: 0 }),
    );
  }

  async onSubagentEnded(params: {
    childSessionKey: string;
    reason: string;
  }): Promise<void> {
    try {
      await this.client.onSubagentEnded(params);
    } catch {
      // 子代理清理失败不阻塞
    }
  }

  async dispose(): Promise<void> {
    this.client.close();
  }

  // ---- 降级保护 ----

  private async runWithFallback<T>(
    primary: () => Promise<T>,
    fallback: () => Promise<T>,
  ): Promise<T> {
    if (!this.config.fallbackToDefaultEngine || !this.defaultEngine) {
      return primary();
    }

    if (this.circuitBreaker?.isOpen()) {
      return fallback();
    }

    try {
      const result = await primary();
      this.circuitBreaker?.recordSuccess();
      return result;
    } catch (e) {
      this.circuitBreaker?.recordFailure();
      console.warn("MyMemoryGo request failed, falling back to default engine", e);
      return fallback();
    }
  }
}
```

---

## 12. 配置

### 12.1 MyMemoryGo 配置扩展

在现有 `configs/config.yaml` 中新增以下配置段：

```yaml
# ContextEngine gRPC 服务配置
context_engine:
  grpc:
    address: "localhost:50051"
    max_receive_message_size: 10485760  # 10MB
    enable_health_check: true
    enable_reflection: true
    metrics:
      enabled: true
      path: "/metrics"
      port: 9090
  memory:
    recent_message_budget_ratio: 0.7   # 近期消息占 token budget 的比例
    summary_trigger_tokens: 3000       # 触发主动压缩的 token 阈值
    max_summary_retries: 3             # 摘要生成重试次数
    fallback_summary: true             # 生成失败时启用降级摘要

# LLM 文本生成配置（用于摘要生成）
llm:
  provider: "openai"
  model: "gpt-4o-mini"
  api_key_env: "OPENAI_API_KEY"        # 从环境变量读取 API Key
  base_url: ""                         # 留空使用默认，或填自定义端点
  max_tokens: 2048
  timeout: 30s
```

### 12.2 OpenClaw 配置

```json5
{
  plugins: {
    slots: {
      contextEngine: "mymemory-go",    // 必须匹配 registerContextEngine 的 id
    },
    entries: {
      "mymemory-go": {
        enabled: true,
        config: {
          grpcAddress: "localhost:50051",
          fallbackToDefaultEngine: true,
        },
      },
    },
  },
}
```

---

## 13. 实现计划

### 阶段一：基础骨架

1. 创建 `proto/context_engine.proto` 并生成 Go 代码
2. 新增 domain 实体（`conversation.go`, `summary.go`）
3. 新增 repository 接口（`conversation_repo.go`, `summary_repo.go`, `message_embedding_repo.go`, `llm_repo.go`）
4. 扩展 `db_setup.go` 新增表的 schema（v3 migration）
5. 实现 SQLite CRUD 操作（`conversation_ops.go`, `summary_ops.go`, `message_embedding_ops.go`）
6. 实现 `MessageVectorIndex`

### 阶段二：核心生命周期

7. 实现 `ContextEngineService`（Ingest + IngestBatch）
8. 实现 Assemble（三层检索 + 合并 + 完整性保证）
9. 实现 Compact（LLM 摘要 + 重试降级 + 标记清理）
10. 实现 AfterTurn + Bootstrap + Maintain + 子代理钩子
11. 实现 LLM Provider 基础设施（`internal/infrastructure/llm/provider.go`）

### 阶段三：gRPC + 插件

12. gRPC Server + Handler 实现（含健康检查、反射、指标）
13. `cmd/context-engine/main.go` 组合根
14. OpenClaw TS 插件桥接（含 CircuitBreaker）
15. 配置扩展（context_engine + llm 配置段）

### 阶段四：优化与增强

16. 对话消息 FTS5 混合检索（向量 + 全文加权合并）
17. Streaming Assemble（gRPC server-streaming）
18. 增量摘要（基于已有摘要扩展而非重新生成）
19. 多会话跨会话记忆
20. 精确 token 计数（集成 tiktoken）

---

## 14. 风险与注意事项

| 风险 | 缓解措施 |
|------|---------|
| gRPC 调用延迟影响 OpenClaw 性能 | 本地部署、连接池、消息批处理、CircuitBreaker 降级 |
| 向量检索结果丢失对话顺序 | 近期消息顺序保留 + 检索结果按时间排序 + `ensureConversationIntegrity` |
| LLM 摘要生成成本和延迟 | 仅压缩远期消息、摘要缓存、降级策略、可配置触发阈值 |
| 服务不可用 | OpenClaw 插件 CircuitBreaker + fallbackToDefaultEngine |
| 对话消息与知识记忆索引混淆 | 使用独立 HNSW 索引（MessageVectorIndex），按 sessionID 隔离 |
| Compact 后检索质量下降 | 历史摘要作为 system 消息注入，保留上下文连续性 |
| v2→v3 schema migration | 使用 metadata 表 `__schema_version` 追踪，渐进式迁移 |

---

## 15. 未来扩展

- **流式 assemble**：支持 gRPC server-streaming 返回消息，降低首 token 延迟
- **多会话管理**：跨会话记忆检索（将其他会话的摘要/知识注入当前上下文）
- **增量摘要**：基于已有摘要 + 新消息进行增量更新，减少 LLM 调用
- **自定义检索策略**：支持配置不同的检索/压缩策略（纯向量、纯全文、混合权重可调）
- **工具调用感知**：Assemble 时根据 `availableTools` 调整知识记忆检索策略
- **identifierPolicy**：Compact 时支持 OpenClaw 的标识符保留策略（strict/off/custom）
