package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// MemoryStorage implements Storage entirely in-memory for testing.
type MemoryStorage struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemoryStorage initializes an in-memory storage instance.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		files: make(map[string][]byte),
	}
}

func (s *MemoryStorage) Save(ctx context.Context, key string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read data into memory: %w", err)
	}

	s.files[key] = data
	return nil
}

func (s *MemoryStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.files[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *MemoryStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.files, key)
	return nil
}

func (s *MemoryStorage) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.files[key]
	return ok, nil
}
