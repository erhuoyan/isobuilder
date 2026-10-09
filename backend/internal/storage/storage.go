// Package storage uploads finished ISOs to an S3-compatible object store and
// hands back downloadable URLs. When no bucket is configured it degrades to
// serving the file from local disk.
package storage

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"isobuilder/internal/config"
)

// uploadPartSize is the multipart chunk size. Object stores reject parts above
// 5 GiB (and, for compatibility, some below 5 MiB); 64 MiB keeps a 40 GiB image
// at ~640 parts and streams without buffering the file.
const uploadPartSize = 64 << 20

// uploadConcurrency bounds parallel part uploads.
const uploadConcurrency = 4

// Backend stores build artifacts.
type Backend interface {
	// Kind is "s3" or "local".
	Kind() string
	// Bucket returns the configured bucket, empty for the local backend.
	Bucket() string
	// Endpoint returns the configured endpoint, empty for the local backend.
	Endpoint() string
	// Put uploads localPath as key and reports bytes transferred.
	Put(ctx context.Context, localPath, key string, onProgress func(int64)) error
	// URL returns a download URL. The local backend returns an empty URL, which
	// tells the HTTP layer to stream the file itself.
	URL(ctx context.Context, key string) (string, time.Time, error)
}

// Local keeps artifacts on disk.
type Local struct{ dir string }

// Kind implements Backend.
func (Local) Kind() string { return "local" }

// Bucket implements Backend.
func (Local) Bucket() string { return "" }

// Endpoint implements Backend.
func (Local) Endpoint() string { return "" }

// Put implements Backend; the file is already in place.
func (Local) Put(context.Context, string, string, func(int64)) error { return nil }

// URL implements Backend.
func (Local) URL(context.Context, string) (string, time.Time, error) { return "", time.Time{}, nil }

// S3 uploads to an S3-compatible endpoint (AWS S3, MinIO, Ceph RGW, 阿里云 OSS
// S3 兼容端点, 京东云 OSS ...).
type S3 struct {
	client   *s3.Client
	presign  *s3.PresignClient
	uploader *manager.Uploader
	cfg      config.StorageConfig
}

// Kind implements Backend.
func (s *S3) Kind() string { return "s3" }

// Bucket implements Backend.
func (s *S3) Bucket() string { return s.cfg.Bucket }

// Endpoint implements Backend.
func (s *S3) Endpoint() string { return s.cfg.Endpoint }

// Switcher is a Backend whose concrete implementation can be replaced at
// runtime, which is how the settings page takes effect without a restart.
type Switcher struct {
	mu      sync.RWMutex
	current Backend
}

// NewSwitcher wraps an initial backend.
func NewSwitcher(b Backend) *Switcher { return &Switcher{current: b} }

// Set replaces the active backend.
func (s *Switcher) Set(b Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = b
}

// Current returns the active backend.
func (s *Switcher) Current() Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// Kind implements Backend.
func (s *Switcher) Kind() string { return s.Current().Kind() }

// Bucket implements Backend.
func (s *Switcher) Bucket() string { return s.Current().Bucket() }

// Endpoint implements Backend.
func (s *Switcher) Endpoint() string { return s.Current().Endpoint() }

// Put implements Backend.
func (s *Switcher) Put(ctx context.Context, localPath, key string, onProgress func(int64)) error {
	return s.Current().Put(ctx, localPath, key, onProgress)
}

// URL implements Backend.
func (s *Switcher) URL(ctx context.Context, key string) (string, time.Time, error) {
	return s.Current().URL(ctx, key)
}

// New builds the configured backend. An empty bucket selects the local backend.
func New(cfg config.StorageConfig) Backend {
	if !cfg.Enabled() {
		return Local{}
	}
	return NewS3(cfg)
}

