package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func sha256Hex(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

func TestManifestRecordsContentHash(t *testing.T) {
	buildDir := t.TempDir()
	m := NewRenderManifest(buildDir)

	content := []byte("<h1>hello</h1>")
	if err := m.WriteFile(filepath.Join(buildDir, "index.html"), content, 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	entry, ok := m.Files()["index.html"]
	if !ok {
		t.Fatal("index.html 未被记录")
	}
	if entry.SHA != sha256Hex(string(content)) {
		t.Errorf("SHA = %q，与内容不符", entry.SHA)
	}
	if entry.Size != int64(len(content)) {
		t.Errorf("Size = %d，期望 %d", entry.Size, len(content))
	}
	if entry.ModTime == 0 {
		t.Error("ModTime 未被记录")
	}
}

func TestManifestSaveLoadRoundTrip(t *testing.T) {
	appDir := t.TempDir()
	buildDir := filepath.Join(appDir, "output")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}

	m := NewRenderManifest(buildDir)
	if err := m.WriteFile(filepath.Join(buildDir, "a.html"), []byte("a"), 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if err := m.Save(appDir); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	loaded, err := LoadPreviousManifest(appDir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if loaded["a.html"].SHA != sha256Hex("a") {
		t.Errorf("往返后 SHA = %q，与写入时不符", loaded["a.html"].SHA)
	}
}

// v1 清单只有路径没有指纹，读出来 SHA 必须为空，
// 这样部署侧才会保守地把这些文件当作"已变化"全部重传，而不是误判为一致而漏传。
func TestLoadPreviousManifestReadsV1(t *testing.T) {
	appDir := t.TempDir()
	v1 := `{"version":1,"generatedAt":"2026-01-01T00:00:00Z","files":["index.html","post/a/index.html"]}`
	if err := os.WriteFile(filepath.Join(appDir, ManifestFileName), []byte(v1), 0o644); err != nil {
		t.Fatalf("写 v1 清单失败: %v", err)
	}

	loaded, err := LoadPreviousManifest(appDir)
	if err != nil {
		t.Fatalf("加载 v1 清单失败: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("条目数 = %d，期望 2", len(loaded))
	}
	if entry, ok := loaded["index.html"]; !ok || entry.SHA != "" {
		t.Errorf("v1 条目应存在且 SHA 为空，实际 ok=%v SHA=%q", ok, entry.SHA)
	}
}

func TestLoadPreviousManifestMissingReturnsNil(t *testing.T) {
	loaded, err := LoadPreviousManifest(t.TempDir())
	if err != nil {
		t.Fatalf("清单不存在时不应报错: %v", err)
	}
	if loaded != nil {
		t.Error("清单不存在时应返回 nil，调用方据此判定为首次渲染")
	}
}

// size 与 mtime 都没变时直接复用上一轮的 SHA，避免重新读盘计算——
// 这是 images/media 这类大文件在"拷贝被跳过"时不产生额外 IO 的关键。
func TestManifestReusesPreviousHashWhenUnchanged(t *testing.T) {
	buildDir := t.TempDir()
	target := filepath.Join(buildDir, "asset.bin")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat 失败: %v", err)
	}

	// 故意放一个与真实内容不符的 SHA：若结果等于它，就证明走的是缓存而非重算
	const sentinel = "sentinel-not-a-real-hash"
	previous := map[string]FileEntry{
		"asset.bin": {SHA: sentinel, Size: info.Size(), ModTime: info.ModTime().UnixNano()},
	}

	m := NewRenderManifestWithPrevious(buildDir, previous)
	m.Track(target)

	if got := m.Files()["asset.bin"].SHA; got != sentinel {
		t.Errorf("SHA = %q，期望复用上一轮的 %q", got, sentinel)
	}
}

func TestManifestRecomputesHashWhenSizeChanged(t *testing.T) {
	buildDir := t.TempDir()
	target := filepath.Join(buildDir, "asset.bin")
	if err := os.WriteFile(target, []byte("new-payload"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}

	previous := map[string]FileEntry{
		"asset.bin": {SHA: "stale", Size: 1, ModTime: 1},
	}
	m := NewRenderManifestWithPrevious(buildDir, previous)
	m.Track(target)

	if got := m.Files()["asset.bin"].SHA; got != sha256Hex("new-payload") {
		t.Errorf("SHA = %q，文件已变化时应重新计算", got)
	}
}

// 清单写出的是 v2 格式：只有 entries，没有 v1 的 files 数组
func TestManifestSaveWritesV2(t *testing.T) {
	appDir := t.TempDir()
	buildDir := filepath.Join(appDir, "output")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}

	m := NewRenderManifest(buildDir)
	if err := m.WriteFile(filepath.Join(buildDir, "a.html"), []byte("a"), 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if err := m.Save(appDir); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(appDir, ManifestFileName))
	if err != nil {
		t.Fatalf("读清单失败: %v", err)
	}
	var payload struct {
		Version int                  `json:"version"`
		Entries map[string]FileEntry `json:"entries"`
		Files   []string             `json:"files"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("解析清单失败: %v", err)
	}
	if payload.Version != 2 {
		t.Errorf("version = %d，期望 2", payload.Version)
	}
	if len(payload.Files) != 0 {
		t.Error("v2 清单不应再写 v1 的 files 数组")
	}
	if len(payload.Entries) != 1 {
		t.Errorf("entries 条目数 = %d，期望 1", len(payload.Entries))
	}
}

// 孤儿清理必须能吃 v1 清单读出来的零值 entry
func TestCleanOrphansWorksWithHashlessPrevious(t *testing.T) {
	buildDir := t.TempDir()
	stale := filepath.Join(buildDir, "gone.html")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}

	m := NewRenderManifest(buildDir)
	if err := m.WriteFile(filepath.Join(buildDir, "kept.html"), []byte("new"), 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	previous := map[string]FileEntry{"gone.html": {}, "kept.html": {}}
	if removed := CleanOrphans(buildDir, previous, m); removed != 1 {
		t.Errorf("清理数 = %d，期望 1", removed)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("孤儿文件没有被删除")
	}
	if _, err := os.Stat(filepath.Join(buildDir, "kept.html")); err != nil {
		t.Error("本次仍存在的文件被误删了")
	}
}
