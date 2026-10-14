package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	aiquery "quizzivy/internal/modules/ai/application/query"
	aidomain "quizzivy/internal/modules/ai/domain"
	auditquery "quizzivy/internal/modules/audit/application/query"
	auditdomain "quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/domain"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type centre struct{ recordingHome }

func (centre) Accounts(context.Context, time.Time) (domain.Accounts, error) {
	return domain.Accounts{Total: 10, Enabled: 9, Students: 7, Teachers: 2, TeachersDisabled: 1, MustChangePassword: 2}, nil
}
func (centre) MustChangePassword(context.Context) (int, error)     { return 2, nil }
func (centre) ActiveSince(context.Context, time.Time) (int, error) { return 5, nil }
func (centre) ActiveByDay(context.Context, time.Time, time.Time) ([]domain.ActiveDay, error) {
	return nil, nil
}
func (centre) ClassCounts(context.Context) (domain.ClassCounts, error) {
	return domain.ClassCounts{Active: 4}, nil
}
func (centre) DisabledTeacherClasses(context.Context) (domain.DisabledTeacherClasses, error) {
	return domain.DisabledTeacherClasses{}, nil
}
func (centre) Held(context.Context) (domain.Held, error) { return domain.Held{Attempts: 9}, nil }

func adminApp() *application.Application {
	branding := cqrs.HandlerFunc[organizationquery.Branding, organizationmodel.Branding](func(context.Context, organizationquery.Branding) (organizationmodel.Branding, error) {
		return organizationmodel.Branding{Name: "Trung tâm Ánh Dương"}, nil
	})
	recent := cqrs.HandlerFunc[auditquery.Recent, []auditdomain.Entry](func(context.Context, auditquery.Recent) ([]auditdomain.Entry, error) {
		return nil, nil
	})
	credits := cqrs.HandlerFunc[aiquery.AdminCredits, aidomain.AdminCredits](func(context.Context, aiquery.AdminCredits) (aidomain.AdminCredits, error) {
		return aidomain.AdminCredits{Pool: aidomain.Pool{Plan: 100, Used: 40}}, nil
	})
	return application.New(centre{}).WithAdmin(branding, recent, domain.Plan{}, nil).WithCredits(credits)
}

func holder(keys ...access.Key) access.Principal {
	return access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(keys...)}
}

func asJSON(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func present(m map[string]json.RawMessage, key string) bool {
	v, ok := m[key]
	return ok && string(v) != "null"
}

func TestTheOverviewIsFilledFromTheKeysTheCallerHolds(t *testing.T) {
	cases := []struct {
		name string
		who  access.Principal
		want map[string]bool
	}{
		{"a custom role holding only the audit key", holder(access.SystemAuditRead),
			map[string]bool{"people": false, "classes": false, "dailyActive": false, "recentChanges": true, "storage": false}},
		{"a manager of people", holder(access.PeopleUsersManage),
			map[string]bool{"people": true, "classes": false, "dailyActive": true, "recentChanges": false, "storage": false}},
		{"the owner of the settings", holder(access.SystemSettingsWrite),
			map[string]bool{"people": false, "classes": false, "dailyActive": false, "recentChanges": false, "storage": true}},
		{"an Admin", holder(access.All()...),
			map[string]bool{"people": true, "classes": true, "dailyActive": true, "recentChanges": true, "storage": true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := gatedBy(t, c.who, "GET /admin/overview", access.WorkspaceAdmin)
			res, err := dashboardhttp.NewDashboard(adminApp()).GetAdminOverview(ctx, openapi.GetAdminOverviewRequestObject{})
			if err != nil {
				t.Fatal(err)
			}
			body := asJSON(t, res)
			for key, want := range c.want {
				if got := present(body, key); got != want {
					t.Errorf("%s present = %v, want %v (%s)", key, got, want, body[key])
				}
			}
			if _, ok := body["recentChanges"]; !ok {
				t.Error("a withheld section must be the key with a null, not a missing key")
			}
			if string(body["attention"]) == "null" {
				t.Error("attention must be a list, empty when nothing needs doing")
			}
			if string(body["organizationName"]) != `"Trung tâm Ánh Dương"` {
				t.Errorf("organizationName = %s", body["organizationName"])
			}
		})
	}
}

func TestTheSummaryIsNullWithoutThePeopleKey(t *testing.T) {
	for name, c := range map[string]struct {
		who  access.Principal
		want string
	}{
		"a manager of people": {holder(access.PeopleUsersManage), "2"},
		"an auditor":          {holder(access.SystemAuditRead), "null"},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := gatedBy(t, c.who, "GET /admin/summary", access.WorkspaceAdmin)
			res, err := dashboardhttp.NewDashboard(adminApp()).GetAdminSummary(ctx, openapi.GetAdminSummaryRequestObject{})
			if err != nil {
				t.Fatal(err)
			}
			if got := string(asJSON(t, res)["mustChangePassword"]); got != c.want {
				t.Errorf("mustChangePassword = %s, want %s", got, c.want)
			}
		})
	}
}

func TestUsageWithNothingStatedIsNullsAndZeroesNeverMissingKeys(t *testing.T) {
	ctx := gatedBy(t, holder(access.SystemSettingsWrite), "GET /admin/usage", access.WorkspaceAdmin)
	res, err := dashboardhttp.NewDashboard(adminApp()).GetUsage(ctx, openapi.GetUsageRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	body := asJSON(t, res)
	if string(body["plan"]) != "null" || string(body["aiCredits"]) != "null" {
		t.Errorf("plan = %s, aiCredits = %s, want both null: no plan is stated and the key is not held", body["plan"], body["aiCredits"])
	}
	var seats map[string]json.RawMessage
	if err := json.Unmarshal(body["seats"], &seats); err != nil {
		t.Fatal(err)
	}
	if string(seats["used"]) != "9" || string(seats["limit"]) != "null" {
		t.Errorf("seats = %s, want 9 used and a null limit", body["seats"])
	}
	if string(body["attemptsKept"]) != `{"count":9}` {
		t.Errorf("attemptsKept = %s", body["attemptsKept"])
	}
}

func TestUsageQuotesTheAICreditsToWhoHoldsTheKey(t *testing.T) {
	ctx := gatedBy(t, holder(access.SystemSettingsWrite, access.AICreditsManage), "GET /admin/usage", access.WorkspaceAdmin)
	res, err := dashboardhttp.NewDashboard(adminApp()).GetUsage(ctx, openapi.GetUsageRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(asJSON(t, res)["aiCredits"]); got != `{"pool":100,"used":40}` {
		t.Errorf("aiCredits = %s", got)
	}
}

func TestAnAppWithoutTheAdminPortsAnswersNotImplemented(t *testing.T) {
	ctx := gatedBy(t, holder(access.All()...), "GET /admin/overview", access.WorkspaceAdmin)
	_, err := dashboardhttp.NewDashboard(application.New(centre{})).GetAdminOverview(ctx, openapi.GetAdminOverviewRequestObject{})
	if !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("error = %v, want httpx.ErrNotImplemented", err)
	}
}
