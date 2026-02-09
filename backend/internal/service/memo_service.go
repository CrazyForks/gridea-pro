package service

import (
	"context"
	"gridea-pro/backend/internal/domain"
)

type MemoService struct {
	repo domain.MemoRepository
}

func NewMemoService(repo domain.MemoRepository) *MemoService {
	return &MemoService{repo: repo}
}

func (s *MemoService) LoadMemos(ctx context.Context) ([]domain.Memo, error) {
	return s.repo.GetAll(ctx)
}

func (s *MemoService) SaveMemos(ctx context.Context, memos []domain.Memo) error {
	return s.repo.SaveAll(ctx, memos)
}
