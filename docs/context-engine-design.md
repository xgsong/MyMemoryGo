# MyMemoryGo ContextEngine 适配方案

> 文档版本：v1.0.0
> 日期：2026-04-08
> 状态：草稿

## 1. 背景与目标

本文档描述如何将 MyMemoryGo 项目适配为 OpenClaw 的 ContextEngine 实现，使 MyMemoryGo 能够作为 OpenClaw 的可插拔上下文管理模块。

### 1.1 集成目标

| 目标 | 描述 |
|------|------|
| **完整实现 ContextEngine 接口** | MyMemoryGo 实现 OpenClaw `ContextEngine` 接口的所有必需方法 |
| **处理 OpenClaw 会话消息** | 直接接收和处理 OpenClaw 的 `AgentMessage`，存储到 MyMemoryGo 的 SQLite |
| **混合压缩策略** | 近期消息顺序保留，远期消息通过向量检索或摘要压缩 |
| **深度复用现有能力** | 复用 `embedding provider` 和 `hybrid search` 模块 |
| **独立服务部署** | MyMemoryGo 作为独立 gRPC 服务运行，OpenClaw 通过插件桥接调用 |

### 1.2 OpenClaw ContextEngine 简介

ContextEngine 是 OpenClaw 中控制模型上下文构建的可插拔模块，参与四个生命周期节点：

1. **Ingest** — 新消息添加到会话时调用
2. **Assemble** — 每次模型运行前调用，返回在 token 预算内的有序消息
3. **Compact** — 上下文窗口满或用户执行 `/compact` 时调用
4. **AfterTurn** — 运行完成后调用

