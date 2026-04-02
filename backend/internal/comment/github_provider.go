package comment

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"log/slog"
	"time"
)

// GitHubProvider 处理 Gitalk 和 Giscus (REST API fallback)
type GitHubProvider struct {
	*BaseProvider
	config *GitHubConfig
}

func NewGitHubProvider(config *GitHubConfig, proxyURL string, logger *slog.Logger) *GitHubProvider {
	return &GitHubProvider{
		BaseProvider: NewBaseProvider(15*time.Second, proxyURL, logger),
		config:       config,
	}
}

// GitHub Issue Comment
type githubComment struct {
	ID        int64      `json:"id"`
	Body      string     `json:"body"`
	User      githubUser `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	HtmlUrl   string     `json:"html_url"`
	IssueUrl  string     `json:"issue_url"`
}

type githubUser struct {
	Login     string `json:"login"`
	AvatarUrl string `json:"avatar_url"`
	HtmlUrl   string `json:"html_url"`
}

func (p *GitHubProvider) GetComments(ctx context.Context, articleID string) ([]domain.Comment, error) {
	// 获取指定文章的评论需要先找到对应的 Issue，逻辑较复杂，暂时只实现 GetRecentComments
	return nil, fmt.Errorf("%w: GitHubProvider GetComments not implemented yet (requires Issue lookup logic)", ErrNotImplemented)
}

func (p *GitHubProvider) GetRecentComments(ctx context.Context, limit int) ([]domain.Comment, error) {
	// https://docs.github.com/en/rest/issues/comments?apiVersion=2022-11-28#list-issue-comments-for-a-repository
	// GET /repos/{owner}/{repo}/issues/comments

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments?sort=created&direction=desc&per_page=%d", p.config.Owner, p.config.Repo, limit)

	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}

	// 如果配置了 AccessToken (目前 Config 暂未包含，但 Gitalk 主要靠前端，后端作为辅助)
	// 如果 future 需要后端 Token，可以在 GitHubConfig 中添加并在这里使用

	var ghComments []githubComment
	// DoJSON handle status code check and decoding
	if err := p.DoJSON(ctx, "GET", apiURL, nil, &ghComments, headers); err != nil {
		return nil, err
	}

	var comments []domain.Comment
	for _, c := range ghComments {
		comments = append(comments, domain.Comment{
			ID:        fmt.Sprintf("%d", c.ID),
			Avatar:    c.User.AvatarUrl,
			Nickname:  c.User.Login,
			URL:       c.User.HtmlUrl,
			Content:   c.Body, // Markdown
			CreatedAt: c.CreatedAt,
			ArticleID: c.IssueUrl, // 暂时用 Issue URL 代替
			// ParentID: "",
		})
	}

	return comments, nil
}

// GetAdminComments implementation
func (p *GitHubProvider) GetAdminComments(ctx context.Context, page, pageSize int) ([]domain.Comment, int64, error) {
	// TODO: Implement pagination for GitHub
	comments, err := p.GetRecentComments(ctx, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// Warning: Total count is fake here
	return comments, int64(len(comments)), nil
}

func (p *GitHubProvider) PostComment(ctx context.Context, comment *domain.Comment) error {
	// 需要 Token 才能发送
	return fmt.Errorf("%w: PostComment requires authentication token", ErrAuthFailed)
}

func (p *GitHubProvider) DeleteComment(ctx context.Context, commentID string) error {
	return fmt.Errorf("%w: DeleteComment requires authentication token", ErrAuthFailed)
}
