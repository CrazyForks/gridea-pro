# Gridea Pro 后端架构重构：从「能跑就行」到「职责分明」

> 本文记录了 Gridea Pro 后端在一次集中重构中所做的全部架构调整，涵盖端口治理、包合并与提升、巨型结构体拆分、以及渲染引擎独立抽包等四个阶段。每一步都遵循同一个原则——**让代码结构忠实地反映系统的领域边界**。

## 一、背景：当 service 包变成杂货铺

Gridea Pro 是一款基于 Wails（Go + Vue）的桌面端静态博客系统。后端采用经典的分层架构：`Facade → Service → Repository → Domain`。随着功能迭代，`internal/service/` 目录膨胀到了 23 个文件，其中既有 `PostService`、`TagService` 这类标准 CRUD 服务，又有 `RendererService` 及其 7 个渲染辅助文件——一个生成 RSS、Sitemap、搜索索引、编译 LESS、复制静态资源的 SSG（Static Site Generator）引擎。两类截然不同的职责挤在同一个包里，带来三个问题：

1. **认知负担**：新开发者打开 `service/` 看到 23 个文件，无法快速判断哪些属于业务逻辑、哪些属于渲染管线。
2. **耦合蔓延**：渲染模块的常量（`DirOutput`、`DefaultPostPath`）和类型（RSS XML 结构体）暴露在 `service` 包的公共命名空间中，任何同包文件都可以随意引用。
3. **测试隔离困难**：想单独测试渲染逻辑，却不得不处理同包中其他 Service 的依赖初始化。

除此之外，还存在几个「小而确定」的技术债务：端口配置不一致、单文件包 `model` 缺乏存在意义、`deployer` 子包命名层级冗余。本次重构一次性解决所有这些问题。

---

## 二、第一步：端口配置统一（2077 / 6060 → 6606）

### 问题

后端 `preview_service.go` 中正式环境的默认起始端口为 `2077`，而前端评论页面（`comments/index.vue`、`CommentItem.vue`）的 fallback 端口硬编码为 `6060`。两个数字毫无关联，且都没有文档说明选择理由。一旦后端启动在 `2077`，前端却去连 `6060`，评论预览功能就会静默失败。

### 方案

统一为 `6606`，保留开发环境的 `3367` 不变。后端已有端口冲突自动递增逻辑（`tryListen` 循环），因此只需修改默认值。

### 改动清单

| 文件 | 改动 |
|------|------|
| `backend/internal/service/preview_service.go:21` | `DefaultProdStartPort = 2077` → `6606` |
| `frontend/src/views/comments/index.vue` | 4 处 `6060` → `6606` |
| `frontend/src/views/comments/components/CommentItem.vue` | 1 处 `6060` → `6606` |

改动量极小，但消除了一个跨端配置不一致的隐患。这类问题的特点是：平时不会被注意到，一旦触发却很难定位——因为前端的网络请求会悄无声息地超时，不会抛出任何与「端口」相关的错误信息。

---

## 三、第二步：包合并与提升

### 3.1 model → domain：消灭单文件包

`internal/model/` 目录只有一个文件 `theme_config.go`，定义了主题配置 schema 的结构体。Go 社区的共识是：**如果一个包只有一个文件且不太可能扩展，它可能不配成为独立包**。

将其合并到 `domain/` 时遇到了命名冲突——`model.ThemeConfig` 与已有的 `domain.ThemeConfig`（站点主题配置）重名。分析两者的语义：

- `domain.ThemeConfig`：用户在站点设置中填写的配置值（站点名称、文章路径、分页大小等）
- `model.ThemeConfig`：主题 `config.json` 的 schema 定义（字段名、类型、默认值、下拉选项等）

后者描述的是「配置的结构」而非「配置的值」，因此重命名为 `ThemeConfigSchema`，语义更精确。

