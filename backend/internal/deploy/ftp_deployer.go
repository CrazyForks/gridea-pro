package deploy

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"gridea-pro/backend/internal/domain"

	"github.com/jlaffaye/ftp"
)

// FtpProvider 实现了 FTP 文件上传部署策略
type FtpProvider struct{}

// NewFtpProvider 创建 FtpProvider
func NewFtpProvider() *FtpProvider {
	return &FtpProvider{}
}

// FTP 模式常量：对应 setting.ftpMode 字段。
const (
	ftpModePlain         = "ftp"           // 明文（不安全），仅为兼容老配置保留
	ftpModeExplicitTLS   = "ftps-explicit" // 推荐：先建立 TCP，再 AUTH TLS 升级
	ftpModeImplicitTLS   = "ftps-implicit" // 首包就是 TLS（通常 990 端口）
	plainFTPInsecureWarn = "⚠️ 警告：当前使用明文 FTP。用户名、密码和所有上传内容将以明文形式在网络上传输，\n" +
		"   任何可以嗅探链路的中间节点（公共 WiFi / 运营商 / 被攻陷的路由器）都能直接获取。\n" +
		"   强烈建议切换到 SFTP 或 FTPS（在平台设置里选择 \"ftps-explicit\"）。"
)

// Deploy 实现 Provider 接口
// 流程：FTP 连接 → 登录 → 按部署清单增量同步（站点目录原地更新）
func (p *FtpProvider) Deploy(ctx context.Context, outputDir string, setting *domain.Setting, logger LogFunc) error {
	logger("🚀 开始准备 FTP 部署...")

	// 1. 验证配置
	server := setting.Server()
	if server == "" {
		return fmt.Errorf(domain.ErrSftpConfigMissing)
	}

	username := setting.Username()
	if username == "" {
		return fmt.Errorf(domain.ErrSftpConfigMissing)
	}

	password := setting.Password()
	if password == "" {
		return fmt.Errorf(domain.ErrSftpConfigMissing)
	}

	port := 21
	if p := setting.Port(); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			port = v
		}
	}

	remotePath := setting.RemotePath()
	if remotePath == "" {
		remotePath = "/"
	}

	// 2. FTP 连接
	addr := fmt.Sprintf("%s:%d", server, port)
	mode := setting.FtpMode()
	if mode == "" {
		mode = ftpModePlain
	}
	logger(fmt.Sprintf("正在连接 %s (模式: %s) ...", addr, mode))

	dialOpts := []ftp.DialOption{ftp.DialWithTimeout(15 * time.Second)}
	switch mode {
	case ftpModeExplicitTLS, ftpModeImplicitTLS:
		tlsCfg := &tls.Config{
			ServerName:         server,
			InsecureSkipVerify: setting.AllowInsecureTLS(),
		}
		if tlsCfg.InsecureSkipVerify {
			logger("⚠️ 已启用\"允许不安全 TLS\"，服务端证书不会被校验，仅建议 NAS 自签场景使用")
		}
		if mode == ftpModeExplicitTLS {
			dialOpts = append(dialOpts, ftp.DialWithExplicitTLS(tlsCfg))
		} else {
			dialOpts = append(dialOpts, ftp.DialWithTLS(tlsCfg))
		}
	default:
		// 明文 FTP：给用户醒目警告，凭证与内容将以明文传输
		logger(plainFTPInsecureWarn)
	}

	conn, err := ftp.Dial(addr, dialOpts...)
	if err != nil {
		return fmt.Errorf("FTP 连接失败: %w", err)
	}
	defer conn.Quit()

	// 3. 登录
	if err := conn.Login(username, password); err != nil {
		return fmt.Errorf("FTP 登录失败: %w", err)
	}

	logger("FTP 连接成功")

	// 4. 增量同步：站点目录全程原地，只在其内部增删文件。
	//    早期版本先上传到 staging 再整目录 rename，虽然切换是原子的，
	//    却让远端目录换了 inode，把 docker bind mount 挂在旧 inode 上的站点打成空目录（issue #139）。
	appDir := appDirFromOutput(outputDir)
	manifest := LoadDeployManifest(appDir, DeployTargetKey("ftp", server, remotePath))
	if !manifest.Known() {
		logger("未找到该目标的部署记录，本次将完整上传，且不清理远端可能存在的旧文件")
	}

	result, err := SyncTree(ctx, &ftpFS{conn: conn}, SyncOptions{
		LocalDir:   outputDir,
		RemoteRoot: remotePath,
		Manifest:   manifest,
		Logger:     logger,
	})
	if err != nil {
		return err
	}

	logger(fmt.Sprintf("✅ FTP 部署成功！上传 %d 个 / 跳过 %d 个 / 清理 %d 个，目标目录 %s",
		result.Uploaded, result.Skipped, result.Deleted, remotePath))
	return nil
}

// ftpFS 把 ftp.ServerConn 适配成 RemoteFS
type ftpFS struct {
	conn *ftp.ServerConn
}

func (f *ftpFS) ReadDir(dir string) ([]RemoteEntry, error) {
	list, err := f.conn.List(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]RemoteEntry, 0, len(list))
	for _, e := range list {
		// 部分服务器在 LIST 响应里返回全路径，统一取基名
		name := path.Base(e.Name)
		if name == "." || name == ".." || name == "/" {
			continue
		}
		entries = append(entries, RemoteEntry{Name: name, IsDir: e.Type == ftp.EntryTypeFolder})
	}
	return entries, nil
}

// MkdirAll 递归创建远程目录。FTP 没有 mkdir -p，只能逐级试探：
// 能 CWD 进去就说明已存在，否则先建父目录再建自己。
func (f *ftpFS) MkdirAll(dir string) error {
	if dir == "/" || dir == "." || dir == "" {
		return nil
	}
	if err := f.conn.ChangeDir(dir); err == nil {
		_ = f.conn.ChangeDir("/")
		return nil
	}
	if err := f.MkdirAll(path.Dir(dir)); err != nil {
		return err
	}
	// 并发或竞态下目录可能已被建出来，此时 MKD 报错但结果正确，用 CWD 复核
	if err := f.conn.MakeDir(dir); err != nil {
		if cwdErr := f.conn.ChangeDir(dir); cwdErr != nil {
			return err
		}
		_ = f.conn.ChangeDir("/")
	}
	return nil
}

// Upload 直接 STOR 覆盖目标文件。
//
// 这里不像 SFTP 那样走"临时文件 + rename"：FTP 的 RNFR/RNTO 在不少服务器上受限或被禁用，
// 一旦改名失败就要回退重传，反而比直接覆盖更容易失败。STOR 本身就是覆盖语义。
func (f *ftpFS) Upload(localPath, remotePath string) error {
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	return f.conn.Stor(remotePath, local)
}

func (f *ftpFS) Remove(remotePath string) error { return f.conn.Delete(remotePath) }

func (f *ftpFS) RemoveDir(dir string) error { return f.conn.RemoveDir(dir) }
