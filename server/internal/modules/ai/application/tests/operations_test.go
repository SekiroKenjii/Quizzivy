package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

const (
	sourceText = "Cities around the world are rethinking how they use open land. Fifty years ago, planners saw parks as decoration."
	left       = 4242
)

var teacher = actor.Actor{ID: teacherID, Scope: access.Scope{UserID: teacherID}}

var graders = access.NewSet(access.AIUse, access.TeachingGrading)

type fakeCompleter struct {
	mu       sync.Mutex
	requests []ports.Request
	usage    ports.Usage
	fail     error
	object   func(ports.Request) string
}

func (c *fakeCompleter) Complete(_ context.Context, req ports.Request) (ports.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	answer := canned(req)
	if c.object != nil {
		answer = c.object(req)
	}
	response := ports.Response{Object: json.RawMessage(answer), Usage: c.usage, RequestID: "req_ops"}
	return response, c.fail
}

func canned(req ports.Request) string {
	var schema struct {
		Required []string `json:"required"`
	}
	_ = json.Unmarshal(req.Schema, &schema)
	switch strings.Join(schema.Required, ",") {
	case "questions":
		return `{"questions":[{"type":"single_choice","prompt":"What did planners see parks as?","options":["Decoration","Roads","Cars","Shops"],"answers":["Decoration"],"explanation":"Paragraph A."}]}`
	case "findings":
		return `{"findings":[{"question":1,"severity":"minor","issue":"The wording is loose.","fix":{"kind":"none","description":"Tighten it.","label":"","option":"","blank":0,"text":""}}]}`
	case "changes":
		return `{"changes":[{"question":1,"kind":"reword","prompt":"What were parks thought to be?","order":[],"option":"","text":""}]}`
	case "explanation":
		return `{"explanation":"Because paragraph A says so."}`
	case "distractors":
		return `{"distractors":[{"text":"Libraries","why":"Sounds civic."}]}`
	case "score", "score,confidence,reasons,comment":
		return `{"score":0.9,"confidence":"high","reasons":["Matches."],"comment":"Close."}`
	case "insights":
		return `{"insights":[{"kind":"problem","title":"A common slip","body":"Six papers wrote the plural.","action":"practice","questions":[1]}]}`
	case "duplicates,tags":
		return `{"duplicates":[{"a":"q1","b":"q2","why":"Same grammar point."}],"tags":[{"question":"q3","tags":["Tenses"],"level":"b1"}]}`
	}
	return `{}`
}

func (c *fakeCompleter) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var all []string
	for _, r := range c.requests {
		all = append(all, r.System)
		for _, b := range r.Blocks {
			all = append(all, b.Text)
		}
	}
	return strings.Join(all, "\n")
}

type reads struct {
	mu    sync.Mutex
	calls []string
	scope access.Scope
	err   error

	passage   domain.Passage
	paper     domain.Paper
	question  domain.Question
	bank      domain.Bank
	answer    domain.GradedAnswer
	results   domain.Results
	prefs     domain.Preferences
	prefsErr  error
	asked     int
	lastLimit int
}

func (r *reads) note(call string, scope access.Scope) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
	r.scope = scope
	return r.err
}

func (r *reads) Passage(_ context.Context, scope access.Scope, _, _ string) (domain.Passage, error) {
	return r.passage, r.note("passage", scope)
}

func (r *reads) Draft(_ context.Context, scope access.Scope, _ string) (domain.Paper, error) {
	return r.paper, r.note("draft", scope)
}

func (r *reads) Get(_ context.Context, scope access.Scope, _ string) (domain.Question, error) {
	return r.question, r.note("question", scope)
}

func (r *reads) Own(_ context.Context, scope access.Scope, limit int) (domain.Bank, error) {
	r.lastLimit = limit
	return r.bank, r.note("own", scope)
}

func (r *reads) Answer(_ context.Context, scope access.Scope, _, _ string) (domain.GradedAnswer, error) {
	return r.answer, r.note("answer", scope)
}

func (r *reads) Results(_ context.Context, scope access.Scope, _ string) (domain.Results, error) {
	return r.results, r.note("results", scope)
}

func (r *reads) Preferences(context.Context, string) (domain.Preferences, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked++
	return r.prefs, r.prefsErr
}

func (r *reads) Permissions(context.Context, string) (access.Set, error) { return access.Set{}, nil }

func (r *reads) called() string { return strings.Join(r.calls, ",") }

