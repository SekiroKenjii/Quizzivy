package assignments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/assignments"
)

// sitAttempt records that the student sat the assignment: one attempt in the
// given status, holding one answer worth `earned` (auto-graded) and, when
// `pending` is set, one short answer nobody has graded.
func sitAttempt(t *testing.T, pool *pgxpool.Pool, w world, assignmentID, status string, earned string, pending bool) string {
	t.Helper()
	ctx := context.Background()
	var section, question, attemptID string
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `
		INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		VALUES ($1::uuid, (SELECT count(*) FROM app.test_version_sections WHERE test_version_id = $1::uuid), 'Phần')
		RETURNING id::text`, w.versionID).Scan(&section))
	must(pool.QueryRow(ctx, `
		INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
		VALUES ($1::uuid, 0, 'single_choice', 'Câu?', '5.00') RETURNING id::text`, section).Scan(&question))
	must(pool.QueryRow(ctx, `
		INSERT INTO app.attempts
		  (assignment_id, test_version_id, student_id, attempt_no, status, session_id,
		   shuffle_seed, beacon_token_hash, started_at, deadline_at, submitted_at,
		   graded_at, void_reason, score_earned, score_total)
		VALUES ($1::uuid, $2::uuid, $3::uuid,
		        (SELECT coalesce(max(attempt_no), 0) + 1 FROM app.attempts
		          WHERE assignment_id = $1::uuid AND student_id = $3::uuid),
		        $4::app.attempt_status, gen_random_uuid(), 7, sha256('b'::bytea),
		        now() - interval '30 minutes', now() + interval '30 minutes',
		        CASE WHEN $4 = 'in_progress' THEN NULL ELSE now() END,
		        CASE WHEN $4 = 'graded' THEN now() END,
		        CASE WHEN $4 = 'voided' THEN 'reset by the teacher' END,
		        CASE WHEN $4 IN ('submitted', 'timed_out', 'graded') THEN $5::numeric END,
		        CASE WHEN $4 IN ('submitted', 'timed_out', 'graded') THEN '10.00'::numeric END)
		RETURNING id::text`, assignmentID, w.versionID, w.student, status, earned).Scan(&attemptID))
	_, err := pool.Exec(ctx, `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual, auto_score)
		VALUES ($1::uuid, $2::uuid, '{"type":"choice","optionIds":[]}', false, $3::numeric)`,
		attemptID, question, earned)
	must(err)
	if pending {
		var essay string
		must(pool.QueryRow(ctx, `
			INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
			VALUES ($1::uuid, 1, 'short_answer', 'Viết.', '5.00') RETURNING id::text`, section).Scan(&essay))
		_, err = pool.Exec(ctx, `
			INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual)
			VALUES ($1::uuid, $2::uuid, '{"type":"text","value":"..."}', true)`, attemptID, essay)
		must(err)
	}
	return attemptID
}

func createFor(t *testing.T, store *assignments.Store, w world, in assignments.WriteInput) assignments.Assignment {
	t.Helper()
	a, err := store.Create(context.Background(), request(w), in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return a
}

func ids(cards []assignments.StudentCard) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.ID)
	}
	return out
}

func only(t *testing.T, cards []assignments.StudentCard, want string) assignments.StudentCard {
	t.Helper()
	if len(cards) != 1 || cards[0].ID != want {
		t.Fatalf("cards %v, want exactly [%s]", ids(cards), want)
	}
	return cards[0]
}

// §9's three sections, and the one thing that appears in none of them.
func TestAStudentsHomeSortsByWhatTheyCanDoNext(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	now := time.Now()

	open := createFor(t, store, w, legalInput(w))
	later := legalInput(w)
	later.OpensAt, later.ClosesAt = now.Add(time.Hour), now.Add(2*time.Hour)
	scheduled := createFor(t, store, w, later)
	past := legalInput(w)
	past.OpensAt, past.ClosesAt = now.Add(-3*time.Hour), now.Add(-time.Hour)
	missed := createFor(t, store, w, past)
	draft := legalInput(w)
	draft.Draft = true
	unpublished := createFor(t, store, w, draft)

	sections, err := store.ForStudent(context.Background(), w.student, now)
	if err != nil {
		t.Fatal(err)
	}
	due := only(t, sections.DueNow, open.ID)
	if due.AttemptsUsed != 0 || due.HasLiveAttempt || due.Score != nil {
		t.Errorf("a fresh assignment: %+v", due)
	}
	only(t, sections.Upcoming, scheduled.ID)
	if len(sections.Completed) != 0 {
		t.Errorf("completed %v, want none", ids(sections.Completed))
	}
	for _, absent := range []string{missed.ID, unpublished.ID} {
		all := append(append(ids(sections.DueNow), ids(sections.Upcoming)...), ids(sections.Completed)...)
		for _, id := range all {
			if id == absent {
				t.Errorf("%s is listed; a draft and a missed assignment belong nowhere", absent)
			}
		}
	}
}

