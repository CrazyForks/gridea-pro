package domain

import "context"

type SeoSetting struct {
	EnableJsonLD            bool   `json:"enableJsonLD"`
	EnableOpenGraph         bool   `json:"enableOpenGraph"`
	EnableCanonicalURL      bool   `json:"enableCanonicalURL"`
	MetaKeywords            string `json:"metaKeywords"`
	GoogleAnalyticsID       string `json:"googleAnalyticsId"`
	GoogleSearchConsoleCode string `json:"googleSearchConsoleCode"`
	BaiduAnalyticsID        string `json:"baiduAnalyticsId"`
	CustomHeadCode          string `json:"customHeadCode"`
}

type SeoSettingRepository interface {
	GetSeoSetting(ctx context.Context) (SeoSetting, error)
	SaveSeoSetting(ctx context.Context, setting SeoSetting) error
}
