package repository

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type postRepository struct {
	mu     sync.RWMutex
	appDir string
}

func NewPostRepository(appDir string) domain.PostRepository {
	return &postRepository{
		appDir: appDir,
	}
}

// local struct to handle YAML frontmatter parsing, especially for Date string
type postYaml struct {
	Title      string   `yaml:"title"`
	Date       string   `yaml:"date"` // Parse as string first
	Tags       []string `yaml:"tags"`
	TagIDs     []string `yaml:"tag_ids"`
	Categories []string `yaml:"categories"`
	Published  bool     `yaml:"published"`
	HideInList bool     `yaml:"hideInList"`
	Feature    string   `yaml:"feature"`
	IsTop      bool     `yaml:"isTop"`
}

func (r *postRepository) Create(ctx context.Context, post *domain.Post) error {
	return r.save(ctx, post, false)
}

func (r *postRepository) Update(ctx context.Context, post *domain.Post) error {
	return r.save(ctx, post, true)
}

func (r *postRepository) save(ctx context.Context, post *domain.Post, isUpdate bool) error {
	// Respect context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	postsDir := filepath.Join(r.appDir, "posts")
	postImageDir := filepath.Join(r.appDir, "post-images")
	_ = os.MkdirAll(postsDir, 0755)
	_ = os.MkdirAll(postImageDir, 0755)

	// Basic Validation
	if err := post.Validate(); err != nil {
		return err
	}

	tagsStr := strings.Join(post.Tags, ",")
	tagsStr = escapeYAMLString(tagsStr)

	// formatted tags "tag1", "tag2"
	var formattedTags []string
	for _, t := range post.Tags {
		formattedTags = append(formattedTags, fmt.Sprintf("'%s'", escapeYAMLString(t)))
	}
	tagsStr = strings.Join(formattedTags, ", ")

	// formatted tagIDs
	var formattedTagIDs []string
	for _, id := range post.TagIDs {
		formattedTagIDs = append(formattedTagIDs, fmt.Sprintf("'%s'", escapeYAMLString(id)))
	}
	tagIDsStr := strings.Join(formattedTagIDs, ", ")

	categoriesStr := strings.Join(post.Categories, ",")
	feature := post.FeatureImagePath

	// Handle Image Copy (Logic preserved from original Save)
	if post.FeatureImage.Name != "" && post.FeatureImage.Path != "" {
		ext := filepath.Ext(post.FeatureImage.Name)
		newPath := filepath.Join(postImageDir, post.FileName+ext)
		if err := CopyFile(post.FeatureImage.Path, newPath); err == nil {
			feature = "/post-images/" + post.FileName + ext
			// Cleanup temp file if necessary
			if post.FeatureImage.Path != newPath && strings.Contains(post.FeatureImage.Path, postImageDir) {
				_ = os.Remove(post.FeatureImage.Path)
			}
		}
	}
	// If FeatureImagePath was already set (e.g. existing post), keep it if no new image uploaded
	if feature == "" && post.Feature != "" {
		feature = post.Feature
	} else if feature == "" {
		feature = ""
	}

	dateStr := post.Date.Format(domain.TimeLayout)

	mdContent := fmt.Sprintf(`---
title: '%s'
date: %s
tags: [%s]
tag_ids: [%s]
categories: [%s]
published: %t
hideInList: %t
feature: %s
isTop: %t
---
%s`,
		escapeYAMLString(post.Title),
		dateStr,
		tagsStr,
		tagIDsStr,
		categoriesStr,
		post.Published,
		post.HideInList,
		feature,
		post.IsTop,
		post.Content,
	)

	postPath := filepath.Join(postsDir, post.FileName+".md")

	if isUpdate {
		// handle rename
		if post.DeleteFileName != "" && post.DeleteFileName != post.FileName {
			oldPath := filepath.Join(postsDir, post.DeleteFileName+".md")
			_ = os.Remove(oldPath)
		}
	} else {
		// check exist for Create?
		if _, err := os.Stat(postPath); err == nil {
			return fmt.Errorf("post file already exists: %s", post.FileName)
		}
	}

	// Idempotent check
	existingContent, err := os.ReadFile(postPath)
	if err == nil && string(existingContent) == mdContent {
		return nil
	}

	if err := os.WriteFile(postPath, []byte(mdContent), 0644); err != nil {
		return fmt.Errorf("failed to write post file: %w", err)
	}

	return nil
}

