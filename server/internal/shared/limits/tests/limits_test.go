package limits_test

import (
	"slices"
	"testing"

	"quizzivy/internal/shared/limits"
)

func TestAStoredValueWinsOverTheDeployAndTheDeployOverTheHostingPlan(t *testing.T) {
	got := limits.Resolve(
		limits.Stored{limits.EmailsPerDay: 40},
		limits.Env{limits.EmailsPerDay: 60, limits.MediaPerTeacherBytes: 2 * limits.GiB},
	)
	cases := []struct {
		name   limits.Name
		value  int64
		custom bool
		source limits.Source
	}{
		{limits.EmailsPerDay, 40, true, limits.SourceDeploy},
		{limits.MediaPerTeacherBytes, 2 * limits.GiB, false, limits.SourceDeploy},
		{limits.EmailsPerMonth, limits.ResendFreeEmailsPerMonth, false, limits.SourceResendFree},
		{limits.StorageBytes, limits.R2FreeStorageBytes, false, limits.SourceR2Free},
		{limits.DatabaseBytes, limits.NeonLaunchDatabaseBytes, false, limits.SourceNeonLaunch},
		{limits.ImportBytes, limits.AppImportBytes, false, limits.SourceApp},
		{limits.WarnAtPercent, limits.AppWarnAtPercent, false, limits.SourceApp},
	}
	for _, c := range cases {
		l := got[c.name]
		if l.Value == nil || *l.Value != c.value || l.Custom != c.custom || l.Default.Source != c.source {
			t.Errorf("%s = %+v, want %d custom=%v from %s", c.name, l, c.value, c.custom, c.source)
		}
	}
	if v := *got[limits.EmailsPerDay].Default.Value; v != 60 {
		t.Errorf("emails per day default = %d, want the deploy's 60 shown under the stored 40", v)
	}
}

func TestSeatsAndTeachersHaveNoLimitUntilOneIsStored(t *testing.T) {
	got := limits.Resolve(nil, nil)
	for _, n := range []limits.Name{limits.Seats, limits.Teachers} {
		if _, ok := got.Of(n); ok || got[n].Default.Source != limits.SourceNone || got[n].Default.Value != nil {
			t.Errorf("%s = %+v, want no limit and no default", n, got[n])
		}
	}
	set := limits.Resolve(limits.Stored{limits.Seats: 120}, nil)
	if v, ok := set.Of(limits.Seats); !ok || v != 120 {
		t.Errorf("seats = %d %v, want 120", v, ok)
	}
	if got.WarnPercent() != limits.AppWarnAtPercent {
		t.Errorf("warn at = %d, want %d", got.WarnPercent(), limits.AppWarnAtPercent)
	}
}

func TestChangesNamesEachLimitWithItsBeforeAndAfter(t *testing.T) {
	before := limits.Stored{limits.Seats: 100, limits.EmailsPerDay: 80, limits.WarnAtPercent: 90}
	after := limits.Stored{limits.Seats: 150, limits.WarnAtPercent: 90, limits.StorageBytes: 20 * limits.GiB}
	got := before.Changes(after)
	names := make([]limits.Name, len(got))
	for i, c := range got {
		names[i] = c.Name
	}
	if want := []limits.Name{limits.Seats, limits.StorageBytes, limits.EmailsPerDay}; !slices.Equal(names, want) {
		t.Fatalf("changes = %v, want %v in the page's order", names, want)
	}
	if *got[0].Before != 100 || *got[0].After != 150 {
		t.Errorf("seats = %+v", got[0])
	}
	if got[1].Before != nil || *got[1].After != 20*limits.GiB {
		t.Errorf("storage = %+v, want from the default", got[1])
	}
	if *got[2].Before != 80 || got[2].After != nil {
		t.Errorf("emails per day = %+v, want back to the default", got[2])
	}
	if len(before.Changes(before)) != 0 || !before.Equal(limits.Stored{limits.EmailsPerDay: 80, limits.WarnAtPercent: 90, limits.Seats: 100}) {
		t.Error("an equal set must change nothing")
	}
}

func TestRefusedNamesAValueOutsideItsRangeAndAnUnknownName(t *testing.T) {
	s := limits.Stored{
		limits.Seats: 0, limits.Teachers: 1, limits.StorageBytes: limits.MiB - 1, limits.ImportBytes: limits.MiB,
		limits.WarnAtPercent: 101, limits.EmailsPerDay: 100_000, "mystery": 5,
	}
	got := s.Refused()
	want := []limits.Name{limits.Seats, limits.StorageBytes, limits.WarnAtPercent, "mystery"}
	if !slices.Equal(got, want) {
		t.Errorf("refused = %v, want %v", got, want)
	}
}

func TestEveryLimitHasAColumnAndARange(t *testing.T) {
	for _, n := range limits.Names() {
		lo, hi := limits.Range(n)
		if limits.Column(n) == "" || lo < 1 || hi <= lo {
			t.Errorf("%s: column %q range %d..%d", n, limits.Column(n), lo, hi)
		}
	}
	targets, gather := limits.ScanTargets()
	if len(targets) != len(limits.Names()) || len(limits.Stored{}.Arguments()) != len(targets) {
		t.Fatalf("targets %d, arguments %d, names %d", len(targets), len(limits.Stored{}.Arguments()), len(limits.Names()))
	}
	v := int64(42)
	*(targets[0].(**int64)) = &v
	if got := gather(); len(got) != 1 || got[limits.Names()[0]] != 42 {
		t.Errorf("gathered = %v", got)
	}
}