详见：[OpenClaw ContextEngine 文档](file:///home/xgsong/Projects/openclaw/docs/concepts/context-engine.md)

---

## 2. 整体架构

### 2.1 架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         OpenClaw                                 │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │              ContextEngine Plugin (JS/TS)                   │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │ │
│  │  │   ingest()   │  │  assemble()  │  │   compact()  │     │ │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │ │
│  └─────────┼─────────────────┼─────────────────┼─────────────┘ │
└────────────┼─────────────────┼─────────────────┼───────────────┘
             │                 │                 │
             │ gRPC            │ gRPC            │ gRPC
             ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    MyMemoryGo (Go Service)                       │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   ContextEngine gRPC Server                  │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │ │
│  │  │  IngestMsg   │  │  AssembleCtx  │  │  CompactCtx   │      │ │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │ │
│  └─────────┼─────────────────┼─────────────────┼──────────────┘ │
│  ┌─────────┼─────────────────┼─────────────────┼──────────────┐ │
│  │         ▼                 ▼                 ▼              │ │
│  │  ┌─────────────────────────────────────────────────────┐    │ │
│  │  │              ContextEngine Domain                    │    │ │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │    │ │
│  │  │  │Conversation │  │  Vector      │  │  Summary  │  │    │ │
│  │  │  │  Memory     │  │  Index      │  │  Cache    │  │    │ │
│  │  │  └─────────────┘  └─────────────┘  └───────────┘  │    │ │
│  │  └─────────────────────────────────────────────────────┘    │ │
│  └────────────────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │           Shared Infrastructure                             │ │
│  │  ┌──────────────────┐  ┌────────────────────────────────┐ │ │
│  │  │  Embedding       │  │  SQLite (Persistence)            │ │ │
│  │  │  Provider       │  │  - conversation_messages         │ │ │
│  │  │  (复用现有)       │  │  - message_embeddings           │ │ │
│  │  └──────────────────┘  └────────────────────────────────┘ │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流

1. **Ingest 流程**：
   ```
   OpenClaw → ingest(sessionId, message) → gRPC → MyMemoryGo
   → 存储 ConversationMessage → 生成 Embedding → 写入向量索引
   ```

2. **Assemble 流程**：
   ```
   OpenClaw → assemble(sessionId, messages, tokenBudget, prompt) → gRPC → MyMemoryGo
   → 近期消息顺序保留 → 远期消息向量检索 → 重组 → 返回 AgentMessage[]
   ```

3. **Compact 流程**：
   ```
   OpenClaw → compact(sessionId, tokenBudget, force) → gRPC → MyMemoryGo
   → 对远期消息生成摘要 或 基于检索重组 → 更新存储
   ```

---

## 3. gRPC 接口定义

### 3.1 Proto 文件

新建文件：`proto/context_engine.proto`

```protobuf
syntax = "proto3";

package contextengine;

option go_package = "github.com/mymemorygo/memory/proto";

// AgentMessage 对应 OpenClaw 的 AgentMessage
message AgentMessage {
  string role = 1;           // "user" | "assistant" | "system" | "tool" | "tool_result"
  string content = 2;
  int64 timestamp = 3;
  string id = 4;
  ToolCall tool_call = 5;
  string tool_name = 6;
  string tool_use_id = 7;
}

message ToolCall {
  string id = 1;
  string name = 2;
  string arguments = 3;      // JSON string
}

// Assemble 返回结果
message AssembleResult {
  repeated AgentMessage messages = 1;
  int32 estimated_tokens = 2;
  string system_prompt_addition = 3;
}

// Compact 返回结果
message CompactResult {
  bool ok = 1;
  bool compacted = 2;
  string reason = 3;
  CompactDetails result = 4;
}

message CompactDetails {
  string summary = 1;
  string first_kept_entry_id = 2;
  int32 tokens_before = 3;
  int32 tokens_after = 4;
}

message IngestResult {
  bool ingested = 1;
}

message IngestBatchResult {
  int32 ingested_count = 1;
}

message BootstrapResult {
  bool bootstrapped = 1;
  int32 imported_messages = 2;
  string reason = 3;
}

message MaintenanceResult {
  bool changed = 1;
  int64 bytes_freed = 2;
  int32 rewritten_entries = 3;
  string reason = 4;
}

message Empty {}

// ContextEngine 服务
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

  // 元数据
  rpc GetInfo(Empty) returns (EngineInfo);
}

// 请求消息
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

message EngineInfo {
  string id = 1;
  string name = 2;
  string version = 3;
  bool owns_compaction = 4;
}
```

---

## 4. 领域模型设计

### 4.1 扩展现有 Memory Entity

改造 `internal/domain/entity/memory.go`，新增 `ConversationMessage` 类型：

```go
// internal/domain/entity/memory.go

package entity

// MemoryType 扩展支持对话消息类型
type MemoryType string

const (
    MemoryTypeKnowledge MemoryType = "knowledge"  // 现有知识记忆
    MemoryTypeConversation MemoryType = "conversation"  // 新增：对话消息
)

// ConversationMessage 对话消息实体
type ConversationMessage struct {
    ID          string    `json:"id"`
    SessionID   string    `json:"session_id"`
    SessionKey  string    `json:"session_key"`
    Role        string    `json:"role"`           // user/assistant/system/tool/tool_result
    Content     string    `json:"content"`
    Timestamp   int64     `json:"timestamp"`
    ToolName    string    `json:"tool_name,omitempty"`
    ToolCallID  string    `json:"tool_call_id,omitempty"`
    ToolArgs    string    `json:"tool_args,omitempty"`   // JSON string
    MessageHash string    `json:"message_hash"`          // 去重用
    IsHeartbeat bool      `json:"is_heartbeat,omitempty"`
    CreatedAt   int64     `json:"created_at"`
}

// Memory 扩展后支持对话消息
type Memory struct {
    ID        string     `json:"id"`
    Type      MemoryType `json:"type"`
    Content   string     `json:"content"`
    Metadata  string     `json:"metadata"`  // JSON, 存储对话消息的额外信息

    // 向量相关（复用现有）
    Vector    []float32  `json:"vector,omitempty"`
    Provider  string     `json:"provider,omitempty"`
}
```

### 4.2 Summary Cache 实体

```go
// internal/domain/entity/summary.go (新文件)

package entity

// ConversationSummary 对话摘要
type ConversationSummary struct {
    ID            string   `json:"id"`
    SessionID     string   `json:"session_id"`
    SessionKey    string   `json:"session_key"`
    StartMsgID    string   `json:"start_msg_id"`     // 摘要的起始消息ID
    EndMsgID      string   `json:"end_msg_id"`       // 摘要的结束消息ID
    SummaryText   string   `json:"summary_text"`
    TokenCount    int      `json:"token_count"`
    CreatedAt     int64    `json:"created_at"`
    Version       int      `json:"version"`          // 摘要版本，用于更新
}
```

---

## 5. 数据库设计

### 5.1 新增表

```sql
-- conversation_messages 表：存储会话消息
CREATE TABLE IF NOT EXISTS conversation_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    session_key TEXT,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    tool_name TEXT,
    tool_call_id TEXT,
    tool_args TEXT,
    message_hash TEXT UNIQUE NOT NULL,  -- 用于去重
    is_heartbeat INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,

    INDEX idx_session_id (session_id),
    INDEX idx_session_key (session_key),
    INDEX idx_timestamp (timestamp),
    INDEX idx_role (role)
);

-- message_embeddings 表：消息向量索引
CREATE TABLE IF NOT EXISTS message_embeddings (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    vector BLOB NOT NULL,  -- 序列化的向量
    dimension INTEGER NOT NULL,
    created_at INTEGER NOT NULL,

    FOREIGN KEY (message_id) REFERENCES conversation_messages(id) ON DELETE CASCADE,
    INDEX idx_session_id (session_id),
    INDEX idx_message_id (message_id)
);

-- conversation_summaries 表：对话摘要缓存
CREATE TABLE IF NOT EXISTS conversation_summaries (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    session_key TEXT,
    start_msg_id TEXT NOT NULL,
    end_msg_id TEXT NOT NULL,
    summary_text TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    version INTEGER DEFAULT 1,
    created_at INTEGER NOT NULL,

    INDEX idx_session_id (session_id),
    INDEX idx_start_msg_id (start_msg_id),
    INDEX idx_end_msg_id (end_msg_id),
    UNIQUE INDEX idx_session_range (session_id, start_msg_id, end_msg_id)
);
```

---

## 6. 项目结构

### 6.1 新增目录

```
MyMemoryGo/
├── proto/                          # 新增：gRPC proto 文件
│   ├── context_engine.proto
│   └── gen/                       # 生成的 Go 代码
│       ├── context_engine.pb.go
│       └── context_engine_grpc.pb.go
│
├── cmd/
│   ├── memory/
│   │   └── main.go
│   └── context-engine/            # 新增：ContextEngine 服务入口
│       └── main.go
│
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── memory.go          # 扩展：支持 ConversationMessage
│   │   │   └── summary.go         # 新增：ConversationSummary
│   │   └── repository/
│   │       ├── memory_repository.go
│   │       └── summary_repository.go  # 新增
│   │
│   ├── application/
│   │   └── service/
│   │       ├── context_engine.go  # 新增：ContextEngine 应用服务
│   │       └── search.go
│   │
│   ├── infrastructure/
│   │   ├── persistence/
│   │   │   └── sqlite/
│   │   │       ├── conversation_ops.go  # 新增：对话消息 CRUD
│   │   │       └── summary_ops.go        # 新增：摘要 CRUD
│   │   │
│   │   ├── embedding/
│   │   │   └── provider.go        # 复用现有：向量生成接口
│   │   └── llm/
│   │       └── provider.go        # 新增：LLM文本生成接口
│   │
│   └── interface/
│       ├── grpc/                  # 新增：gRPC 服务端
│       │   ├── server.go
│       │   └── context_engine_service.go
│       └── api/
│           └── handlers.go
│
└── docs/
    └── context-engine-design.md   # 本文档
```

---

## 7. 核心实现

### 7.1 ContextEngine 应用服务

```go
// internal/application/service/context_engine.go

package service

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "math"
    "strings"
    "time"

    "github.com/mymemorygo/memory/internal/config"
    "github.com/mymemorygo/memory/internal/domain/entity"
    "github.com/mymemorygo/memory/internal/infrastructure/embedding"
    "github.com/mymemorygo/memory/internal/infrastructure/llm"
    "github.com/mymemorygo/memory/internal/infrastructure/persistence/sqlite"
)

type ContextEngineService struct {
    config        *config.Config
    memoryRepo    *sqlite.MemoryRepository
    summaryRepo   *sqlite.SummaryRepository
    embedProvider embedding.Provider  // 专门用于向量生成
    llmProvider   llm.Provider        // 专门用于LLM文本生成(摘要等)
}

func NewContextEngineService(
    config *config.Config,
    memoryRepo *sqlite.MemoryRepository,
    summaryRepo *sqlite.SummaryRepository,
    embedProvider embedding.Provider,
    llmProvider llm.Provider,
) *ContextEngineService {
    return &ContextEngineService{
        config:        config,
        memoryRepo:    memoryRepo,
        summaryRepo:   summaryRepo,
        embedProvider: embedProvider,
        llmProvider:   llmProvider,
    }
}

// Ingest 处理单条消息
func (s *ContextEngineService) Ingest(ctx context.Context, req *IngestRequest) (*IngestResult, error) {
    msg := req.Message

    // 生成消息哈希用于去重
    hash := s.hashMessage(msg)
    if s.memoryRepo.ExistsByHash(ctx, hash) {
        return &IngestResult{Ingested: false}, nil
    }

    // 存储对话消息
    convMsg := &entity.ConversationMessage{
        ID:          msg.Id,
        SessionID:   req.SessionId,
        SessionKey:  req.SessionKey,
        Role:        msg.Role,
        Content:     msg.Content,
        Timestamp:   msg.Timestamp,
        ToolName:    msg.ToolName,
        ToolCallID:  msg.ToolCallId,
        ToolArgs:    msg.ToolArgs,
        MessageHash: hash,
        IsHeartbeat: req.IsHeartbeat,
        CreatedAt:   timestamp(),
    }

    if err := s.memoryRepo.SaveConversationMessage(ctx, convMsg); err != nil {
        return nil, err
    }

    // 生成并存储 embedding
    if err := s.generateAndStoreEmbedding(ctx, convMsg); err != nil {
        // embedding 失败不影响 ingest 成功
        log.Warn("failed to generate embedding", "error", err)
    }

    return &IngestResult{Ingested: true}, nil
}

// IngestBatch 批量处理消息
func (s *ContextEngineService) IngestBatch(ctx context.Context, req *IngestBatchRequest) (*IngestBatchResult, error) {
    // 1. 先批量去重
    validMsgs := make([]*entity.ConversationMessage, 0, len(req.Messages))
    hashes := make([]string, 0, len(req.Messages))
    
    for _, msg := range req.Messages {
        hash := s.hashMessage(msg)
        hashes = append(hashes, hash)
        
        convMsg := &entity.ConversationMessage{
            ID:          msg.Id,
            SessionID:   req.SessionId,
            SessionKey:  req.SessionKey,
            Role:        msg.Role,
            Content:     msg.Content,
            Timestamp:   msg.Timestamp,
            ToolName:    msg.ToolName,
            ToolCallID:  msg.ToolCallId,
            ToolArgs:    msg.ToolArgs,
            MessageHash: hash,
            IsHeartbeat: req.IsHeartbeat,
            CreatedAt:   timestamp(),
        }
        validMsgs = append(validMsgs, convMsg)
    }
    
    // 2. 批量查询已存在的hash
    existingHashes, err := s.memoryRepo.ExistsByHashes(ctx, hashes)
    if err != nil {
        return nil, err
    }
    
    // 3. 过滤出需要插入的新消息
    newMsgs := make([]*entity.ConversationMessage, 0)
    for i, msg := range validMsgs {
        if !existingHashes[msg.MessageHash] {
            newMsgs = append(newMsgs, msg)
        }
    }
    
    if len(newMsgs) == 0 {
        return &IngestBatchResult{IngestedCount: 0}, nil
    }
    
    // 4. 批量插入消息
    if err := s.memoryRepo.SaveConversationMessages(ctx, newMsgs); err != nil {
        return nil, err
    }
    
    // 5. 批量生成embedding
    texts := make([]string, 0, len(newMsgs))
    for _, msg := range newMsgs {
        texts = append(texts, formatMessageForEmbedding(msg))
    }
    
    vectors, err := s.embedProvider.EmbedBatch(ctx, texts)
    if err != nil {
        log.Warn("failed to batch generate embeddings", "error", err)
        return &IngestBatchResult{IngestedCount: int32(len(newMsgs))}, nil
    }
    
    // 6. 批量存储embedding
    embeddings := make([]*entity.MessageEmbedding, 0, len(newMsgs))
    for i, msg := range newMsgs {
        if i >= len(vectors) {
            break
        }
        emb := &entity.MessageEmbedding{
            ID:        generateID(),
            MessageID: msg.ID,
            SessionID: msg.SessionID,
            Provider:  s.embedProvider.Name(),
            Vector:    vectors[i],
            Dimension: len(vectors[i]),
            CreatedAt: timestamp(),
        }
        embeddings = append(embeddings, emb)
    }
    
    if err := s.memoryRepo.SaveEmbeddings(ctx, embeddings); err != nil {
        log.Warn("failed to batch save embeddings", "error", err)
    }
    
    return &IngestBatchResult{IngestedCount: int32(len(newMsgs))}, nil
}

// Assemble 组装上下文
func (s *ContextEngineService) Assemble(ctx context.Context, req *AssembleRequest) (*AssembleResult, error) {
    // 1. 获取近期消息（顺序保留）
    recentMessages, recentTokens := s.getRecentMessages(ctx, req.SessionId, req.TokenBudget)

    // 2. 估算远期消息需要的 token
    budgetForOld := req.TokenBudget - recentTokens
    if budgetForOld < 0 {
        budgetForOld = 0
    }

    // 3. 远期消息通过向量检索获取
    retrievedMessages, retrievedTokens, err := s.retrieveOldMessages(
        ctx, req.SessionId, req.Prompt, budgetForOld,
    )
    if err != nil {
        return nil, err
    }

    // 4. 合并消息
    finalMessages := s.mergeMessages(recentMessages, retrievedMessages, req.TokenBudget)

    // 5. 估算总 token
    totalTokens := recentTokens + retrievedTokens

    return &AssembleResult{
        Messages:            finalMessages,
        EstimatedTokens:     int32(totalTokens),
        SystemPromptAddition: s.buildMemoryPromptAddition(req.AvailableTools, req.CitationsMode),
    }, nil
}

// Compact 压缩上下文
func (s *ContextEngineService) Compact(ctx context.Context, req *CompactRequest) (*CompactResult, error) {
    // 1. 获取需要压缩的消息范围
    oldMessages, err := s.getOldMessagesForCompaction(ctx, req.SessionId)
    if err != nil {
        return &CompactResult{Ok: false, Compacted: false, Reason: err.Error()}, nil
    }

    if len(oldMessages) == 0 {
        return &CompactResult{Ok: true, Compacted: false, Reason: "no messages to compact"}, nil
    }

    // 2. 生成摘要（复用 embedding provider 调用 LLM）
    summaryText, tokenCount, err := s.generateSummary(ctx, oldMessages, req.CustomInstructions)
    if err != nil {
        return &CompactResult{Ok: false, Compacted: false, Reason: err.Error()}, nil
    }

    // 3. 保存摘要
    summary := &entity.ConversationSummary{
        ID:          generateID(),
        SessionID:   req.SessionId,
        SessionKey:  req.SessionKey,
        StartMsgID:  oldMessages[0].ID,
        EndMsgID:    oldMessages[len(oldMessages)-1].ID,
        SummaryText: summaryText,
        TokenCount:  tokenCount,
        CreatedAt:   timestamp(),
    }
    if err := s.summaryRepo.Save(ctx, summary); err != nil {
        return nil, err
    }

    // 4. 标记旧消息为已压缩
    if err := s.markMessagesCompacted(ctx, oldMessages); err != nil {
        log.Warn("failed to mark messages compacted", "error", err)
    }

    // 5. 删除旧消息的 embedding
    if err := s.deleteOldEmbeddings(ctx, oldMessages); err != nil {
        log.Warn("failed to delete old embeddings", "error", err)
    }

    firstKeptID := ""
    if len(oldMessages) > 0 {
        firstKeptID = oldMessages[len(oldMessages)-1].ID
    }

    return &CompactResult{
        Ok:        true,
        Compacted: true,
        Result: &CompactDetails{
            Summary:          summaryText,
            FirstKeptEntryId: firstKeptID,
            TokensBefore:     s.estimateTokens(oldMessages),
            TokensAfter:      tokenCount,
        },
    }, nil
}

// --- 私有辅助方法 ---

func (s *ContextEngineService) hashMessage(msg *AgentMessage) string {
    data := fmt.Sprintf("%s:%s:%d:%s:%s", msg.Role, msg.Content, msg.Timestamp, msg.ToolName, msg.ToolCallId)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

func (s *ContextEngineService) generateAndStoreEmbedding(ctx context.Context, msg *entity.ConversationMessage) error {
    // 构建用于 embedding 的文本
    text := formatMessageForEmbedding(msg)

    vec, err := s.embedProvider.Embed(ctx, text)
    if err != nil {
        return err
    }

    emb := &entity.MessageEmbedding{
        ID:        generateID(),
        MessageID: msg.ID,
        SessionID: msg.SessionID,
        Provider:  s.embedProvider.Name(),
        Vector:    vec,
        Dimension: len(vec),
        CreatedAt: timestamp(),
    }

    return s.memoryRepo.SaveEmbedding(ctx, emb)
}

func (s *ContextEngineService) getRecentMessages(ctx context.Context, sessionID string, budget int) ([]*AgentMessage, int) {
    messages, err := s.memoryRepo.GetRecentConversationMessages(ctx, sessionID, budget)
    if err != nil {
        log.Warn("failed to get recent messages", "error", err)
        return nil, 0
    }

    return s.convertToAgentMessages(messages), s.estimateTokens(messages)
}

func (s *ContextEngineService) retrieveOldMessages(ctx context.Context, sessionID, query string, budget int) ([]*AgentMessage, int, error) {
    // 使用混合搜索检索相关消息
    results, err := s.searchMessages(ctx, sessionID, query, budget)
    if err != nil {
        return nil, 0, err
    }

    return s.convertToAgentMessages(results), s.estimateTokens(results), nil
}

func (s *ContextEngineService) mergeMessages(recent, retrieved []*AgentMessage, budget int) []*AgentMessage {
    // 1. 先确保近期消息的对话完整性（user-assistant配对）
    validRecent := s.ensureConversationIntegrity(recent)
    recentTokens := s.estimateTokens(validRecent)
    
    // 2. 剩余预算用于检索消息
    remainingBudget := budget - recentTokens
    if remainingBudget <= 0 {
        return validRecent[:s.truncateToTokenBudget(validRecent, budget)]
    }
    
    // 3. 对检索到的消息按时间排序，确保对话时序
    sortedRetrieved := s.sortMessagesByTimestamp(retrieved)
    
    // 4. 确保检索消息的对话完整性，过滤零散的单条消息
    validRetrieved := s.ensureConversationIntegrity(sortedRetrieved)
    
    // 5. 按预算选择检索消息，优先保留完整的对话对
    selectedRetrieved := s.selectMessagesByBudget(validRetrieved, remainingBudget)
    
    // 6. 合并消息：历史检索消息在前，近期消息在后，整体按时间排序
    merged := append(selectedRetrieved, validRecent...)
    merged = s.sortMessagesByTimestamp(merged)
    
    // 7. 最后再校验一次token预算
    final := s.truncateToTokenBudget(merged, budget)
    
    return final
}

// ensureConversationIntegrity 确保对话完整性，保留完整的user-assistant配对
func (s *ContextEngineService) ensureConversationIntegrity(messages []*AgentMessage) []*AgentMessage {
    if len(messages) == 0 {
        return nil
    }
    
    result := make([]*AgentMessage, 0, len(messages))
    var lastUserMsg *AgentMessage
    
    for _, msg := range messages {
        switch msg.Role {
        case "user":
            // 如果有未配对的user消息，说明中间丢失了assistant回复，丢弃之前的
            if lastUserMsg != nil {
                lastUserMsg = nil
            }
            lastUserMsg = msg
        case "assistant", "tool_result":
            // 只有当前面有配对的user消息时才保留
            if lastUserMsg != nil {
                result = append(result, lastUserMsg)
                result = append(result, msg)
                lastUserMsg = nil
            }
        case "system":
            // system消息可以单独保留
            result = append(result, msg)
        }
    }
    
    return result
}

func (s *ContextEngineService) generateSummary(ctx context.Context, messages []*entity.ConversationMessage, instructions string) (string, int, error) {
    prompt := s.buildSummaryPrompt(messages, instructions)
    maxRetries := s.config.LLM.MaxRetries
    var lastErr error
    
    // 重试机制
    for i := 0; i < maxRetries; i++ {
        select {
        case <-ctx.Done():
            return "", 0, ctx.Err()
        default:
            response, err := s.llmProvider.Generate(ctx, prompt)
            if err == nil {
                return response.Text, response.Tokens, nil
            }
            lastErr = err
            log.Warn("summary generation failed, retrying", "attempt", i+1, "error", err)
            
            // 指数退避
            if i < maxRetries-1 {
                time.Sleep(time.Duration(math.Pow(2, float64(i))) * time.Second)
            }
        }
    }
    
    // 重试全部失败，启用降级策略
    if s.config.LLM.FallbackSummary {
        log.Warn("all retries failed, using fallback summary", "error", lastErr)
        return s.generateFallbackSummary(messages), s.estimateTokens(messages) / 10, nil
    }
    
    return "", 0, lastErr
}

// generateFallbackSummary 生成降级摘要，无需调用LLM
func (s *ContextEngineService) generateFallbackSummary(messages []*entity.ConversationMessage) string {
    var buf bytes.Buffer
    buf.WriteString(fmt.Sprintf("对话摘要（共%d条消息，时间范围：%s ~ %s）\n", 
        len(messages),
        time.Unix(messages[0].Timestamp, 0).Format(time.RFC3339),
        time.Unix(messages[len(messages)-1].Timestamp, 0).Format(time.RFC3339),
    ))
    
    // 只保留关键信息点
    userCount := 0
    assistantCount := 0
    toolCount := 0
    for _, msg := range messages {
        switch msg.Role {
        case "user":
            userCount++
        case "assistant":
            assistantCount++
        case "tool_result", "tool":
            toolCount++
        }
    }
    
    buf.WriteString(fmt.Sprintf("用户消息：%d条，助手回复：%d条，工具调用：%d次\n", userCount, assistantCount, toolCount))
    
    // 提取前3条和最后3条消息的前200字符作为预览
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

func (s *ContextEngineService) buildMemoryPromptAddition(tools []string, citationsMode string) string {
    // 类似 OpenClaw 的 buildMemorySystemPromptAddition
    var lines []string
    lines = append(lines, "## Memory Recall")
    lines = append(lines, fmt.Sprintf("citations=%s", citationsMode))
    // 添加可用工具相关的记忆提示
    return strings.Join(lines, "\n")
}
```

### 7.2 gRPC 服务端

```go
// internal/interface/grpc/server.go

package grpc

import (
    "net"

    "google.golang.org/grpc"

    pb "github.com/mymemorygo/memory/proto/gen"
    "github.com/mymemorygo/memory/internal/application/service"
)

type Server struct {
    config        *config.Config
    grpcServer    *grpc.Server
    service       *service.ContextEngineService
    metricsServer *http.Server
}

func NewServer(cfg *config.Config, svc *service.ContextEngineService) *Server {
    opts := []grpc.ServerOption{
        grpc.MaxRecvMsgSize(cfg.ContextEngine.GRPC.MaxReceiveMessageSize),
    }
    
    grpcServer := grpc.NewServer(opts...)
    
    // 注册健康检查服务
    if cfg.ContextEngine.GRPC.EnableHealthCheck {
        healthpb.RegisterHealthServer(grpcServer, health.NewServer())
    }
    
    // 启用反射
    if cfg.ContextEngine.GRPC.EnableReflection {
        reflection.Register(grpcServer)
    }
    
    pb.RegisterContextEngineServiceServer(grpcServer, &contextEngineServer{svc: svc})
    
    return &Server{
        config:     cfg,
        grpcServer: grpcServer,
        service:    svc,
    }
}

func (s *Server) Serve(lis net.Listener) error {
    // 启动监控服务
    if s.config.ContextEngine.GRPC.Metrics.Enabled {
        go s.startMetricsServer()
    }
    return s.grpcServer.Serve(lis)
}

func (s *Server) GracefulShutdown() {
    s.grpcServer.GracefulStop()
    // 关闭监控服务
    if s.metricsServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        s.metricsServer.Shutdown(ctx)
    }
}

// startMetricsServer 启动Prometheus监控服务
func (s *Server) startMetricsServer() {
    mux := http.NewServeMux()
    mux.Handle(s.config.ContextEngine.GRPC.Metrics.Path, promhttp.Handler())
    
    s.metricsServer = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.config.ContextEngine.GRPC.Metrics.Port),
        Handler: mux,
    }
    
    log.Info("starting metrics server", "port", s.config.ContextEngine.GRPC.Metrics.Port)
    if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Error("metrics server failed", "error", err)
    }
}

// --- Service Implementation ---

type contextEngineServer struct {
    pb.UnimplementedContextEngineServiceServer
    svc *service.ContextEngineService
}

func (s *contextEngineServer) Ingest(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResult, error) {
    return s.svc.Ingest(ctx, req)
}

func (s *contextEngineServer) Assemble(ctx context.Context, req *pb.AssembleRequest) (*pb.AssembleResult, error) {
    return s.svc.Assemble(ctx, req)
}

func (s *contextEngineServer) Compact(ctx context.Context, req *pb.CompactRequest) (*pb.CompactResult, error) {
    return s.svc.Compact(ctx, req)
}

// ... 其他方法实现
```

### 7.3 服务入口

```go
// cmd/context-engine/main.go

package main

import (
    "flag"
    "log"
    "net"

    "github.com/mymemorygo/memory/internal/application/service"
    "github.com/mymemorygo/memory/internal/infrastructure/embedding"
    "github.com/mymemorygo/memory/internal/infrastructure/persistence/sqlite"
    "github.com/mymemorygo/memory/internal/interface/grpc"
)

var (
    grpcAddr = flag.String("grpc-addr", "localhost:50051", "gRPC server address")
)

func main() {
    flag.Parse()

    // 初始化依赖
    db, err := sqlite.Open("data/memory.db")
    if err != nil {
        log.Fatal("failed to open database", "error", err)
    }
    defer db.Close()

    memoryRepo := sqlite.NewMemoryRepository(db)
    summaryRepo := sqlite.NewSummaryRepository(db)

    embedProvider, err := embedding.NewProvider(embedding.Config{
        Provider: "openai",
        APIKey:   getEnv("OPENAI_API_KEY", ""),
    })
    if err != nil {
        log.Fatal("failed to create embedding provider", "error", err)
    }

    // 加载配置
    cfg, err := config.Load()
    if err != nil {
        log.Fatal("failed to load config", "error", err)
    }
    
    // 初始化LLM provider
    llmProvider, err := llm.NewProvider(cfg.LLM)
    if err != nil {
        log.Fatal("failed to create LLM provider", "error", err)
    }
    
    // 创建 ContextEngine 服务
    contextEngineSvc := service.NewContextEngineService(
        cfg,
        memoryRepo,
        summaryRepo,
        embedProvider,
        llmProvider,
    )

    // 启动 gRPC 服务
    lis, err := net.Listen("tcp", *grpcAddr)
    if err != nil {
        log.Fatal("failed to listen", "error", err)
    }

    server := grpc.NewServer(cfg, contextEngineSvc)
    log.Info("starting ContextEngine gRPC server", "address", *grpcAddr)
    if err := server.Serve(lis); err != nil {
        log.Fatal("failed to serve", "error", err)
    }
}
```

---

## 8. OpenClaw 插件桥接

### 8.1 插件结构

在 OpenClaw 项目中创建插件：

```
extensions/
└── mymemory-context-engine/
    ├── openclaw.plugin.json
    ├── package.json
    └── src/
        ├── index.ts           # 插件入口
        ├── grpc-client.ts     # gRPC 客户端封装
        └── context-engine.ts  # ContextEngine 接口实现
```

### 8.2 openclaw.plugin.json

```json
{
  "id": "mymemory-context-engine",
  "name": "MyMemoryGo Context Engine",
  "version": "1.0.0",
  "kind": "context-engine",
  "description": "Context engine backed by MyMemoryGo memory service",
  "entry": "dist/index.js",
  "runtime": "node",
  "config": {
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
      "description": "Fallback to default context engine when service is unavailable"
    }
  }
}
```

### 8.3 ContextEngine 实现

```typescript
// src/context-engine.ts

import type {
  ContextEngine,
  ContextEngineInfo,
  AssembleResult,
  CompactResult,
  IngestResult,
  ContextEngineRuntimeContext,
  AgentMessage,
} from "../../src/context-engine/types.js";

import { GrpcClient } from "./grpc-client.js";

export class MyMemoryGoContextEngine implements ContextEngine {
  readonly info: ContextEngineInfo = {
    id: "mymemory-go",
    name: "MyMemoryGo Context Engine",
    version: "1.0.0",
    ownsCompaction: true,
  };

  private client: GrpcClient;
  private defaultEngine: ContextEngine;
  private circuitBreaker: CircuitBreaker;
  private config: MyMemoryGoConfig;

  constructor(config: MyMemoryGoConfig, defaultEngine: ContextEngine) {
    this.config = config;
    this.defaultEngine = defaultEngine;
    this.client = new GrpcClient(config.grpcAddress, {
      timeout: config.requestTimeout,
      maxRetries: config.maxRetries,
    });
    
    // 初始化熔断器
    if (config.circuitBreakerEnabled) {
      this.circuitBreaker = new CircuitBreaker({
        failureThreshold: 5,
        recoveryTimeout: 30000,
        requestVolumeThreshold: 10,
      });
    }
  }

  async ingest(params: {
    sessionId: string;
    sessionKey?: string;
    message: AgentMessage;
    isHeartbeat?: boolean;
  }): Promise<IngestResult> {
    try {
      return await this.runWithFallback(
        () => this.client.ingest(params),
        () => this.defaultEngine.ingest(params)
      );
    } catch (e) {
      // 降级策略：ingest失败不影响主流程，记录日志即可
      console.warn("Ingest to MyMemoryGo failed, falling back", e);
      return { ingested: false };
    }
  }

  async assemble(params: {
    sessionId: string;
    sessionKey?: string;
    messages: AgentMessage[];
    tokenBudget?: number;
    availableTools?: Set<string>;
    citationsMode?: MemoryCitationsMode;
    model?: string;
    prompt?: string;
  }): Promise<AssembleResult> {
    return this.client.assemble(params);
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
    return this.client.compact(params);
  }

  async assemble(params: {
    sessionId: string;
    sessionKey?: string;
    messages: AgentMessage[];
    tokenBudget?: number;
    availableTools?: Set<string>;
    citationsMode?: MemoryCitationsMode;
    model?: string;
    prompt?: string;
  }): Promise<AssembleResult> {
    return this.runWithFallback(
      () => this.client.assemble(params),
      () => this.defaultEngine.assemble(params)
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
      () => this.defaultEngine.compact(params)
    );
  }

  async dispose(): Promise<void> {
    this.client.close();
  }

  /**
   * 带降级保护的执行方法
   */
  private async runWithFallback<T>(
    primary: () => Promise<T>,
    fallback: () => Promise<T>
  ): Promise<T> {
    if (!this.config.fallbackToDefaultEngine || !this.circuitBreaker) {
      return primary();
    }

    if (this.circuitBreaker.isOpen()) {
      console.warn("Circuit breaker is open, falling back to default engine");
      return fallback();
    }

    try {
      const result = await primary();
      this.circuitBreaker.recordSuccess();
      return result;
    } catch (e) {
      this.circuitBreaker.recordFailure();
      console.warn("Request failed, falling back to default engine", e);
      return fallback();
    }
  }
}
```

---

## 9. 配置

### 9.1 MyMemoryGo 配置

```yaml
# configs/config.yaml

context_engine:
  grpc:
    address: "localhost:50051"
    max_receive_message_size: 10485760  # 10MB
    enable_health_check: true  # 启用健康检查
    enable_reflection: true  # 启用gRPC反射
    metrics:
      enabled: true  # 启用Prometheus监控
      path: "/metrics"  # 监控指标路径
      port: 9090  # 监控端口

embedding:
  provider: "openai"  # 或其他支持的 provider
  model: "text-embedding-3-small"
  dimension: 1536

llm:
  provider: "openai"  # 或其他支持的 provider
  model: "gpt-3.5-turbo"  # 摘要生成模型
  max_retries: 3  # 生成失败重试次数
  timeout: 30s  # 接口超时时间
  fallback_summary: true  # 生成失败时启用降级摘要

memory:
  recent_message_budget_ratio: 0.7   # 近期消息占总 budget 的比例
  summary_trigger_tokens: 3000       # 触发摘要的 token 阈值
```

### 9.2 OpenClaw 配置

```json5
{
  plugins: {
    slots: {
      contextEngine: "mymemory-context-engine",
    },
    entries: {
      "mymemory-context-engine": {
        enabled: true,
        config: {
          grpcAddress: "localhost:50051",
        },
      },
    },
  },
}
```

---

## 10. 实现计划

### 阶段一：基础架构
1. 创建 `proto/context_engine.proto` 并生成 Go 代码
2. 实现 `conversation_messages` 表和 CRUD 操作
3. 实现 gRPC 服务端框架
4. 实现 `ContextEngineService` 的 `Ingest` 方法

### 阶段二：核心功能
5. 实现 `Assemble` 方法（近期消息 + 向量检索）
6. 实现 `Compact` 方法（摘要生成）
7. 集成 embedding provider 生成摘要

### 阶段三：插件集成
8. 实现 OpenClaw 插件桥接
9. 配置和文档完善
10. 端到端测试

---

## 11. 风险与注意事项

| 风险 | 缓解措施 |
|------|---------|
| gRPC 调用延迟影响 OpenClaw 性能 | 本地部署、连接池、消息批处理 |
| 向量检索结果可能丢失对话顺序 | 混合策略保留近期消息顺序，检索结果按时间排序 |
| LLM 摘要生成成本 | 预算控制、摘要缓存、仅压缩远期消息 |
| 服务可用性 | OpenClaw 插件添加重试逻辑、超时处理 |

---

## 12. 未来扩展

- **流式 assemble**：支持 gRPC streaming 返回消息
- **多会话管理**：支持跨会话记忆
- **增量摘要**：基于已有摘要进行增量更新
- **自定义检索策略**：支持配置不同的检索/压缩策略
