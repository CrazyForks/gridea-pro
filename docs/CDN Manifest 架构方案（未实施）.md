# CDN 部署上传架构重构：Manifest 方案

> **状态：未实施** — 方案已设计完成，待后续评审后决定是否实施。

## Context

当前 CDN 部署上传的问题：`UploadMediaForDeploy()` 按原始目录结构上传文件（`post-images/cover.png` → CDN 仓库中也是 `post-images/cover.png`），用户配置的 `savePath` 模板（如 `{year}/{month}/{filename}{.suffix}`）仅用于测试上传，部署时完全没有使用。这导致 CDN 仓库的文件结构与博客仓库完全一致，CDN 形同虚设——没有真正利用 savePath 的分类归档能力。

**目标**：部署时也使用 savePath 模板为每个媒体文件生成独立的 CDN 路径，同时通过 manifest 机制保证旧文件 URL 不失效。

## 核心设计：Manifest 映射

引入一个持久化的 JSON manifest 文件，记录每个本地媒体文件到 CDN 路径的映射关系。

**为什么需要 manifest？**
- savePath 模板包含时间变量（`{year}/{month}`），每次部署解析结果可能不同
- 必须保证已上传文件的 CDN 路径不变，否则旧文章的图片链接会断裂
- HTML 渲染时需要知道每个文件的精确 CDN 路径（不再是简单的前缀替换）

**Manifest 文件格式**（`config/cdn_manifest.json`）：
```json
{
  "version": 1,
  "entries": {
    "post-images/cover.png": {
      "cdnPath": "2026/03/cover.png",
      "contentSha": "da39a3ee5e6b...",
      "createdAt": "2026-03-14T10:30:00Z"
    }
  }
}
```
- Key = 本地相对路径（如 `post-images/cover.png`）
- `cdnPath` = 通过 savePath 模板解析后的 CDN 远程路径，一旦生成不再改变
- `contentSha` = git blob SHA1（复用现有 `gitBlobSHA()` 函数），用于去重

## 部署流程变更

**当前流程**：RenderAll() → CDN Upload → Platform Deploy

**新流程**：
1. **BuildManifest** — 扫描媒体目录，对新文件用 savePath 解析 CDN 路径，已有文件保持原路径
2. **RenderAll** — HTML 后处理器读取 manifest，按文件逐一替换 URL（而非前缀替换）
3. **UploadFromManifest** — 仅上传新增/变更的文件到 CDN 仓库
4. **Platform Deploy** — 不变

## 实现步骤

### 1. 新建 Domain Model
**新文件**: `backend/internal/domain/cdn_manifest.go`

```go
type CdnManifestEntry struct {
    CdnPath    string `json:"cdnPath"`
    ContentSha string `json:"contentSha"`
    CreatedAt  string `json:"createdAt"`
}

type CdnManifest struct {
    Version int                         `json:"version"`
    Entries map[string]CdnManifestEntry `json:"entries"`
}

type CdnManifestRepository interface {
    GetManifest(ctx context.Context) (CdnManifest, error)
    SaveManifest(ctx context.Context, manifest CdnManifest) error
}
```

### 2. 新建 Repository
**新文件**: `backend/internal/repository/cdn_manifest_repo.go`

完全参照 `cdn_setting_repo.go` 的模式：`mu sync.RWMutex` + `cache` + `loaded` + `loadIfNeeded()`。存储路径：`config/cdn_manifest.json`。

### 3. 修改 CdnUploadService
**文件**: `backend/internal/service/cdn_upload_service.go`

- 构造函数新增 `cdnManifestRepo` 参数
- 新增 `BuildManifest(ctx, appDir, logger)` 方法：
  - 加载现有 manifest（不存在则初始化空 manifest）
  - 扫描 `post-images/`、`images/`、`media/` 三个目录
  - 对每个文件计算 `gitBlobSHA`：
    - 已有条目且 SHA 相同 → 跳过（保持原 cdnPath）
    - 已有条目但 SHA 不同 → 重新用 savePath 解析新 cdnPath
    - 无条目 → 用 savePath 解析新 cdnPath，新增条目
  - 保存更新后的 manifest
  - 返回需要上传的文件列表（内部存储或返回）
