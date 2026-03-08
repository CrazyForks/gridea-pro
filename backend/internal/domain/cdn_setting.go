package domain

import "context"

type CdnSetting struct {
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider"`
	GithubUser   string `json:"githubUser"`
	GithubRepo   string `json:"githubRepo"`
	GithubBranch string `json:"githubBranch"`
	BaseURL      string `json:"baseUrl"`
}

type CdnSettingRepository interface {
	GetCdnSetting(ctx context.Context) (CdnSetting, error)
	SaveCdnSetting(ctx context.Context, setting CdnSetting) error
}