```go
// domain/theme_config_model.go
package domain

type ThemeConfigSchema struct {
    Name         string            `json:"name"`
    Version      string            `json:"version"`
    Engine       string            `json:"engine"`
    CustomConfig []ThemeConfigItem `json:"customConfig"`
}
```

使用 `git mv` 移动文件以保留 git 历史，再批量替换所有 `model.ThemeConfig` 引用为 `domain.ThemeConfigSchema`。

### 3.2 service/deployer → deploy：层级扁平化

原来的部署模块位于 `internal/service/deployer/`，这个路径暗示它是 service 层的附属，但实际上 deployer 是一个独立的策略模式实现（Git / Vercel），不依赖 service 层的任何类型。将其提升到 `internal/deploy/`，与 service 平级，更准确地反映其在架构中的地位。

```
# Before                          # After
internal/service/deployer/        internal/deploy/
├── interface.go                  ├── interface.go
├── git_deployer.go              ├── git_deployer.go
└── vercel_deployer.go           └── vercel_deployer.go
```

包名从 `deployer` 改为 `deploy`，遵循 Go 标准库的命名惯例（`net/http` 而非 `net/httper`）。

---

## 四、第三步：RendererService 巨型结构体拆分

### 问题诊断

重构前，`RendererService` 是一个拥有 2000+ 行代码的庞然大物，所有方法都挂在同一个 receiver 上：

```go
type RendererService struct {
    // 十几个字段：repos、renderer、logger、appDir...
}

func (s *RendererService) RenderAll(ctx context.Context) error { ... }
func (s *RendererService) RenderIndex(...) error { ... }
func (s *RendererService) RenderRSS(...) error { ... }
func (s *RendererService) CopyThemeAssets(...) error { ... }
// ... 还有 20+ 个方法
```

这违反了单一职责原则。RSS 生成、LESS 编译、分页渲染——这些是完全独立的关注点，却共享同一个结构体的全部字段。

### 对比方案：结构体拆分 vs 目录拆分

在实施前，我们对比了两种方案：

**方案 A（目录拆分 / 垂直领域切割）**：将 service 按 `core/`、`engine/`、`system/` 划分子目录。这本质上是在**移动文件**，不改变代码的内部结构。`RendererService` 搬到 `engine/` 后仍然是一个 2000 行的单体。

**方案 B（结构体拆分）**：先将 `RendererService` 的方法按职责拆分为独立的辅助结构体，每个结构体只持有自己需要的依赖。`RendererService` 退化为一个薄薄的协调层。

最终采用**先 B 后 A** 的策略——先拆结构体解决核心问题，再移包解决分类问题。

### 拆分结果

| 结构体 | 文件 | 职责 | 核心依赖 |
|--------|------|------|----------|
| `TemplateDataBuilder` | `data_builder.go` | 将 domain 数据转换为模板视图数据 | 各 Repository |
| `PageRenderer` | `page_renderer.go` | 渲染 HTML 页面并写入文件系统 | `ThemeRenderer`, `TemplateDataBuilder` |
| `SeoGenerator` | `seo_generator.go` | 生成 RSS、Sitemap、Robots.txt | 无外部依赖 |
| `SearchIndexBuilder` | `search_builder.go` | 生成搜索索引 JSON | 无外部依赖 |
| `AssetManager` | `asset_manager.go` | 静态资源复制、LESS 编译 | `ThemeConfigService` |

`RendererService`（后更名为 `Engine`）变为纯粹的组合器：

```go
type Engine struct {
    dataBuilder   *TemplateDataBuilder
    pageRenderer  *PageRenderer
    seoGenerator  *SeoGenerator
    searchBuilder *SearchIndexBuilder
    assetManager  *AssetManager
    // ...
}

func (s *Engine) RenderAll(ctx context.Context) error {
    // 1. 复制资源
    // 2. 构建模板数据
    // 3. 并发渲染页面
    // 4. 并发生成 SEO 和搜索文件
}
```

