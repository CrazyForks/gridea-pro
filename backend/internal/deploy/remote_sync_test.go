package deploy

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeRemoteFS 是 RemoteFS 的内存实现，同时记录所有改动型调用，
// 便于断言"站点目录本身有没有被动过"。
type fakeRemoteFS struct {
	files map[string][]byte
	dirs  map[string]bool

	removedDirs  []string
	removedFiles []string
	uploaded     []string
	readDirFails bool
	onUpload     func(remotePath string)
	removeErr    func(remotePath string) error
	uploadErr    func(remotePath string) error
}

func newFakeRemoteFS(dirs ...string) *fakeRemoteFS {
	f := &fakeRemoteFS{files: map[string][]byte{}, dirs: map[string]bool{}}
	for _, d := range dirs {
		f.dirs[d] = true
	}
	return f
}

func (f *fakeRemoteFS) seed(remotePath string, content string) {
	f.files[remotePath] = []byte(content)
	for d := path.Dir(remotePath); d != "/" && d != "."; d = path.Dir(d) {
		f.dirs[d] = true
	}
}

func (f *fakeRemoteFS) ReadDir(dir string) ([]RemoteEntry, error) {
	if f.readDirFails {
		return nil, fmt.Errorf("模拟：服务器不支持列目录")
	}
	if !f.dirs[dir] {
		return nil, fmt.Errorf("目录不存在: %s", dir)
	}
	seen := map[string]bool{}
	var entries []RemoteEntry
	for p := range f.files {
		if path.Dir(p) == dir {
			entries = append(entries, RemoteEntry{Name: path.Base(p)})
		}
	}
	for d := range f.dirs {
		if d != dir && path.Dir(d) == dir && !seen[d] {
			seen[d] = true
			entries = append(entries, RemoteEntry{Name: path.Base(d), IsDir: true})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func (f *fakeRemoteFS) MkdirAll(dir string) error {
	for d := dir; d != "/" && d != "."; d = path.Dir(d) {
		f.dirs[d] = true
	}
	return nil
}

func (f *fakeRemoteFS) Upload(localPath, remotePath string) error {
	if f.onUpload != nil {
		f.onUpload(remotePath)
	}
	if f.uploadErr != nil {
		if err := f.uploadErr(remotePath); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	if !f.dirs[path.Dir(remotePath)] {
		return fmt.Errorf("父目录不存在: %s", path.Dir(remotePath))
	}
	f.files[remotePath] = data
	f.uploaded = append(f.uploaded, remotePath)
	return nil
}

func (f *fakeRemoteFS) Remove(remotePath string) error {
	if f.removeErr != nil {
		if err := f.removeErr(remotePath); err != nil {
			return err
		}
	}
	if _, ok := f.files[remotePath]; !ok {
		return fmt.Errorf("文件不存在 %s: %w", remotePath, os.ErrNotExist)
	}
	delete(f.files, remotePath)
	f.removedFiles = append(f.removedFiles, remotePath)
	return nil
}

func (f *fakeRemoteFS) RemoveDir(dir string) error {
	for p := range f.files {
		if strings.HasPrefix(p, dir+"/") {
			return fmt.Errorf("目录非空: %s", dir)
		}
	}
	for d := range f.dirs {
		if strings.HasPrefix(d, dir+"/") {
			return fmt.Errorf("目录非空: %s", dir)
		}
	}
	delete(f.dirs, dir)
	f.removedDirs = append(f.removedDirs, dir)
	return nil
}

// writeLocal 在 outputDir 下建文件，返回 outputDir
func writeLocal(t *testing.T, outputDir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(outputDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("建目录失败: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
}

func setupLocal(t *testing.T, files map[string]string) (appDir, outputDir string) {
	t.Helper()
	appDir = t.TempDir()
	outputDir = filepath.Join(appDir, "output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("建 output 失败: %v", err)
	}
	writeLocal(t, outputDir, files)
	return appDir, outputDir
}

func syncOnce(t *testing.T, fs RemoteFS, appDir, outputDir, root string) (SyncResult, error) {
	t.Helper()
	m := LoadDeployManifest(appDir, DeployTargetKey("sftp", "example.com", 22, root))
	return SyncTree(context.Background(), fs, SyncOptions{
		LocalDir:   outputDir,
		RemoteRoot: root,
		Manifest:   m,
		Logger:     func(string) {},
	})
}

// issue #139 的回归测试：同步全程不能删除或重建站点目录本身，
// 否则 docker bind mount 会挂在旧 inode 上，容器内看到空目录。
func TestSyncTreeKeepsRemoteRootInPlace(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"index.html":      "<h1>v2</h1>",
		"post/a/index.md": "a",
	})

	const root = "/data/site"
	fs := newFakeRemoteFS(root)
	fs.seed(root+"/index.html", "<h1>v1</h1>")

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	for _, d := range fs.removedDirs {
		if d == root {
			t.Fatalf("站点目录 %s 被删除了，docker bind mount 会因此失效", root)
		}
	}
	if !fs.dirs[root] {
		t.Fatalf("站点目录 %s 在同步后消失了", root)
	}
	if got := string(fs.files[root+"/index.html"]); got != "<h1>v2</h1>" {
		t.Errorf("index.html 内容 = %q，期望更新为 v2", got)
	}
	if _, ok := fs.files[root+"/post/a/index.md"]; !ok {
		t.Error("新增的子目录文件没有被上传")
	}
}

func TestSyncTreeSkipsUnchangedFiles(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"index.html": "hello",
		"about.html": "about",
	})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	first, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	if first.Uploaded != 2 {
		t.Fatalf("首次同步上传数 = %d，期望 2", first.Uploaded)
	}

	second, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("二次同步失败: %v", err)
	}
	if second.Uploaded != 0 {
		t.Errorf("内容未变时上传数 = %d，期望 0", second.Uploaded)
	}
	if second.Skipped != 2 {
		t.Errorf("跳过数 = %d，期望 2", second.Skipped)
	}
}

func TestSyncTreeReuploadsWhenContentChanged(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{"index.html": "v1"})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}

	writeLocal(t, outputDir, map[string]string{"index.html": "v2-changed"})

	second, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("二次同步失败: %v", err)
	}
	if second.Uploaded != 1 {
		t.Errorf("内容变化后上传数 = %d，期望 1", second.Uploaded)
	}
	if got := string(fs.files[root+"/index.html"]); got != "v2-changed" {
		t.Errorf("远端内容 = %q，期望 v2-changed", got)
	}
}

