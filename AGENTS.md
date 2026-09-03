# Agent Classroom · 项目约定

## 项目结构

仓库根为 Go 后端（模块 `github.com/StellarisJAY/agent-classroom`），与前端 `web/`、文档 `docs/` 并列。

```
agent-classroom/
├── go.mod
├── cmd/
│   └── server/
│       └── main.go                 # 入口：加载配置 → 装配依赖 → 启动 gin
├── internal/
│   ├── config/                     # viper 配置（config.go + config.yaml，env 覆盖）
│   ├── bootstrap/
│   │   └── app.go                  # NewApp(cfg) *gin.Engine：手动装配全部依赖
│   ├── types/                      # 实体 + DTO + repo/service 接口 + 业务错误
│   │   ├── models.go               # 实体结构体（10 表，GORM tag，jsonb 用 datatypes.JSON）
│   │   ├── dto.go                  # 请求/响应 DTO
│   │   ├── interfaces.go           # repo + service 接口定义
│   │   └── errors.go               # 业务错误码 / 错误类型
│   ├── application/
│   │   ├── service/                # 业务逻辑实现（依赖 repo 接口 + model 适配器）
│   │   └── repo/                   # GORM 数据访问实现（依赖 *gorm.DB）
│   ├── model/                      # 模型适配器（LLM / TTS 等多种类型）
│   │   ├── model.go                # 通用接口/类型（ChatMessage、ProviderConfig…）
│   │   ├── llm/                    # LLM 适配器（client.go / openai.go）
│   │   └── tts/                    # TTS 适配器（client.go / openai_tts.go）
│   ├── handler/                    # HTTP 层：解析请求 → 调 service → 统一响应
│   │   └── response.go             # 统一响应封装 + bind/校验 helper（并入 handler）
│   ├── router/
│   │   └── router.go               # 路由分组注册 + 中间件挂载
│   ├── middleware/                 # auth(JWT) / cors / recovery / logger
│   └── util/                       # 工具目录
│       ├── jwt.go                  # JWT 签发/校验
│       ├── crypto.go               # AES-256-GCM
│       └── sse.go                  # SSE 流式写入
├── migrations/                     # golang-migrate .sql DDL
├── web/                            # 前端（Vue 3 + Vite + TS）
└── docs/                           # 设计文档
```

## 技术选型

| 用途 | 库 |
|---|---|
| Web 框架 | gin |
| ORM | GORM（postgres driver + datatypes）|
| 迁移 | golang-migrate |
| 配置 | viper（yaml + env）|
| JWT | golang-jwt/v5 |
| UUID | google/uuid（UUIDv7）|
| API Key 加密 | crypto/aes（AES-256-GCM）|
| 日志 | 标准库 log/slog |

## 依赖注入约定

- 手动注入，装配链单向：`config → gorm.DB → repo(实现) → service(实现) → model 适配器 → handler → router → *gin.Engine`，集中在 `bootstrap/app.go`。
- `repo` / `service` 构造函数返回 `types.*Repo` / `types.*Service` **接口**（如 `func NewUserRepo(db *gorm.DB) types.UserRepo`），便于测试 mock。
- `service` 只依赖接口与 model 客户端，不 import 具体 repo 实现。
