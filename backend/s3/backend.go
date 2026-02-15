// Package s3 provides an S3-compatible backend for omnistorage.
//
// This backend works with:
//   - AWS S3
//   - Cloudflare R2
//   - MinIO
//   - Wasabi
//   - DigitalOcean Spaces
//   - Any S3-compatible object storage
//
// Basic usage:
//
//	backend, err := s3.New(s3.Config{
//	    Bucket: "my-bucket",
//	    Region: "us-east-1",
//	})
//
// For S3-compatible services:
//
//	backend, err := s3.New(s3.Config{
//	    Bucket:       "my-bucket",
//	    Endpoint:     "https://play.min.io",
//	    UsePathStyle: true,
//	})
package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/grokify/omnistorage"
)

func init() {
	omnistorage.Register("s3", NewFromConfig)
}

// Errors specific to the S3 backend.
var (
	ErrBucketRequired = errors.New("s3: bucket is required")
)

// Backend implements omnistorage.ExtendedBackend for S3-compatible storage.
type Backend struct {
	client         *s3.Client
	transferClient *transfermanager.Client
	config         Config
	closed         bool
	mu             sync.RWMutex
}

// New creates a new S3 backend with the given configuration.
func New(cfg Config) (*Backend, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.PartSize == 0 {
		cfg.PartSize = 5 * 1024 * 1024 // 5MB
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 5
	}

	// Build AWS config options
	var optFns []func(*config.LoadOptions) error

	// Region
	if cfg.Region != "" {
		optFns = append(optFns, config.WithRegion(cfg.Region))
	}

	// Credentials
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		creds := credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
		optFns = append(optFns, config.WithCredentialsProvider(creds))
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		return nil, fmt.Errorf("s3: loading AWS config: %w", err)
	}

	// Build S3 client options
	var s3OptFns []func(*s3.Options)

	// Custom endpoint
	if cfg.Endpoint != "" {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	// Path-style addressing
	if cfg.UsePathStyle {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, s3OptFns...)

	// Create transfer manager client
	transferClient := transfermanager.New(client, func(o *transfermanager.Options) {
		o.PartSizeBytes = cfg.PartSize
		o.Concurrency = cfg.Concurrency
	})

	return &Backend{
		client:         client,
		transferClient: transferClient,
		config:         cfg,
	}, nil
}

// NewFromConfig creates a new S3 backend from a config map.
// This is used by the omnistorage registry.
func NewFromConfig(configMap map[string]string) (omnistorage.Backend, error) {
	cfg := ConfigFromMap(configMap)
	return New(cfg)
}

// NewWriter creates a writer for the given path.
func (b *Backend) NewWriter(ctx context.Context, p string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	key := b.fullKey(p)
	cfg := omnistorage.ApplyWriterOptions(opts...)

	return &s3Writer{
		backend:     b,
		ctx:         ctx,
		key:         key,
		buffer:      &bytes.Buffer{},
		contentType: cfg.ContentType,
		metadata:    cfg.Metadata,
	}, nil
}

// NewReader creates a reader for the given path.
func (b *Backend) NewReader(ctx context.Context, p string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	key := b.fullKey(p)
	cfg := omnistorage.ApplyReaderOptions(opts...)

	// Build GetObject input
	input := &s3.GetObjectInput{
		Bucket: aws.String(b.config.Bucket),
		Key:    aws.String(key),
	}

	// Handle range requests
	if cfg.Offset > 0 || cfg.Limit > 0 {
		var rangeHeader string
		if cfg.Limit > 0 {
			rangeHeader = fmt.Sprintf("bytes=%d-%d", cfg.Offset, cfg.Offset+cfg.Limit-1)
		} else {
			rangeHeader = fmt.Sprintf("bytes=%d-", cfg.Offset)
		}
		input.Range = aws.String(rangeHeader)
	}

	// Get object
	result, err := b.client.GetObject(ctx, input)
	if err != nil {
		return nil, b.translateError(err, p)
	}

	return result.Body, nil
}