// 远端文件被手动删掉时，即使清单认为传过，也必须重传
func TestSyncTreeReuploadsWhenRemoteFileMissing(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{"index.html": "hello"})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	delete(fs.files, root+"/index.html")

	second, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("二次同步失败: %v", err)
	}
	if second.Uploaded != 1 {
		t.Errorf("远端文件缺失时上传数 = %d，期望 1", second.Uploaded)
	}
}

// 只删自己传过的文件；用户手动放在站点目录里的文件必须原样保留
func TestSyncTreeDeletesOnlyItsOwnOrphans(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"index.html": "home",
		"old.html":   "will be deleted",
	})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)
	fs.seed(root+"/CNAME", "blog.example.com")
	fs.seed(root+"/.well-known/acme-challenge/token", "challenge")
	// 这个文件既不在清单里、也不在保留名单里：它单独验证第一道防线
	//（删除集合只取自清单）确实在起作用，而不是靠保留名单兜住的
	fs.seed(root+"/user-notes.txt", "用户自己传的文件")

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}

	if err := os.Remove(filepath.Join(outputDir, "old.html")); err != nil {
		t.Fatalf("删除本地文件失败: %v", err)
	}

	second, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("二次同步失败: %v", err)
	}
	if second.Deleted != 1 {
		t.Errorf("清理数 = %d，期望 1", second.Deleted)
	}
	if _, ok := fs.files[root+"/old.html"]; ok {
		t.Error("本地已删除的 old.html 仍留在远端")
	}
	if _, ok := fs.files[root+"/CNAME"]; !ok {
		t.Error("用户的 CNAME 被误删了")
	}
	if _, ok := fs.files[root+"/.well-known/acme-challenge/token"]; !ok {
		t.Error("用户的 .well-known 文件被误删了")
	}
	if _, ok := fs.files[root+"/user-notes.txt"]; !ok {
		t.Error("不在保留名单里的用户文件被误删了——删除集合不该来自远端实际内容")
	}
}

