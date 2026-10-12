package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
)

var held = domain.Reservation{ID: "r-1", Month: domain.MonthOf(september, saigon), UserID: "u-1", Feature: domain.FeatureGenerate, Credits: 200}

func billed(input, output int64) ports.Response {
	return ports.Response{Usage: ports.Usage{InputTokens: input, OutputTokens: output}, RequestID: "req_1"}
}

func TestFinishSettlesWhatTheProviderBilledAndReleasesWhatItDidNot(t *testing.T) {
	for name, c := range map[string]struct {
		response ports.Response
		err      error
		settles  bool
	}{
		"a completion":                            {billed(1_000, 500), nil, true},
		"a refusal, which the provider billed":    {billed(900, 40), fmt.Errorf("%w: stop_reason refusal", ports.ErrRefused), true},
		"an answer that is not an object":         {billed(900, 4_096), fmt.Errorf("%w: cut off", ports.ErrInvalidResponse), true},
		"an unavailable provider":                 {ports.Response{}, fmt.Errorf("%w: 529", ports.ErrUnavailable), false},
		"a rejected request":                      {ports.Response{}, fmt.Errorf("%w: 400", ports.ErrRejected), false},
		"a cancelled caller":                      {ports.Response{}, context.Canceled, false},
		"an error nobody classified":              {ports.Response{}, errors.New("socket closed"), false},
		"a refusal that reported no tokens":       {ports.Response{}, ports.ErrRefused, false},
		"a completion that reported no tokens":    {ports.Response{}, nil, false},
		"a rejection that carried tokens":         {billed(5, 5), ports.ErrRejected, false},
		"an unavailable answer that carried some": {billed(5, 5), ports.ErrUnavailable, false},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			settled, err := f.app.Commands.Finish.Handle(context.Background(), command.Finish{
				Reservation: held, Tier: ports.TierCapable, Response: c.response, Err: c.err,
			})
			if err != nil {
				t.Fatal(err)
			}
			if c.settles {
				if len(f.repo.settled) != 1 || len(f.repo.released) != 0 {
					t.Fatalf("settled %d, released %d; want it settled once", len(f.repo.settled), len(f.repo.released))
				}
				got := f.repo.settled[0]
				if got.Tier != domain.TierCapable || got.RequestID != "req_1" || got.Reservation != held {
					t.Errorf("settled %+v", got)
				}
				if got.Tokens.Input != c.response.Usage.InputTokens || got.Tokens.Output != c.response.Usage.OutputTokens {
					t.Errorf("the ledger would record %+v for %+v", got.Tokens, c.response.Usage)
				}
				if want := c.response.Usage.InputTokens + 2*c.response.Usage.OutputTokens; got.Metered != want || settled.Charged != want {
					t.Errorf("metered %d, charged %d; want %d at one credit an input token and two an output token", got.Metered, settled.Charged, want)
				}
				return
			}
			if len(f.repo.released) != 1 || len(f.repo.settled) != 0 || f.repo.released[0] != held {
				t.Fatalf("settled %d, released %v; want the reservation released", len(f.repo.settled), f.repo.released)
			}
			if settled.Charged != 0 {
				t.Errorf("a released reservation charged %d", settled.Charged)
			}
		})
	}
}

