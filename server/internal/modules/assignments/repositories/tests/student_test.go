//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"quizzivy/internal/platform/db"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
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

func createFor(t *testing.T, store *repositories.Postgres, w world, in domain.WriteInput) domain.Assignment {
	t.Helper()
	a, err := store.Create(context.Background(), request(w), in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return a
}

func ids(cards []domain.StudentCard) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.ID)
	}
	return out
}

func only(t *testing.T, cards []domain.StudentCard, want string) domain.StudentCard {
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
	store := repositories.NewPostgres(db.NewContext(pool))
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
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))

	var outsider string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role_id) VALUES ($1,'Ngoài lớp',(SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id::text`,
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
	if _, err := store.StudentDetail(ctx, a.ID, outsider); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("detail for an outsider: %v, want ErrForbidden", err)
	}
}

// Reached through the class and named directly is one assignment, not two.
func TestATargetOnBothListsIsOneCard(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
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
	store := repositories.NewPostgres(db.NewContext(pool))
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
	store := repositories.NewPostgres(db.NewContext(pool))
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
	store := repositories.NewPostgres(db.NewContext(pool))
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
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()

	in := legalInput(w)
	in.MaxAttempts = 2
	in.Integrity = domain.Integrity{
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
	store := repositories.NewPostgres(db.NewContext(pool))
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
		  (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
		VALUES ('audio', $1, 'audio/mpeg', 1000, 1000, 'a.mp3', sha256(convert_to($1, 'UTF8')), $2::uuid, $2::uuid)
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
	store := repositories.NewPostgres(db.NewContext(pool))
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

// A token outlives the account by its TTL, so these reads check it themselves.
func TestADisabledStudentReadsNothingWhileTheirTokenLasts(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections.DueNow) != 1 {
		t.Fatalf("before disabling: %d due cards, want 1", len(sections.DueNow))
	}

	if _, err := pool.Exec(ctx,
		`UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, w.student); err != nil {
		t.Fatal(err)
	}

	sections, err = store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(sections.DueNow) + len(sections.Upcoming) + len(sections.Completed); n != 0 {
		t.Errorf("%d cards for a disabled student", n)
	}
	if _, err := store.StudentDetail(ctx, a.ID, w.student); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("intro for a disabled student: %v, want ErrForbidden", err)
	}
}

// The four facts S-03 and S-04 draw, from the pinned version and the roster.
func TestTheCardCarriesWhatTheStudentsScreensDraw(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()

	var section string
	if err := pool.QueryRow(ctx, `
		INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		VALUES ($1::uuid, 0, 'Phần') RETURNING id::text`, w.versionID).Scan(&section); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if _, err := pool.Exec(ctx, `
			INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
			VALUES ($1::uuid, $2::int, 'single_choice', 'Câu?', '2.50')`, section, i); err != nil {
			t.Fatal(err)
		}
	}
	a := createFor(t, store, w, legalInput(w))

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections.DueNow) != 1 {
		t.Fatalf("%d due cards, want 1", len(sections.DueNow))
	}
	card := sections.DueNow[0]
	if card.ClassName == nil {
		t.Error("the card names no class, and the student is in exactly one targeted class")
	}
	if card.QuestionCount != 3 {
		t.Errorf("questionCount %d, want 3", card.QuestionCount)
	}
	if card.TotalPoints != 10 {
		t.Errorf("totalPoints %v, want 10", card.TotalPoints)
	}

	d, err := store.StudentDetail(ctx, a.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if d.TeacherName == nil || *d.TeacherName != "Giáo viên" {
		t.Errorf("teacher name %v, want the assignment's author", d.TeacherName)
	}
	if d.QuestionCount != 3 || d.TotalPoints != 10 {
		t.Errorf("intro facts: %d câu, %v điểm", d.QuestionCount, d.TotalPoints)
	}
	if d.ShowsTranscript {
		t.Error("showsTranscript on a paper with no listening question")
	}
}

