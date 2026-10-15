package worker

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
)

var archiveName = regexp.MustCompile(`^export-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.zip$`)

func (r Runner) sweep(ctx context.Context) {
	entries, err := os.ReadDir(r.WorkDir)
	if err != nil {
		if !os.IsNotExist(err) {
			r.Logger.Warn("data export: list the work directory", "error", err)
		}
		return
	}
	names := map[string]string{}
	for _, entry := range entries {
		if match := archiveName.FindStringSubmatch(entry.Name()); match != nil && entry.Type().IsRegular() {
			names[match[1]] = entry.Name()
		}
	}
	if len(names) == 0 {
		return
	}
	ids := make([]string, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	running, err := r.Queue.Running(ctx, ids)
	if err != nil {
		r.Logger.Warn("data export: look for the running exports", "error", err)
		return
	}
	for _, id := range running {
		delete(names, id)
	}
	for id, name := range names {
		if err := os.Remove(filepath.Join(r.WorkDir, name)); err != nil && !os.IsNotExist(err) {
			r.Logger.Warn("data export: remove an orphaned archive", "exportId", id, "error", err)
			continue
		}
		r.Logger.Info("data export: removed an orphaned archive", "exportId", id)
	}
}
