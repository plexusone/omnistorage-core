package object

import (
	"context"
	"io"
)

// CopyPath copies an object from src to dst, potentially across different backends.
// This is a client-side copy that streams data through the caller.
//
// If srcBackend and dstBackend are the same ExtendedBackend with Copy support,
// consider using ExtendedBackend.Copy() for server-side copy instead.
//
// Options can be passed to configure the destination writer.
func CopyPath(ctx context.Context, srcBackend Backend, srcPath string, dstBackend Backend, dstPath string, opts ...WriterOption) error {
	// Open source for reading
	r, err := srcBackend.NewReader(ctx, srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	// Open destination for writing
	w, err := dstBackend.NewWriter(ctx, dstPath, opts...)
	if err != nil {
		return err
	}

	// Copy data
	_, err = io.Copy(w, r)
	if err != nil {
		_ = w.Close()
		return err
	}

	return w.Close()
}

// CopyPathWithHash copies an object and verifies the content hash.
// Returns the computed hash of the copied data.
// Returns an error if the copy fails or if hash computation fails.
func CopyPathWithHash(ctx context.Context, srcBackend Backend, srcPath string, dstBackend Backend, dstPath string, hashType HashType, opts ...WriterOption) (string, error) {
	// Open source for reading
	r, err := srcBackend.NewReader(ctx, srcPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	// Open destination for writing
	w, err := dstBackend.NewWriter(ctx, dstPath, opts...)
	if err != nil {
		return "", err
	}

	// Create hash writer
	h := NewHash(hashType)
	if h == nil {
		_ = w.Close()
		return "", ErrNotSupported
	}

	// Copy data through hash
	mw := io.MultiWriter(w, h)
	_, err = io.Copy(mw, r)
	if err != nil {
		_ = w.Close()
		return "", err
	}

	if err := w.Close(); err != nil {
		return "", err
	}

	return HashBytesFromSum(h.Sum(nil)), nil
}

// HashBytesFromSum converts a hash sum to hex string.
func HashBytesFromSum(sum []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(sum)*2)
	for i, b := range sum {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}

// SmartCopy attempts server-side copy first, falling back to client-side copy.
// Use this when you want the best performance but need a guaranteed fallback.
//
// If both backends are the same ExtendedBackend with Copy support, uses server-side copy.
// Otherwise, falls back to CopyPath.
func SmartCopy(ctx context.Context, srcBackend Backend, srcPath string, dstBackend Backend, dstPath string, opts ...WriterOption) error {
	// Check if same backend and supports server-side copy
	if srcBackend == dstBackend {
		if ext, ok := srcBackend.(ExtendedBackend); ok && ext.Features().Copy {
			err := ext.Copy(ctx, srcPath, dstPath)
			if err == nil || err != ErrNotSupported {
				return err
			}
			// Fall through to client-side copy if not supported
		}
	}

	return CopyPath(ctx, srcBackend, srcPath, dstBackend, dstPath, opts...)
}
