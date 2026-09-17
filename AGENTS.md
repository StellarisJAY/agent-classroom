# Agent Classroom · 项目约定

## 项目结构

仓库根为 Go 后端（模块 `github.com/StellarisJAY/agent-classroom`），与前端 `web/`、文档 `docs/` 并列。

```
agent-classroom/
├── go.mod
├── cmd/
│   └── server/
│       └── main.go                 # 入口：加载配置 → bootstrap.NewApp → 优雅退出
├── internal/
│   ├── config/                     # viper 配置（config.go + config.yaml，env 覆盖）
│   │   ├── config.go               # 含 Extractor / Storage / Log 等配置段
│   │   └── config.yaml
│   ├── bootstrap/
│   │   ├── app.go                  # NewApp(cfg) *App：手动装配全部依赖 + 优雅停机
│   │   └── logger.go               # slog 日志装配
│   ├── types/                      # 实体 + DTO + repo/service 接口 + 业务错误
│   │   ├── base.go                 # ID 类型别名（= uuid.UUID）+ UUIDv7 生成/解析 helper
│   │   ├── interfaces.go           # 仅跨域接口：TransactionManager、Storage
│   │   ├── errors.go               # 业务错误码 / 错误类型
│   │   ├── user.go                 # User 实体 + UserRepo/UserService
│   │   ├── model_config.go         # 用户模型配置 + Repo/Service
│   │   ├── course.go               # Course 实体 + CourseRepo/CourseService
│   │   ├── outline.go              # Outline + 版本历史 + OutlineRepo/OutlineHistoryRepo
│   │   ├── section.go              # Section（slide/quiz/demo）+ Repo/Service
│   │   ├── slide.go                # Slide 内容数据结构
│   │   ├── question.go             # 测试题实体 + QuestionRepo
│   │   ├── document.go             # 参考文档 + DocumentRepo
│   │   ├── conversation.go         # 问答会话 Conversation/Message + Repo + DiscussionService/Sink
│   │   ├── learn.go                # 学习进度 / 问答会话相关
│   │   └── retry.go                # 生成调用重试策略 RetryPolicy（尝试次数 + 指数退避）
│   ├── agent/                      # 统一 agent loop 封装（仅依赖 model）
│   │   ├── agent.go                # loop 驱动：业务装配上下文、轮次上限强制收尾、逐条消息回调
│   │   ├── tool.go                 # OpenAI 工具 schema 组装（FunctionTool/ObjectSchema）
│   │   ├── prompt.go               # go:embed 引入 prompts/ 下的提示词（强制收尾）
│   │   ├── prompts/force_final.md  # 轮次上限强制收尾提示词（随二进制嵌入）
│   │   └── agent_test.go           # agent loop 单元测试
│   ├── application/
│   │   ├── service/                # 业务逻辑实现（依赖 repo 接口 + model 适配器）
│   │   │   ├── course.go           # 课程 CRUD（列表/创建/文档状态）+ CourseService 结构体装配；建课不触发提取，文档由大纲生成任务收敛
│   │   │   ├── generate_outline.go # 大纲生成域：任务状态机 + 生成首步文档提取幂等收敛 + LLM 生成核心 + 大纲查询/版本管理
│   │   │   ├── generator_slide.go  # Slide 环节生成
│   │   │   ├── generator_quiz.go   # 测试题生成
│   │   │   ├── generator_demo.go   # 互动演示生成（HTML 模板拼接 + CSP 禁网络）
│   │   │   ├── discussion.go       # 讨论模式：上下文装配 + agent loop 驱动 + 消息落库
│   │   │   ├── discussion_tools.go # 讨论模式工具 schema 定义与按环节类型裁剪
│   │   │   ├── prompt.go           # go:embed 引入 prompts/ 下的提示词
│   │   │   ├── prompts/            # LLM 提示词模板（.md，随二进制嵌入）
│   │   │   └── templates/          # 演示环节 HTML 骨架模板
│   │   └── repo/                   # GORM 数据访问实现（依赖 *gorm.DB）
│   │       ├── db.go               # 数据库连接初始化
│   │       ├── migrate.go          # 建表：幂等建 ENUM + AutoMigrate + 原始 SQL
│   │       ├── transaction.go      # ctx 携带事务 + base 嵌入统一取连接
│   │       └── <domain>.go         # 各领域 repo 实现 + 集成测试
│   ├── model/                      # 大模型适配层（不依赖 types/服务层，仅标准库）
│   │   ├── model.go                # 通用类型（ChatMessage、ProviderConfig）+ Registry
│   │   ├── llm/                    # LLM 适配器（OpenAI 兼容实现）
│   │   ├── image/                  # 图像生成适配器（openai 兼容 / bailian 专属协议）
│   │   ├── extractor/              # 参考文档提取器（local / mineru（官方 Go SDK，无 token 回退 Flash）/ chain 兜底链）
│   │   └── tts/                    # 预留：TTS 适配器（暂无实现）
│   ├── storage/                    # 对象存储实现（local / minio），实现 types.Storage
│   ├── handler/                    # HTTP 层：解析请求 → 调 service → 统一响应
│   │   ├── response.go             # 统一响应封装 + bind/校验 helper
│   │   ├── sse.go                  # SSE 写出器（事件帧 + Flush + 心跳保活，讨论模式用）
│   │   └── discussion.go           # 讨论模式：提问 SSE 流 + 问答历史
│   ├── router/
│   │   └── router.go               # 路由分组注册 + 中间件挂载 + /uploads 存储代理
│   ├── middleware/                 # auth(JWT) / cors / recovery / logger / context(用户身份)
│   └── util/                       # 工具目录
│       ├── jwt.go                  # JWT 签发/校验
│       ├── crypto.go               # AES-256-GCM
│       ├── document.go             # pdf/docx 文本解析
│       ├── retry.go                # 通用重试 Retry()（指数退避 + ctx 取消，策略见 types.RetryPolicy）
│       └── json.go                 # JSON helper
├── web/                            # 前端（Vue 3 + Vite + TS + Pinia + Naive UI，pnpm 管理），结构见下
└── docs/                           # 设计文档，各文档内容见「docs 文档说明」
```

