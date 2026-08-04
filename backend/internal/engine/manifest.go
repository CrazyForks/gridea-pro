package engine

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ManifestFileName 记录渲染产物清单的文件名，放在 appDir 根目录下
// 放在 appDir 根目录（而非 output 内）是为了避免被部署到服务器
const ManifestFileName = ".render-manifest.json"

// FileEntry 描述一个渲染产物的内容指纹。
//
// SHA 是文件内容的 sha256（hex）。部署侧用它判断"这个文件跟上次传上去的是否一致"，
// 从而跳过未变化的文件；空字符串表示指纹未知（读到 v1 旧清单时），调用方必须当作"已变化"处理。
//
// Size / ModTime 只用于校验 SHA 缓存是否仍然有效：两者与磁盘上的实际文件一致时，
// 才可以复用上一轮算好的 SHA，避免重复读盘。
type FileEntry struct {
	SHA     string `json:"sha"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"` // Unix 纳秒
}

// RenderManifest 跟踪本次渲染生成的文件及其内容指纹。渲染结束时计算"上次有、本次没"的
// 孤儿文件并删除，既保证已删除文章的 HTML 不会残留（解决 issue #26），
// 也不会误删用户放在 output 目录里的自定义文件（CNAME、ads.txt 等）。
//
// 清单同时作为部署侧的 SHA 缓存：部署时不必再遍历整个 output 重新计算哈希。
type RenderManifest struct {
	mu       sync.Mutex
	files    map[string]FileEntry
	previous map[string]FileEntry
	buildDir string
}

// manifestPayload 是持久化到磁盘的结构。
// Entries 为 v2 格式；Files 是 v1 遗留字段，仅在读取旧清单时使用，不再写入。
type manifestPayload struct {
	Version     int                  `json:"version"`
	GeneratedAt time.Time            `json:"generatedAt"`
	Entries     map[string]FileEntry `json:"entries,omitempty"`
	Files       []string             `json:"files,omitempty"`
}

const manifestVersion = 2

// NewRenderManifest 创建一个新的 manifest，buildDir 用于路径相对化
func NewRenderManifest(buildDir string) *RenderManifest {
	return &RenderManifest{
		files:    make(map[string]FileEntry),
		buildDir: filepath.Clean(buildDir),
	}
}

// NewRenderManifestWithPrevious 在 NewRenderManifest 基础上注入上一轮清单。
// 上一轮的 SHA 会在文件的 size/mtime 均未变化时被复用，从而避免重新读盘计算哈希——
// 对 images/media 这类"拷贝时被跳过"的大文件尤其重要。
func NewRenderManifestWithPrevious(buildDir string, previous map[string]FileEntry) *RenderManifest {
	m := NewRenderManifest(buildDir)
	m.previous = previous
	return m
}

// Track 记录一个已写入的绝对路径，并计算内容指纹。buildDir 外的路径会被静默忽略。
// 并发安全，可由多个 goroutine 同时调用
func (m *RenderManifest) Track(absPath string) {
	if m == nil {
		return
	}
	rel, ok := m.relPath(absPath)
	if !ok {
		return
	}
	m.record(rel, m.fingerprint(rel, absPath, nil))
}

// TrackWithContent 在内容已在内存中时记录文件，直接据此计算 SHA，省去一次回读。
func (m *RenderManifest) TrackWithContent(absPath string, data []byte) {
	if m == nil {
		return
	}
	rel, ok := m.relPath(absPath)
	if !ok {
		return
	}
	m.record(rel, m.fingerprint(rel, absPath, data))
}

func (m *RenderManifest) record(rel string, e FileEntry) {
	m.mu.Lock()
	m.files[rel] = e
	m.mu.Unlock()
}

// fingerprint 计算文件指纹。data 非 nil 时直接用它算 SHA；否则优先复用上一轮
// size/mtime 均未变化的 SHA，都不可用时才读盘。
// 文件无法 stat 时返回零值 entry（SHA 为空 = 未知），调用方据此保守地当作"已变化"。
func (m *RenderManifest) fingerprint(rel, absPath string, data []byte) FileEntry {
	info, err := os.Stat(absPath)
	if err != nil {
		return FileEntry{}
	}
	e := FileEntry{Size: info.Size(), ModTime: info.ModTime().UnixNano()}

	if data != nil {
		e.SHA = fmt.Sprintf("%x", sha256.Sum256(data))
		return e
	}

	// previous 在构造后只读，无需加锁
	prev, ok := m.previous[rel]
	if ok && prev.SHA != "" && prev.Size == e.Size && prev.ModTime == e.ModTime {
		e.SHA = prev.SHA
		return e
	}

	sha, err := HashFile(absPath)
	if err != nil {
		return FileEntry{Size: e.Size, ModTime: e.ModTime}
	}
	e.SHA = sha
	return e
}

