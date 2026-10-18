package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage defines the interface for persisting uploaded raw documents.
type Storage interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// LocalStorage implements Storage using the local filesystem or mounted volume.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage initializes local filesystem storage at the designated path.
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create local storage directory %s: %w", basePath, err)
	}
	return &LocalStorage{basePath: basePath}, nil
}

func (s *LocalStorage) resolvePath(key string) string {
	return filepath.Join(s.basePath, filepath.Clean(key))
}

// Save writes an incoming stream atomically to disk via a temporary file.
func (s *LocalStorage) Save(ctx context.Context, key string, r io.Reader) error {
	finalPath := s.resolvePath(key)
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tempFile, err := os.CreateTemp(dir, "upload-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath) // Clean up if rename hasn't succeeded
	}()

	if _, err := io.Copy(tempFile, r); err != nil {
		return fmt.Errorf("failed to copy data to temp file: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("failed to atomically rename temp file to %s: %w", finalPath, err)
	}

	return nil
}

// Open opens an existing file for reading.
func (s *LocalStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	path := s.resolvePath(key)
	return os.Open(path)
}

// Delete removes an existing stored file.
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	path := s.resolvePath(key)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Exists checks if a stored file exists.
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	path := s.resolvePath(key)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