func (r *postRepository) Delete(ctx context.Context, fileName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	postsDir := filepath.Join(r.appDir, "posts")
	postPath := filepath.Join(postsDir, fileName+".md")

	content, err := os.ReadFile(postPath)
	if err == nil {
		post, _ := r.parsePost(string(content), fileName+".md")

		// Delete feature image
		if post.Feature != "" && !strings.HasPrefix(post.Feature, "http") {
			featurePath := filepath.Join(r.appDir, strings.TrimPrefix(post.Feature, "/"))
			_ = os.Remove(featurePath)
		}

		// Delete embedded images
		re := regexp.MustCompile(`!\[.*?\]\((.+?)\)`)
		matches := re.FindAllStringSubmatch(post.Content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				imgPath := match[1]
				if !strings.HasPrefix(imgPath, "http") {
					fullPath := filepath.Join(r.appDir, strings.TrimPrefix(imgPath, "/"))
					_ = os.Remove(fullPath)
				}
			}
		}
	}

	return os.Remove(postPath)
}

func (r *postRepository) GetByFileName(ctx context.Context, fileName string) (*domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	postPath := filepath.Join(r.appDir, "posts", fileName+".md")
	content, err := os.ReadFile(postPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read post file: %w", err)
	}

	post, err := r.parsePost(string(content), fileName+".md")
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) List(ctx context.Context, page, size int) ([]domain.Post, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	postsDir := filepath.Join(r.appDir, "posts")
	// Ensure dir exists
	if _, err := os.Stat(postsDir); os.IsNotExist(err) {
		return []domain.Post{}, 0, nil
	}

	files, err := os.ReadDir(postsDir)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read posts dir: %w", err)
	}

	var allPosts []domain.Post
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(postsDir, file.Name()))
		if err != nil {
			continue
		}
		post, err := r.parsePost(string(content), file.Name())
		if err != nil {
			continue
		}
		allPosts = append(allPosts, post)
	}

	// Sort by date desc
	sort.Slice(allPosts, func(i, j int) bool {
		return allPosts[i].Date.After(allPosts[j].Date)
	})

	// JSON Cache Side-effect (preserved)
	dbPath := filepath.Join(r.appDir, "config", "posts.json")
	db := map[string]interface{}{"posts": allPosts}
	_ = SaveJSONFileIdempotent(dbPath, db)

	// Pagination
	total := int64(len(allPosts))
	start := (page - 1) * size
	if start < 0 {
		start = 0
	}
	if start >= len(allPosts) {
		return []domain.Post{}, total, nil
	}
	end := start + size
	if end > len(allPosts) {
		end = len(allPosts)
	}

	return allPosts[start:end], total, nil
}

func (r *postRepository) GetAll(ctx context.Context) ([]domain.Post, error) {
	posts, _, err := r.List(ctx, 1, 100000)
	return posts, err
}

// Helpers

func (r *postRepository) parsePost(content string, filename string) (domain.Post, error) {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return domain.Post{}, fmt.Errorf("invalid post format")
	}

	var meta postYaml
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return domain.Post{}, err
	}

	postContent := strings.TrimSpace(parts[2])
	abstract := r.extractAbstract(postContent)

	// Parse Date
	parsedDate, err := time.Parse(domain.TimeLayout, meta.Date)
	if err != nil {
		// Fallback to now or try other formats?
		// For now, default to Now if parse fails, or log error?
		// Gridea default: Use creation time or Now.
		parsedDate = time.Now()
	}

	post := domain.Post{
		Title:      meta.Title,
		Date:       parsedDate,
		Tags:       meta.Tags,
		TagIDs:     meta.TagIDs,
		Categories: meta.Categories,
		Published:  meta.Published,
		HideInList: meta.HideInList,
		Feature:    meta.Feature,
		IsTop:      meta.IsTop,
		Content:    postContent,
		Abstract:   abstract,
		FileName:   strings.TrimSuffix(filename, ".md"),
	}

	return post, nil
}

func (r *postRepository) extractAbstract(content string) string {
	re := regexp.MustCompile(`(?i)\n\s*<!--\s*more\s*-->\s*\n`)
	loc := re.FindStringIndex(content)
	if loc != nil {
		return strings.TrimSpace(content[:loc[0]])
	}
	return ""
}

func escapeYAMLString(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	return s
}
