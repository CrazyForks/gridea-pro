package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteGate_HitAndConsume(t *testing.T) {
	g := &WriteGate{paths: map[string]time.Time{}}
	g.MarkSelfWrite("/tmp/a")

	if !g.IsSelfWrite("/tmp/a") {
		t.Fatal("expected hit on fresh mark")
	}
	// 命中即消费：同一路径再次查询应该 miss
	if g.IsSelfWrite("/tmp/a") {
		t.Error("IsSelfWrite should consume the entry")
	}
}

func TestWriteGate_MissUnknownPath(t *testing.T) {
	g := &WriteGate{paths: map[string]time.Time{}}
	g.MarkSelfWrite("/tmp/a")
	if g.IsSelfWrite("/tmp/b") {
		t.Error("other path should miss")
	}
}

func TestWriteGate_TTLExpiry(t *testing.T) {
	g := &WriteGate{paths: map[string]time.Time{}}
	// 手动写入一个已过期的时间戳
	g.paths["/tmp/old"] = time.Now().Add(-3 * time.Second)
	if g.IsSelfWrite("/tmp/old") {
		t.Error("stale entry should not count as self-write")
	}
	// 过期 entry 也会被消费，避免残留误杀后续
	if _, ok := g.paths["/tmp/old"]; ok {
		t.Error("expired entry should be removed after IsSelfWrite")
	}
}

// WriteFileAtomic 调用完后，目标路径立刻被标记成 self-write。
// 这是 ResourceWatcher 过滤自激事件的契约入口。
func TestWriteFileAtomic_MarksSelfWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.md")

	if err := WriteFileAtomic(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}

	if !DefaultWriteGate.IsSelfWrite(path) {
		t.Error("WriteFileAtomic should have marked the target path in DefaultWriteGate")
	}
}
