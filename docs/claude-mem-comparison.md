# MyMemoryGo vs Claude-Mem 深度对比分析报告

---

## 一、项目定位对比

### MyMemoryGo
**定位**：通用 AI 持久化记忆组件，为 AI 助手提供长期记忆存储和检索能力

**核心场景**：
- AI 代码助手的记忆持久化
- 对话历史存储和检索
- 知识库管理系统
- 代码片段管理

### Claude-Mem
**定位**：专为 Claude Code 设计的持久化记忆压缩系统，通过生命周期钩子自动捕获上下文

**核心场景**：
- Claude Code 会话上下文持久化
- 工具使用观察自动记录
- 会话间知识连续性保持
- MCP 工具集成搜索

---

## 二、架构设计对比

### MyMemoryGo 架构（分层架构）

```
┌─────────────────────────────────────────┐
│         Interface Layer (API)           │
│         handlers.go, server.go          │
├─────────────────────────────────────────┤
│       Application Layer (Service)       │
│       service/*.go                      │
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

**特点**：
- 经典 DDD 分层架构
- Go 语言实现，类型安全
- 支持双存储后端（SQLite + FileStore）
- 独立的 embedding 模块

### Claude-Mem 架构（模块化架构）

```
┌─────────────────────────────────────────┐
│          CLI Handlers Layer             │
│  session-init, observation, complete    │
├─────────────────────────────────────────┤
│         Worker Service Layer            │
│  HTTP API + Web Viewer + Search Tools   │
├─────────────────────────────────────────┤
│       Services Layer (Core)             │
│  ┌───────────┬───────────┬───────────┐  │
│  │  SQLite   │   Chroma  │  Worker   │  │
│  │  Store    │   Vector  │  Agents   │  │
│  └───────────┴───────────┴───────────┘  │
├─────────────────────────────────────────┤
│        Infrastructure Layer             │
│  Hooks, Plugins, MCP, Integrations      │
└─────────────────────────────────────────┘
```

**特点**：
- TypeScript/Node.js 实现
- Worker 服务 + HTTP API 模式
- SQLite + Chroma 向量数据库双存储
- 深度集成 Claude Code 钩子系统

---

## 三、核心功能对比

### 1. 向量嵌入（Embedding）

| 特性 | MyMemoryGo | Claude-Mem |
|------|-----------|-----------|
| **实现方式** | OpenAI 兼容 HTTP 客户端 | Chroma 向量数据库集成 |
| **支持模型** | OpenAI、Ollama、vLLM、LocalAI | Chroma 内置模型 |
| **缓存机制** | LRU 缓存（SHA256 哈希） | Chroma 内置缓存 |
| **批量处理** | 支持批量请求 | 支持批量处理 |
| **配置灵活性** | 高（预设 + 自定义） | 中（Chroma 配置） |

**MyMemoryGo 优势**：
- 支持更多向量模型提供商
- 灵活的配置和预设系统
- 内置 LRU 缓存优化

**Claude-Mem 优势**：
- Chroma 向量数据库原生支持
- 混合搜索（语义 + 关键词）
- 向量搜索与元数据过滤结合

### 2. 存储层对比

| 特性 | MyMemoryGo | Claude-Mem |
|------|-----------|-----------|
| **主存储** | SQLite（WAL 模式） | SQLite（WAL 模式） |
| **向量存储** | SQLite 向量列 | Chroma 专用向量库 |
| **文件存储** | FileStore（JSON 文件） | 无 |
| **迁移系统** | 简单版本控制 | 完整迁移框架 |
| **优化配置** | 基础 PRAGMA | 高级 MMAP + 缓存优化 |

**SQLite 优化对比**：

**MyMemoryGo**：
```sql
PRAGMA journal_mode = WAL
PRAGMA synchronous = NORMAL
PRAGMA foreign_keys = ON
```

**Claude-Mem**（更激进）：
```sql
PRAGMA journal_mode = WAL
PRAGMA synchronous = NORMAL
PRAGMA foreign_keys = ON
PRAGMA temp_store = memory
PRAGMA mmap_size = 256MB
PRAGMA cache_size = 10000
```

### 3. 搜索功能对比

| 搜索类型 | MyMemoryGo | Claude-Mem |
|---------|-----------|-----------|
| **向量搜索** | ✅ 余弦相似度 | ✅ Chroma 语义搜索 |
| **全文搜索** | ✅ SQLite FTS5 | ✅ SQLite FTS5 |
| **混合搜索** | ✅ 可配置权重 | ✅ 混合搜索策略 |
| **MMR 重排序** | ✅ 最大边界相关 | ❌ 无 |
| **时间衰减** | ✅ 指数衰减 | ❌ 无 |
| **元数据过滤** | ✅ 基础过滤 | ✅ 高级过滤（类型/日期/项目） |
| **时间线查询** | ❌ 无 | ✅ Timeline Builder |
| **渐进式披露** | ❌ 无 | ✅ 3 层工作流模式 |

**MyMemoryGo 搜索算法**：
```go
// 混合搜索：向量 + 全文 + 时间衰减 + MMR
opts := repository.NewSearchOptionsBuilder().
    WithLimit(20).
    WithMinScore(0.6).
    WithVectorWeight(0.7).
    WithFulltextWeight(0.3).
    WithMMR(0.75).
    WithDecay(720 * time.Hour).
    WithReferenceTime(time.Now()).
    Build()