### 前端结构（web/）

技术栈：Vue 3 + Vite + TypeScript + Pinia + Vue Router + Naive UI + KaTeX，包管理用 pnpm。

```
web/
├── index.html
├── vite.config.ts / vitest.config.ts
├── eslint.config.ts
├── tsconfig*.json                  # app / node / vitest 三套 tsconfig 项目引用
└── src/
    ├── main.ts                     # 入口：装配 Pinia / Router / Naive UI 主题
    ├── App.vue
    ├── env.d.ts
    ├── assets/main.css             # 全局样式入口
    ├── composables/useBreakpoint.ts # 断点响应式（移动端/桌面端切换）
    ├── router/index.ts             # 路由：/login、/（课程列表）、/create、
    │                               # /course/:id/learn、/preview/:id
    ├── api/                        # HTTP 层：http.ts（axios 封装）、error.ts、token.ts
    │   ├── auth.ts / model-config.ts / course.ts
    │   ├── learn.ts                # 学习页接口（demo 未就绪部分由 learn.mock.ts 兜底；问答历史走 api/discussion.ts）
    │   ├── discussion.ts           # 讨论模式 SSE 单流接口 + mapAction 协议转换 + 问答历史
    │   ├── sse.ts                  # 通用 SSE 消费器（fetch + ReadableStream 帧解析）
    │   └── types.ts                # 后端响应类型定义
    ├── stores/                     # Pinia：auth / course / generation / learn /
    │                               # conversation / discussion（讨论模式状态机）/
    │                               # learnStrokes（白板笔画纯函数派生）/ model-config / theme
    ├── views/                      # 页面：LoginView、CourseListView、CreateView、
    │                               # GenerateView（SSE 生成进度）、LearnView
    ├── layouts/                    # MainLayout / GenerateLayout / LearnLayout
    ├── components/
    │   ├── app/                    # AppNavbar 导航栏、SettingsModal 设置弹窗、
    │   │                           # ModelConfigPanel 模型配置面板
    │   ├── course/                 # CourseCard / CourseFilters / CourseGrid
    │   ├── generate/               # OutlineList 大纲列表、SectionProgressList 环节生成进度
    │   └── learn/                  # 学习页：StageSlide/StageQuiz/StageDemo 三类环节、
    │                               # ChatPanel（问答壳，拆出 ChatMessages/ChatComposer 复用）、
    │                               # DiscussionPanel（讨论侧板）、WhiteboardLayer（含讨论叠加层）、
    │                               # TeacherBar（讨论中切换终止按钮）、StageToolbar（环节工具栏，含
    │                               # 讨论模式切换）、LeaveButton（退出课程按钮）、SectionDrawer 等
    ├── theme/                      # index.ts（Naive UI themeOverrides 浅/深色）+ tokens.ts（设计 token）
    └── __tests__/                  # vitest 单元测试：discussion（含 discussion-protocol 协议解析）、
                                    # learnStrokes、learn-whiteboard、model-config、sse、token
```

### docs 文档说明

| 文档 | 内容 |
|---|---|
| `需求方案.md` | 产品需求总纲：项目概述、技术栈、数据模型（课程/环节/问答会话）、Slide/Quiz/三类互动演示环节定义、核心流程（创建→大纲→串行生成→学习→问答）、模型配置与用户干预权 |
| `数据库设计.md` | PostgreSQL 表结构：通用约定（UUIDv7 主键、无软删除、jsonb 存环节详情）、6 个枚举类型、users/model_configs/courses/outlines/sections/questions/documents/progresses/conversations/messages 各表字段与约束 |
| `前端设计.md` | 前端实现约定：技术选型（Naive UI + Pinia + KaTeX）、设计 token（slate 灰 + teal 强调色、浅/深主题）、页面与组件划分、API 约定 |
| `slide数据结构.md` | Slide 环节 content/steps 两阶段生成的 jsonb 结构：画布属性、元素类型（text/formula/shape/list/image）、坐标定位与讲解步骤动作（underline/highlight/box）|
| `agent_loop.md` | 统一 agent loop 封装规范（internal/agent）：Tool/Handler/Runner 类型、上下文注入由业务装配、轮次上限强制收尾与失败回传语义、使用示例与测试约定 |
| `讨论模式方案.md` | 学习页讨论模式全案：单流 agent loop + 合成工具结果、工具集、状态机、消息落库、API 设计、实施切片 |