// 清单缺失（首次部署 / 换机器）时宁可留孤儿也不能删
func TestSyncTreeWithoutManifestDeletesNothing(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{"index.html": "home"})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)
	fs.seed(root+"/legacy.html", "unknown origin")

	result, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if result.Deleted != 0 {
		t.Errorf("无清单时清理数 = %d，期望 0", result.Deleted)
	}
	if _, ok := fs.files[root+"/legacy.html"]; !ok {
		t.Error("无清单时不应删除任何远端文件")
	}
}

// 构建产物为空多半意味着渲染出了问题，此时同步会把线上站点删空，必须提前中止
func TestSyncTreeAbortsWhenLocalEmpty(t *testing.T) {
	appDir, outputDir := setupLocal(t, nil)
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)
	fs.seed(root+"/index.html", "live site")

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err == nil {
		t.Fatal("构建产物为空时应当报错中止")
	}
	if _, ok := fs.files[root+"/index.html"]; !ok {
		t.Error("中止后远端站点被动过了")
	}
}

// 列目录失败只应影响"远端是否存在"的判断，不能中断部署
func TestSyncTreeToleratesReadDirFailure(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{"index.html": "hello"})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)
	fs.readDirFails = true

	result, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("列目录失败不应中断部署，却返回: %v", err)
	}
	if result.Uploaded != 1 {
		t.Errorf("上传数 = %d，期望 1", result.Uploaded)
	}
}

func TestSyncTreePrunesEmptyDirs(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"index.html":     "home",
		"tag/go/index.h": "go",
	})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(outputDir, "tag")); err != nil {
		t.Fatalf("删除本地目录失败: %v", err)
	}

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("二次同步失败: %v", err)
	}
	if fs.dirs[root+"/tag/go"] {
		t.Error("清空后的远端目录 tag/go 没有被回收")
	}
	if !fs.dirs[root] {
		t.Error("站点根目录不应被回收")
	}
}

// 用户中途取消部署时要立刻停手，并且已经传上去的文件必须落进清单，
// 否则下次部署会重复上传，而且它们永远进不了删除集合，成为删不掉的孤儿。
func TestSyncTreeStopsOnCancelAndKeepsProgress(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"a.html": "a", "b.html": "b", "c.html": "c", "d.html": "d",
	})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	ctx, cancel := context.WithCancel(context.Background())
	fs.onUpload = func(string) { cancel() } // 第一个文件传完就取消

	m := LoadDeployManifest(appDir, DeployTargetKey("sftp", "example.com", 22, root))
	_, err := SyncTree(ctx, fs, SyncOptions{
		LocalDir:   outputDir,
		RemoteRoot: root,
		Manifest:   m,
		Logger:     func(string) {},
	})
	if err == nil {
		t.Fatal("取消后应返回错误")
	}
	if len(fs.uploaded) != 1 {
		t.Errorf("取消后仍上传了 %d 个文件，期望停在 1 个", len(fs.uploaded))
	}

	reloaded := LoadDeployManifest(appDir, DeployTargetKey("sftp", "example.com", 22, root))
	if !reloaded.Known() {
		t.Fatal("取消前已上传的文件没有被记进清单")
	}
	uploadedRel := path.Base(fs.uploaded[0])
	if _, ok := reloaded.Get(uploadedRel); !ok {
		t.Errorf("清单里缺少已上传的 %s", uploadedRel)
	}
}