外部 API 保持完全不变——`SetMenuRepo()`、`SetTheme()`、`RenderAll()` 的签名不动，消费方无感知。

---

## 五、第四步：渲染引擎独立成包

### 动机

结构体拆分后，`service/` 目录仍有 23 个文件。虽然内部职责已经清晰，但从包级别看，CRUD 服务和 SSG 引擎仍然共享 `package service` 的命名空间。渲染模块的常量 `DirOutput`、类型 `rssFeed` 等对 `PostService` 毫无意义，却出现在同一个包的自动补全列表中。

### 执行

创建 `internal/engine/` 包，使用 `git mv` 迁移 9 个文件：

```
service/renderer_service.go    → engine/engine.go
service/renderer_data.go       → engine/data_builder.go
service/renderer_pages.go      → engine/page_renderer.go
service/renderer_assets.go     → engine/asset_manager.go
service/renderer_seo.go        → engine/seo_generator.go
service/renderer_search.go     → engine/search_builder.go
service/renderer_constants.go  → engine/constants.go
service/types_render.go        → engine/types.go
service/theme_config_service.go→ engine/theme_config_service.go
```

关键的命名决策：

- `RendererService` → `Engine`：在 `engine` 包内，`engine.Engine` 比 `engine.RendererService` 更符合 Go 的命名习惯（避免包名与类型名重复语义）
- `NewRendererService()` → `New()`：Go 惯例是当构造函数返回包的核心类型时，使用 `New()`

### 消费方更新

5 个文件需要调整导入：

```go
// Before
renderer := service.NewRendererService(appDir, postRepo, themeRepo, settingRepo)

// After
renderer := engine.New(appDir, postRepo, themeRepo, settingRepo)
```

`ThemeConfigService` 一并迁入 `engine/`��因为它只服务于 `AssetManager` 和 `TemplateDataBuilder`。如果留在 `service/` 中，`engine` 包就需要反向依赖 `service` 包，形成不合理的依赖方向。

### 最终目录对比

```
# Before (23 files)              # After
internal/service/                 internal/engine/        (9 files, SSG 渲染)
├── renderer_service.go          ├── engine.go
├── renderer_data.go             ├── data_builder.go
├── renderer_pages.go            ├── page_renderer.go
├── renderer_assets.go           ├── asset_manager.go
├── renderer_seo.go              ├── seo_generator.go
├── renderer_search.go           ├── search_builder.go
├── renderer_constants.go        ├── constants.go
├── types_render.go              ├── types.go
├── renderer_fallback.go         └── theme_config_service.go
├── theme_config_service.go
├── post_service.go              internal/service/       (14 files, 纯业务)
├── tag_service.go               ├── post_service.go
├── deploy_service.go            ├── tag_service.go
├── ...                          ├── deploy_service.go
                                 └── ...
```

---

## 六、验证与回顾

每一阶段完成后都通过 `go build ./...` 和 `go vet ./...` 验证。全部使用 `git mv` 迁移文件，保留完整的 git blame 历史。

回顾整次重构，核心收益可以归纳为：

1. **依赖方向清晰**：`facade → engine / service → domain`，不存在反向或循环依赖
2. **关注点隔离**：CRUD 逻辑与 SSG 渲染在包级别物理隔离，IDE 自动补全不再混杂无关符号
3. **可测试性提升**：`SeoGenerator` 和 `SearchIndexBuilder` 没有外部依赖，可以直接单元测试
4. **外部 API 零破坏**：所有 Facade 层的公开方法签名不变，前端无需任何调整

最后一点经验值得记录：**结构体拆分先于目录迁移**。如果直接把一个 2000 行的 `RendererService` 搬到新包里，它仍然是一个 2000 行的单体——只是换了个地址。先在原地完成职责拆分，确认每个结构体的边界和依赖关系，再整体迁移到独立包，才是正确的顺序。重构的目标不是让目录树变好看，而是让每一行代码都出现在它应该出现的地方。
