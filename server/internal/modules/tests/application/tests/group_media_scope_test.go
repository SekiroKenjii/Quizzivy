//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/core/adapters"
	mediaapp "quizzivy/internal/modules/media/application"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type bindingWorld struct {
	tx                     pgx.Tx
	groups                 *repositories.GroupsPostgres
	app                    *application.Application
	a, b, admin            string
	scopeA, scopeB, anyone access.Scope
}

func newBindingWorld(t *testing.T) *bindingWorld {
	t.Helper()
	tx, a, groups := groupTransaction(t)
	tests := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	kinds := adapters.MediaKinds{Media: mediaapp.New(mediarepo.NewPostgres(db.NewContext(tx)), nil, nil)}
	w := &bindingWorld{tx: tx, groups: groups, app: application.New(tests).WithGroups(groups, kinds), a: a}
	w.b, w.admin = scopedUser(t, tx, "teacher"), scopedUser(t, tx, "admin")
	w.scopeA, w.scopeB, w.anyone = access.Scope{UserID: a}, access.Scope{UserID: w.b}, access.Scope{UserID: w.admin, All: true}
	return w
}

func withImage(t *testing.T, bundle domain.GroupBundle, asset string) domain.GroupBundle {
	t.Helper()
	bundle.Group.Stimuli = append(bundle.Group.Stimuli, domain.GroupStimulus{ID: groupIdentity(t), Title: "Sơ đồ", Gaps: []domain.GroupGapBinding{},
		Content: json.RawMessage(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"image","assetId":%q,"alt":"Sơ đồ"}]}`, asset))})
	return bundle
}

func withMemberImage(bundle domain.GroupBundle, asset string) domain.GroupBundle {
	kind := string(mediadomain.KindImage)
	bundle.Questions[0].Input.MediaAssetID = &asset
	bundle.Questions[0].MediaAssetKind = &kind
	return bundle
}

func (w *bindingWorld) create(scope access.Scope, bundle domain.GroupBundle) (domain.StoredGroup, error) {
	return w.groups.Create(context.Background(), domain.CreateGroupInput{Bundle: bundle, ActorID: scope.UserID, Now: time.Now(), Scope: scope, Grants: bothKeys})
}

func (w *bindingWorld) update(scope access.Scope, stored domain.StoredGroup, bundle domain.GroupBundle) (domain.StoredGroup, error) {
	return w.groups.Update(context.Background(), domain.UpdateGroupInput{Bundle: bundle, GroupMutation: domain.GroupMutation{
		ID: stored.Bundle.Group.ID, ExpectedRevision: stored.Revision, ActorID: scope.UserID, Now: time.Now(), Scope: scope, Grants: bothKeys}})
}

func (w *bindingWorld) groupsOf(t *testing.T, owner string) int {
	t.Helper()
	var n int
	if err := w.tx.QueryRow(context.Background(), `SELECT count(*) FROM app.question_groups WHERE owner_id = $1`, owner).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAGroupBindsOnlyAssetsItsWriterCanRead(t *testing.T) {
	w := newBindingWorld(t)
	for name, c := range map[string]struct {
		bundle func(asset string) domain.GroupBundle
		kind   string
	}{
		"a stimulus image":           {func(asset string) domain.GroupBundle { return withImage(t, storedGroupFixture(t, ""), asset) }, "image"},
		"a stimulus and a recording": {func(asset string) domain.GroupBundle { return storedGroupFixture(t, asset) }, "audio"},
		"a member question's image":  {func(asset string) domain.GroupBundle { return withMemberImage(storedGroupFixture(t, ""), asset) }, "image"},
	} {
		t.Run(name, func(t *testing.T) {
			before := w.groupsOf(t, w.b)
			for label, asset := range map[string]string{"A's": storedGroupAsset(t, w.tx, w.a, c.kind), "a missing": uuid.NewString()} {
				if _, err := w.create(w.scopeB, c.bundle(asset)); !errors.Is(err, mediadomain.ErrNotFound) {
					t.Errorf("B binding %s asset: %v, want the not-found a missing asset gets", label, err)
				}
			}
			if after := w.groupsOf(t, w.b); after != before {
				t.Errorf("B's refused groups left %d rows behind", after-before)
			}
			if _, err := w.create(w.scopeB, c.bundle(storedGroupAsset(t, w.tx, w.b, c.kind))); err != nil {
				t.Errorf("B binding B's own asset: %v", err)
			}
			if _, err := w.create(w.anyone, c.bundle(storedGroupAsset(t, w.tx, w.a, c.kind))); err != nil {
				t.Errorf("scope.all binding A's asset: %v", err)
			}
		})
	}
	mixed := withImage(t, withImage(t, storedGroupFixture(t, ""), storedGroupAsset(t, w.tx, w.b, "image")), storedGroupAsset(t, w.tx, w.a, "image"))
	if _, err := w.create(w.scopeB, mixed); !errors.Is(err, mediadomain.ErrNotFound) {
		t.Errorf("B binding B's image beside A's: %v, want the not-found a missing asset gets", err)
	}
}

func TestAGroupUpdateCannotAddAnUnreadableAssetButKeepsItsOwn(t *testing.T) {
	w := newBindingWorld(t)
	stored, err := w.create(w.scopeB, storedGroupFixture(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.update(w.scopeB, stored, withImage(t, stored.Bundle, storedGroupAsset(t, w.tx, w.a, "image"))); !errors.Is(err, mediadomain.ErrNotFound) {
		t.Errorf("B adding A's image to B's group: %v, want the not-found a missing asset gets", err)
	}

	bound, err := w.update(w.anyone, stored, withImage(t, stored.Bundle, storedGroupAsset(t, w.tx, w.a, "image")))
	if err != nil {
		t.Fatalf("scope.all binding A's image into B's group: %v", err)
	}
	if _, err := w.update(w.scopeB, bound, bound.Bundle); err != nil {
		t.Errorf("B re-saving the group as it is, with the asset scope.all bound: %v", err)
	}
}

func TestPrepareRefusesAnUnreadableMemberAssetBeforeValidatingIt(t *testing.T) {
	w := newBindingWorld(t)
	who := actor.Actor{ID: w.b, Scope: w.scopeB}
	for label, asset := range map[string]string{"A's": storedGroupAsset(t, w.tx, w.a, "image"), "a missing": uuid.NewString()} {
		bundle := storedGroupFixture(t, "")
		bundle.Questions[0].Input.MediaAssetID = &asset
		bundle.Questions[0].Input.Audio = &questions.AudioPolicy{AllowSeek: true}
		if _, err := w.app.Commands.CreateGroup.Handle(context.Background(), command.CreateGroup{Bundle: bundle, Actor: who, Grants: bothKeys}); !errors.Is(err, questions.ErrMediaNotFound) {
			t.Errorf("B's member binding %s image with an audio policy: %v, want the media not-found and no validation of its kind", label, err)
		}
	}

	stored, err := w.app.Commands.CreateGroup.Handle(context.Background(), command.CreateGroup{Bundle: storedGroupFixture(t, ""), Actor: who, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	foreign := storedGroupAsset(t, w.tx, w.a, "image")
	bundle := stored.Bundle
	bundle.Questions[0].Input.MediaAssetID = &foreign
	mutation := model.GroupMutation{ID: stored.Bundle.Group.ID, ExpectedRevision: stored.Revision, Actor: who, Grants: bothKeys}
	if _, err := w.app.Commands.UpdateGroup.Handle(context.Background(), command.UpdateGroup{Mutation: mutation, Bundle: bundle}); !errors.Is(err, questions.ErrMediaNotFound) {
		t.Errorf("B's update binding A's image to a member: %v, want the media not-found", err)
	}
}