// A student on two targeted classes has no single answer, so they get none.
func TestTheClassNameIsOmittedUnlessThereIsExactlyOne(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()

	var second string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`,
		"Lớp hai "+nonce(t), w.admin).Scan(&second); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1::uuid`, second)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1::uuid`, second)
	})
	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, second, w.student, w.admin); err != nil {
		t.Fatal(err)
	}

	in := legalInput(w)
	in.ClassIDs = []string{w.class, second}
	createFor(t, store, w, in)

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections.DueNow) != 1 {
		t.Fatalf("%d due cards, want 1", len(sections.DueNow))
	}
	if name := sections.DueNow[0].ClassName; name != nil {
		t.Errorf("the card named %q, but the student is in two targeted classes", *name)
	}
}

// A student named directly was not reached through a class at all.
func TestAStudentReachedByNameGetsNoClassName(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))

	in := legalInput(w)
	in.ClassIDs = nil
	in.StudentIDs = []string{w.student}
	createFor(t, store, w, in)

	sections, err := store.ForStudent(context.Background(), w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections.DueNow) != 1 {
		t.Fatalf("%d due cards, want 1", len(sections.DueNow))
	}
	if name := sections.DueNow[0].ClassName; name != nil {
		t.Errorf("the card named %q for a student reached by name", *name)
	}
}

func secondClass(t *testing.T, pool *pgxpool.Pool, w world) string {
	t.Helper()
	ctx := context.Background()
	name := "Lớp hai " + nonce(t)
	t.Cleanup(func() {
		for _, stmt := range []string{
			`DELETE FROM app.assignment_classes
			  WHERE class_id IN (SELECT id FROM app.classes WHERE name = $1)`,
			`DELETE FROM app.classes WHERE name = $1`,
		} {
			if _, err := pool.Exec(context.Background(), stmt, name); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})

	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2::uuid) RETURNING id::text`,
		name, w.admin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, id, w.student, w.admin); err != nil {
		t.Fatal(err)
	}
	return id
}

func sameIDs(got []string, want ...string) bool {
	return slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want)))
}

func TestACardListsEveryTargetedClassOfTheStudent(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	second := secondClass(t, pool, w)

	in := legalInput(w)
	in.ClassIDs = []string{w.class, second}
	a := createFor(t, store, w, in)

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	card := only(t, sections.DueNow, a.ID)
	if card.ClassName != nil {
		t.Errorf("the card named %q, but the student is in two targeted classes", *card.ClassName)
	}
	if card.ClassID != nil {
		t.Errorf("classId %s, want none for a student in two targeted classes", *card.ClassID)
	}
	if !sameIDs(card.ClassIDs, w.class, second) {
		t.Errorf("classIds %v, want both %s and %s", card.ClassIDs, w.class, second)
	}

	d, err := store.StudentDetail(ctx, a.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(d.ClassIDs, w.class, second) {
		t.Errorf("the intro's classIds %v, want both %s and %s", d.ClassIDs, w.class, second)
	}
}

func TestAnArchivedClassIsNotNamedOnTheCard(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	second := secondClass(t, pool, w)

	in := legalInput(w)
	in.ClassIDs = []string{w.class, second}
	a := createFor(t, store, w, in)
	if _, err := pool.Exec(ctx,
		`UPDATE app.classes SET archived_at = now() WHERE id = $1::uuid`, second); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := pool.QueryRow(ctx,
		`SELECT name FROM app.classes WHERE id = $1::uuid`, w.class).Scan(&name); err != nil {
		t.Fatal(err)
	}

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	card := only(t, sections.DueNow, a.ID)
	if !sameIDs(card.ClassIDs, w.class) {
		t.Errorf("classIds %v, want only %s: the other class is archived", card.ClassIDs, w.class)
	}
	if card.ClassID == nil || *card.ClassID != w.class {
		t.Errorf("classId is not %s, the one class the student still sees", w.class)
	}
	if card.ClassName == nil || *card.ClassName != name {
		t.Errorf("className is not %q, the one class the student still sees", name)
	}
}

func TestAStudentReachedByNameHasNoClassIDs(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))

	in := legalInput(w)
	in.ClassIDs = nil
	in.StudentIDs = []string{w.student}
	a := createFor(t, store, w, in)

	sections, err := store.ForStudent(context.Background(), w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	card := only(t, sections.DueNow, a.ID)
	if card.ClassIDs == nil || len(card.ClassIDs) != 0 {
		t.Errorf("classIds %#v, want an empty list for a student reached by name", card.ClassIDs)
	}
}