type ops struct {
	repo      *fakeRepository
	app       *application.Application
	completer *fakeCompleter
	reads     *reads
	logs      *strings.Builder
}

func newOps(t *testing.T, configured bool) ops {
	t.Helper()
	repo := &fakeRepository{currentRule: domain.Rules{PoolCredits: 200_000, DefaultAllowance: 20_000, WarnPercent: 80, OnTeacherLimit: domain.LimitRequest}}
	repo.usageOut = domain.MonthUsage{Left: left}
	r := &reads{
		prefs:   domain.Preferences{Language: domain.English, ReadStudentAnswers: true},
		passage: domain.Passage{Title: "Passage 1", Text: sourceText},
		paper: domain.Paper{Questions: []domain.PaperQuestion{{
			ID: "0198f5f0-0000-7000-8000-0000000000b1", Number: 1, Type: "single_choice", Prompt: "What did planners see parks as?",
			Options: []domain.Option{{ID: "0198f5f0-0000-7000-8000-0000000000c1", Text: "Decoration", Correct: true}, {ID: "0198f5f0-0000-7000-8000-0000000000c2", Text: "Roads"}, {ID: "0198f5f0-0000-7000-8000-0000000000c3", Text: "Cars"}},
		}}},
		question: domain.Question{
			ID: "0198f5f0-0000-7000-8000-0000000000b1", Type: "single_choice", Prompt: "She ___ in Hanoi since 2019.",
			Options: []domain.Option{{ID: "o1", Text: "has lived", Correct: true}, {ID: "o2", Text: "lived"}},
		},
		bank: domain.Bank{Questions: []domain.BankQuestion{
			{ID: "b1", Type: "single_choice", Prompt: "Choose the form: She ___ since 2019.", Tags: []string{"grammar"}},
			{ID: "b2", Type: "single_choice", Prompt: "Choose the form: He ___ since 2020.", Tags: []string{"grammar"}, UsedInTests: 2},
			{ID: "b3", Type: "short_answer", Prompt: "Name one park.", Level: ""},
		}},
		answer: domain.GradedAnswer{Type: "short_answer", Prompt: "What should cities plan first?", Points: 1, Accepted: []string{"public green space"}, Answer: "green space"},
		results: domain.Results{HandedIn: 18, Questions: []domain.ResultQuestion{
			{Number: 1, Type: "short_answer", Prompt: "Cities should plan?", Key: "public green space", Wrong: []domain.WrongAnswer{{Text: "parks", Count: 3}}},
		}},
	}
	logs := &strings.Builder{}
	completer := &fakeCompleter{usage: ports.Usage{InputTokens: 50, OutputTokens: 25}}
	opts := application.Options{
		Prices: testPrices, Zone: saigon, Logger: slog.New(slog.NewTextHandler(logs, nil)),
		Tests: r, Questions: r, Grading: r, Analysis: r, People: r,
	}
	if configured {
		opts.Completer = completer
	}
	app := application.New(repo, opts)
	app.SetClock(func() time.Time { return september })
	return ops{repo: repo, app: app, completer: completer, reads: r, logs: logs}
}

type operation struct {
	name    string
	feature domain.Feature
	tier    ports.Tier
	run     func(ops) (domain.Usage, error)
}

func operations() []operation {
	return []operation{
		{"generate", domain.FeatureGenerate, ports.TierCapable, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Generate.Handle(context.Background(), query.Generate{
				Actor: teacher, TestID: "t-1", StimulusID: "s-1", Types: []string{"single_choice"}, Count: 3, Level: "b2", Explanations: true, Language: domain.English,
			})
			return got.Usage, err
		}},
		{"check", domain.FeatureCheck, ports.TierCapable, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Check.Handle(context.Background(), query.Check{Actor: teacher, TestID: "t-1", Checks: []string{"key"}})
			return got.Usage, err
		}},
		{"variant", domain.FeatureVariant, ports.TierCapable, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Variant.Handle(context.Background(), query.Variant{Actor: teacher, TestID: "t-1", Changes: []string{"reword"}, Difficulty: "same"})
			return got.Usage, err
		}},
		{"explanation", domain.FeatureExplanation, ports.TierCapable, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Explanation.Handle(context.Background(), query.Explanation{Actor: teacher, QuestionID: "q-1", Language: domain.English})
			return got.Usage, err
		}},
		{"distractors", domain.FeatureDistractors, ports.TierCapable, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Distractors.Handle(context.Background(), query.Distractors{Actor: teacher, QuestionID: "q-1"})
			return got.Usage, err
		}},
		{"score", domain.FeatureScore, ports.TierCapable, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Score.Handle(context.Background(), query.Score{Actor: teacher, Grants: graders, AttemptID: "a-1", QuestionID: "q-1"})
			return got.Usage, err
		}},
		{"insights", domain.FeatureInsights, ports.TierCapable, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Insights.Handle(context.Background(), query.Insights{Actor: teacher, Grants: graders, AssignmentID: "as-1"})
			return got.Usage, err
		}},
		{"cleanup", domain.FeatureCleanup, ports.TierFast, func(o ops) (domain.Usage, error) {
			got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{"duplicates", "tags"}})
			return got.Usage, err
		}},
	}
}