```

**Claude-Mem 搜索策略**：
```typescript
// 混合搜索策略：元数据过滤 -> Chroma 排序 -> 交集 -> 水合
async findByConcept(concept: string, options: StrategySearchOptions) {
  // Step 1: SQLite 元数据过滤
  const metadataResults = this.sessionSearch.findByConcept(concept, filterOptions);
  
  // Step 2: Chroma 语义排序
  const ids = metadataResults.map(obs => obs.id);
  const chromaResults = await this.chromaSync.queryChroma(concept, ids.length);
  
  // Step 3: 交集（保留元数据匹配，按 Chroma 排序）
  const rankedIds = this.intersectWithRanking(ids, chromaResults.ids);
  
  // Step 4: 按排序水合
  const observations = this.sessionStore.getObservationsByIds(rankedIds, { limit });
}
```

### 4. 生命周期管理对比

**MyMemoryGo**：
- **手动管理**：通过 API/SDK 显式调用
- **核心操作**：Store、Get、Search、Delete、List
- **无自动钩子**：需要外部系统集成

**Claude-Mem**：
- **自动钩子系统**：5 个生命周期钩子
  1. **SessionStart**：会话初始化
  2. **UserPromptSubmit**：用户提示提交
  3. **PostToolUse**：工具使用后观察
  4. **Stop**：会话停止
  5. **SessionEnd**：会话结束

**Claude-Mem 生命周期流程**：

```
┌─────────────────┐
│  SessionStart   │ → 创建会话记录 → 初始化 SDK Agent
└─────────────────┘
         ↓
┌─────────────────┐
│ UserPromptSubmit│ → 语义上下文注入 → 保存到 DB
└─────────────────┘
         ↓
┌─────────────────┐
│   PostToolUse   │ → 捕获工具调用 → 隐私检查 → 存储观察
└─────────────────┘
         ↓
┌─────────────────┐
│      Stop       │ → 从活跃会话移除 → 允许清理
└─────────────────┘
         ↓
┌─────────────────┐
│   SessionEnd    │ → 最终清理 → 触发摘要生成
└─────────────────┘
```

**示例代码（Claude-Mem session-init）**：
```typescript
export const sessionInitHandler: EventHandler = {
  async execute(input: NormalizedHookInput): Promise<HookResult> {
    // 1. 确保 Worker 运行
    const workerReady = await ensureWorkerRunning();
    
    // 2. 初始化会话
    const initResponse = await workerHttpRequest('/api/sessions/init', {
      method: 'POST',
      body: JSON.stringify({
        contentSessionId: sessionId,
        project,
        prompt,
        platformSource
      })
    });
    
    // 3. 初始化 SDK Agent（仅 Claude Code）
    if (input.platform !== 'cursor') {
      await workerHttpRequest(`/sessions/${sessionDbId}/init`, {
        method: 'POST',
        body: JSON.stringify({ userPrompt: cleanedPrompt, promptNumber })
      });
    }
    
    // 4. 语义上下文注入（Chroma 查询）
    const semanticRes = await workerHttpRequest('/api/context/semantic', {
      method: 'POST',
      body: JSON.stringify({ q: prompt, project, limit: 5 })
    });
    
    return {
      continue: true,
      additionalContext: semanticRes.context
    };
  }
};
```

### 5. 数据模型对比

**MyMemoryGo 核心实体**：
```go
type Memory struct {
    ID        string
    Path      string
    StartLine int
    EndLine   int
    Content   string
    Source    SourceType  // Code, Conversation, Document
    Timestamp time.Time
    Embedding []float32   // 向量嵌入
}
```

**Claude-Mem 核心实体**：
```typescript
// Observation（观察）
interface Observation {
    id: number;
    sessionId: number;
    toolName: string;
    toolInput: any;
    toolResponse: any;
    content: string;
    files: string[];
    timestamp: string;
    embedding?: number[];  // Chroma 存储
}