func TestAStudentOutsideTheTargetsSeesNothing(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))

	var outsider string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Ngoài lớp','student') RETURNING id::text`,
		"asg-o-"+nonce(t)+"@example.com").Scan(&outsider); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, outsider) })

	sections, err := store.ForStudent(ctx, outsider, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(sections.DueNow) + len(sections.Upcoming) + len(sections.Completed); n != 0 {
		t.Errorf("%d cards for a student who is not targeted", n)
	}
	if _, err := store.StudentDetail(ctx, a.ID, outsider); !errors.Is(err, assignments.ErrForbidden) {
		t.Errorf("detail for an outsider: %v, want ErrForbidden", err)
	}
}

// Reached through the class and named directly is one assignment, not two.
func TestATargetOnBothListsIsOneCard(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	in := legalInput(w)
	in.StudentIDs = []string{w.student}
	a := createFor(t, store, w, in)

	sections, err := store.ForStudent(context.Background(), w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	only(t, sections.DueNow, a.ID)
}

// A live attempt is burning a server-side clock the student cannot see from
// anywhere else, so it is due whatever the window says.
func TestALiveAttemptIsDueEvenAfterTheWindowClosed(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	a := createFor(t, store, w, legalInput(w))
	live := sitAttempt(t, pool, w, a.ID, "in_progress", "0.00", false)

	sections, err := store.ForStudent(context.Background(), w.student, time.Now().Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	card := only(t, sections.DueNow, a.ID)
	if !card.HasLiveAttempt || card.AttemptsUsed != 0 {
		t.Errorf("live: %+v", card)
	}
	if card.LastAttemptID == nil || *card.LastAttemptID != live {
		t.Errorf("lastAttemptId %v, want the live one", card.LastAttemptID)
	}
}

func TestAFinishedAttemptMovesItToCompletedWithAScoreOnlyWhenShown(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	ctx := context.Background()

	shown := createFor(t, store, w, legalInput(w))
	hidden := legalInput(w)
	hidden.Review.ShowScore = false
	quiet := createFor(t, store, w, hidden)
	sitAttempt(t, pool, w, shown.ID, "submitted", "3.50", true)
	sitAttempt(t, pool, w, quiet.ID, "graded", "5.00", false)

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections.DueNow) != 0 {
		t.Errorf("due %v, want none: both were sat and allow one attempt", ids(sections.DueNow))
	}
	if len(sections.Completed) != 2 {
		t.Fatalf("completed %v, want both", ids(sections.Completed))
	}
	for _, c := range sections.Completed {
		switch c.ID {
		case shown.ID:
			if c.AttemptsUsed != 1 || c.Score == nil {
				t.Fatalf("shown: %+v", c)
			}
			if c.Score.Earned != 3.5 || c.Score.Total != 10 || c.Score.PendingManual != 1 {
				t.Errorf("score %+v, want 3.5/10 with 1 pending", *c.Score)
			}
		case quiet.ID:
			if c.Score != nil {
				t.Errorf("score shown although review.showScore is off: %+v", *c.Score)
			}
		}
	}
}

func TestAVoidedAttemptIsNotUsed(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	a := createFor(t, store, w, legalInput(w))
	sitAttempt(t, pool, w, a.ID, "voided", "0.00", false)

	sections, err := store.ForStudent(context.Background(), w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	card := only(t, sections.DueNow, a.ID)
	if card.AttemptsUsed != 0 || card.LastAttemptID != nil || card.Score != nil {
		t.Errorf("a voided attempt leaked into the card: %+v", card)
	}
}

func TestTheIntroStatesThePaperItIsFor(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	ctx := context.Background()

	in := legalInput(w)
	in.MaxAttempts = 2
	in.Integrity = assignments.Integrity{
		RequireFullscreen: true, BlockCopyPaste: false, MaxFocusLoss: 2,
		OnLimitExceeded: "warn", MinAwayMs: 3000,
	}
	a := createFor(t, store, w, in)

	d, err := store.StudentDetail(ctx, a.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if d.Review.ShowScore != true || d.Review.ShowCorrectAnswers || d.Review.ShowExplanations {
		t.Errorf("review did not round-trip: %+v", d.Review)
	}
	if d.MaxAttempts != 2 || !d.Integrity.RequireFullscreen || d.Integrity.BlockCopyPaste ||
		d.Integrity.MaxFocusLoss != 2 || d.Integrity.OnLimitExceeded != "warn" {
		t.Errorf("policy did not round-trip: %+v", d.Integrity)
	}
	if d.HasAudio || d.AudioMaxPlays != nil {
		t.Errorf("no listening question, yet hasAudio=%v maxPlays=%v", d.HasAudio, d.AudioMaxPlays)
	}
}

// "Mỗi câu nghe được phát tối đa 2 lần": the strictest cap on the paper, and
// nothing when every listening question is unlimited.
func TestTheIntroReportsTheStrictestAudioCap(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	id := nonce(t)
	var section, asset string
	must(pool.QueryRow(ctx, `
		INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		VALUES ($1::uuid, 0, 'Nghe') RETURNING id::text`, w.versionID).Scan(&section))
	must(pool.QueryRow(ctx, `
		INSERT INTO app.media_assets
		  (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
		VALUES ('audio', $1, 'audio/mpeg', 1000, 1000, 'a.mp3', sha256(convert_to($1, 'UTF8')), $2::uuid)
		RETURNING id::text`, "audio/asg-"+id+".mp3", w.admin).Scan(&asset))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM app.media_assets WHERE id = $1::uuid`, asset)
	})
	listening := func(ordinal int, maxPlays *int) {
		_, err := pool.Exec(ctx, `
			INSERT INTO app.test_version_questions
			  (test_version_section_id, ordinal, type, prompt, points,
			   media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after, audio_max_plays)
			VALUES ($1::uuid, $2, 'single_choice', 'Nghe?', '5.00', $3::uuid, 'audio', false, false, $4)`,
			section, ordinal, asset, maxPlays)
		must(err)
	}

	listening(0, nil)
	d, err := store.StudentDetail(ctx, a.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if !d.HasAudio || d.AudioMaxPlays != nil {
		t.Errorf("one unlimited listening question: hasAudio=%v maxPlays=%v", d.HasAudio, d.AudioMaxPlays)
	}

	three, two := 3, 2
	listening(1, &three)
	listening(2, &two)
	d, err = store.StudentDetail(ctx, a.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if d.AudioMaxPlays == nil || *d.AudioMaxPlays != 2 {
		t.Errorf("maxPlays %v, want the strictest, 2", d.AudioMaxPlays)
	}
}

// A tab closed before the clock ran out leaves an in_progress row past its
// deadline. The server times it out at the next contact and refuses a resume,
// so the home must not offer one -- the attempt is spent, and says so.
func TestAnAttemptLeftOpenPastItsDeadlineIsSpentNotLive(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := assignments.NewStore(pool)
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))
	stale := sitAttempt(t, pool, w, a.ID, "in_progress", "0.00", false)
	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET started_at = now() - interval '2 hours',
		                         deadline_at = now() - interval '1 hour' WHERE id = $1::uuid`, stale); err != nil {
		t.Fatal(err)
	}

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections.DueNow) != 0 {
		t.Errorf("due %v, want none: nothing here can be resumed or started", ids(sections.DueNow))
	}
	card := only(t, sections.Completed, a.ID)
	if card.HasLiveAttempt || card.AttemptsUsed != 1 || card.Score != nil {
		t.Errorf("stale attempt: %+v, want spent, not live, no score", card)
	}

	d, err := store.StudentDetail(ctx, a.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if d.HasLiveAttempt || d.AttemptsUsed != 1 {
		t.Errorf("intro would offer a resume the server refuses: %+v", d.StudentCard)
	}
}