- 新增 `UploadFromManifest(ctx, appDir, logger)` 方法：
  - 仅上传 BuildManifest 标记为需要上传的文件
  - 复用现有 `uploadToGitHub()`，`errgroup` 5 并发
- 保留 `TestUpload()` 不变（测试上传不经过 manifest）

### 4. 修改 HTML 后处理器
**文件**: `backend/internal/engine/html_postprocessor.go`

- `HtmlPostProcessor` 新增 `manifest *domain.CdnManifest` 字段
- `NewHtmlPostProcessor` 新增 manifest 参数
- `rewriteCdnURLs()` 改为 manifest 逐文件替换：
  ```go
  // 遍历 manifest.Entries
  for localPath, entry := range manifest.Entries {
      oldPath := "/" + localPath          // e.g. "/post-images/cover.png"
      newPath := cdnBase + "/" + entry.CdnPath  // e.g. "https://cdn.jsdelivr.net/.../2026/03/cover.png"
      // 在 src="...", href="..." 等属性中替换
  }
  ```
- 保留旧的前缀替换逻辑作为 fallback（manifest 为空时使用）

### 5. 修改 Engine
**文件**: `backend/internal/engine/engine.go`

- `Engine` 结构体新增 `cdnManifestRepo` 字段 + `SetCdnManifestRepo()` setter
- `RenderAll()` 中加载 manifest 并传入 `NewHtmlPostProcessor()`（约第 170-188 行）

### 6. 修改部署流程
**文件**: `backend/internal/service/deploy_service.go`

`DeployToRemote()` 改为：
```
1. BuildManifest     ← 新增，在 RenderAll 之前
2. RenderAll         ← 现在使用 manifest 做 URL 重写
3. UploadFromManifest ← 替换原来的 UploadMediaForDeploy
4. Platform Deploy   ← 不变
```

### 7. 服务注册
**文件**: `backend/internal/facade/app.go`

- 创建 `cdnManifestRepo`，传入 `CdnUploadService` 构造函数
- 调用 `rendererService.SetCdnManifestRepo(cdnManifestRepo)`
- 在 `UpdateAppDir()` 中同步更新

## 关键文件清单

| 操作 | 文件路径 |
|------|---------|
| 新建 | `backend/internal/domain/cdn_manifest.go` |
| 新建 | `backend/internal/repository/cdn_manifest_repo.go` |
| 修改 | `backend/internal/service/cdn_upload_service.go` |
| 修改 | `backend/internal/engine/html_postprocessor.go` |
| 修改 | `backend/internal/engine/engine.go` |
| 修改 | `backend/internal/service/deploy_service.go` |
| 修改 | `backend/internal/facade/app.go` |

前端无需改动（savePath 配置 UI 已存在）。

## 边界情况处理

- **用户修改 savePath**：已有文件保持原 cdnPath 不变，仅新文件使用新模板
- **文件内容变更**（同名但内容不同）：SHA 不同，生成新 cdnPath，重新上传
- **本地文件被删除**：manifest 中保留旧条目（避免已渲染页面的 CDN URL 失效）
- **首次部署**（无 manifest）：所有文件作为新文件处理，全部用 savePath 解析并上传
- **manifest 文件丢失**：等同首次部署，全部重新处理

## 验证

1. 配置 savePath 为 `{year}/{month}/{filename}{.suffix}`，部署后检查 CDN 仓库文件路径是否为 `2026/03/cover.png` 而非 `post-images/cover.png`
2. 部署后检查生成的 HTML 中图片 URL 是否正确指向 CDN 路径
3. 修改 savePath 后再次部署，确认旧文件路径不变、新文件使用新路径
4. 删除 `cdn_manifest.json` 后重新部署，确认所有文件重新生成 CDN 路径并上传
5. 本地预览仍使用本地路径（不受影响）