// NewS3 builds an S3 client for a specific configuration. It is also used to
// pull base images from a bucket that is not the artifact bucket.
func NewS3(cfg config.StorageConfig) *S3 {
	httpClient := &http.Client{Transport: &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: cfg.InsecureTLS},
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}}
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		HTTPClient:  httpClient,
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.PathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &S3{
		client:  client,
		presign: s3.NewPresignClient(client),
		cfg:     cfg,
		uploader: manager.NewUploader(client, func(u *manager.Uploader) {
			u.PartSize = uploadPartSize
			u.Concurrency = uploadConcurrency
			u.LeavePartsOnError = false
		}),
	}
}

// Ping verifies the endpoint, credentials and bucket with a cheap HeadBucket.
// The settings page calls it so a typo fails at save time, not at build time.
func (s *S3) Ping(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.cfg.Bucket)}); err != nil {
		return fmt.Errorf("无法访问 %s/%s: %w", s.cfg.Endpoint, s.cfg.Bucket, err)
	}
	return nil
}

// Head reports the size of one object, or an error explaining why it is not
// readable. Used before queueing a download so a wrong key fails immediately.
func (s *S3) Head(ctx context.Context, key string) error {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("对象 %s/%s 不可读: %w", s.cfg.Bucket, key, err)
	}
	if out.ContentLength != nil && *out.ContentLength == 0 {
		return fmt.Errorf("对象 %s/%s 为空文件", s.cfg.Bucket, key)
	}
	return nil
}

// Get streams an object into dest, reporting the bytes and total size. It is how
// a base image is pulled from an object store address instead of being uploaded.
func (s *S3) Get(ctx context.Context, key, dest string, onProgress func(int64, int64)) (int64, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("读取 %s/%s: %w", s.cfg.Bucket, key, err)
	}
	defer out.Body.Close()

	var total int64
	if out.ContentLength != nil {
		total = *out.ContentLength
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}

	var reader io.Reader = out.Body
	if onProgress != nil {
		reader = &downloadReader{r: out.Body, total: total, onProgress: onProgress}
	}
	written, err := io.Copy(f, reader)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(dest)
		return written, fmt.Errorf("下载 %s 失败: %w", key, err)
	}
	return written, nil
}

// Put implements Backend.
func (s *S3) Put(ctx context.Context, localPath, key string, onProgress func(int64)) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f
	if onProgress != nil {
		reader = &progressReader{r: f, onProgress: onProgress}
	}
	_, err = s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.Bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}
	return nil
}

// URL implements Backend. A PublicBase turns the key into a permanent public
// URL; otherwise a presigned URL is generated.
func (s *S3) URL(ctx context.Context, key string) (string, time.Time, error) {
	if s.cfg.PublicBase != "" {
		return s.cfg.PublicBase + "/" + strings.TrimPrefix(key, "/"), time.Time{}, nil
	}
	out, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.cfg.Bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String("attachment; filename=\"" + path.Base(key) + "\""),
	}, func(o *s3.PresignOptions) {
		o.Expires = s.cfg.PresignTTL
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("presign %s: %w", key, err)
	}
	return out.URL, time.Now().UTC().Add(s.cfg.PresignTTL), nil
}

// ObjectKey builds the object key for an artifact.
func ObjectKey(prefix, buildID, filename string) string {
	parts := make([]string, 0, 3)
	if prefix != "" {
		parts = append(parts, strings.Trim(prefix, "/"))
	}
	parts = append(parts, buildID, filename)
	return path.Join(parts...)
}

type downloadReader struct {
	r          io.Reader
	total      int64
	sent       int64
	onProgress func(int64, int64)
}

func (d *downloadReader) Read(b []byte) (int, error) {
	n, err := d.r.Read(b)
	if n > 0 {
		d.sent += int64(n)
		d.onProgress(d.sent, d.total)
	}
	return n, err
}

type progressReader struct {
	r          io.Reader
	onProgress func(int64)
	sent       int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.sent += int64(n)
		p.onProgress(p.sent)
	}
	return n, err
}