func TestEveryOperationAnswersOffWhileNoProviderIsConfiguredAndTouchesNothing(t *testing.T) {
	for _, op := range operations() {
		t.Run(op.name, func(t *testing.T) {
			o := newOps(t, false)
			if _, err := op.run(o); !errors.Is(err, domain.ErrOff) {
				t.Fatalf("err = %v, want ErrOff", err)
			}
			if len(o.repo.reserved) != 0 || len(o.reads.calls) != 0 || o.reads.asked != 0 {
				t.Errorf("reserved %d, read %q, asked for preferences %d times; want nothing touched", len(o.repo.reserved), o.reads.called(), o.reads.asked)
			}
		})
	}
}

func TestEveryOperationReservesItsCeilingCallsItsTierAndSettlesWhatWasBilled(t *testing.T) {
	for _, op := range operations() {
		t.Run(op.name, func(t *testing.T) {
			o := newOps(t, true)
			usage, err := op.run(o)
			if err != nil {
				t.Fatal(err)
			}
			if len(o.repo.reserved) != 1 || o.repo.reserved[0].Feature != op.feature || o.repo.reserved[0].Credits != op.feature.Ceiling() || o.repo.reserved[0].UserID != teacherID {
				t.Errorf("reserved %+v, want the %s ceiling of %d for the teacher", o.repo.reserved, op.feature, op.feature.Ceiling())
			}
			if len(o.completer.requests) != 1 {
				t.Fatalf("%d provider calls, want 1", len(o.completer.requests))
			}
			req := o.completer.requests[0]
			if req.Tier != op.tier || req.System == "" || len(req.Blocks) != 1 || req.Blocks[0].Kind != ports.BlockText || req.MaxTokens < 1 || !json.Valid(req.Schema) {
				t.Errorf("request %+v: want the %s tier, a system prompt, one text block, a token limit and a valid schema", req, op.tier)
			}
			if len(o.repo.settled) != 1 || len(o.repo.released) != 0 {
				t.Fatalf("settled %d, released %d; want it settled once", len(o.repo.settled), len(o.repo.released))
			}
			if got := o.repo.settled[0]; got.RequestID != "req_ops" || got.Tokens.Input != 50 || got.Tokens.Output != 25 || got.Tier != domain.Tier(op.tier) {
				t.Errorf("settled %+v", got)
			}
			charged := int64(100)
			if op.tier == ports.TierFast {
				charged = 1
			}
			if usage != (domain.Usage{Credits: charged, Left: left}) {
				t.Errorf("usage %+v, want the %d credits the call was charged and what is left", usage, charged)
			}
		})
	}
}

func TestARefusedReservationNeverReachesTheProviderOrTheReads(t *testing.T) {
	for _, refusal := range []error{domain.ErrPaused, &domain.Exhausted{Scope: domain.ScopeTeacher}, &domain.Exhausted{Scope: domain.ScopePool}} {
		for _, op := range operations() {
			t.Run(fmt.Sprintf("%s/%v", op.name, refusal), func(t *testing.T) {
				o := newOps(t, true)
				o.repo.reserveErr = refusal
				if _, err := op.run(o); err != refusal {
					t.Fatalf("err = %v, want %v as it is", err, refusal)
				}
				if len(o.completer.requests) != 0 || len(o.repo.settled) != 0 || len(o.repo.released) != 0 {
					t.Errorf("called %d times, settled %d, released %d; want nothing after a refusal", len(o.completer.requests), len(o.repo.settled), len(o.repo.released))
				}
			})
		}
	}
}