func TestSettlePricesTheTierTheCallRanOn(t *testing.T) {
	f := newFixture(t)
	for _, tier := range []ports.Tier{ports.TierCapable, ports.TierFast} {
		if _, err := f.app.Commands.Settle.Handle(context.Background(), command.Settle{
			Reservation: held, Tier: tier, Usage: ports.Usage{InputTokens: 100_000, OutputTokens: 10_000}, RequestID: "req",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := f.repo.settled[0].Metered, int64(100_000+2*10_000); got != want {
		t.Errorf("the capable tier metered %d, want %d", got, want)
	}
	if got, want := f.repo.settled[1].Metered, int64(15); got != want {
		t.Errorf("the fast tier metered %d, want %d (100k at $0.10 and 10k at $0.50 per million)", got, want)
	}
}

func TestSettleRefusesATierItHasNoPriceForAndATokenCountNoProviderReports(t *testing.T) {
	f := newFixture(t)
	for name, cmd := range map[string]command.Settle{
		"an unknown tier":   {Reservation: held, Tier: "slow", Usage: ports.Usage{InputTokens: 1}},
		"a negative count":  {Reservation: held, Tier: ports.TierFast, Usage: ports.Usage{OutputTokens: -1}},
		"a count too large": {Reservation: held, Tier: ports.TierFast, Usage: ports.Usage{InputTokens: 1 << 40}},
	} {
		if _, err := f.app.Commands.Settle.Handle(context.Background(), cmd); err == nil {
			t.Errorf("%s was settled", name)
		}
	}
	if len(f.repo.settled) != 0 {
		t.Errorf("the repository was asked to settle %d times", len(f.repo.settled))
	}
}

func TestACallThatCostMoreThanItsCeilingIsLoggedAndTheLedgerKeepsTheTokens(t *testing.T) {
	f := newFixture(t)
	f.repo.settleOut = domain.Settled{Charged: 200, Metered: 260}
	settled, err := f.app.Commands.Settle.Handle(context.Background(), command.Settle{
		Reservation: held, Tier: ports.TierCapable, Usage: ports.Usage{InputTokens: 260}, RequestID: "req_over",
	})
	if err != nil || settled.Charged != 200 || settled.Metered != 260 {
		t.Fatalf("settled %+v, %v", settled, err)
	}
	if !strings.Contains(f.logs.String(), "ai call cost more than its ceiling") || !strings.Contains(f.logs.String(), "req_over") {
		t.Errorf("the overrun was not logged: %s", f.logs)
	}
}

func TestAnErrorFromTheStoreReachesTheCaller(t *testing.T) {
	f := newFixture(t)
	f.repo.settleErr = domain.ErrReservationGone
	if _, err := f.app.Commands.Settle.Handle(context.Background(), command.Settle{Reservation: held, Tier: ports.TierFast, Usage: ports.Usage{InputTokens: 1}}); !errors.Is(err, domain.ErrReservationGone) {
		t.Errorf("settle: %v", err)
	}
	f.repo.releaseErr = errors.New("the database is away")
	if _, err := f.app.Commands.Release.Handle(context.Background(), command.Release{Reservation: held}); err == nil {
		t.Error("a failed release was swallowed")
	}
	if _, err := f.app.Commands.Finish.Handle(context.Background(), command.Finish{Reservation: held, Tier: ports.TierFast, Err: ports.ErrRejected}); err == nil {
		t.Error("a failed release by Finish was swallowed")
	}
}

func TestTheFirstSettlementPastTheWarningTellsTheTeacherOnce(t *testing.T) {
	f := newFixture(t)
	f.repo.settleOut = domain.Settled{Charged: 500, Metered: 500, Warning: &domain.Warning{
		UserID: "u-1", Month: held.Month, Used: 16_500, Allowance: 20_000,
	}}
	if _, err := f.app.Commands.Settle.Handle(context.Background(), command.Settle{Reservation: held, Tier: ports.TierCapable, Usage: ports.Usage{InputTokens: 500}}); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 1 {
		t.Fatalf("%d notices, want one", len(f.notifier.sent))
	}
	got := f.notifier.sent[0]
	if got.UserID != "u-1" || got.Kind != notificationsdomain.AIAllowanceWarning || got.DedupeKey != "ai_warning:2090-09-01" || got.Merge != notificationsdomain.Replace {
		t.Errorf("notice %+v", got)
	}
	if params, ok := got.Params.(notificationsdomain.AllowanceWarning); !ok || params.Used != 16_500 || params.Allowance != 20_000 {
		t.Errorf("params %+v", got.Params)
	}

	f.repo.settleOut = domain.Settled{}
	if _, err := f.app.Commands.Settle.Handle(context.Background(), command.Settle{Reservation: held, Tier: ports.TierCapable, Usage: ports.Usage{InputTokens: 5}}); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 1 {
		t.Errorf("a settlement that crossed nothing told someone: %d notices", len(f.notifier.sent))
	}
}
