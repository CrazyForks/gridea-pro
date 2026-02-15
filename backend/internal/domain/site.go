package domain

// SiteData aggregates data for the frontend (View Model)
// Added json tags for frontend compatibility.
type SiteData struct {
	ThemeConfig ThemeConfig            `json:"themeConfig"`
	Posts       []Post                 `json:"posts"`
	Tags        []Tag                  `json:"tags"`
	Menus       []Menu                 `json:"menus"`
	ThemeCustom map[string]interface{} `json:"themeCustom"`
	Comment     CommentSettings        `json:"comment"`
	Themes      []Theme                `json:"themes"`
	Setting     Setting                `json:"setting"`
}
