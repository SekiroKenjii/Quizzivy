package worker

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"quizzivy/internal/modules/exports/application/ports"
	"quizzivy/internal/modules/exports/domain"
)

const byteOrderMark = "\xEF\xBB\xBF"

type archiver struct {
	source domain.Source
	table  ports.NewTable
	now    func() time.Time
}

func (a archiver) write(ctx context.Context, path string, includeAnswers bool) (int64, error) {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("exports: create the archive: %w", err)
	}
	defer func() { _ = out.Close() }()
	archive := zip.NewWriter(out)
	files := domain.Files(includeAnswers)
	if err := a.text(archive, domain.ReadmeFile, byteOrderMark+domain.Readme(files)); err != nil {
		return 0, err
	}
	err = a.source.Read(ctx, func(snapshot domain.Snapshot) error {
		for _, file := range files {
			if err := a.csv(ctx, archive, snapshot, file, includeAnswers); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := archive.Close(); err != nil {
		return 0, fmt.Errorf("exports: finish the archive: %w", err)
	}
	if err := out.Sync(); err != nil {
		return 0, fmt.Errorf("exports: flush the archive: %w", err)
	}
	info, err := out.Stat()
	if err != nil {
		return 0, fmt.Errorf("exports: measure the archive: %w", err)
	}
	return info.Size(), nil
}

func (a archiver) entry(archive *zip.Writer, name string) (io.Writer, error) {
	w, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: a.now()})
	if err != nil {
		return nil, fmt.Errorf("exports: add %s: %w", name, err)
	}
	return w, nil
}

func (a archiver) text(archive *zip.Writer, name, body string) error {
	w, err := a.entry(archive, name)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(body)); err != nil {
		return fmt.Errorf("exports: write %s: %w", name, err)
	}
	return nil
}

func (a archiver) csv(ctx context.Context, archive *zip.Writer, snapshot domain.Snapshot, file domain.File, includeAnswers bool) error {
	w, err := a.entry(archive, file.Name)
	if err != nil {
		return err
	}
	table, err := a.table(w)
	if err != nil {
		return fmt.Errorf("exports: start %s: %w", file.Name, err)
	}
	if err := table.Row(file.Header()...); err != nil {
		return fmt.Errorf("exports: write %s: %w", file.Name, err)
	}
	err = snapshot.Rows(ctx, file, includeAnswers, func(cells []string) error {
		if err := table.Row(cells...); err != nil {
			return fmt.Errorf("exports: write %s: %w", file.Name, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := table.Flush(); err != nil {
		return fmt.Errorf("exports: write %s: %w", file.Name, err)
	}
	return nil
}