// 删除失败（连接断了、权限不足）时必须保留清单条目：抹掉条目意味着这个文件
// 再也进不了任何一次删除集合，会变成永久孤儿，而且用户还会收到"清理成功"的假象。
func TestSyncTreeKeepsManifestEntryWhenDeleteFails(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"index.html": "home",
		"old.html":   "will be deleted",
	})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	if err := os.Remove(filepath.Join(outputDir, "old.html")); err != nil {
		t.Fatalf("删除本地文件失败: %v", err)
	}

	fs.removeErr = func(string) error { return fmt.Errorf("connection reset by peer") }
	result, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("删除失败不应中断部署: %v", err)
	}
	if result.Deleted != 0 {
		t.Errorf("删除失败却计入 Deleted = %d，期望 0", result.Deleted)
	}
	if result.DeleteFailed != 1 {
		t.Errorf("DeleteFailed = %d，期望 1", result.DeleteFailed)
	}

	// 关键：条目必须还在清单里，下次才有机会重试
	reloaded := LoadDeployManifest(appDir, DeployTargetKey("sftp", "example.com", 22, root))
	if _, ok := reloaded.Get("old.html"); !ok {
		t.Fatal("删除失败后条目被移出清单，该文件已成为永久孤儿")
	}

	// 恢复后下一次部署应当真的把它删掉
	fs.removeErr = nil
	retry, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("重试同步失败: %v", err)
	}
	if retry.Deleted != 1 {
		t.Errorf("恢复后重试的清理数 = %d，期望 1", retry.Deleted)
	}
	if _, ok := fs.files[root+"/old.html"]; ok {
		t.Error("重试后文件仍未被删除")
	}
}

// 远端文件本来就不存在时，目的已经达成，条目应当从清单里移除而不是无限重试
func TestSyncTreeForgetsEntryWhenRemoteAlreadyGone(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{
		"index.html": "home",
		"old.html":   "will be deleted",
	})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}
	if err := os.Remove(filepath.Join(outputDir, "old.html")); err != nil {
		t.Fatalf("删除本地文件失败: %v", err)
	}
	// 远端文件被别人先删了
	delete(fs.files, root+"/old.html")

	result, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if result.DeleteFailed != 0 {
		t.Errorf("文件本就不存在不应计入失败，DeleteFailed = %d", result.DeleteFailed)
	}

	reloaded := LoadDeployManifest(appDir, DeployTargetKey("sftp", "example.com", 22, root))
	if _, ok := reloaded.Get("old.html"); ok {
		t.Error("远端已不存在的文件仍留在清单里，会被无限重试")
	}
}

// 清单被外部改坏、混入越界路径时，删除操作绝不能打到站点根目录之外
func TestSyncTreeRejectsEscapingManifestPaths(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{"index.html": "home"})
	const root = "/srv/www/site"

	fs := newFakeRemoteFS(root)
	fs.seed("/srv/secret.conf", "机密配置")
	fs.seed("/srv/www/other-site/index.html", "别的站点")

	target := DeployTargetKey("sftp", "example.com", 22, root)
	m := LoadDeployManifest(appDir, target)
	m.Set("index.html", "stale-sha")
	m.Set("../../secret.conf", "sha")
	m.Set("../other-site/index.html", "sha")
	m.Set("/srv/secret.conf", "sha")
	if err := m.Save(); err != nil {
		t.Fatalf("保存清单失败: %v", err)
	}

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	if _, ok := fs.files["/srv/secret.conf"]; !ok {
		t.Error("站点根目录之外的文件被删除了")
	}
	if _, ok := fs.files["/srv/www/other-site/index.html"]; !ok {
		t.Error("相邻站点的文件被删除了")
	}
	for _, d := range fs.removedDirs {
		if d == "/srv" || d == "/srv/www" {
			t.Errorf("站点根目录的祖先 %s 被尝试删除", d)
		}
	}
}

