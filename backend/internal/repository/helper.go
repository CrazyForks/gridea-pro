package repository

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// Helper functions for JSON DB

// LoadJSONFile 读取并解析 JSON 文件
func LoadJSONFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// SaveJSONFile saves data to a JSON file atomically.
// It writes to a temp file first, flushes to disk, and then renames to target.
func SaveJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data, 0644)
}

// SaveJSONFileIdempotent saves data to a JSON file atomically, but only if content changes.
func SaveJSONFileIdempotent(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	// Read existing file to compare
	if existingData, err := os.ReadFile(path); err == nil {
		if string(existingData) == string(data) {
			return nil // Content matches, skip write
		}
	}

	return WriteFileAtomic(path, data, 0644)
}

// WriteFileAtomic writes data to a file atomically by writing to a temp file and renaming.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create temp file in the same directory to ensure atomic rename works (same FS)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(filename)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName) // Clean up temp file if rename fails

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}

	// Ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	// 在 rename 前打上 self-write 标记。rename 完成后 fsnotify 几乎立刻就会投递事件，
	// ResourceWatcher 据此过滤"app 保存 → watcher → 再次渲染"的自激循环。
	// 外部编辑器（不走本函数）的改动不会触发 WriteGate，watcher 仍会照常响应。
	DefaultWriteGate.MarkSelfWrite(filename)

	// Atomic rename
	return os.Rename(tmpName, filename)
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// FileMutex 管理对应文件的读写锁
// 简单实现：使用全局 map 或 sync.Map 存储每个文件的锁？
// 或者更简单：每个 Repository 实例持有一个 Global Lock for that resource type.
// Gridea Pro 是单用户桌面应用，通常只会有一个实例在运行。
// 为了简化，我们在每个具体 Repository struct 中使用 RWMutex 即可。
