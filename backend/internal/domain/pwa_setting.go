package domain

import "context"

type PwaSetting struct {
	Enabled         bool   `json:"enabled"`
	AppName         string `json:"appName"`
	ShortName       string `json:"shortName"`
	Description     string `json:"description"`
	ThemeColor      string `json:"themeColor"`
	BackgroundColor string `json:"backgroundColor"`
	Orientation     string `json:"orientation"`
	CustomIcon      bool   `json:"customIcon"`
}

type PwaSettingRepository interface {
	GetPwaSetting(ctx context.Context) (PwaSetting, error)
	SavePwaSetting(ctx context.Context, setting PwaSetting) error
}
