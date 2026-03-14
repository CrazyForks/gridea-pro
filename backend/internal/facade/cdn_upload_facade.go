package facade

import (
	"context"
	"gridea-pro/backend/internal/service"
)

type CdnUploadFacade struct {
	internal *service.CdnUploadService
}

func NewCdnUploadFacade(s *service.CdnUploadService) *CdnUploadFacade {
	return &CdnUploadFacade{internal: s}
}

// TestCdnUpload 测试上传，返回 CDN 访问 URL
func (f *CdnUploadFacade) TestCdnUpload() (string, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.TestUpload(ctx)
}
