package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gridea-pro/backend/internal/engine"
)

// uploadPartSuffix 是上传中间态文件的后缀，见 sftpFS.Upload
const uploadPartSuffix = ".gridea-part"

// RemoteEntry 是远端目录中的一项
type RemoteEntry struct {
	Name  string
	IsDir bool
}

// RemoteFS 是远端文件系统的最小抽象，由 SFTP / FTP 各自实现。
// 所有路径都是远端绝对路径，一律使用 / 分隔。
type RemoteFS interface {
	// ReadDir 列出目录内容。目录不存在时应返回错误，由调用方按"空目录"处理。
	ReadDir(dir string) ([]RemoteEntry, error)
	// MkdirAll 递归创建目录；目录已存在不应报错。
	MkdirAll(dir string) error
	// Upload 把本地文件写入远端 remotePath，覆盖同名文件。
	// 实现方应尽量保证对读取方的原子性（先写临时文件再 rename）。
	Upload(localPath, remotePath string) error
	// Remove 删除一个远端文件
	Remove(remotePath string) error
	// RemoveDir 删除一个远端空目录；目录非空时应返回错误（由调用方忽略）。
	RemoveDir(dir string) error
}

// SyncOptions 描述一次远端同步
type SyncOptions struct {
	// LocalDir 是构建输出目录
	LocalDir string
	// RemoteRoot 是远端站点根目录。同步全程只在其内部增删文件，
	// 目录本身既不删除也不重命名——这是 docker bind mount 能持续生效的前提（issue #139）。
	RemoteRoot string
	// Manifest 记录上次成功部署的远端状态；Entries 为空表示远端现状未知。
	Manifest *DeployManifest
	// Logger 输出进度
	Logger LogFunc
}

// SyncResult 汇总一次同步的结果
type SyncResult struct {
	Uploaded int
	Skipped  int
	Deleted  int
	// DeleteFailed 是清理阶段删除失败、已留待下次部署重试的文件数
	DeleteFailed int
}