// Exists checks if a path exists.
func (b *Backend) Exists(ctx context.Context, p string) (bool, error) {
	if err := b.checkClosed(); err != nil {
		return false, err
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	key := b.fullKey(p)

	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.config.Bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		var nsk *types.NotFound
		if errors.As(err, &nsk) {
			return false, nil
		}
		// Also check for NoSuchKey error
		var apiErr interface{ ErrorCode() string }
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey" {
				return false, nil
			}
		}
		return false, b.translateError(err, p)
	}

	return true, nil
}

// Delete removes a path.
func (b *Backend) Delete(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	key := b.fullKey(p)

	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.config.Bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// S3 delete is idempotent, but check for other errors
		var nsk *types.NotFound
		if errors.As(err, &nsk) {
			return nil
		}
		return b.translateError(err, p)
	}

	return nil
}

// List lists paths with the given prefix.
func (b *Backend) List(ctx context.Context, prefix string) ([]string, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPrefix := b.fullKey(prefix)

	var paths []string
	paginator := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.config.Bucket),
		Prefix: aws.String(fullPrefix),
	})

	for paginator.HasMorePages() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3: listing objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			// Remove prefix to get relative path
			relPath := strings.TrimPrefix(*obj.Key, b.config.Prefix)
			relPath = strings.TrimPrefix(relPath, "/")
			if relPath != "" {
				paths = append(paths, relPath)
			}
		}
	}

	return paths, nil
}

// Close releases any resources held by the backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	return nil
}

// Stat returns metadata about an object.
func (b *Backend) Stat(ctx context.Context, p string) (omnistorage.ObjectInfo, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	key := b.fullKey(p)

	result, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.config.Bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, b.translateError(err, p)
	}

	// Get size
	var size int64
	if result.ContentLength != nil {
		size = *result.ContentLength
	}

	// Get mod time
	var modTime time.Time
	if result.LastModified != nil {
		modTime = *result.LastModified
	}

	// Get content type
	var contentType string
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	// Get ETag as MD5 hash (for non-multipart uploads)
	hashes := make(map[omnistorage.HashType]string)
	if result.ETag != nil {
		etag := strings.Trim(*result.ETag, "\"")
		// ETag is MD5 for non-multipart uploads (no hyphen)
		if !strings.Contains(etag, "-") {
			hashes[omnistorage.HashMD5] = etag
		}
	}

	return &omnistorage.BasicObjectInfo{
		ObjectPath:        p,
		ObjectSize:        size,
		ObjectModTime:     modTime,
		ObjectIsDir:       false, // S3 doesn't have real directories
		ObjectContentType: contentType,
		ObjectHashes:      hashes,
	}, nil
}

// Mkdir creates a directory (no-op for S3, directories are implicit).
func (b *Backend) Mkdir(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// S3 doesn't need directories - they're implicit from object keys
	// Some tools create zero-byte objects with trailing slash to represent directories
	// We'll do that for compatibility
	key := b.fullKey(p)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(b.config.Bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader([]byte{}),
		ContentLength: aws.Int64(0),
	})

	if err != nil {
		return fmt.Errorf("s3: creating directory marker: %w", err)
	}

	return nil
}

// Rmdir removes a directory (removes directory marker if present).
func (b *Backend) Rmdir(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	key := b.fullKey(p)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	// Check if directory is empty
	result, err := b.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.config.Bucket),
		Prefix:  aws.String(key),
		MaxKeys: aws.Int32(2), // Just need to know if there's more than the marker
	})
	if err != nil {
		return fmt.Errorf("s3: checking directory: %w", err)
	}

	// Count non-marker objects
	count := 0
	for _, obj := range result.Contents {
		if obj.Key != nil && *obj.Key != key {
			count++
		}
	}

	if count > 0 {
		return fmt.Errorf("s3: directory not empty: %s", p)
	}

	// Delete the directory marker
	_, err = b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.config.Bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return b.translateError(err, p)
	}

	return nil
}

