package command

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"time"
)

const (
	sweepBatch         = 100
	objectDeleteBudget = 30 * time.Second
)

// Sweep is one pass of the retention policy over the files that have left the
// library, as of Now, or of the service's clock when Now is zero. It dates the
// wait of every file nothing names yet, cancels the wait of every file that
// something names again, and removes the object of every file that has gone
// domain.RetiredFileGrace unnamed, a batch at a time until none is left or ctx
// ends. A file whose removal fails is passed over for the rest of the pass, so
// it cannot hold back the others.
type Sweep struct {
	Now time.Time
}

// Swept counts one pass. Cause is the first failure's error, for the log.
type Swept struct {
	Unreferenced int
	Cancelled    int
	Removed      int
	Failed       int
	Cause        error
}

type SweepHandler struct {
	*support.Service
}

func (s SweepHandler) Handle(ctx context.Context, cmd Sweep) (Swept, error) {
	now := cmd.Now
	if now.IsZero() {
		now = s.Now()
	}
	var out Swept
	observed, err := s.Repo.Observe(ctx, now)
	if err != nil {
		return out, err
	}
	out.Unreferenced, out.Cancelled = observed.Unreferenced, observed.Named
	return out, s.removeDue(ctx, now, &out)
}

func (s SweepHandler) removeDue(ctx context.Context, now time.Time, out *Swept) error {
	due := now.Add(-domain.RetiredFileGrace)
	after := ""
	for {
		ids, err := s.Repo.Due(ctx, due, after, sweepBatch)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			after = id
			s.remove(ctx, domain.PurgeInput{ID: id, Due: due, Now: now, Remove: s.deleteObject}, out)
		}
		if len(ids) < sweepBatch {
			return nil
		}
	}
}

func (s SweepHandler) remove(ctx context.Context, in domain.PurgeInput, out *Swept) {
	outcome, err := s.Repo.Purge(ctx, in)
	switch {
	case err != nil:
		out.Failed++
		if out.Cause == nil {
			out.Cause = err
		}
	case outcome == domain.PurgeRemoved:
		out.Removed++
	case outcome == domain.PurgeNamed:
		out.Cancelled++
	}
}

func (s SweepHandler) deleteObject(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, objectDeleteBudget)
	defer cancel()
	return s.Object.Delete(ctx, key)
}
