package service

import (
	"context"
	"errors"
	"gridea-pro/backend/internal/domain"
	"testing"
)

// MockPostRepository
type MockPostRepository struct {
	Files map[string]*domain.Post
}

func (m *MockPostRepository) Create(ctx context.Context, post *domain.Post) error {
	if _, exists := m.Files[post.FileName]; exists {
		return nil // Simulate ignore or overwrite depending on implementation, but repository typically errors if duplicate?
		// In actual FS repo, it creates file.
	}
	m.Files[post.FileName] = post
	return nil
}

func (m *MockPostRepository) Update(ctx context.Context, post *domain.Post) error {
	m.Files[post.FileName] = post
	return nil
}

func (m *MockPostRepository) Delete(ctx context.Context, fileName string) error {
	delete(m.Files, fileName)
	return nil
}

func (m *MockPostRepository) GetByFileName(ctx context.Context, fileName string) (*domain.Post, error) {
	if post, exists := m.Files[fileName]; exists {
		return post, nil
	}
	return nil, errors.New("post not found")
}
func (m *MockPostRepository) List(ctx context.Context, page, size int) ([]domain.Post, int64, error) {
	return nil, 0, nil
}
func (m *MockPostRepository) GetAll(ctx context.Context) ([]domain.Post, error) {
	return nil, nil
}

// MockTagRepository
type MockTagRepository struct{}

func (m *MockTagRepository) List(ctx context.Context) ([]domain.Tag, error) { return nil, nil }
func (m *MockTagRepository) Create(ctx context.Context, tag *domain.Tag) error {
	tag.ID = "mock-id" // simulate ID generation
	return nil
}
func (m *MockTagRepository) Update(ctx context.Context, tag *domain.Tag) error { return nil }
func (m *MockTagRepository) Delete(ctx context.Context, id string) error       { return nil }

// MockCategoryRepository
type MockCategoryRepository struct{}

func (m *MockCategoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	return nil, nil
}
func (m *MockCategoryRepository) SaveAll(ctx context.Context, categories []domain.Category) error {
	return nil
}
func (m *MockCategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	return nil
}
func (m *MockCategoryRepository) Update(ctx context.Context, slug string, category *domain.Category) error {
	return nil
}
func (m *MockCategoryRepository) Delete(ctx context.Context, slug string) error {
	return nil
}
func (m *MockCategoryRepository) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	return nil, errors.New("not found")
}

// MockMediaRepository
type MockMediaRepository struct{}

func (m *MockMediaRepository) SaveImages(ctx context.Context, files []domain.UploadedFile) ([]string, error) {
	return nil, nil
}

func TestPostService_SavePost_Rename(t *testing.T) {
	// Setup Mocks
	postRepo := &MockPostRepository{Files: make(map[string]*domain.Post)}
	tagRepo := &MockTagRepository{}
	catRepo := &MockCategoryRepository{}
	mediaRepo := &MockMediaRepository{}

	tagService := NewTagService(tagRepo)
	catService := NewCategoryService(catRepo)

	service := NewPostService(postRepo, tagRepo, tagService, catService, mediaRepo)

	ctx := context.Background()

	// 1. Prepare initial state: "old.md" exists
	oldPost := &domain.Post{
		Title:    "Old Title",
		FileName: "old.md",
		Content:  "Old Content",
	}
	postRepo.Files["old.md"] = oldPost

	// 2. Rename operation: "old.md" -> "new.md"
	newPost := &domain.Post{
		Title:          "New Title",
		FileName:       "new.md",
		DeleteFileName: "old.md", // Request Rename
		Content:        "New Content",
	}

	// 3. Execute SavePost
	err := service.SavePost(ctx, newPost)
	if err != nil {
		t.Fatalf("SavePost failed: %v", err)
	}

	// 4. Verify
	// "old.md" should NOT exist
	if _, exists := postRepo.Files["old.md"]; exists {
		t.Errorf("old.md should be deleted after rename")
	}

	// "new.md" SHOULD exist
	saved, exists := postRepo.Files["new.md"]
	if !exists {
		t.Errorf("new.md should be created after rename")
	} else {
		if saved.Title != "New Title" {
			t.Errorf("Expected title 'New Title', got '%s'", saved.Title)
		}
	}
}
