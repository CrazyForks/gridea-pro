package domain

// Added json tags for frontend compatibility.

type FileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type UploadedFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
