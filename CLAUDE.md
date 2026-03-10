# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gridea Pro is a desktop static blog writing client built with **Wails** (Go backend + Vue 3 frontend). It manages articles (Markdown), tags, categories, menus, themes, and supports multi-platform deployment (GitHub, Vercel, Netlify, SFTP, Gitee, Coding).

## Build & Development Commands

```bash
# Development (hot reload)
wails dev

# Production build
wails build

# Frontend only
cd frontend && npm install && npm run build

# Go build
go build -o build/bin/gridea-pro .

# Go tests
go test ./backend/...

# Frontend lint/format
cd frontend && npm run lint:fix
cd frontend && npm run format
```

**Prerequisites**: Go 1.22+, Node.js 18+, Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## Architecture

### Data Flow

```
Vue 3 Frontend → Wails RPC → Facade Layer → Service Layer → Repository Layer → JSON Files
```

- **No database** — all data (posts, tags, settings, etc.) stored as JSON files in the user's site directory
- **Wails bindings** expose Go facades to the frontend as callable JS functions
- **Events** (`EventsEmit`/`EventsOn`) used for async communication (e.g. `app-site-loaded`, `app-site-reload`, `preview-site`, `publish-site`)

### Backend (`backend/`)

| Layer | Path | Purpose |
|-------|------|---------|
| **Facade** | `internal/facade/` | Wails-bound API surface; wraps services |
| **Service** | `internal/service/` | Business logic |
| **Repository** | `internal/repository/` | JSON file I/O with in-memory caching |
| **Domain** | `internal/domain/` | Core data models (Post, Tag, Category, Setting, etc.) |
| **Engine** | `internal/engine/` | Static site generation (HTML rendering, SEO, assets) |
| **Deploy** | `internal/deploy/` | Git and Vercel deployment handlers |
| **Render** | `internal/render/` | Template renderers (Pongo2/Jinja2, EJS, Go templates) |

Key files:
- `backend/pkg/boot/boot.go` — App bootstrap, Wails menu, all service bindings
- `backend/internal/facade/app.go` — `AppServices` struct, `NewAppServices()`, `InvalidateAllCaches()`
- `backend/internal/engine/engine.go` — Core render orchestrator

### Frontend (`frontend/src/`)

| Area | Path | Purpose |
|------|------|---------|
| **Views** | `views/` | Page components (articles, settings, theme, etc.) |
| **Stores** | `stores/` | Pinia stores (`site.ts` is the main data store) |
| **Wails bindings** | `wailsjs/go/` | Auto-generated TS types and facade method stubs |
| **Components** | `components/` | Reusable UI (Monaco editor, Radix UI wrappers) |
| **Locales** | `locales/` | i18n JSON files (12 languages, `zh-CN` is primary) |

Key tech: Vue 3 Composition API, Vue Router, Pinia, Tailwind CSS 4, Radix Vue, Monaco Editor, Vite

### Wails TS Model Binding

`frontend/src/wailsjs/go/models.ts` is **manually maintained** (not auto-generated in this project). When Go domain structs change, update the corresponding TS classes here. Watch out for type mapping — Go `map[string]map[string]any` maps to TS `Record<string, Record<string, any>>`.

## Key Conventions

- **Language**: Code comments and UI strings in Chinese (zh-CN primary). Commit messages in Chinese.
- **Setting storage**: `PlatformConfigs` uses `map[string]map[string]any` (not `json.RawMessage`) with custom `MarshalJSON` to preserve field order matching the UI form layout.
- **Repository caching**: Repositories cache data in memory; call `Invalidate()` to force re-read from disk. `InvalidateAllCaches()` in `facade/app.go` handles all repos.
- **Facade pattern**: Every service exposed to frontend is wrapped in a facade (e.g., `SettingFacade` wraps `SettingService`). Facades handle error conversion to user-friendly messages.
