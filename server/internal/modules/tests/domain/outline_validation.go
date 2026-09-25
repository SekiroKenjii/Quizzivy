package domain

import (
	"slices"
	"strings"
)

func validateMixedOutline(in UpdateInput) []FieldError {
	var errs []FieldError
	add := func(field, message string) { errs = append(errs, FieldError{Field: field, Message: message}) }
	if in.GroupOutline && !in.SetSections {
		add("sections", "Cần gửi toàn bộ cấu trúc đề khi lưu thứ tự nhóm.")
	}
	groups := map[string]bool{}
	sections := map[string]bool{}
	for i, section := range in.Sections {
		sectionID := strings.ToLower(section.ID)
		if sectionID != "" && sections[sectionID] {
			add(sectionField(i, "id"), "Một phần chỉ được xuất hiện một lần trong đề.")
		}
		sections[sectionID] = true
		if !in.GroupOutline {
			if section.SetUnits {
				add(sectionField(i, "units"), "Cần định dạng cấu trúc nhóm để lưu ngữ liệu chung.")
			}
			continue
		}
		if !section.SetUnits {
			add(sectionField(i, "units"), "Cần gửi đủ thứ tự câu hỏi và nhóm của phần.")
			continue
		}
		errs = append(errs, validateSectionUnits(i, section, groups)...)
	}
	return errs
}

func validateSectionUnits(index int, section SectionInput, groups map[string]bool) []FieldError {
	var errs []FieldError
	add := func(message string) {
		errs = append(errs, FieldError{Field: sectionField(index, "units"), Message: message})
	}
	questions := []string{}
	if len(section.Units) > 32768 {
		add("Phần vượt giới hạn số mục trong cấu trúc.")
	}
	for _, unit := range section.Units {
		id := strings.ToLower(unit.ID)
		switch unit.Kind {
		case "question":
			questions = append(questions, id)
		case "group":
			if groups[id] {
				add("Một nhóm ngữ liệu chỉ được xuất hiện một lần trong đề.")
			}
			groups[id] = true
		default:
			add("Loại mục trong cấu trúc không hợp lệ.")
		}
	}
	projection := make([]string, len(section.QuestionIDs))
	for i, id := range section.QuestionIDs {
		projection[i] = strings.ToLower(id)
	}
	if !slices.Equal(questions, projection) {
		add("Thứ tự câu hỏi không khớp cấu trúc đã gửi.")
	}
	return errs
}
