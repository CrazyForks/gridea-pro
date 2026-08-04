package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeployManifestRoundTrip(t *testing.T) {
	appDir := t.TempDir()
	target := DeployTargetKey("sftp", "example.com", 22, "/srv/www")

	m := LoadDeployManifest(appDir, target)
	if m.Known() {
		t.Fatal("全新清单不应声称掌握远端现状")
	}

	m.Set("index.html", "sha-a")
	m.Set("post/1/index.html", "sha-b")
	if err := m.Save(); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	reloaded := LoadDeployManifest(appDir, target)
	if !reloaded.Known() {
		t.Fatal("重新加载后应掌握远端现状")
	}
	if sha, ok := reloaded.Get("index.html"); !ok || sha != "sha-a" {
		t.Errorf("index.html 的 SHA = %q (ok=%v)，期望 sha-a", sha, ok)
	}

	reloaded.Delete("index.html")
	if _, ok := reloaded.Get("index.html"); ok {
		t.Error("删除后仍能查到条目")
	}
}

// 不同的远端目标必须各自记账，否则同一站点部署到两处会互相覆盖
func TestDeployManifestIsolatesTargets(t *testing.T) {
	appDir := t.TempDir()

	a := LoadDeployManifest(appDir, DeployTargetKey("sftp", "a.example.com", 22, "/srv/www"))
	a.Set("index.html", "sha-a")
	if err := a.Save(); err != nil {
		t.Fatalf("保存 a 失败: %v", err)
	}

	b := LoadDeployManifest(appDir, DeployTargetKey("sftp", "b.example.com", 22, "/srv/www"))
	if b.Known() {
		t.Error("另一个目标的清单不应读到 a 的内容")
	}

	// 同主机不同目录同样要隔离
	c := LoadDeployManifest(appDir, DeployTargetKey("sftp", "a.example.com", 22, "/srv/other"))
	if c.Known() {
		t.Error("同主机不同远端目录的清单不应互相串用")
	}
}

// 版本不匹配时清单按"不存在"处理：宁可全量重传，也不能拿着不可信的记录去删远端文件
func TestDeployManifestRejectsUnknownVersion(t *testing.T) {
	appDir := t.TempDir()
	target := DeployTargetKey("ftp", "example.com", 21, "/www")

	corrupt := `{"version":999,"target":"` + target + `","entries":{"index.html":"sha"}}`
	if err := os.WriteFile(deployManifestPath(appDir, target), []byte(corrupt), 0o644); err != nil {
		t.Fatalf("写测试清单失败: %v", err)
	}

	m := LoadDeployManifest(appDir, target)
	if m.Known() {
		t.Error("版本不匹配的清单不应被采信")
	}
}

func TestDeployManifestIgnoresCorruptFile(t *testing.T) {
	appDir := t.TempDir()
	target := DeployTargetKey("sftp", "example.com", 22, "/srv/www")

	if err := os.WriteFile(deployManifestPath(appDir, target), []byte("{ not json"), 0o644); err != nil {
		t.Fatalf("写测试清单失败: %v", err)
	}

	m := LoadDeployManifest(appDir, target)
	if m.Known() {
		t.Error("损坏的清单不应被采信")
	}
	if err := m.Save(); err != nil {
		t.Errorf("损坏清单之后应能正常覆盖保存，却返回: %v", err)
	}
}

func TestAppDirFromOutput(t *testing.T) {
	appDir := filepath.Join("/tmp", "site")
	if got := appDirFromOutput(filepath.Join(appDir, "output")); got != appDir {
		t.Errorf("appDirFromOutput = %q，期望 %q", got, appDir)
	}
	if got := appDirFromOutput(filepath.Join(appDir, "output") + string(filepath.Separator)); got != appDir {
		t.Errorf("带尾分隔符时 appDirFromOutput = %q，期望 %q", got, appDir)
	}
}
