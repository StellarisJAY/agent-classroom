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
│   │   └── learn.go                # 学习进度 / 问答会话相关
│   ├── application/
│   │   ├── service/                # 业务逻辑实现（依赖 repo 接口 + model 适配器）
│   │   │   ├── generator_slide.go  # Slide 环节生成
│   │   │   ├── generator_quiz.go   # 测试题生成
│   │   │   ├── generator_demo.go   # 互动演示生成（HTML 模板拼接 + CSP 禁网络）
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
│   │   ├── extractor/              # 参考文档提取器（local / mineru / chain 兜底链）
│   │   └── tts/                    # 预留：TTS 适配器（暂无实现）
│   ├── storage/                    # 对象存储实现（local / minio），实现 types.Storage
│   ├── handler/                    # HTTP 层：解析请求 → 调 service → 统一响应
│   │   └── response.go             # 统一响应封装 + bind/校验 helper
│   ├── router/
│   │   └── router.go               # 路由分组注册 + 中间件挂载 + /uploads 存储代理
│   ├── middleware/                 # auth(JWT) / cors / recovery / logger / context(用户身份)
│   └── util/                       # 工具目录
│       ├── jwt.go                  # JWT 签发/校验
│       ├── crypto.go               # AES-256-GCM
│       ├── document.go             # pdf/docx 文本解析
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
    ├── router/index.ts             # 路由：/login、/（课程列表）、/create、
    │                               # /course/:id/learn、/preview/:id
    ├── api/                        # HTTP 层：http.ts（fetch 封装）、error.ts、token.ts
    │   ├── auth.ts / model-config.ts / course.ts
    │   ├── learn.ts                # 学习页接口（demo/问答未就绪部分由 learn.mock.ts 兜底）
    │   └── types.ts                # 后端响应类型定义
    ├── stores/                     # Pinia：auth / course / generation / learn /
    │                               # conversation / model-config / theme
    ├── views/                      # 页面：LoginView、CourseListView、CreateView、
    │                               # GenerateView（SSE 生成进度）、LearnView
    ├── layouts/                    # MainLayout / GenerateLayout / LearnLayout
    ├── components/
    │   ├── app/                    # 导航栏、设置弹窗、模型配置面板
    │   ├── course/                 # 课程卡片 / 筛选 / 网格
    │   ├── generate/               # 大纲列表、环节生成进度
    │   └── learn/                  # 学习页：StageSlide/StageQuiz/StageDemo 三类环节、
    │                               # ChatPanel（问答）、TeacherBar、SectionDrawer 等
    ├── theme/                      # 设计 token + Naive UI themeOverrides（浅/深色）
    └── __tests__/                  # vitest 单元测试
```

### docs 文档说明

| 文档 | 内容 |
|---|---|
| `需求方案.md` | 产品需求总纲：项目概述、技术栈、数据模型（课程/环节/问答会话）、Slide/Quiz/三类互动演示环节定义、核心流程（创建→大纲→串行生成→学习→问答）、模型配置与用户干预权 |
| `数据库设计.md` | PostgreSQL 表结构：通用约定（UUIDv7 主键、无软删除、jsonb 存环节详情）、6 个枚举类型、users/model_configs/courses/outlines/sections/questions/documents/progresses/conversations/messages 各表字段与约束 |
| `前端设计.md` | 前端实现约定：技术选型（Naive UI + Pinia + KaTeX）、设计 token（slate 灰 + teal 强调色、浅/深主题）、页面与组件划分、API 约定 |
| `slide数据结构.md` | Slide 环节 content/steps 两阶段生成的 jsonb 结构：画布属性、元素类型（text/formula/shape/list/image）、坐标定位与讲解步骤动作（underline/highlight/box）|

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

- LLM 提示词一律放 `internal/application/service/prompts/*.md`，经 `go:embed` 引入（`prompt.go`），不硬编码在 Go 代码里。
- 互动演示环节以 `templates/*.html` 骨架拼接生成，产出页面启用 CSP 禁止网络访问，写入 `section.content`。
- 参考文档提取走 `model/extractor` 链：按配置选 local / mineru / chain（外部优先、本地兜底）。

## 测试约定

- 后端测试用 testify：单元测试直接 `go test ./...`。
- 依赖数据库的集成测试（`*_integration_test.go`、`transaction_test.go`）需设置 `TEST_DB_*` 环境变量（TEST_DB_HOST/PORT/USER/PASSWORD/NAME），未设置时自动 skip。
- 前端（web/）：`pnpm test:unit`（vitest）、`pnpm type-check`（vue-tsc）、`pnpm lint`。