func TestACallThatFailsIsSettledOnlyForWhatTheProviderBilledAndTheCallerIsToldToTryAgain(t *testing.T) {
	for name, c := range map[string]struct {
		err         error
		usage       ports.Usage
		settles     bool
		unavailable bool
	}{
		"the provider is down":                {fmt.Errorf("%w: 529", ports.ErrUnavailable), ports.Usage{}, false, true},
		"a refusal, which was billed":         {fmt.Errorf("%w: stop", ports.ErrRefused), ports.Usage{InputTokens: 40, OutputTokens: 5}, true, true},
		"an answer cut off, which was billed": {fmt.Errorf("%w: cut", ports.ErrInvalidResponse), ports.Usage{InputTokens: 40, OutputTokens: 4096}, true, true},
		"a rejected request":                  {fmt.Errorf("%w: 400", ports.ErrRejected), ports.Usage{}, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			o := newOps(t, true)
			o.completer.fail, o.completer.usage = c.err, c.usage
			_, err := operations()[0].run(o)
			if err == nil || errors.Is(err, domain.ErrUnavailable) != c.unavailable {
				t.Fatalf("err = %v, want unavailable %v", err, c.unavailable)
			}
			if c.settles != (len(o.repo.settled) == 1) || c.settles == (len(o.repo.released) == 1) {
				t.Errorf("settled %d, released %d, want settled %v", len(o.repo.settled), len(o.repo.released), c.settles)
			}
			if strings.Contains(err.Error(), "sk-") {
				t.Errorf("the error carries a key: %v", err)
			}
		})
	}
}