// SyncTree 把本地构建产物增量同步到远端。
//
// 与"清空远端再全量上传"和"上传到 staging 再整目录 rename"两种旧做法相比，
// 它全程不动 RemoteRoot 目录本身，只在其内部增删文件。这既保住了 docker bind mount
// 按 inode 绑定的挂载（issue #139），也顺带让用户手动放在站点目录里的文件不再被抹掉。
//
// 三个集合的算法：
//   - 上传 = 本地有，且（清单没记过 ∪ 指纹与清单不符 ∪ 远端实际不存在）
//   - 跳过 = 本地有，且指纹与清单一致，且远端确实还在
//   - 删除 = 清单记过、本地已没有，且不在保留名单里
//
// 删除集合取自清单而非远端实际内容，因此我们只会删自己传上去的文件；
// 清单缺失（首次部署 / 换了机器）时删除集合为空，宁可留下孤儿也不误删。
func SyncTree(ctx context.Context, fs RemoteFS, opts SyncOptions) (SyncResult, error) {
	var result SyncResult
	logger := opts.Logger
	if logger == nil {
		logger = func(string) {}
	}

	if opts.Manifest == nil {
		return result, fmt.Errorf("内部错误：部署清单未初始化")
	}

	local, err := collectLocalFiles(opts.LocalDir)
	if err != nil {
		return result, fmt.Errorf("扫描本地构建产物失败: %w", err)
	}
	if len(local) == 0 {
		return result, fmt.Errorf("构建产物为空，已中止部署以免清空线上站点")
	}

	if err := fs.MkdirAll(opts.RemoteRoot); err != nil {
		return result, fmt.Errorf("创建远端目录 %s 失败: %w", opts.RemoteRoot, err)
	}

	manifest := opts.Manifest

	// 列出远端实际内容，用于发现"清单说传过、远端其实没有"的文件。
	//
	// 清单为空时这一步没有意义（所有文件都会因"清单没记过"而上传），可以省掉——
	// 在 FTP 上每个目录都是一次 LIST 往返，站点根目录下若有用户自己的大目录会白等很久。
	//
	// 枚举失败则退回完全信任清单。注意这是有代价的降级：一旦某个文件在远端被手动删除，
	// 而清单又认为它已经传过，这次就补不回来，站点会持续 404，直到内容变化触发重传。
	var remote map[string]struct{}
	remoteKnown := false
	if manifest.Known() {
		remote, remoteKnown = listRemoteFiles(fs, opts.RemoteRoot)
		if !remoteKnown {
			logger("提示：无法列出远端目录内容，本次仅依据本地清单判断需要上传的文件；" +
				"若远端有文件被手动删除，本次可能不会补传")
		} else {
			cleanupStaleParts(fs, opts.RemoteRoot, remote, logger)
		}
	}
	plan := buildSyncPlan(local, manifest, remote, remoteKnown)
	result.Skipped = len(local) - len(plan.Upload)

	if len(plan.Upload) == 0 {
		logger("远端内容与本地一致，无需上传")
	} else {
		logger(fmt.Sprintf("需要上传 %d 个文件（跳过 %d 个未变化的文件）", len(plan.Upload), result.Skipped))
	}

	// 上传阶段
	createdDirs := map[string]struct{}{opts.RemoteRoot: {}}
	for _, rel := range plan.Upload {
		select {
		case <-ctx.Done():
			if err := manifest.Save(); err != nil {
				logger(fmt.Sprintf("提示：部署清单保存失败（%v），下次部署会重新上传全部文件", err))
			}
			return result, ctx.Err()
		default:
		}

		remoteFile := path.Join(opts.RemoteRoot, rel)
		if err := ensureRemoteDir(fs, path.Dir(remoteFile), createdDirs); err != nil {
			if saveErr := manifest.Save(); saveErr != nil {
				logger(fmt.Sprintf("提示：部署清单保存失败（%v），下次部署会重新上传全部文件", saveErr))
			}
			return result, err
		}

		localFile := filepath.Join(opts.LocalDir, filepath.FromSlash(rel))
		if err := fs.Upload(localFile, remoteFile); err != nil {
			// 上传失败时远端可能残留半成品甚至被清空，清单里的旧指纹已不可信；
			// 移除该条目，强制下次部署重传这个文件。
			manifest.Delete(rel)
			if saveErr := manifest.Save(); saveErr != nil {
				logger(fmt.Sprintf("提示：部署清单保存失败（%v），下次部署会重新上传全部文件", saveErr))
			}
			return result, fmt.Errorf("上传 %s 失败: %w", rel, err)
		}

		manifest.Set(rel, local[rel].SHA)
		result.Uploaded++
		if result.Uploaded%20 == 0 {
			logger(fmt.Sprintf("已上传 %d/%d 个文件...", result.Uploaded, len(plan.Upload)))
			// 阶段性落盘：中断后下次部署不必重传这些文件
			if err := manifest.Save(); err != nil {
				logger(fmt.Sprintf("提示：部署清单保存失败（%v），下次部署会重新上传全部文件", err))
			}
		}
	}

	if err := manifest.Save(); err != nil {
		logger(fmt.Sprintf("提示：部署清单保存失败（%v），下次部署会重新上传全部文件", err))
	}

	// 清理阶段：删除上次传过、这次已经不存在的文件
	if len(plan.Delete) > 0 {
		logger(fmt.Sprintf("正在清理 %d 个已删除的旧文件...", len(plan.Delete)))
		var pruned []string
		for _, rel := range plan.Delete {
			select {
			case <-ctx.Done():
				if err := manifest.Save(); err != nil {
					logger(fmt.Sprintf("提示：部署清单保存失败（%v），下次部署会重新上传全部文件", err))
				}
				return result, ctx.Err()
			default:
			}

			err := fs.Remove(path.Join(opts.RemoteRoot, rel))
			switch {
			case err == nil, errors.Is(err, os.ErrNotExist):
				// 删掉了，或者本来就不在——两种情况目的都已达成，可以忘掉这个条目
			default:
				// 连接中断、权限不足、只读文件系统等：文件还在远端。
				// 此时绝不能把条目从清单里抹掉，否则它再也进不了任何一次删除集合，
				// 会变成谁都清不掉的永久孤儿（已删除文章的页面会一直留在线上）。
				logger(fmt.Sprintf("提示：删除 %s 失败（%v），已保留记录，下次部署会重试", rel, err))
				result.DeleteFailed++
				continue
			}

			manifest.Delete(rel)
			pruned = append(pruned, rel)
			result.Deleted++
		}
		if err := manifest.Save(); err != nil {
			logger(fmt.Sprintf("提示：部署清单保存失败（%v），下次部署会重新上传全部文件", err))
		}
		pruneEmptyDirs(fs, opts.RemoteRoot, pruned)

		if result.DeleteFailed > 0 {
			logger(fmt.Sprintf("⚠️ 有 %d 个旧文件未能删除，仍留在服务器上，下次部署会自动重试", result.DeleteFailed))
		}
	}

	return result, nil
}

