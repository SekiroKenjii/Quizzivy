package domain_test

import (
	"quizzivy/internal/modules/tests/domain"
	"testing"
)

func TestMixedOutlineRejectsLossyOrAmbiguousStructure(t *testing.T) {
	group := domain.SectionUnit{Kind: "group", ID: "01935000-0000-7000-8000-0000000000aa"}
	question := domain.SectionUnit{Kind: "question", ID: "01935000-0000-7000-8000-0000000000bb"}
	valid := func() domain.UpdateInput {
		return domain.UpdateInput{GroupOutline: true, SetSections: true, Sections: []domain.SectionInput{{ID: "section", Title: "Part", QuestionIDs: []string{question.ID}, SetUnits: true, Units: []domain.SectionUnit{group, question}}}}
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*domain.UpdateInput){
		"missing format":   func(in *domain.UpdateInput) { in.GroupOutline = false },
		"missing sections": func(in *domain.UpdateInput) { in.SetSections = false },
		"missing units":    func(in *domain.UpdateInput) { in.Sections[0].SetUnits = false },
		"lost projection":  func(in *domain.UpdateInput) { in.Sections[0].QuestionIDs = nil },
		"duplicate group": func(in *domain.UpdateInput) {
			in.Sections = append(in.Sections, domain.SectionInput{Title: "Other", SetUnits: true, Units: []domain.SectionUnit{group}})
		},
		"duplicate section": func(in *domain.UpdateInput) {
			in.Sections = append(in.Sections, domain.SectionInput{ID: "SECTION", Title: "Other", SetUnits: true})
		},
		"unknown unit": func(in *domain.UpdateInput) { in.Sections[0].Units[0].Kind = "material" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := valid()
			change(&in)
			if err := in.Validate(); err == nil {
				t.Fatal("invalid mixed outline accepted")
			}
		})
	}
	empty := domain.UpdateInput{GroupOutline: true, SetSections: true, Sections: []domain.SectionInput{{Title: "Empty", SetUnits: true}}}
	if err := empty.Validate(); err != nil {
		t.Fatal(err)
	}
}
