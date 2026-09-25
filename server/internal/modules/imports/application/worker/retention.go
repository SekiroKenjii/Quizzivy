package worker

import (
	"context"
	"time"

	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
)

// Sweeper applies the retention policy: it closes imports left idle, then
// removes the files of terminal imports the policy no longer keeps. Objects are
// deleted before the draft and the removal mark, so a failure leaves the import
// to the next sweep rather than marked with bytes still stored.
type Sweeper struct {
	Repo   ports.Retention
	Store  ports.ObjectRemover
	Policy domain.Retention
	Batch  int
}

// Swept counts one sweep's work.
type Swept struct {
	Closed, Removed, Failed int
}

// Sweep works through everything due as of now, a batch at a time, until
// nothing is left or ctx ends. An import whose removal fails is passed over
// for the rest of the sweep, so it cannot hold back the others.
func (s Sweeper) Sweep(ctx context.Context, now time.Time) (Swept, error) {
	var out Swept
	if err := s.closeIdle(ctx, now, &out); err != nil {
		return out, err
	}
	return out, s.removeDue(ctx, now, &out)
}

func (s Sweeper) closeIdle(ctx context.Context, now time.Time, out *Swept) error {
	for {
		closed, err := s.Repo.CloseIdle(ctx, now.Add(-s.Policy.Idle), s.Batch)
		if err != nil {
			return err
		}
		out.Closed += len(closed)
		if len(closed) < s.Batch {
			return nil
		}
	}
}

func (s Sweeper) removeDue(ctx context.Context, now time.Time, out *Swept) error {
	var after domain.Cursor
	for {
		due, err := s.Repo.ExpiredFiles(ctx, now.Add(-s.Policy.AfterCommit), now.Add(-s.Policy.AfterCancel), after, s.Batch)
		if err != nil {
			return err
		}
		for _, next := range due {
			after = next
			if err := s.remove(ctx, next.ID); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				out.Failed++
				continue
			}
			out.Removed++
		}
		if len(due) < s.Batch {
			return nil
		}
	}
}

func (s Sweeper) remove(ctx context.Context, importID string) error {
	keys, err := s.Repo.FilesOf(ctx, importID)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := s.Store.Delete(ctx, key); err != nil {
			return err
		}
	}
	return s.Repo.FilesRemoved(ctx, importID)
}