// Two assignments: one with a live attempt is due, never completed.
func TestTheCardCarriesTheLiveDeadlineAndTheSubmissionTime(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()

	finished := createFor(t, store, w, legalInput(w)) // maxAttempts 1
	sitAttempt(t, pool, w, finished.ID, "submitted", "8.00", false)

	running := legalInput(w)
	running.MaxAttempts = 2
	live := createFor(t, store, w, running)
	sitAttempt(t, pool, w, live.ID, "in_progress", "0.00", false)

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	done := only(t, sections.Completed, finished.ID)
	if done.LastSubmittedAt == nil {
		t.Error("a submitted attempt with no submission time")
	}
	if done.LiveDeadlineAt != nil {
		t.Errorf("no attempt is live, but a deadline is offered: %v", done.LiveDeadlineAt)
	}
	if done.LiveAnsweredCount != nil {
		t.Errorf("no attempt is live, but %d answers are counted", *done.LiveAnsweredCount)
	}

	resumable := only(t, sections.DueNow, live.ID)
	if !resumable.HasLiveAttempt {
		t.Fatal("the attempt is in progress and inside its deadline")
	}
	if resumable.LiveDeadlineAt == nil {
		t.Error("a live attempt with no deadline to show")
	}
}

func askInAttempt(t *testing.T, pool *pgxpool.Pool, attemptID, kind string, blanks int) (string, []string) {
	t.Helper()
	ctx := context.Background()
	var question string
	if err := pool.QueryRow(ctx, `
		INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
		SELECT sec.id, (SELECT max(q.ordinal) + 1 FROM app.test_version_questions q
		                 WHERE q.test_version_section_id = sec.id),
		       $2::app.question_type, 'Câu?', '1.00'
		  FROM app.attempts at
		  JOIN app.test_version_sections sec ON sec.test_version_id = at.test_version_id
		 WHERE at.id = $1::uuid
		 ORDER BY sec.ordinal DESC LIMIT 1
		RETURNING id::text`, attemptID, kind).Scan(&question); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, blanks)
	for i := range ids {
		if err := pool.QueryRow(ctx, `
			INSERT INTO app.test_version_blanks (test_version_question_id, ordinal)
			VALUES ($1::uuid, $2) RETURNING id::text`, question, i+1).Scan(&ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	return question, ids
}

func liveCount(t *testing.T, store *repositories.Postgres, w world, assignmentID string) *int {
	t.Helper()
	sections, err := store.ForStudent(context.Background(), w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	all := append(append(sections.DueNow, sections.Upcoming...), sections.Completed...)
	return only(t, all, assignmentID).LiveAnsweredCount
}

// countOf prints a live count for a failure message: its value, or "none".
func countOf(n *int) string {
	if n == nil {
		return "none"
	}
	return strconv.Itoa(*n)
}

func TestTheLiveCountFollowsTheNavigatorsRule(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))
	attempt := sitAttempt(t, pool, w, a.ID, "in_progress", "0.00", false)

	const b1, b2 = "\x00b1", "\x00b2"
	cases := []struct {
		name    string
		kind    string
		blanks  int
		payload string
		says    bool
	}{
		{"a choice with an option picked", "single_choice", 0, `{"type":"choice","optionIds":["0198a3f2-7c1e-7a40-8000-000000000001"]}`, true},
		{"a choice with nothing picked", "multiple_choice", 0, `{"type":"choice","optionIds":[]}`, false},
		{"true/false answered false", "true_false", 0, `{"type":"true_false","value":false}`, true},
		{"true/false with no value", "true_false", 0, `{"type":"true_false","value":null}`, false},
		{"a choice whose options are not a list", "single_choice", 0, `{"type":"choice","optionIds":"x"}`, false},
		{"a text of spaces", "short_answer", 0, `{"type":"text","value":"   "}`, false},
		{"a text of every space JavaScript trims", "short_answer", 0, `{"type":"text","value":"\u0009\u000a\u000b\u000c\u000d\u0020\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"}`, false},
		{"a text of a space JavaScript keeps", "short_answer", 0, `{"type":"text","value":"\u200b"}`, true},
		{"a text with words", "short_answer", 0, `{"type":"text","value":"I wake up at six."}`, true},
		{"no blank typed into", "fill_blank", 2, `{"type":"fill_blank","values":{}}`, false},
		{"blanks holding only spaces", "fill_blank", 2, `{"type":"fill_blank","values":{"` + b1 + `":" ","` + b2 + `":""}}`, false},
		{"one blank of two", "fill_blank", 2, `{"type":"fill_blank","values":{"` + b1 + `":"went"}}`, false},
		{"one blank and one of spaces", "fill_blank", 2, `{"type":"fill_blank","values":{"` + b1 + `":"went","` + b2 + `":"  "}}`, false},
		{"one blank and one of the spaces JavaScript trims", "fill_blank", 2, `{"type":"fill_blank","values":{"` + b1 + `":"went","` + b2 + `":" \t"}}`, false},
		{"every blank filled", "fill_blank", 2, `{"type":"fill_blank","values":{"` + b1 + `":"went","` + b2 + `":"has"}}`, true},
		{"every blank filled beside a stale value", "fill_blank", 2, `{"type":"fill_blank","values":{"` + b1 + `":"went","` + b2 + `":"has","stale":"x"}}`, true},
		{"only a stale value", "fill_blank", 2, `{"type":"fill_blank","values":{"stale":"x"}}`, false},
		{"a fill-in whose question has no blanks", "fill_blank", 0, `{"type":"fill_blank","values":{}}`, false},
		{"an answer of no known type", "short_answer", 0, `{"type":"essay","value":"words"}`, false},
		{"an answer with no type", "short_answer", 0, `{"value":"words"}`, false},
	}

	got := liveCount(t, store, w, a.ID)
	if got == nil || *got != 0 {
		t.Fatalf("with only an empty choice saved the count is %s, want 0", countOf(got))
	}
	want := 0
	for _, c := range cases {
		question, blanks := askInAttempt(t, pool, attempt, c.kind, c.blanks)
		payload := c.payload
		for i, id := range blanks {
			payload = strings.ReplaceAll(payload, []string{b1, b2}[i], id)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO app.attempt_answers (attempt_id, question_id, payload)
			VALUES ($1::uuid, $2::uuid, $3::jsonb)`, attempt, question, payload); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if c.says {
			want++
		}
		got := liveCount(t, store, w, a.ID)
		if got == nil || *got != want {
			t.Fatalf("%s: the count is %s, want %d", c.name, countOf(got), want)
		}
	}
}

func TestTheLiveCountIsAbsentWithoutALiveAttempt(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()

	fresh := createFor(t, store, w, legalInput(w))
	if got := liveCount(t, store, w, fresh.ID); got != nil {
		t.Errorf("an assignment nobody started counts %d answers", *got)
	}

	attempt := sitAttempt(t, pool, w, fresh.ID, "in_progress", "0.00", false)
	question, _ := askInAttempt(t, pool, attempt, "true_false", 0)
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload)
		VALUES ($1::uuid, $2::uuid, '{"type":"true_false","value":true}')`, attempt, question); err != nil {
		t.Fatal(err)
	}
	if got := liveCount(t, store, w, fresh.ID); got == nil || *got != 1 {
		t.Fatalf("a live attempt with one answer counts %s, want 1", countOf(got))
	}

	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET started_at = now() - interval '2 hours',
		                         deadline_at = now() - interval '1 hour' WHERE id = $1::uuid`, attempt); err != nil {
		t.Fatal(err)
	}
	if got := liveCount(t, store, w, fresh.ID); got != nil {
		t.Errorf("an attempt past its deadline still counts %d answers", *got)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET status = 'submitted', submitted_at = now(), deadline_at = now() + interval '1 hour',
		                         score_earned = 0, score_total = 10 WHERE id = $1::uuid`, attempt); err != nil {
		t.Fatal(err)
	}
	if got := liveCount(t, store, w, fresh.ID); got != nil {
		t.Errorf("a submitted attempt still counts %d answers", *got)
	}
}

