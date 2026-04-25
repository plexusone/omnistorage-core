package object

// WriterOption configures a writer created by Backend.NewWriter.
type WriterOption func(*WriterConfig)

// WriterConfig holds configuration for creating a writer.
type WriterConfig struct {
	// BufferSize is the buffer size in bytes.
	// 0 means use the backend's default.
	BufferSize int

	// ContentType is a MIME type hint for the content.
	// Some backends (S3, HTTP) use this for Content-Type headers.
	ContentType string

	// Metadata is backend-specific metadata.
	// For S3, these become object metadata.
	// For file backend, this is ignored.
	Metadata map[string]string
}

// WithBufferSize sets the buffer size for the writer.
func WithBufferSize(size int) WriterOption {
	return func(c *WriterConfig) {
		c.BufferSize = size
	}
}

// WithContentType sets the content type hint.
func WithContentType(contentType string) WriterOption {
	return func(c *WriterConfig) {
		c.ContentType = contentType
	}
}

// WithMetadata sets backend-specific metadata.
func WithMetadata(metadata map[string]string) WriterOption {
	return func(c *WriterConfig) {
		c.Metadata = metadata
	}
}

// ApplyWriterOptions applies options to a WriterConfig.
func ApplyWriterOptions(opts ...WriterOption) *WriterConfig {
	config := &WriterConfig{}
	for _, opt := range opts {
		opt(config)
	}
	return config
}

// ReaderOption configures a reader created by Backend.NewReader.
type ReaderOption func(*ReaderConfig)

// ReaderConfig holds configuration for creating a reader.
type ReaderConfig struct {
	// BufferSize is the buffer size in bytes.
	// 0 means use the backend's default.
	BufferSize int

	// Offset is the byte offset to start reading from.
	// Not all backends support this.
	Offset int64

	// Limit is the maximum number of bytes to read.
	// 0 means no limit.
	Limit int64
}

// WithReaderBufferSize sets the buffer size for the reader.
func WithReaderBufferSize(size int) ReaderOption {
	return func(c *ReaderConfig) {
		c.BufferSize = size
	}
}

// WithOffset sets the byte offset to start reading from.
func WithOffset(offset int64) ReaderOption {
	return func(c *ReaderConfig) {
		c.Offset = offset
	}
}

// WithLimit sets the maximum number of bytes to read.
func WithLimit(limit int64) ReaderOption {
	return func(c *ReaderConfig) {
		c.Limit = limit
	}
}

// ApplyReaderOptions applies options to a ReaderConfig.
func ApplyReaderOptions(opts ...ReaderOption) *ReaderConfig {
	config := &ReaderConfig{}
	for _, opt := range opts {
		opt(config)
	}
	return config
}
