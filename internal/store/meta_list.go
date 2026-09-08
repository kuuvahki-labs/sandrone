package store

import (
	"context"
	"strings"
	"sync"
)

// mapMetadata keeps list order (including error selection) independent of I/O
// completion order. Unknown Store implementations retain serial access.
func mapMetadata[T any](ctx context.Context, st Store, entries []ListedEntry, read func(ListedEntry) (T, error)) ([]T, []error) {
	values := make([]T, len(entries))
	errs := make([]error, len(entries))
	workers := 1
	if allowsConcurrentReads(st) {
		workers = min(4, len(entries))
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range jobs {
				if err := ctx.Err(); err != nil {
					errs[i] = err
					continue
				}
				values[i], errs[i] = read(entries[i])
			}
		})
	}
	for i := range entries {
		if ctx.Err() != nil {
			for j := i; j < len(entries); j++ {
				errs[j] = ctx.Err()
			}
			break
		}
		select {
		case jobs <- i:
		case <-ctx.Done():
			errs[i] = ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	return values, errs
}

func metadataEntries(ctx context.Context, st Store, prefix string) ([]ListedEntry, error) {
	entries, err := ListPrefix(ctx, st, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]ListedEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir && strings.HasSuffix(entry.Key, ".json") {
			out = append(out, entry)
		}
	}
	return out, nil
}