func TestTheLiveCountIsTheStudentsOwn(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))
	classmate := w
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role_id)
		 VALUES ($1, 'Bạn cùng lớp', (SELECT id FROM app.roles WHERE builtin_key = 'student'))
		 RETURNING id::text`,
		"asg-c-"+nonce(t)+"@example.com").Scan(&classmate.student); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.attempts WHERE student_id = $1::uuid`, classmate.student)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1::uuid`, classmate.student)
	})
	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`,
		w.class, classmate.student, w.admin); err != nil {
		t.Fatal(err)
	}
	theirs := sitAttempt(t, pool, classmate, a.ID, "in_progress", "0.00", false)
	question, _ := askInAttempt(t, pool, theirs, "true_false", 0)
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload)
		VALUES ($1::uuid, $2::uuid, '{"type":"true_false","value":true}')`, theirs, question); err != nil {
		t.Fatal(err)
	}
	if got := liveCount(t, store, w, a.ID); got != nil {
		t.Errorf("a classmate's live attempt counts %d answers on this student's card", *got)
	}
}

func TestTheLiveCountFollowsTheNavigatorsRuleForTheNewTypes(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	a := createFor(t, store, w, legalInput(w))
	attempt := sitAttempt(t, pool, w, a.ID, "in_progress", "0.00", false)

	const o1, o2, o3, i1, i2 = "\x00o1", "\x00o2", "\x00o3", "\x00i1", "\x00i2"
	cases := []struct {
		name    string
		kind    string
		payload string
		says    bool
	}{
		{"choose two with one pick", "choose_two", `{"type":"choice","optionIds":["` + o1 + `"]}`, false},
		{"choose two with two picks", "choose_two", `{"type":"choice","optionIds":["` + o1 + `","` + o2 + `"]}`, true},
		{"choose two with three picks", "choose_two", `{"type":"choice","optionIds":["` + o1 + `","` + o2 + `","` + o3 + `"]}`, false},
		{"choose two with one pick twice", "choose_two", `{"type":"choice","optionIds":["` + o1 + `","` + o1 + `"]}`, false},
		{"choose two with a repeated pick", "choose_two", `{"type":"choice","optionIds":["` + o1 + `","` + o2 + `","` + o1 + `"]}`, true},
		{"tfng with a pick", "tfng", `{"type":"choice","optionIds":["` + o1 + `"]}`, true},
		{"matching with no pairs", "matching", `{"type":"matching","pairs":{}}`, false},
		{"matching with one item of two", "matching", `{"type":"matching","pairs":{"` + i1 + `":"` + o1 + `"}}`, false},
		{"matching with a null pair", "matching", `{"type":"matching","pairs":{"` + i1 + `":"` + o1 + `","` + i2 + `":null}}`, false},
		{"matching with every item paired", "matching", `{"type":"matching","pairs":{"` + i1 + `":"` + o1 + `","` + i2 + `":"` + o1 + `"}}`, true},
		{"matching pairs that are not an object", "matching", `{"type":"matching","pairs":["x"]}`, false},
		{"an essay of spaces", "essay", `{"type":"text","value":"  "}`, false},
		{"an essay with words", "essay", `{"type":"text","value":"Some argue that…"}`, true},
	}
	want := 0
	for _, c := range cases {
		question, options, items := askNewTypeInAttempt(t, pool, attempt, c.kind)
		payload := strings.NewReplacer(o1, options[0], o2, options[1], o3, options[2], i1, itemAt(items, 0), i2, itemAt(items, 1)).Replace(c.payload)
		if _, err := pool.Exec(ctx, `
			INSERT INTO app.attempt_answers (attempt_id, question_id, payload)
			VALUES ($1::uuid, $2::uuid, $3::jsonb)`, attempt, question, payload); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if c.says {
			want++
		}
		if got := liveCount(t, store, w, a.ID); got == nil || *got != want {
			t.Fatalf("%s: the count is %s, want %d", c.name, countOf(got), want)
		}
	}
}

func itemAt(items []string, i int) string {
	if i < len(items) {
		return items[i]
	}
	return "none"
}

func askNewTypeInAttempt(t *testing.T, pool *pgxpool.Pool, attemptID, kind string) (string, []string, []string) {
	t.Helper()
	ctx := context.Background()
	kindOf := map[string]string{"choose_two": "multiple_choice", "tfng": "tfng", "matching": "matching", "essay": "essay"}[kind]
	var selectCount *int
	var reuse *bool
	switch kind {
	case "choose_two":
		n := 2
		selectCount = &n
	case "matching":
		yes := true
		reuse = &yes
	}
	var question string
	if err := pool.QueryRow(ctx, `
		INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, select_count, match_allow_reuse)
		SELECT sec.id, (SELECT max(q.ordinal) + 1 FROM app.test_version_questions q
		                 WHERE q.test_version_section_id = sec.id),
		       $2::app.question_type, 'Câu?', '1.00', $3, $4
		  FROM app.attempts at
		  JOIN app.test_version_sections sec ON sec.test_version_id = at.test_version_id
		 WHERE at.id = $1::uuid
		 ORDER BY sec.ordinal DESC LIMIT 1
		RETURNING id::text`, attemptID, kindOf, selectCount, reuse).Scan(&question); err != nil {
		t.Fatal(err)
	}
	if kind == "essay" {
		return question, []string{"none", "none", "none"}, nil
	}
	options := make([]string, 3)
	for i := range options {
		if err := pool.QueryRow(ctx, `INSERT INTO app.test_version_options (test_version_question_id, ordinal, text, is_correct)
			VALUES ($1::uuid, $2, 'x', $3) RETURNING id::text`, question, i, kind != "matching" && i < 2).Scan(&options[i]); err != nil {
			t.Fatal(err)
		}
	}
	if kind != "matching" {
		return question, options, nil
	}
	items := make([]string, 2)
	for i := range items {
		if err := pool.QueryRow(ctx, `INSERT INTO app.test_version_match_items (test_version_question_id, ordinal, text, correct_option_id)
			VALUES ($1::uuid, $2, 'item', $3::uuid) RETURNING id::text`, question, i, options[0]).Scan(&items[i]); err != nil {
			t.Fatal(err)
		}
	}
	return question, options, items
}

func TestACompletedCardCarriesThePapersBandOnlyWhenTheScoreIsShown(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	store := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()

	shown := createFor(t, store, w, legalInput(w))
	hidden := legalInput(w)
	hidden.Review.ShowScore = false
	quiet := createFor(t, store, w, hidden)
	attempts := []string{
		sitAttempt(t, pool, w, shown.ID, "graded", "7.00", false),
		sitAttempt(t, pool, w, quiet.ID, "graded", "7.00", false),
	}
	for _, q := range []string{
		`DELETE FROM app.test_version_options o USING app.test_version_questions q, app.test_version_sections s
		  WHERE o.test_version_question_id = q.id AND s.id = q.test_version_section_id AND s.test_version_id = $1::uuid`,
		`UPDATE app.test_version_questions q SET type = 'essay' FROM app.test_version_sections s
		  WHERE s.id = q.test_version_section_id AND s.test_version_id = $1::uuid`,
	} {
		if _, err := pool.Exec(ctx, q, w.versionID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual, manual_score, band, graded_by, graded_at)
		SELECT at, q.id, '{"type":"text","value":"An essay."}', true, round(q.points * 6.5 / 9, 2), 6.5, $3::uuid, now()
		  FROM unnest($2::uuid[]) AS at
		  CROSS JOIN app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		ON CONFLICT (attempt_id, question_id) DO UPDATE
		   SET payload = excluded.payload, requires_manual = true,
		       manual_score = excluded.manual_score, band = excluded.band,
		       graded_by = excluded.graded_by, graded_at = excluded.graded_at`, w.versionID, attempts, w.admin); err != nil {
		t.Fatal(err)
	}

	sections, err := store.ForStudent(ctx, w.student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, c := range sections.Completed {
		switch c.ID {
		case shown.ID:
			found++
			if c.Score == nil || c.Score.Band == nil || *c.Score.Band != 6.5 {
				t.Errorf("shown card score %+v, want band 6.5", c.Score)
			}
		case quiet.ID:
			found++
			if c.Score != nil {
				t.Errorf("score and band shown although review.showScore is off: %+v", *c.Score)
			}
		}
	}
	if found != 2 {
		t.Errorf("%d of the two cards are among the completed ones", found)
	}
}
