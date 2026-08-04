package deploy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
)

// deployManifestVersion 是清单格式版本。版本不匹配时清单按"不存在"处理，
// 部署降级为全量上传且不删除任何远端文件——宁可留孤儿，也不误删。
const deployManifestVersion = 1

// DeployManifest 记录"上一次成功部署到某个远端目标时，我们放了哪些文件上去、内容各是什么"。
//
// 它与 engine 的渲染清单回答的是不同问题：渲染清单描述本地构建产物有什么，
// 这份清单描述远端应该有什么。两者做差集才能得出"要传哪些、要删哪些"。
//
// 删除集合只从这份清单里取，是"永不误删用户文件"的核心保证：用户自己传到站点目录的
// CNAME、.well-known/、搜索引擎验证文件从未被我们记录过，因此永远不会进入删除集合。
type DeployManifest struct {
	mu   sync.Mutex
	path string

	Version int               `json:"version"`
	Target  string            `json:"target"`
	Entries map[string]string `json:"entries"` // 相对路径（/ 分隔）→ sha256 hex
}

// deployManifestPath 按部署目标算出清单文件路径。
// 同一个站点可以同时部署到多个远端，因此按 平台+主机+远端目录 分键，互不覆盖。
func deployManifestPath(appDir, target string) string {
	sum := sha256.Sum256([]byte(target))
	return filepath.Join(appDir, fmt.Sprintf(".deploy-manifest-%x.json", sum[:6]))
}

// DeployTargetKey 构造部署目标的唯一标识。
//
// remotePath 必须归一化后入键：用户把 /var/www/html 改成 /var/www/html/ 属于同一个目标，
// 若因此换了键，上一份清单就此失联，那批文件会变成谁也删不掉的孤儿。
// 端口同样是目标的一部分——同一主机上两个不同端口的站点若共用一个键，
// 会把对方的文件算进自己的删除集合，那是会真删线上文件的。
func DeployTargetKey(platform, server string, port int, remotePath string) string {
	return fmt.Sprintf("%s|%s:%d|%s", platform, server, port, path.Clean(remotePath))
}

// LoadDeployManifest 读取指定目标的部署清单。
// 文件不存在、解析失败或版本不匹配时返回一个 Entries 为空的清单——
// 调用方据此判断"远端现状未知"，此时应当只上传、不删除。
func LoadDeployManifest(appDir, target string) *DeployManifest {
	m := &DeployManifest{
		path:    deployManifestPath(appDir, target),
		Version: deployManifestVersion,
		Target:  target,
		Entries: make(map[string]string),
	}

	data, err := os.ReadFile(m.path)
	if err != nil {
		return m
	}

	var loaded DeployManifest
	if err := json.Unmarshal(data, &loaded); err != nil {
		return m
	}
	if loaded.Version != deployManifestVersion || loaded.Target != target {
		return m
	}
	if loaded.Entries != nil {
		m.Entries = loaded.Entries
	}
	return m
}

// Known 返回清单是否掌握远端现状。为 false 时调用方必须放弃删除孤儿。
func (m *DeployManifest) Known() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Entries) > 0
}

// Get 查询某个相对路径上次部署的 SHA
func (m *DeployManifest) Get(rel string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sha, ok := m.Entries[rel]
	return sha, ok
}

// Set 记录一次成功上传
func (m *DeployManifest) Set(rel, sha string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Entries[rel] = sha
}

// Delete 记录一次成功删除
func (m *DeployManifest) Delete(rel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Entries, rel)
}

// Paths 返回清单中记录的所有相对路径，按字典序排列
func (m *DeployManifest) Paths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.Entries))
	for p := range m.Entries {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Save 持久化清单。
//
// 调用方应当在上传/删除的每个阶段结束后都保存一次，而不是只在全部成功后保存：
// 部署中断时已经传上去的文件必须被记住，否则下次部署会把它们当成"没传过"而重复上传，
// 更糟的是它们会永远进不了删除集合，变成删不掉的孤儿。
func (m *DeployManifest) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	payload := struct {
		Version int               `json:"version"`
		Target  string            `json:"target"`
		Entries map[string]string `json:"entries"`
	}{Version: deployManifestVersion, Target: m.Target, Entries: m.Entries}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(m.path, data)
}

// writeFileAtomic 先写同目录临时文件再 rename 覆盖目标。
//
// 清单在一次部署里会被反复重写（每传若干文件保存一次），直接 os.WriteFile 意味着
// 每次都存在一个"已截断、未写完"的窗口。用户在部署中途强退 App 恰好撞上这个窗口，
// 留下的半截 JSON 会让下次加载失败：部署清单失效只是丢掉孤儿追踪，
// 渲染清单失效却会让引擎降级为清空整个输出目录，连用户自己放进去的文件一起删掉。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
