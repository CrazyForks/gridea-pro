//go:build unix

package deploy

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// osRemoteFS 用本地文件系统实现 RemoteFS，用于验证同步逻辑在真实文件系统上的行为
type osRemoteFS struct{ t *testing.T }

func (f *osRemoteFS) ReadDir(dir string) ([]RemoteEntry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]RemoteEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, RemoteEntry{Name: it.Name(), IsDir: it.IsDir()})
	}
	return entries, nil
}

func (f *osRemoteFS) MkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }

func (f *osRemoteFS) Upload(localPath, remotePath string) error {
	in, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(remotePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func (f *osRemoteFS) Remove(remotePath string) error { return os.Remove(remotePath) }

func (f *osRemoteFS) RemoveDir(dir string) error { return os.Remove(dir) }

func inodeOf(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s 失败: %v", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("当前平台拿不到 inode，跳过")
	}
	return uint64(st.Ino)
}

// issue #139 的根本契约：站点目录的 inode 必须在多轮部署后保持不变。
//
// docker 的 bind mount 在容器启动时就把挂载点绑到了当时那个 inode 上，之后与目录名无关。
// 旧实现"上传到 staging 再整目录 rename"会让同名目录换成新 inode，
// 容器里于是看到一个空目录，nginx 返回 403，只有重启容器才能恢复。
func TestSyncTreeKeepsRemoteRootInode(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"index.html":            "v1",
		"post/hello/index.html": "hello",
	})

	remoteRoot := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(remoteRoot, 0o755); err != nil {
		t.Fatalf("建远端目录失败: %v", err)
	}
	// 模拟容器启动那一刻解析到的 inode
	mountedInode := inodeOf(t, remoteRoot)

	fs := &osRemoteFS{t: t}
	target := DeployTargetKey("sftp", "example.com", remoteRoot)

	sync := func() SyncResult {
		t.Helper()
		m := LoadDeployManifest(appDir, target)
		res, err := SyncTree(context.Background(), fs, SyncOptions{
			LocalDir:   outputDir,
			RemoteRoot: remoteRoot,
			Manifest:   m,
			Logger:     func(string) {},
		})
		if err != nil {
			t.Fatalf("同步失败: %v", err)
		}
		return res
	}

	sync()
	if got := inodeOf(t, remoteRoot); got != mountedInode {
		t.Fatalf("首次部署后站点目录 inode 变了（%d → %d），docker 挂载会失效", mountedInode, got)
	}

	// 第二轮：改内容 + 删文件 + 加文件，覆盖上传与清理两条路径
	writeLocal(t, outputDir, map[string]string{
		"index.html":  "v2",
		"newpage.htm": "new",
	})
	if err := os.RemoveAll(filepath.Join(outputDir, "post")); err != nil {
		t.Fatalf("删除本地目录失败: %v", err)
	}

	res := sync()
	if got := inodeOf(t, remoteRoot); got != mountedInode {
		t.Fatalf("二次部署后站点目录 inode 变了（%d → %d），docker 挂载会失效", mountedInode, got)
	}
	if res.Deleted == 0 {
		t.Error("本地已删除的文件没有在远端被清理")
	}

	// 内容确实更新到位，而不是靠"什么都不做"保住 inode
	data, err := os.ReadFile(filepath.Join(remoteRoot, "index.html"))
	if err != nil || string(data) != "v2" {
		t.Errorf("远端 index.html = %q (err=%v)，期望 v2", string(data), err)
	}
	if _, err := os.Stat(filepath.Join(remoteRoot, "post", "hello", "index.html")); !os.IsNotExist(err) {
		t.Error("本地已删除的 post/hello/index.html 仍在远端")
	}
}