// Session（会话）
interface Session {
    id: number;
    contentSessionId: string;
    project: string;
    prompt: string;
    platformSource: 'claude-code' | 'cursor' | 'gemini-cli';
    startTime: string;
    endTime?: string;
}

// Summary（摘要）
interface Summary {
    sessionId: number;
    content: string;
    generatedAt: string;
}
```

---

## 四、技术栈对比

| 维度 | MyMemoryGo | Claude-Mem |
|------|-----------|-----------|
| **语言** | Go 1.25.7 | TypeScript/Node.js 18+ |
| **运行时** | 编译型二进制 | Node.js/Bun |
| **向量库** | SQLite 向量列 | Chroma 专用库 |
| **数据库** | SQLite | SQLite + Chroma |
| **部署** | 单二进制文件 | npm 包 + Worker 服务 |
| **依赖管理** | Go Modules | npm/pnpm |
| **测试框架** | Go testing | Bun Test |
| **代码行数** | ~5K Go 代码 | ~50K TS 代码 |

---

## 五、设计理念对比

### MyMemoryGo 设计原则

1. **简洁性**：最小化依赖，单二进制部署
2. **灵活性**：支持多种向量模型和存储后端
3. **可嵌入性**：作为库或独立服务使用
4. **算法优化**：内置 MMR、时间衰减等高级搜索算法

### Claude-Mem 设计原则

1. **自动化**：零手动干预，钩子自动捕获
2. **渐进式披露**：3 层工作流（索引 → 时间线 → 详情）
3. **语义优先**：Chroma 向量搜索为核心
4. **生态集成**：深度集成 Claude Code、Cursor、Gemini CLI

---

## 六、核心差异总结

### 架构差异

| 方面 | MyMemoryGo | Claude-Mem |
|------|-----------|-----------|
| **架构模式** | 分层架构（DDD） | 模块化架构（Worker + Hooks） |
| **集成方式** | SDK/API 调用 | 生命周期钩子自动触发 |
| **部署模式** | 单二进制 | Worker 服务 + npm 包 |
| **数据流** | 手动 CRUD | 自动捕获 → 存储 → 检索 |

### 功能差异

| 功能 | MyMemoryGo | Claude-Mem |
|------|-----------|-----------|
| **向量模型** | 自研 OpenAI 兼容客户端 | Chroma 集成 |
| **搜索算法** | MMR + 时间衰减 | 混合搜索 + 时间线 |
| **上下文注入** | 手动查询 | 自动语义注入 |
| **隐私控制** | 基础 | 隐私标签 + 自动检查 |
| **Web UI** | 无 | Web Viewer（实时流） |
| **MCP 工具** | 无 | 4 个 MCP 搜索工具 |

### 使用场景差异

**MyMemoryGo 更适合**：
- 需要自定义向量模型的场景
- 需要高级搜索算法（MMR、时间衰减）
- 作为独立组件集成到现有系统
- 资源受限环境（单二进制）

**Claude-Mem 更适合**：
- Claude Code 用户
- 需要自动上下文捕获
- 需要语义搜索和时间线浏览
- 多 IDE 集成（Claude Code、Cursor、Gemini CLI）

---

## 七、可借鉴的设计

### MyMemoryGo 可借鉴 Claude-Mem

1. **生命周期钩子系统**：实现自动上下文捕获
2. **渐进式披露模式**：索引 → 时间线 → 详情的 3 层工作流
3. **语义上下文自动注入**：每次查询自动注入相关记忆
4. **Web Viewer UI**：实时记忆流可视化
5. **MCP 工具集成**：提供标准化搜索接口

### Claude-Mem 可借鉴 MyMemoryGo

1. **MMR 重排序算法**：提升搜索结果多样性
2. **时间衰减机制**：让近期记忆更相关
3. **灵活的向量模型支持**：支持更多本地/云端模型
4. **LRU 缓存优化**：减少重复嵌入计算
5. **双存储后端**：支持文件存储作为轻量选项

---

## 八、总结

**MyMemoryGo** 是一个**通用型记忆组件**，强调灵活性、算法优化和易部署性，适合作为独立组件集成到各种 AI 系统中。

**Claude-Mem** 是一个**专用型记忆系统**，强调自动化、语义搜索和生态集成，通过钩子系统实现零干预的上下文持久化。

两者在核心功能（SQLite 存储、向量搜索）上有共通之处，但在设计理念、目标用户和使用场景上存在显著差异。MyMemoryGo 更像是一个"记忆数据库"，而 Claude-Mem 更像是一个"记忆助手"。

---

**分析完成！** 📊