// 上传失败后清单里不能留着该文件的旧指纹，否则下次会误判为"没变过"而跳过
func TestSyncTreeDropsManifestEntryWhenUploadFails(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{"index.html": "v1"})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}

	writeLocal(t, outputDir, map[string]string{"index.html": "v2"})
	fs.uploadErr = func(string) error { return fmt.Errorf("磁盘写满") }
	if _, err := syncOnce(t, fs, appDir, outputDir, root); err == nil {
		t.Fatal("上传失败时应返回错误")
	}

	reloaded := LoadDeployManifest(appDir, DeployTargetKey("sftp", "example.com", 22, root))
	if _, ok := reloaded.Get("index.html"); ok {
		t.Error("上传失败后清单仍留着旧指纹，下次部署会误判为未变化而跳过")
	}

	// 恢复后必须重传
	fs.uploadErr = nil
	retry, err := syncOnce(t, fs, appDir, outputDir, root)
	if err != nil {
		t.Fatalf("重试同步失败: %v", err)
	}
	if retry.Uploaded != 1 {
		t.Errorf("恢复后上传数 = %d，期望 1", retry.Uploaded)
	}
}

// 上次中断留下的 .gridea-part 半成品要被清掉，否则它公网可访问且没人管得着
func TestSyncTreeCleansStalePartFiles(t *testing.T) {
	appDir, outputDir := setupLocal(t, map[string]string{"index.html": "home"})
	const root = "/srv/www"
	fs := newFakeRemoteFS(root)

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("首次同步失败: %v", err)
	}

	fs.seed(root+"/index.html"+uploadPartSuffix, "half written")
	fs.seed(root+"/post/a.html"+uploadPartSuffix, "half written")

	if _, err := syncOnce(t, fs, appDir, outputDir, root); err != nil {
		t.Fatalf("二次同步失败: %v", err)
	}

	for _, p := range []string{root + "/index.html" + uploadPartSuffix, root + "/post/a.html" + uploadPartSuffix} {
		if _, ok := fs.files[p]; ok {
			t.Errorf("残留的临时文件 %s 没有被清理", p)
		}
	}
	if _, ok := fs.files[root+"/index.html"]; !ok {
		t.Error("正式文件被误删了")
	}
}

func TestIsSafeRelPath(t *testing.T) {
	safe := []string{"index.html", "post/a/index.html", ".well-known/x", "a.b.c"}
	for _, p := range safe {
		if !isSafeRelPath(p) {
			t.Errorf("%q 应被判为安全", p)
		}
	}

	unsafe := []string{
		"", ".", "..",
		"../secret", "../../secret", "a/../../b",
		"/etc/passwd", "/srv/x",
		"./a", "a//b", "a/./b", "a/",
	}
	for _, p := range unsafe {
		if isSafeRelPath(p) {
			t.Errorf("%q 应被判为不安全", p)
		}
	}
}

func TestIsPreservedRemotePath(t *testing.T) {
	preserved := []string{
		"CNAME",
		".well-known/acme-challenge/abc123",
		".htaccess",
		"web.config",
		"ads.txt",
		"BingSiteAuth.xml",
		"google1a2b3c4d.html",
		"baidu_verify_code-xyz.html",
	}
	for _, p := range preserved {
		if !isPreservedRemotePath(p) {
			t.Errorf("%q 应当被保留", p)
		}
	}

	deletable := []string{
		"index.html",
		"post/hello/index.html",
		"styles/main.css",
		"robots.txt",
		"sitemap.xml",
		// 主题里以 google 开头的资源不在根目录，不该被误保留
		"styles/google-fonts.css",
	}
	for _, p := range deletable {
		if isPreservedRemotePath(p) {
			t.Errorf("%q 不应被保留", p)
		}
	}
}