// syncPlan 是一次同步的行动清单，路径均为相对 RemoteRoot 的 / 分隔路径
type syncPlan struct {
	Upload []string
	Delete []string
}

// buildSyncPlan 计算上传与删除集合。
// remoteKnown 为 false 时表示远端实际内容不可知，此时不因"远端缺失"追加上传。
func buildSyncPlan(local map[string]engine.FileEntry, manifest *DeployManifest, remote map[string]struct{}, remoteKnown bool) syncPlan {
	var plan syncPlan

	for rel, entry := range local {
		if !isSafeRelPath(rel) {
			continue
		}
		known, recorded := manifest.Get(rel)
		switch {
		case !recorded:
			// 清单没记过：可能是新文件，也可能是清单丢失，一律上传
		case entry.SHA == "" || known == "":
			// 指纹未知（读到 v1 旧清单或哈希计算失败）：保守重传
		case known != entry.SHA:
			// 内容变了
		case remoteKnown && !containsRemote(remote, rel):
			// 清单说传过，但远端实际没有——被手动删了或上次传到一半中断
		default:
			continue
		}
		plan.Upload = append(plan.Upload, rel)
	}

	// 删除集合只从清单取：没被我们传过的文件（用户自己放的 CNAME、.well-known 等）
	// 永远进不了这个集合。清单为空时集合自然为空，即"现状未知就不删"。
	for _, rel := range manifest.Paths() {
		// 清单是本地 JSON，可能被外部改动或被历史版本写坏。
		// path.Join 会把 "../.." 解析掉，一个坏条目就足以让删除操作打到站点目录之外，
		// 因此在这里挡住——engine 侧的 CleanOrphans 同样有这道防御。
		if !isSafeRelPath(rel) {
			continue
		}
		if _, stillExists := local[rel]; stillExists {
			continue
		}
		if isPreservedRemotePath(rel) {
			continue
		}
		plan.Delete = append(plan.Delete, rel)
	}

	sort.Strings(plan.Upload)
	sort.Strings(plan.Delete)
	return plan
}

func containsRemote(remote map[string]struct{}, rel string) bool {
	_, ok := remote[rel]
	return ok
}

// isSafeRelPath 校验一个相对路径能否安全地拼到远端根目录之下。
//
// 站点根目录之外的一切都不属于我们，误删的后果是用户服务器上的真实文件消失，
// 所以任何形式的越界都在这里挡掉：绝对路径、含 .. 的路径、以及未归一化的写法
// （"a//b"、"./a"），后者会让同一个文件在清单里有多种写法，破坏比对。
func isSafeRelPath(rel string) bool {
	if rel == "" || rel == "." || rel == ".." {
		return false
	}
	if strings.HasPrefix(rel, "/") {
		return false
	}
	if path.Clean(rel) != rel {
		return false
	}
	// Clean 后仍以 ../ 开头说明确实指向父级
	return !strings.HasPrefix(rel, "../")
}

// cleanupStaleParts 清掉上传过程中被中断留下的临时文件。
//
// SFTP 上传走"先写 <目标>.gridea-part 再改名"，进程若在两步之间被杀掉，
// 这个半成品就会留在站点目录里且公网可访问。它不在部署清单里，
// 常规的孤儿清理够不着，只能在这里按后缀识别。
func cleanupStaleParts(fs RemoteFS, root string, remote map[string]struct{}, logger LogFunc) {
	var stale []string
	for rel := range remote {
		if strings.HasSuffix(rel, uploadPartSuffix) && isSafeRelPath(rel) {
			stale = append(stale, rel)
		}
	}
	if len(stale) == 0 {
		return
	}

	sort.Strings(stale)
	removed := 0
	for _, rel := range stale {
		if err := fs.Remove(path.Join(root, rel)); err == nil {
			delete(remote, rel)
			removed++
		}
	}
	if removed > 0 {
		logger(fmt.Sprintf("已清理 %d 个上次中断残留的临时文件", removed))
	}
}

