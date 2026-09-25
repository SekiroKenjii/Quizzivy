package wordconvert

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"
)

func monitorDisk(ctx context.Context, job string, finished <-chan struct{}, result chan<- error, cancel context.CancelFunc) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			result <- nil
			return
		case <-finished:
			result <- diskBudget(job)
			return
		case <-ticker.C:
			if err := diskBudget(job); err != nil {
				cancel()
				result <- err
				return
			}
		}
	}
}

func diskBudget(job string) error {
	var total int64
	entries := 0
	return filepath.WalkDir(job, func(_ string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		entries++
		if entries > 4096 {
			return ErrLimit
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		total += info.Size()
		if total > 256<<20 {
			return ErrLimit
		}
		return nil
	})
}