// Copy copies an object using S3 server-side copy.
func (b *Backend) Copy(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	srcKey := b.fullKey(src)
	dstKey := b.fullKey(dst)

	// S3 CopyObject requires the source as bucket/key
	copySource := fmt.Sprintf("%s/%s", b.config.Bucket, srcKey)

	_, err := b.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(b.config.Bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(dstKey),
	})

	if err != nil {
		return b.translateError(err, src)
	}

	return nil
}

// Move moves an object by copying then deleting.
func (b *Backend) Move(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Copy first
	if err := b.Copy(ctx, src, dst); err != nil {
		return err
	}

	// Delete source
	return b.Delete(ctx, src)
}

// Features returns the capabilities of the S3 backend.
func (b *Backend) Features() omnistorage.Features {
	return omnistorage.Features{
		Copy:                 true,
		Move:                 true, // Implemented as copy+delete
		Mkdir:                true, // Creates marker objects
		Rmdir:                true, // Deletes marker objects
		Stat:                 true,
		Hashes:               []omnistorage.HashType{omnistorage.HashMD5},
		CanStream:            true,
		ServerSideEncryption: true,
		Versioning:           true, // Depends on bucket config
		RangeRead:            true,
		ListPrefix:           true,
	}
}

// fullKey returns the full S3 key for a path.
func (b *Backend) fullKey(p string) string {
	if b.config.Prefix == "" {
		return p
	}
	return path.Join(b.config.Prefix, p)
}

// checkClosed returns an error if the backend is closed.
func (b *Backend) checkClosed() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return omnistorage.ErrBackendClosed
	}
	return nil
}

// translateError converts S3 errors to omnistorage errors.
func (b *Backend) translateError(err error, path string) error {
	if err == nil {
		return nil
	}

	// Check for NotFound
	var nsk *types.NotFound
	if errors.As(err, &nsk) {
		return omnistorage.ErrNotFound
	}

	var nsb *types.NoSuchBucket
	if errors.As(err, &nsb) {
		return fmt.Errorf("s3: bucket not found: %s", b.config.Bucket)
	}

	var nsu *types.NoSuchUpload
	if errors.As(err, &nsu) {
		return fmt.Errorf("s3: upload not found: %s", path)
	}

	// Check error code
	var apiErr interface{ ErrorCode() string }
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return omnistorage.ErrNotFound
		case "AccessDenied":
			return omnistorage.ErrPermissionDenied
		case "InvalidAccessKeyId", "SignatureDoesNotMatch":
			return omnistorage.ErrPermissionDenied
		}
	}

	return fmt.Errorf("s3: %w", err)
}

// s3Writer implements io.WriteCloser for S3.
type s3Writer struct {
	backend     *Backend
	ctx         context.Context
	key         string
	buffer      *bytes.Buffer
	contentType string
	metadata    map[string]string
	closed      bool
	mu          sync.Mutex
}

func (w *s3Writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, omnistorage.ErrWriterClosed
	}

	return w.buffer.Write(p)
}

func (w *s3Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Build UploadObject input
	input := &transfermanager.UploadObjectInput{
		Bucket: aws.String(w.backend.config.Bucket),
		Key:    aws.String(w.key),
		Body:   bytes.NewReader(w.buffer.Bytes()),
	}

	if w.contentType != "" {
		input.ContentType = aws.String(w.contentType)
	}

	if len(w.metadata) > 0 {
		input.Metadata = w.metadata
	}

	// Use transfer manager for potentially large files
	_, err := w.backend.transferClient.UploadObject(w.ctx, input)
	if err != nil {
		return fmt.Errorf("s3: uploading object: %w", err)
	}

	return nil
}

// Ensure Backend implements omnistorage.ExtendedBackend
var _ omnistorage.ExtendedBackend = (*Backend)(nil)
