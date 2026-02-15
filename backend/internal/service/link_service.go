package service

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"sync"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

const (
	nanoIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	nanoIDLength   = 6
)

type LinkService struct {
	repo domain.LinkRepository
	mu   sync.RWMutex
}

func NewLinkService(repo domain.LinkRepository) *LinkService {
	return &LinkService{repo: repo}
}

func (s *LinkService) LoadLinks(ctx context.Context) ([]domain.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repo.List(ctx)
}

func (s *LinkService) SaveLinks(ctx context.Context, links []domain.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo.SaveAll(ctx, links)
}

func (s *LinkService) CreateLink(ctx context.Context, link domain.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	links, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	link.ID = gonanoid.Must(nanoIDLength) // Use nanoIDLength constant
	links = append(links, link)

	return s.repo.SaveAll(ctx, links)
}

func (s *LinkService) UpdateLink(ctx context.Context, link domain.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	links, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	found := false
	for i, l := range links {
		if l.ID == link.ID {
			links[i] = link
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("link not found")
	}

	return s.repo.SaveAll(ctx, links)
}

func (s *LinkService) DeleteLink(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	links, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	var newLinks []domain.Link
	found := false // Added found flag for consistency with other methods
	for _, l := range links {
		if l.ID != id {
			newLinks = append(newLinks, l)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("link not found")
	}

	return s.repo.SaveAll(ctx, newLinks)
}

// FixMissingIDs checks and repairs missing IDs in links.
// Returns true if any changes were made.
func (s *LinkService) FixMissingIDs(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	links, err := s.repo.List(ctx)
	if err != nil {
		return false, err
	}

	hasMissingID := false
	for i := range links {
		if links[i].ID == "" {
			id, err := gonanoid.Generate(nanoIDAlphabet, nanoIDLength)
			if err == nil {
				links[i].ID = id
				hasMissingID = true
				fmt.Printf("Service Patched missing ID for link: %s -> %s\n", links[i].Name, id)
			}
		}
	}

	if hasMissingID {
		if err := s.repo.SaveAll(ctx, links); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}
