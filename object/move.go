package object

import "context"

// MovePath moves an object from src to dst by copying then deleting.
// This works across different backends.
//
// If srcBackend and dstBackend are the same ExtendedBackend with Move support,
// consider using ExtendedBackend.Move() for server-side move instead.
//
// Options can be passed to configure the destination writer.
func MovePath(ctx context.Context, srcBackend Backend, srcPath string, dstBackend Backend, dstPath string, opts ...WriterOption) error {
	// Copy first
	if err := CopyPath(ctx, srcBackend, srcPath, dstBackend, dstPath, opts...); err != nil {
		return err
	}

	// Delete source after successful copy
	return srcBackend.Delete(ctx, srcPath)
}

// SmartMove attempts server-side move first, falling back to copy-then-delete.
// Use this when you want the best performance but need a guaranteed fallback.
//
// If both backends are the same ExtendedBackend with Move support, uses server-side move.
// Otherwise, falls back to MovePath (copy then delete).
func SmartMove(ctx context.Context, srcBackend Backend, srcPath string, dstBackend Backend, dstPath string, opts ...WriterOption) error {
	// Check if same backend and supports server-side move
	if srcBackend == dstBackend {
		if ext, ok := srcBackend.(ExtendedBackend); ok && ext.Features().Move {
			err := ext.Move(ctx, srcPath, dstPath)
			if err == nil || err != ErrNotSupported {
				return err
			}
			// Fall through to copy-delete if not supported
		}
	}

	return MovePath(ctx, srcBackend, srcPath, dstBackend, dstPath, opts...)
}
