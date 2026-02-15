package comment

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ValineProvider LeanCloud/Valine 评论提供者
type ValineProvider struct {
	AppID      string
	AppKey     string
	MasterKey  string
	ServerURLs string
}

func (p *ValineProvider) DeleteComment(ctx context.Context, commentID string) error {
	if p.MasterKey == "" {
		return fmt.Errorf("master key is required to delete comments")
	}

	// LeanCloud DELETE /1.1/classes/Comment/:objectId
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment/%s", strings.TrimRight(p.ServerURLs, "/"), commentID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-LC-Id", p.AppID)
	req.Header.Set("X-LC-Key", p.MasterKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete comment: %s", string(body))
	}

	return nil
}

// NewValineProvider 创建 Valine Provider
func NewValineProvider(appID, appKey, masterKey, serverURLs string) *ValineProvider {
	if serverURLs == "" {
		// 默认 LeanCloud API 域名 (主要用于国际版，国内版通常需要自定义域名)
		serverURLs = "https://leancloud.cn"
	}
	return &ValineProvider{
		AppID:      appID,
		AppKey:     appKey,
		MasterKey:  masterKey,
		ServerURLs: serverURLs,
	}
}

// LeanCloud Comment 结构
type leanCloudComment struct {
	ObjectId  string `json:"objectId"`
	Nick      string `json:"nick"`
	Comment   string `json:"comment"`
	Mail      string `json:"mail"`
	Link      string `json:"link"`
	Pid       string `json:"pid"`
	Rid       string `json:"rid"`
	Pnick     string `json:"pnick"`
	Url       string `json:"url"` // Article URL path
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// LeanCloud Response 结构
type leanCloudResponse struct {
	Results []leanCloudComment `json:"results"`
}

func (p *ValineProvider) GetComments(ctx context.Context, articleID string) ([]domain.Comment, error) {
	// Valine 使用 url 作为文章标识
	//构建查询条件: {"url": articleID}
	where := fmt.Sprintf(`{"url":"%s"}`, articleID)
	params := url.Values{}
	params.Add("where", where)
	params.Add("order", "-createdAt")

	apiURL := fmt.Sprintf("%s/1.1/classes/Comment?%s", p.ServerURLs, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	p.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LeanCloud API error: %d %s", resp.StatusCode, string(body))
	}

	var result leanCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	comments := make([]domain.Comment, 0, len(result.Results))
	for _, c := range result.Results {
		comments = append(comments, domain.Comment{
			ID:         c.ObjectId,
			Nickname:   c.Nick,
			URL:        c.Link,
			Content:    c.Comment,
			CreatedAt:  parseValineTime(c.CreatedAt),
			ArticleID:  c.Url,
			ParentID:   c.Pid,
			ParentNick: c.Pnick,
			Email:      c.Mail,
			Avatar:     p.getGravatar(c.Mail),
		})
	}

	return comments, nil
}

func (p *ValineProvider) GetRecentComments(ctx context.Context, limit int) ([]domain.Comment, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("order", "-createdAt")

	apiURL := fmt.Sprintf("%s/1.1/classes/Comment?%s", p.ServerURLs, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	p.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LeanCloud API error: %d %s", resp.StatusCode, string(body))
	}

	var result leanCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	comments := make([]domain.Comment, 0, len(result.Results))
	for _, c := range result.Results {
		comments = append(comments, domain.Comment{
			ID:         c.ObjectId,
			Nickname:   c.Nick,
			URL:        c.Link,
			Content:    c.Comment,
			CreatedAt:  parseValineTime(c.CreatedAt),
			ArticleID:  c.Url,
			ParentID:   c.Pid,
			ParentNick: c.Pnick,
			Email:      c.Mail,
			Avatar:     p.getGravatar(c.Mail),
			// IsNew: true, // TODO: 根据本地记录判断是否新评论
		})
	}

	return comments, nil
}

func (p *ValineProvider) GetAdminComments(ctx context.Context, page, pageSize int) ([]domain.Comment, int64, error) {
	// Valine (LeanCloud) doesn't have a direct "Get All Comments" for admin easily without querying all classes.
	// But usually we just list 'Comment' class.
	// We need to implement pagination.

	if page < 1 {
		page = 1
	}
	skip := (page - 1) * pageSize
	limit := pageSize

	// 1. Get List
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment?skip=%d&limit=%d&order=-createdAt", strings.TrimRight(p.ServerURLs, "/"), skip, limit)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}
	p.setHeaders(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("failed to fetch admin comments: %d", resp.StatusCode)
	}

	var result leanCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	comments := make([]domain.Comment, 0, len(result.Results))
	for _, c := range result.Results {
		comments = append(comments, domain.Comment{
			ID:         c.ObjectId,
			Nickname:   c.Nick,
			URL:        c.Link,
			Content:    c.Comment,
			CreatedAt:  parseValineTime(c.CreatedAt),
			ArticleID:  c.Url,
			ParentID:   c.Pid,
			ParentNick: c.Pnick,
			Email:      c.Mail,
			Avatar:     p.getGravatar(c.Mail),
		})
	}

	// 2. Get Count
	countURL := fmt.Sprintf("%s/1.1/classes/Comment?count=1&limit=0", strings.TrimRight(p.ServerURLs, "/"))
	reqCount, err := http.NewRequestWithContext(ctx, "GET", countURL, nil)
	if err != nil {
		return comments, 0, nil // Return comments even if count fails
	}
	p.setHeaders(reqCount)
	respCount, err := client.Do(reqCount)
	if err == nil {
		defer respCount.Body.Close()
		if respCount.StatusCode == http.StatusOK {
			var countResult struct {
				Count int64 `json:"count"`
			}
			if err := json.NewDecoder(respCount.Body).Decode(&countResult); err == nil {
				return comments, countResult.Count, nil
			}
		}
	}

	return comments, int64(len(comments)), nil // Fallback count
}

func (p *ValineProvider) PostComment(ctx context.Context, comment *domain.Comment) error {
	// Valine Create Comment
	// POST /1.1/classes/Comment
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment", strings.TrimRight(p.ServerURLs, "/"))

	lcComment := map[string]interface{}{
		"nick":      comment.Nickname,
		"comment":   comment.Content,
		"mail":      comment.Email,
		"link":      comment.URL,
		"url":       comment.ArticleID,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	}

	if comment.ParentID != "" {
		// Fetch parent to get ID/RID
		parent, err := p.getCommentByID(ctx, comment.ParentID)
		if err != nil {
			return fmt.Errorf("failed to fetch parent comment: %w", err)
		}
		lcComment["pid"] = parent.ObjectId
		if parent.Rid != "" {
			lcComment["rid"] = parent.Rid
		} else {
			lcComment["rid"] = parent.ObjectId
		}
		lcComment["pnick"] = parent.Nick
	}

	lcComment["ua"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.2 Safari/605.1.15"

	jsonData, err := json.Marshal(lcComment)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	p.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LeanCloud API error: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *ValineProvider) getCommentByID(ctx context.Context, id string) (*leanCloudComment, error) {
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment/%s", strings.TrimRight(p.ServerURLs, "/"), id)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	p.setHeaders(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var comment leanCloudComment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

func parseValineTime(t string) time.Time {
	// Try standard RFC3339 first
	parsed, err := time.Parse(time.RFC3339, t)
	if err == nil {
		return parsed
	}
	// Try other formats if needed, or return current time/zero time
	return time.Now()
}

func (p *ValineProvider) setHeaders(req *http.Request) {
	req.Header.Set("X-LC-Id", p.AppID)
	if p.MasterKey != "" {
		req.Header.Set("X-LC-Key", fmt.Sprintf("%s,master", p.MasterKey))
	} else {
		req.Header.Set("X-LC-Key", p.AppKey)
	}
}

func (p *ValineProvider) getGravatar(email string) string {
	if email == "" {
		return ""
	}
	email = strings.TrimSpace(strings.ToLower(email))
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("https://cravatar.cn/avatar/%s?d=mp&v=1.4.14", hex.EncodeToString(hash[:]))
}
