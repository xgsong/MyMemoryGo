# MyMemoryGo

🧠 **AI 持久化记忆组件** - 为 AI 助手提供长期记忆能力

[![Go Version](https://img.shields.io/badge/Go-1.25.7-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-green)]()

---

## 📖 目录

- [简介](#简介)
- [核心特性](#核心特性)
- [快速开始](#快速开始)
- [使用示例](#使用示例)
- [API 参考](#api-参考)
- [架构设计](#架构设计)
- [配置说明](#配置说明)
- [开发指南](#开发指南)
- [性能基准](#性能基准)
- [贡献](#贡献)
- [许可证](#许可证)

---

## 简介

MyMemoryGo 是一个专为 AI 助手设计的**长期记忆存储系统**，能够让 AI 记住对话历史、代码片段、用户偏好等信息，并在需要时通过智能搜索快速检索。

### 核心能力

- **💾 持久化存储** - 将 AI 对话和代码片段永久保存到本地数据库
- **🔍 智能搜索** - 支持语义搜索、全文检索和混合搜索
- **🎯 智能排序** - 基于 MMR（最大边界相关）和时间衰减算法优化搜索结果
- **⚡ 高性能** - 基于 SQLite 和向量嵌入的高效检索
- **🔌 灵活集成** - 提供 Go SDK 和 RESTful API，易于集成到现有系统

### 适用场景

- AI 代码助手的记忆持久化
- 对话历史存储和检索
- 知识库管理系统
- 个性化 AI 助手
- 代码片段管理

---

## 核心特性

### 🔍 混合搜索

结合**向量搜索**和**全文搜索**的优势：

| 搜索类型 | 描述 | 权重 |
|---------|------|------|
| **向量搜索** | 基于语义相似度的搜索 | 默认 70% |
| **全文搜索** | 基于关键词匹配的搜索 | 默认 30% |

```go
opts := repository.NewSearchOptionsBuilder().
    WithLimit(10).
    WithMinScore(0.5).
    WithVectorWeight(0.7).
    WithFulltextWeight(0.3).
    Build()

result, err := store.Search(ctx, "user authentication", opts)
```

### 🎯 智能重排序

#### MMR（Maximal Marginal Relevance）

确保搜索结果既**相关**又**多样化**：

```go
opts := repository.NewSearchOptionsBuilder().
    WithMMR(0.7).  // 0.5-1.0，越低越多样化
    Build()
```

#### 时间衰减

根据记忆的时间调整相关性分数：

```go
opts := repository.NewSearchOptionsBuilder().
    WithDecay(720 * time.Hour).  // 30 天半衰期
    WithReferenceTime(time.Now()).
    Build()
```

### 💾 双存储后端

| 存储类型 | 描述 | 适用场景 |
|---------|------|---------|
| **SQLite** | 关系型数据库，支持复杂查询 | 生产环境 |
| **FileStore** | 基于 JSON 文件的轻量存储 | 开发测试 |

---

## 快速开始

### 安装

```bash
git clone https://github.com/xgsong/MyMemoryGo.git
cd MyMemoryGo
go mod download
```

### 基础使用

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/xgsong/MyMemoryGo/internal/domain/entity"
    "github.com/xgsong/MyMemoryGo/internal/domain/repository"
    "github.com/xgsong/MyMemoryGo/internal/infrastructure/embedding"
    "github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/sqlite"
)

func main() {
    ctx := context.Background()

    // 1. 创建嵌入客户端
    embedConfig := &embedding.Config{
        Provider:   "ollama",
        Model:      "nomic-embed-text",
        Dimensions: 768,
        OllamaURL:  "http://localhost:11434",
    }
    embedClient, err := embedding.NewClient(embedConfig)
    if err != nil {
        log.Fatal(err)
    }

    // 2. 创建 SQLite 存储
    store, err := sqlite.NewStore("memory.db", embedClient, &sqlite.Config{
        VectorDims: 768,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    // 3. 存储记忆
    memories := []*entity.Memory{
        {
            ID:        "mem-001",
            Path:      "src/auth.go",
            StartLine: 10,
            EndLine:   50,
            Content:   "User authentication with JWT tokens",
            Source:    entity.SourceCode,
            Timestamp: time.Now(),
        },
    }
    err = store.Store(ctx, memories)
    if err != nil {
        log.Fatal(err)
    }

    // 4. 搜索记忆
    opts := repository.DefaultSearchOptions()
    result, err := store.Search(ctx, "how to authenticate users", opts)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d memories\n", result.Total)
    for _, hit := range result.Hits {
        fmt.Printf("- %s (score: %.2f)\n", hit.Path, hit.Score)
    }
}
```

---

## 使用示例

### 示例 1：代码片段管理

```go
// 存储代码片段
memories := []*entity.Memory{
    {
        ID:        "snippet-001",
        Path:      "utils/validator.go",
        StartLine: 1,
        EndLine:   100,
        Content:   "// Email validation function...",
        Source:    entity.SourceCode,
        Timestamp: time.Now(),
    },
}
store.Store(ctx, memories)

// 搜索相关代码
result, _ := store.Search(ctx, "email validation regex", opts)
```

### 示例 2：对话历史存储

```go
// 存储对话
conversation := &entity.Memory{
    ID:        "conv-2024-01-15",
    Path:      "conversations/2024-01-15",
    Content:   "User asked about authentication best practices",
    Source:    entity.Conversation,
    Timestamp: time.Now(),
}
store.Store(ctx, []*entity.Memory{conversation})

// 检索历史对话
result, _ := store.Search(ctx, "authentication practices", opts)
```

### 示例 3：高级搜索

```go
// 组合搜索：向量 + 全文 + 时间衰减 + MMR
opts := repository.NewSearchOptionsBuilder().
    WithLimit(20).
    WithMinScore(0.6).
    WithVectorWeight(0.7).
    WithFulltextWeight(0.3).
    WithMMR(0.75).
    WithDecay(720 * time.Hour).
    WithReferenceTime(time.Now()).
    WithSourceFilter([]entity.SourceType{entity.SourceCode}).
    Build()

result, err := store.Search(ctx, "JWT token expiration", opts)
```

---

## API 参考

### Store 接口

```go
type Store interface {
    // 存储记忆
    Store(ctx context.Context, memories []*entity.Memory) error
    
    // 获取单个记忆
    Get(ctx context.Context, id string) (*entity.Memory, error)
    
    // 按路径获取记忆
    GetByPath(ctx context.Context, path string) ([]*entity.Memory, error)
    
    // 删除记忆
    Delete(ctx context.Context, id string) error
    DeleteByPath(ctx context.Context, path string) error
    
    // 搜索记忆
    Search(ctx context.Context, query string, opts *repository.SearchOptions) (*entity.SearchResult, error)
    
    // 列出所有记忆
    List(ctx context.Context, source string, pathPrefix string, limit, offset int) ([]*entity.Memory, error)
    
    // 索引管理
    Index(ctx context.Context, memories []*entity.Memory) error
    RemoveFromIndex(ctx context.Context, ids []string) error
    
    // 关闭存储
    Close() error
}
```

### SearchOptions 配置

```go
type SearchOptions struct {
    Limit          int                    // 返回结果数量
    MinScore       float64                // 最小相关度分数
    VectorWeight   float64                // 向量搜索权重
    FulltextWeight float64                // 全文搜索权重
    MMR            float64                // MMR 参数 (0.5-1.0)
    Decay          time.Duration          // 时间衰减周期
    ReferenceTime  time.Time              // 时间参考点
    SourceFilter   []entity.SourceType    // 源类型过滤
}

// 使用 Builder 模式创建配置
opts := repository.NewSearchOptionsBuilder().
    WithLimit(10).
    WithMinScore(0.5).
    WithMMR(0.7).
    Build()
```

---

## 架构设计

### 分层架构

```
┌─────────────────────────────────────────┐
│         Interface Layer (API)           │
├─────────────────────────────────────────┤
│       Application Layer (Service)       │
├─────────────────────────────────────────┤
│          Domain Layer (Core)            │
│  ┌───────────┬───────────┬───────────┐  │
│  │  Entity   │ Repository│  Service  │  │
│  └───────────┴───────────┴───────────┘  │
├─────────────────────────────────────────┤
│      Infrastructure Layer (Impl)        │
│  ┌───────────┬───────────┬───────────┐  │
│  │  SQLite   │ FileStore │ Embedding │  │
│  └───────────┴───────────┴───────────┘  │
└─────────────────────────────────────────┘
```

### 核心组件

| 组件 | 职责 | 位置 |
|------|------|------|
| **Entity** | 领域模型定义 | `internal/domain/entity` |
| **Repository** | 数据访问接口 | `internal/domain/repository` |
| **Service** | 业务逻辑 | `internal/domain/service` |
| **SQLite** | 关系型存储实现 | `internal/infrastructure/persistence/sqlite` |
| **FileStore** | 文件存储实现 | `internal/infrastructure/persistence/filestore` |
| **Embedding** | 向量嵌入生成 | `internal/infrastructure/embedding` |
| **Search** | 搜索算法 | `internal/infrastructure/search` |

---

## 配置说明

### 配置文件

```yaml
# configs/config.yaml

# 存储配置
store:
  type: "sqlite"  # sqlite | filestore
  path: "./data/memories.db"
  
# 嵌入配置
embedding:
  provider: "ollama"  # ollama | openai
  model: "nomic-embed-text"
  dimensions: 768
  ollama_url: "http://localhost:11434"
  
# 搜索配置
search:
  default_limit: 10
  min_score: 0.5
  vector_weight: 0.7
  fulltext_weight: 0.3
  mmr_lambda: 0.7
  
# 日志配置
log:
  level: "info"  # debug | info | warn | error
  format: "text"  # text | json
  output: "stdout"  # stdout | file
```

### 环境变量

```bash
# 存储
MYMEMORY_STORE_TYPE=sqlite
MYMEMORY_STORE_PATH=./data/memories.db

# 嵌入
MYMEMORY_EMBEDDING_PROVIDER=ollama
MYMEMORY_EMBEDDING_MODEL=nomic-embed-text
MYMEMORY_OLLAMA_URL=http://localhost:11434

# 日志
MYMEMORY_LOG_LEVEL=info
MYMEMORY_LOG_FORMAT=text
```

---

## 开发指南

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/pkg/vector/...
go test ./internal/infrastructure/search/...

# 带覆盖率
go test -cover ./...

# 基准测试
go test -bench=. ./internal/pkg/vector/...
```

### 代码规范

```bash
# 格式化代码
go fmt ./...

# 检查代码
go vet ./...

# 运行 linter
golangci-lint run
```

### 构建

```bash
# 构建所有组件
go build ./...

# 构建 CLI
go build -o bin/memory ./cmd/memory

# 构建示例
go build -o bin/example ./cmd/example
```

---

## 性能基准

### 向量计算性能

```bash
go test -bench=BenchmarkCosineSimilarity ./internal/pkg/vector/...
go test -bench=BenchmarkEuclideanDistance ./internal/pkg/vector/...
go test -bench=BenchmarkNormalize ./internal/pkg/vector/...
```

### 搜索性能

```bash
go test -bench=BenchmarkMMRReranker ./internal/infrastructure/search/...
```

---

## 贡献

欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 开发环境设置

```bash
# 克隆仓库
git clone https://github.com/xgsong/MyMemoryGo.git
cd MyMemoryGo

# 安装依赖
go mod download

# 运行测试
go test ./...

# 运行示例
go run ./cmd/example/main.go
```

---

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

---

## 致谢

- [Claude-Mem](https://github.com/thedotmack/claude-mem) - 灵感和架构参考
- [SQLite](https://sqlite.org) - 轻量级数据库
- [Ollama](https://ollama.ai) - 本地嵌入模型

---

<div align="center">
  <strong>MyMemoryGo</strong> - 为 AI 赋予持久记忆
</div>