func TestAnAnswerWithNothingUsableIsSettledAndReportedUnavailable(t *testing.T) {
	for name, object := range map[string]string{
		"no question left":                 `{"questions":[]}`,
		"a question that fails its checks": `{"questions":[{"type":"single_choice","prompt":"x","options":["a","b"],"answers":["c"],"explanation":""}]}`,
		"a field nobody asked for":         `{"questions":[],"note":"hello"}`,
		"not the schema's shape":           `{"unexpected":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			o := newOps(t, true)
			o.completer.object = func(ports.Request) string { return object }
			_, err := operations()[0].run(o)
			if !errors.Is(err, domain.ErrUnavailable) {
				t.Fatalf("err = %v, want ErrUnavailable", err)
			}
			if len(o.repo.settled) != 1 || len(o.repo.released) != 0 {
				t.Errorf("settled %d, released %d; the provider was paid for the answer", len(o.repo.settled), len(o.repo.released))
			}
		})
	}
}

func TestACancelledCallStillReleasesItsReservation(t *testing.T) {
	o := newOps(t, true)
	o.completer.fail = context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := o.app.Queries.Distractors.Handle(ctx, query.Distractors{Actor: teacher, QuestionID: "q-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
	if len(o.repo.released) != 1 {
		t.Errorf("released %d, want the reservation returned although the caller is gone", len(o.repo.released))
	}
}

func TestTheScoreAndInsightsNeedGradingAndAPreferenceThatLetsTheAssistantReadAnswers(t *testing.T) {
	for _, op := range operations() {
		if op.name != "score" && op.name != "insights" {
			continue
		}
		t.Run(op.name+"/without teaching.grading", func(t *testing.T) {
			o := newOps(t, true)
			var err error
			if op.name == "score" {
				_, err = o.app.Queries.Score.Handle(context.Background(), query.Score{Actor: teacher, Grants: access.NewSet(access.AIUse), AttemptID: "a", QuestionID: "q"})
			} else {
				_, err = o.app.Queries.Insights.Handle(context.Background(), query.Insights{Actor: teacher, Grants: access.NewSet(access.AIUse), AssignmentID: "a"})
			}
			if !errors.Is(err, domain.ErrMissingPermission) {
				t.Fatalf("err = %v, want ErrMissingPermission", err)
			}
			if len(o.reads.calls) != 0 || len(o.repo.reserved) != 0 {
				t.Errorf("read %q, reserved %d; want nothing before the permission is checked", o.reads.called(), len(o.repo.reserved))
			}
		})
		t.Run(op.name+"/preference off", func(t *testing.T) {
			o := newOps(t, true)
			o.reads.prefs.ReadStudentAnswers = false
			if _, err := op.run(o); !errors.Is(err, domain.ErrReadOff) {
				t.Fatalf("err = %v, want ErrReadOff", err)
			}
			if len(o.reads.calls) != 0 || len(o.repo.reserved) != 0 || len(o.completer.requests) != 0 {
				t.Errorf("read %q, reserved %d, called %d; want no student answer read once the teacher turned it off", o.reads.called(), len(o.repo.reserved), len(o.completer.requests))
			}
		})
		t.Run(op.name+"/preference unreadable", func(t *testing.T) {
			o := newOps(t, true)
			o.reads.prefsErr = errors.New("database down")
			if _, err := op.run(o); err == nil || len(o.reads.calls) != 0 {
				t.Errorf("err = %v, read %q; want a refusal that reads nothing", err, o.reads.called())
			}
		})
	}
}

func TestAScoreSuggestionIsRefusedForAnythingButAMarkedByHandAnsweredQuestion(t *testing.T) {
	for name, c := range map[string]struct {
		answer domain.GradedAnswer
		err    error
		want   error
	}{
		"a choice question": {domain.GradedAnswer{Type: "single_choice", Answer: "x"}, nil, domain.ErrNotManual},
		"a blank answer":    {domain.GradedAnswer{Type: "short_answer", Points: 1, Answer: " \t "}, nil, domain.ErrUnanswered},
		"in progress":       {domain.GradedAnswer{}, domain.ErrAttemptInProgress, domain.ErrAttemptInProgress},
		"voided":            {domain.GradedAnswer{}, domain.ErrAttemptVoided, domain.ErrAttemptVoided},
		"out of reach":      {domain.GradedAnswer{}, domain.ErrNotFound, domain.ErrNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			o := newOps(t, true)
			o.reads.answer, o.reads.err = c.answer, c.err
			_, err := operations()[5].run(o)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if len(o.repo.reserved) != 0 || len(o.completer.requests) != 0 {
				t.Errorf("reserved %d, called %d; want a refusal before any credit is held", len(o.repo.reserved), len(o.completer.requests))
			}
		})
	}
}

func TestTheScoreIsMovedToTheNearestAllowedOne(t *testing.T) {
	o := newOps(t, true)
	got, err := o.app.Queries.Score.Handle(context.Background(), query.Score{Actor: teacher, Grants: graders, AttemptID: "a", QuestionID: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Suggestion.Score != 1 || got.Suggestion.Confidence != "high" || len(got.Suggestion.Reasons) != 1 || got.Suggestion.Comment != "Close." {
		t.Errorf("suggestion %+v, want 0.9 moved to the allowed 1 of a one-point question", got.Suggestion)
	}
}

func TestEveryReadIsMadeUnderTheCallersScoreAndTheCleanupUnderTheirOwnRows(t *testing.T) {
	o := newOps(t, true)
	admin := actor.Actor{ID: teacherID, Scope: access.Scope{UserID: teacherID, All: true}}
	if _, err := o.app.Queries.Check.Handle(context.Background(), query.Check{Actor: admin, TestID: "t", Checks: []string{"key"}}); err != nil {
		t.Fatal(err)
	}
	if !o.reads.scope.All || o.reads.scope.UserID != teacherID {
		t.Errorf("the draft was read under %+v, want the caller's scope", o.reads.scope)
	}
	if _, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: admin, Checks: []string{"tags"}}); err != nil {
		t.Fatal(err)
	}
	if o.reads.lastLimit != query.MaxCleanupQuestions {
		t.Errorf("the clean-up asked for %d questions, want %d", o.reads.lastLimit, query.MaxCleanupQuestions)
	}
}

func TestAnotherTeachersIdAnswersAsMissingAndCostsNothing(t *testing.T) {
	for _, op := range operations() {
		if op.name == "cleanup" {
			continue
		}
		t.Run(op.name, func(t *testing.T) {
			o := newOps(t, true)
			o.reads.err = domain.ErrNotFound
			if _, err := op.run(o); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
			if len(o.repo.reserved) != 0 || len(o.completer.requests) != 0 {
				t.Errorf("reserved %d, called %d; a missing item costs nothing", len(o.repo.reserved), len(o.completer.requests))
			}
		})
	}
}

func TestGenerateNeedsExactlyOneSourceOfAFewSentences(t *testing.T) {
	base := query.Generate{Actor: teacher, Types: []string{"single_choice"}, Count: 3, Level: "b1", Language: domain.English}
	for name, c := range map[string]struct {
		mod    func(q *query.Generate)
		fields []string
	}{
		"no source":              {func(q *query.Generate) {}, []string{"source"}},
		"both sources":           {func(q *query.Generate) { q.Source, q.TestID, q.StimulusID = sourceText, "t", "s" }, []string{"source"}},
		"a test with no passage": {func(q *query.Generate) { q.TestID = "t" }, []string{"stimulusId"}},
		"a short source":         {func(q *query.Generate) { q.Source = "Too short." }, []string{"source"}},
	} {
		t.Run(name, func(t *testing.T) {
			o := newOps(t, true)
			q := base
			c.mod(&q)
			_, err := o.app.Queries.Generate.Handle(context.Background(), q)
			if got := refusedFields(t, err); strings.Join(got, ",") != strings.Join(c.fields, ",") {
				t.Errorf("refused %v, want %v", got, c.fields)
			}
			if len(o.repo.reserved) != 0 {
				t.Errorf("reserved %d after a refusal", len(o.repo.reserved))
			}
		})
	}

	o := newOps(t, true)
	q := base
	q.TestID, q.StimulusID = "t", "s"
	if _, err := o.app.Queries.Generate.Handle(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if o.reads.called() != "passage" || !strings.Contains(o.completer.text(), sourceText) {
		t.Errorf("read %q and sent %q; want the passage read and sent as the source", o.reads.called(), o.completer.text())
	}
}

func TestAnOperationThatNeedsThePreferredLanguageReadsItOnlyWhenTheRequestNamesNone(t *testing.T) {
	o := newOps(t, true)
	o.reads.prefs.Language = domain.Vietnamese
	if _, err := o.app.Queries.Check.Handle(context.Background(), query.Check{Actor: teacher, TestID: "t", Checks: []string{"key"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(o.completer.text(), "Vietnamese") {
		t.Errorf("the check did not write in the preferred language: %q", o.completer.text())
	}
	o = newOps(t, true)
	o.reads.prefs.Language = domain.Vietnamese
	if _, err := o.app.Queries.Check.Handle(context.Background(), query.Check{Actor: teacher, TestID: "t", Checks: []string{"key"}, Language: domain.English}); err != nil {
		t.Fatal(err)
	}
	if o.reads.asked != 0 || !strings.Contains(o.completer.text(), "in English") {
		t.Errorf("asked %d times, text %q; want the requested language used without asking", o.reads.asked, o.completer.text())
	}
}

func mentions(text, token string) bool {
	for i := 0; ; {
		at := strings.Index(text[i:], token)
		if at < 0 {
			return false
		}
		start, end := i+at, i+at+len(token)
		before := start == 0 || !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-", rune(text[start-1]))
		after := end == len(text) || !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-", rune(text[end]))
		if before && after {
			return true
		}
		i = end
	}
}

func TestNoIdentifierOfAPersonOrARecordReachesAPromptButTheContentDoes(t *testing.T) {
	content := map[string]string{
		"generate": "Cities around the world", "check": "What did planners see parks as?", "variant": "What did planners see parks as?",
		"explanation": "She ___ in Hanoi", "distractors": "has lived", "score": "green space", "insights": "parks", "cleanup": "Choose the form",
	}
	for _, op := range operations() {
		o := newOps(t, true)
		o.reads.question.Options = []domain.Option{{ID: "OPTION-ID-1", Text: "has lived", Correct: true}, {ID: "OPTION-ID-2", Text: "lived"}}
		o.reads.paper.Questions[0].ID = "PAPER-QUESTION-ID"
		for i := range o.reads.paper.Questions[0].Options {
			o.reads.paper.Questions[0].Options[i].ID = fmt.Sprintf("PAPER-OPTION-ID-%d", i)
		}
		for i := range o.reads.bank.Questions {
			o.reads.bank.Questions[i].ID = fmt.Sprintf("BANK-ID-%d", i)
		}
		if _, err := op.run(o); err != nil {
			t.Fatalf("%s: %v", op.name, err)
		}
		sent := o.completer.text()
		if !strings.Contains(sent, content[op.name]) {
			t.Errorf("%s: the prompt lacks the content it is asked about (%q):\n%s", op.name, content[op.name], sent)
		}
		for _, secret := range []string{
			teacherID, "t-1", "s-1", "q-1", "a-1", "as-1", "OPTION-ID", "PAPER-QUESTION-ID", "PAPER-OPTION-ID", "BANK-ID",
			"0198f5f0-0000-7000-8000-0000000000b1", "0198f5f0-0000-7000-8000-0000000000c1",
		} {
			if mentions(sent, secret) {
				t.Errorf("%s: the prompt carries %q, an identifier of a person or a record", op.name, secret)
			}
		}
	}
}
