package domain

// Level identifies a question's optional proficiency level.
type Level string

// Skill identifies a question's optional language skill.
type Skill string

func validLevel(level Level) bool {
	switch level {
	case "pre_a1", "a1", "a2", "b1", "b2", "c1", "c2":
		return true
	}
	return false
}

func validSkill(skill Skill) bool {
	switch skill {
	case "grammar", "vocabulary", "reading", "listening", "writing", "speaking":
		return true
	}
	return false
}

// ValidateAuthoring bounds newly submitted choice options without capping stored content.
func (in Input) ValidateAuthoring() error {
	if in.Type.IsChoice() && len(in.Options) > 8 {
		return &ValidationError{Fields: []FieldError{{Field: "options", Message: "Chỉ được có tối đa tám phương án."}}}
	}
	return nil
}
