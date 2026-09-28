package repositories_test

import (
	"testing"

	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
)

func TestAClassConstraintTheMapDoesNotNameIsOther(t *testing.T) {
	for constraint, want := range map[string]domain.Reference{
		"assignment_classes_class_id_fkey": domain.ReferencedByAssignments,
		"class_members_join_code_id_fkey":  domain.ReferencedByMembers,
		"class_sessions_class_id_fkey":     domain.ReferencedByOther,
		"":                                 domain.ReferencedByOther,
	} {
		if got := repositories.ClassReferencedBy(constraint); got != want {
			t.Errorf("%q answers %q, want %q", constraint, got, want)
		}
	}
}