// collectLocalFiles 扫描构建输出目录，返回 相对路径 → 内容指纹。
//
// 文件清单以磁盘实际内容为准（而非渲染清单），这样用户手动放进 output 的文件
// 也会被正常部署；渲染清单只用作 SHA 缓存，命中时省去一次全文件读取。
func collectLocalFiles(localDir string) (map[string]engine.FileEntry, error) {
	cache, err := engine.LoadPreviousManifest(appDirFromOutput(localDir))
	if err != nil {
		cache = nil
	}

	files := make(map[string]engine.FileEntry)
	err = filepath.Walk(localDir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if name := info.Name(); name == ".git" || name == ".github" {
				return filepath.SkipDir
			}
			return nil
		}
		if name := info.Name(); name == ".DS_Store" || name == ".gitignore" {
			return nil
		}

		rel, err := filepath.Rel(localDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		entry := engine.FileEntry{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
		if cached, ok := cache[rel]; ok && cached.SHA != "" &&
			cached.Size == entry.Size && cached.ModTime == entry.ModTime {
			entry.SHA = cached.SHA
		} else if sha, err := engine.HashFile(p); err == nil {
			entry.SHA = sha
		}

		files[rel] = entry
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// appDirFromOutput 由构建输出目录反推应用数据目录。
// 契约：outputDir 恒为 <appDir>/output，由 DeployService 与 engine 共同保证。
func appDirFromOutput(outputDir string) string {
	return filepath.Dir(filepath.Clean(outputDir))
}

// listRemoteFiles 递归列出远端 root 下的所有文件（相对路径）。
// 第二个返回值为 false 表示无法可靠枚举，调用方应放弃基于"远端是否存在"的判断。
func listRemoteFiles(fs RemoteFS, root string) (map[string]struct{}, bool) {
	files := make(map[string]struct{})
	ok := true

	var walk func(dir, prefix string, depth int)
	walk = func(dir, prefix string, depth int) {
		// 深度上限：防御远端存在符号链接环导致的无限递归
		if depth > 32 {
			ok = false
			return
		}
		entries, err := fs.ReadDir(dir)
		if err != nil {
			// 根目录读不到才算枚举失败；子目录读不到当作空目录，不影响整体判断
			if depth == 0 {
				ok = false
			}
			return
		}
		for _, e := range entries {
			if e.Name == "." || e.Name == ".." || e.Name == "" {
				continue
			}
			rel := e.Name
			if prefix != "" {
				rel = prefix + "/" + e.Name
			}
			if e.IsDir {
				walk(path.Join(dir, e.Name), rel, depth+1)
			} else {
				files[rel] = struct{}{}
			}
		}
	}
	walk(root, "", 0)

	return files, ok
}

// ensureRemoteDir 创建远端目录，已创建过的目录不重复请求
func ensureRemoteDir(fs RemoteFS, dir string, created map[string]struct{}) error {
	if _, ok := created[dir]; ok {
		return nil
	}
	if err := fs.MkdirAll(dir); err != nil {
		return fmt.Errorf("创建远端目录 %s 失败: %w", dir, err)
	}
	created[dir] = struct{}{}
	return nil
}

// pruneEmptyDirs 尝试删除因清理孤儿而变空的目录，由深到浅。
// 非空目录的删除会失败，直接忽略——这里只做尽力而为的收尾。
func pruneEmptyDirs(fs RemoteFS, root string, deleted []string) {
	dirs := make(map[string]struct{})
	for _, rel := range deleted {
		dir := path.Dir(rel)
		for dir != "." && dir != "/" && dir != "" {
			dirs[dir] = struct{}{}
			dir = path.Dir(dir)
		}
	}

	ordered := make([]string, 0, len(dirs))
	for d := range dirs {
		ordered = append(ordered, d)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return strings.Count(ordered[i], "/") > strings.Count(ordered[j], "/")
	})

	for _, d := range ordered {
		_ = fs.RemoveDir(path.Join(root, d))
	}
}

// isPreservedRemotePath 判断一个相对路径是否属于"绝不删除"的文件。
//
// 正常情况下这些文件根本进不了删除集合（我们只删自己传过的），这里是第二道防线：
// 万一历史清单里混入了它们，误删会直接导致自定义域名失效、证书续签失败或站点验证掉线。
func isPreservedRemotePath(rel string) bool {
	if strings.HasPrefix(rel, ".well-known/") {
		return true
	}

	switch rel {
	case "CNAME", // GitHub Pages 自定义域名
		".htaccess", ".user.ini", "web.config", // 服务器配置
		"ads.txt", "app-ads.txt",
		"BingSiteAuth.xml", "sogousiteverification.txt":
		return true
	}

	// 各家搜索引擎的站点验证文件名由平台生成，且必须放在站点根目录，
	// 因此只在根目录做前缀匹配，避免误伤主题里同名前缀的资源
	if !strings.Contains(rel, "/") {
		if strings.HasPrefix(rel, "google") && strings.HasSuffix(rel, ".html") {
			return true
		}
		for _, prefix := range []string{"baidu_verify_", "y_key_", "_verify_"} {
			if strings.HasPrefix(rel, prefix) {
				return true
			}
		}
	}
	return false
}
