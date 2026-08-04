package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"gridea-pro/backend/internal/domain"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SftpProvider 实现了 SFTP 文件上传部署策略
type SftpProvider struct {
	// knownHostsPath 用于 TOFU 形式的 HostKey 校验；为空时走内存级 TOFU（降级）。
	// 通过 NewSftpProviderWithKnownHosts 注入，生产路径应为 AppConfigDir/known_hosts。
	knownHostsPath string
}

// NewSftpProvider 创建默认 SftpProvider（无 known_hosts 持久化，仅进程内 TOFU）。
// 生产路径请用 NewSftpProviderWithKnownHosts。
func NewSftpProvider() *SftpProvider {
	return &SftpProvider{}
}

// NewSftpProviderWithKnownHosts 注入 known_hosts 文件路径，启用跨会话的 HostKey 校验。
func NewSftpProviderWithKnownHosts(knownHostsPath string) *SftpProvider {
	return &SftpProvider{knownHostsPath: knownHostsPath}
}

// Deploy 实现 Provider 接口
// 流程：SSH 连接 → SFTP 客户端 → 按部署清单增量同步（站点目录原地更新）
func (p *SftpProvider) Deploy(ctx context.Context, outputDir string, setting *domain.Setting, logger LogFunc) error {
	logger("🚀 开始准备 SFTP 部署...")

	// 1. 验证配置
	server := setting.Server()
	if server == "" {
		return fmt.Errorf(domain.ErrSftpConfigMissing)
	}

	username := setting.Username()
	if username == "" {
		return fmt.Errorf(domain.ErrSftpConfigMissing)
	}

	port := 22
	if p := setting.Port(); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			port = v
		}
	}

	remotePath := setting.RemotePath()
	if remotePath == "" {
		remotePath = "/var/www/html"
	}

	// 2. 构建 SSH 认证方式
	authMethods, err := p.buildAuthMethods(setting)
	if err != nil {
		return err
	}
	if len(authMethods) == 0 {
		return fmt.Errorf(domain.ErrSftpConfigMissing)
	}

	// 3. SSH 连接：使用 TOFU 形式的 HostKey 校验替代 InsecureIgnoreHostKey。
	//    首次连接会把指纹写入 known_hosts；后续任何指纹变化都会被硬拒绝（防 MITM）。
	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: TrustOnFirstUseHostKeyCallback(p.knownHostsPath, logger),
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", server, port)
	logger(fmt.Sprintf("正在连接 %s ...", addr))

	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer conn.Close()

	logger("SSH 连接成功")

	// 4. 创建 SFTP 客户端
	client, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("SFTP 客户端创建失败: %w", err)
	}
	defer client.Close()

	// 5. 增量同步：站点目录全程原地，只在其内部增删文件。
	//    早期版本先上传到 staging 再整目录 rename，虽然切换是原子的，
	//    却让远端目录换了 inode，把 docker bind mount 挂在旧 inode 上的站点打成空目录（issue #139）。
	appDir := appDirFromOutput(outputDir)
	manifest := LoadDeployManifest(appDir, DeployTargetKey("sftp", server, port, remotePath))
	if !manifest.Known() {
		logger("未找到该目标的部署记录，本次将完整上传，且不清理远端可能存在的旧文件")
	}

	result, err := SyncTree(ctx, &sftpFS{client: client}, SyncOptions{
		LocalDir:   outputDir,
		RemoteRoot: remotePath,
		Manifest:   manifest,
		Logger:     logger,
	})
	if err != nil {
		return err
	}

	logger(fmt.Sprintf("✅ SFTP 部署成功！上传 %d 个 / 跳过 %d 个 / 清理 %d 个，目标目录 %s",
		result.Uploaded, result.Skipped, result.Deleted, remotePath))
	return nil
}

// sftpFS 把 sftp.Client 适配成 RemoteFS
type sftpFS struct {
	client *sftp.Client
}

func (f *sftpFS) ReadDir(dir string) ([]RemoteEntry, error) {
	infos, err := f.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]RemoteEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, RemoteEntry{Name: info.Name(), IsDir: info.IsDir()})
	}
	return entries, nil
}

func (f *sftpFS) MkdirAll(dir string) error { return f.client.MkdirAll(dir) }

func (f *sftpFS) Remove(remotePath string) error { return f.client.Remove(remotePath) }

func (f *sftpFS) RemoveDir(dir string) error { return f.client.RemoveDirectory(dir) }

// Upload 先写同目录下的临时文件再改名，让读取方要么看到旧版本、要么看到新版本，
// 不会读到写了一半的内容。
//
// 改名分两级降级：优先用 OpenSSH 的 posix-rename 扩展（可直接覆盖目标）；
// 服务器不支持时退回"先删目标再改名"；仍失败则直接覆盖写入目标文件。
// 越往后原子性越弱，但都能完成部署——不能因为服务器缺个扩展就让用户发不了站。
func (f *sftpFS) Upload(localPath, remotePath string) error {
	tmpPath := remotePath + uploadPartSuffix
	if err := f.write(localPath, tmpPath, true); err != nil {
		return err
	}

	if err := f.client.PosixRename(tmpPath, remotePath); err == nil {
		return nil
	}

	_ = f.client.Remove(remotePath)
	if err := f.client.Rename(tmpPath, remotePath); err == nil {
		return nil
	}

	// 走到这里说明目标文件已经被删掉了，且改名两次都没成功。
	// 最后一级降级直接覆盖写入目标；失败也不再清理目标文件——
	// 留下一个写坏的文件，也好过留下一个空洞让站点 404。
	_ = f.client.Remove(tmpPath)
	return f.write(localPath, remotePath, false)
}

// write 把本地文件写到远端。cleanupOnError 决定写失败时是否删除写了一半的目标文件：
// 写临时文件时该清（否则残留垃圾），直接写正式文件时不清（清了站点就少一个文件）。
func (f *sftpFS) write(localPath, remotePath string, cleanupOnError bool) error {
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	remote, err := f.client.Create(remotePath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(remote, local); err != nil {
		remote.Close()
		if cleanupOnError {
			_ = f.client.Remove(remotePath)
		}
		return err
	}
	return remote.Close()
}

// buildAuthMethods 根据配置构建 SSH 认证方式
func (p *SftpProvider) buildAuthMethods(setting *domain.Setting) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// 私钥认证
	if pk := setting.PrivateKey(); pk != "" {
		var keyData []byte
		if strings.HasPrefix(pk, "-----BEGIN") {
			// 内联 PEM 内容
			keyData = []byte(pk)
		} else {
			// 文件路径
			data, err := os.ReadFile(pk)
			if err != nil {
				return nil, fmt.Errorf("读取私钥文件失败: %w", err)
			}
			keyData = data
		}

		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	// 密码认证
	if pw := setting.Password(); pw != "" {
		methods = append(methods, ssh.Password(pw))
	}

	return methods, nil
}
