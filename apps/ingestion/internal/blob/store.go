package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type Store struct {
	client *azblob.Client
}

func NewFromEnvironment() (*Store, error) {
	accountURL := os.Getenv("AZURE_STORAGE_ACCOUNT_URL")
	if accountURL == "" {
		return nil, errors.New("AZURE_STORAGE_ACCOUNT_URL is required for Blob access")
	}
	parsed, err := url.Parse(accountURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, errors.New("Azure Storage account URL must be an absolute HTTP(S) URL without credentials")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return nil, errors.New("non-loopback Azure Storage endpoints must use HTTPS")
	}
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create Azure credential: %w", err)
	}
	client, err := azblob.NewClient(accountURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create Blob client: %w", err)
	}
	return &Store{client: client}, nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Store) Read(ctx context.Context, container, name string, maxBytes int64) ([]byte, error) {
	response, err := s.client.DownloadStream(ctx, container, name, nil)
	if err != nil {
		return nil, fmt.Errorf("download Blob: %w", err)
	}
	reader := response.NewRetryReader(ctx, nil)
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Blob: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("Blob exceeds configured size limit")
	}
	return data, nil
}

func (s *Store) DownloadFile(ctx context.Context, container, name, destination string, maxBytes int64) error {
	data, err := s.Read(ctx, container, name, maxBytes)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	return writeAtomic(destination, data)
}

func (s *Store) Upload(ctx context.Context, container, name string, data []byte) error {
	if _, err := s.client.UploadBuffer(ctx, container, name, data, nil); err != nil {
		return fmt.Errorf("upload Blob: %w", err)
	}
	return nil
}

func (s *Store) UploadFile(ctx context.Context, container, name, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file for Blob upload: %w", err)
	}
	defer file.Close()
	if _, err := s.client.UploadFile(ctx, container, name, file, nil); err != nil {
		return fmt.Errorf("upload file to Blob: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, container string) ([]string, error) {
	pager := s.client.NewListBlobsFlatPager(container, nil)
	var names []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list Blob container: %w", err)
		}
		for _, item := range page.Segment.BlobItems {
			if item.Name != nil {
				names = append(names, *item.Name)
			}
		}
	}
	return names, nil
}

func writeAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".blob-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