## 技术选型

| 用途 | 库 |
|---|---|
| Web 框架 | gin |
| ORM | GORM（postgres driver + datatypes）|
| 建表迁移 | `repo.Migrate`（幂等建 ENUM + AutoMigrate + 原始 SQL，无独立迁移工具）|
| 配置 | viper（yaml + env）|
| JWT | golang-jwt/v5 |
| UUID | google/uuid（UUIDv7，经 `types.ID` 别名使用）|
| API Key 加密 | crypto/aes（AES-256-GCM）|
| 对象存储 | minio-go（可切换本地磁盘实现）|
| 文档解析 | ledongthuc/pdf + godocx（本地提取），外部 minerU 可选 |
| 日志 | 标准库 log/slog |
| 测试 | stretchr/testify（单元 + DB 集成测试）|

### 前端（web/）

| 用途 | 库 |
|---|---|
| 框架 / 构建 | Vue 3 + Vite + TypeScript |
| UI 组件库 | Naive UI（图标 @vicons/ionicons5）|
| 状态管理 | Pinia |
| 路由 | Vue Router |
| 公式渲染 | KaTeX |
| 沙箱 | iframe（HTML5 `sandbox`）|
| 生成进度推送 | SSE |
| 测试 / 检查 | vitest + vue-tsc + eslint/oxlint |
| 包管理 | pnpm |

## 后端编码风格约定

- 使用go 1.26版本的新语法风格，比如any替换interface{}、标准库的min/max、"for i := range n"等。
- 内联取地址用 `new(v)`：go 1.26 支持直接 `new(表达式)` 创建指针字面量，禁止编写 `util.Ptr`/`intPtr` 之类的指针辅助函数。
- 禁止手动字符串拼接：使用strings.Join,strings.Builder等工具拼接字符串，禁止用"+"拼接。
- 使用switch-case简化if-else结构,对于只有枚举单一条件判断的分支逻辑使用switch-case使代码更加简洁。
- 使用fmt.FPrintf: 向符合Writer接口的结构写入字符串时，使用fmt.FPrintf替代。

## 依赖注入约定

- 手动注入，装配链单向：`config → gorm.DB → repo(实现) → service(实现) → model 适配器 → handler → router → *gin.Engine`，集中在 `bootstrap/app.go`。
- `repo` / `service` 构造函数返回 `types.*Repo` / `types.*Service` **接口**（如 `func NewUserRepo(db *gorm.DB) types.UserRepo`），便于测试 mock。
- `service` 只依赖接口与 model 客户端，不 import 具体 repo 实现。
- 模型适配层经 `model.Registry` 按 provider 路由：默认注册 OpenAI 兼容实现作回退，特殊 provider（如阿里云百炼）单独适配并注册。

## 事务与仓储约定

- 跨 repo 写操作经 `types.TransactionManager` 开启事务；事务连接写入 `context`（`repo.WithTx`）。
- 各 repo 嵌入 `repo.base`，统一通过 `db(ctx)` 取连接：ctx 内有事务则用事务，否则回退默认 db。
- ID 统一使用 `types.ID`（= uuid.UUID 别名），新建记录用 `types.NewID()`（UUIDv7，按时间有序）；各层不直接 import uuid 生成 ID。

## 生成流程与提示词约定

- LLM 提示词一律放 `prompts/` 子目录（现存 `internal/application/service/prompts/`、`internal/agent/prompts/`），经 `go:embed` 引入（各自包内 `prompt.go`），不硬编码在 Go 代码里。
- 互动演示环节以 `templates/*.html` 骨架拼接生成，产出页面启用 CSP 禁止网络访问，写入 `section.content`。
- 参考文档提取走 `model/extractor` 链：按配置选 local / mineru / chain（外部优先、本地兜底）；minerU 官方 API 用官方 Go SDK（sdk/go），无 token 自动回退免登录 Flash 提取。

## 测试约定

- 后端测试用 testify：单元测试直接 `go test ./...`。
- 依赖数据库的集成测试（`*_integration_test.go`、`transaction_test.go`）需设置 `TEST_DB_*` 环境变量（TEST_DB_HOST/PORT/USER/PASSWORD/NAME），未设置时自动 skip。
- 前端（web/）：`pnpm test:unit`（vitest）、`pnpm type-check`（vue-tsc）、`pnpm lint`。

## 项目约定

- 完成一个任务后，需要更新docs下的文档。
- 对项目目录结构的修改后，需要修改AGENTS.md中的项目结构以保持一致。
- 使用中文输出思考内容、回答用户问题、编写注释和文档。