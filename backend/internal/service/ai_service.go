package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gridea-pro/backend/internal/domain"
)

// builtInZhipuAPIKeyEncrypted 内置 Zhipu API Key 的 AES-GCM 密文（Base64 编码）
// 加密方式：EncryptKey(plainKey) — 详见下方函数
// 暂未配置内置 Key，用户需在偏好设置 → AI 配置 中填写自己的 Key
const builtInZhipuAPIKeyEncrypted = "XdSkHxsdio1XcIq3Fggd5yKNW7XWhSB2X4s0XYcKZSTuQal3JSmEODaeFAhch49hMmuC8Tf9gAOmmN4VihzANkzYWTYMR859evS5UeY="

const zhipuEndpoint = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
const defaultAIModel = "glm-4-flash"

// AIService AI 功能服务
type AIService struct {
	repo domain.AISettingRepository
}

func NewAIService(repo domain.AISettingRepository) *AIService {
	return &AIService{repo: repo}
}

// deriveEncryptKey 从 App 名称派生加密密钥（16 字节 AES-128）
func deriveEncryptKey() []byte {
	h := sha256.Sum256([]byte("Gridea Pro"))
	return h[:16]
}

// EncryptKey 将明文 API Key 加密为 Base64 密文
// 供开发者配置内置 Key 时使用：将输出值填入 builtInZhipuAPIKeyEncrypted 常量
func EncryptKey(plainKey string) (string, error) {
	key := deriveEncryptKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plainKey), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptBuiltInKey 解密内置 Key，失败或未配置则返回空字符串
func decryptBuiltInKey() string {
	if builtInZhipuAPIKeyEncrypted == "" {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(builtInZhipuAPIKeyEncrypted)
	if err != nil {
		return ""
	}
	key := deriveEncryptKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return ""
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return ""
	}
	return string(plaintext)
}

// getAPIKey 优先使用用户自己的 Key，否则用内置 Key
func (s *AIService) getAPIKey(ctx context.Context) (string, error) {
	setting, err := s.repo.GetAISetting(ctx)
	if err == nil && strings.TrimSpace(setting.ZhipuAPIKey) != "" {
		return strings.TrimSpace(setting.ZhipuAPIKey), nil
	}
	if builtIn := decryptBuiltInKey(); builtIn != "" {
		return builtIn, nil
	}
	return "", errors.New("请在「偏好设置 → AI 配置」中设置 Zhipu API Key")
}

// getModel 获取使用的模型，未配置则用默认值
func (s *AIService) getModel(ctx context.Context) string {
	setting, err := s.repo.GetAISetting(ctx)
	if err == nil && strings.TrimSpace(setting.Model) != "" {
		return strings.TrimSpace(setting.Model)
	}
	return defaultAIModel
}

// chatRequest OpenAI 兼容的请求结构
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateSlug 根据文章标题用 AI 生成 SEO 友好的英文 Slug
func (s *AIService) GenerateSlug(ctx context.Context, title string) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", errors.New("文章标题不能为空")
	}

	apiKey, err := s.getAPIKey(ctx)
	if err != nil {
		return "", err
	}
	model := s.getModel(ctx)

	prompt := fmt.Sprintf(
		"Generate a SEO-friendly English URL slug for this article title.\n"+
			"Rules: lowercase letters and numbers only, words separated by hyphens, 3 to 6 words, no explanation, return ONLY the slug.\n"+
			"Title: %s",
		title,
	)

	reqBody := chatRequest{
		Model:       model,
		Messages:    []chatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   50,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("请求构建失败: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, zhipuEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("请求创建失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("响应读取失败: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", errors.New("AI 未返回结果")
	}

	raw := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	// 清理：只保留字母、数字、连字符
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "", errors.New("生成的 Slug 无效，请重试")
	}
	return result, nil
}