// HashFile 流式计算文件的 sha256，避免把大文件整个读进内存
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// relPath 返回规范化的相对路径（使用 / 分隔，跨平台可读）
// 路径不在 buildDir 内时返回 false
func (m *RenderManifest) relPath(absPath string) (string, bool) {
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(m.buildDir, abs)
	if err != nil {
		return "", false
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// WriteFile 等价于 os.WriteFile，但写入成功后会记录路径与内容指纹
func (m *RenderManifest) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	m.TrackWithContent(path, data)
	return nil
}

// CopyFile 拷贝单个文件并记录目标路径
func (m *RenderManifest) CopyFile(src, dst string) error {
	if err := copyFile(src, dst); err != nil {
		return err
	}
	m.Track(dst)
	return nil
}

// CopyDir 递归拷贝目录，跟踪所有被拷贝的文件
func (m *RenderManifest) CopyDir(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("源路径不是目录: %s", src)
	}

	if err := os.MkdirAll(dst, si.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := m.CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := m.CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// Files 返回本次记录的相对路径 → 指纹映射（返回拷贝，调用方可自由修改）
func (m *RenderManifest) Files() map[string]FileEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]FileEntry, len(m.files))
	for k, v := range m.files {
		out[k] = v
	}
	return out
}

// Save 将 manifest 持久化到 appDir/.render-manifest.json
func (m *RenderManifest) Save(appDir string) error {
	payload := manifestPayload{
		Version:     manifestVersion,
		GeneratedAt: time.Now().UTC(),
		Entries:     m.Files(),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(appDir, ManifestFileName), data, 0644)
}

// LoadPreviousManifest 读取 appDir 下的上次清单。
// 若不存在返回 nil, nil —— 调用方据此判断是否为首次渲染。
//
// 兼容 v1 旧清单：v1 只记路径不记指纹，读出来的 entry 的 SHA 为空，
// 表示"内容未知"，使用方（增量部署）必须保守地当作已变化处理。
func LoadPreviousManifest(appDir string) (map[string]FileEntry, error) {
	data, err := os.ReadFile(filepath.Join(appDir, ManifestFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var payload manifestPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}
	if len(payload.Entries) > 0 {
		return payload.Entries, nil
	}
	out := make(map[string]FileEntry, len(payload.Files))
	for _, f := range payload.Files {
		out[f] = FileEntry{}
	}
	return out, nil
}

// CleanOrphans 删除"上次有、本次没"的文件。清理完成后
// 删除过程中变空的目录（从深到浅）
// 仅操作 buildDir 内的文件，永远不触碰 buildDir 外的路径
func CleanOrphans(buildDir string, previous map[string]FileEntry, current *RenderManifest) int {
	buildDir = filepath.Clean(buildDir)
	currentFiles := current.Files()

	var orphans []string
	for f := range previous {
		if _, kept := currentFiles[f]; !kept {
			orphans = append(orphans, f)
		}
	}
	if len(orphans) == 0 {
		return 0
	}

	removed := 0
	dirsToCheck := make(map[string]struct{})
	for _, rel := range orphans {
		abs := filepath.Join(buildDir, filepath.FromSlash(rel))
		// 防御：绝对路径必须仍在 buildDir 内
		absClean, err := filepath.Abs(abs)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(absClean, buildDir+string(filepath.Separator)) {
			continue
		}
		if err := os.Remove(absClean); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				continue
			}
		} else {
			removed++
		}
		// 记录需检查是否变空的父目录（不包括 buildDir 本身）
		dir := filepath.Dir(absClean)
		for strings.HasPrefix(dir, buildDir+string(filepath.Separator)) {
			dirsToCheck[dir] = struct{}{}
			dir = filepath.Dir(dir)
		}
	}

	// 按深度从深到浅删除空目录。非空目录 os.Remove 会失败，静默忽略
	dirs := make([]string, 0, len(dirsToCheck))
	for d := range dirsToCheck {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], string(filepath.Separator)) >
			strings.Count(dirs[j], string(filepath.Separator))
	})
	for _, d := range dirs {
		_ = os.Remove(d)
	}

	return removed
}
