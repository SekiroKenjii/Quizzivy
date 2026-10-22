package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// validateTypeFields refuses a field that belongs to another type and checks
// each type's own: selectCount on a multiple choice, maxWords on a short
// answer, matchAllowReuse and matchItems on matching, rubric on an essay.
func validateTypeFields(in Input, errs *ValidationError) {
	validateSelectCount(in, errs)
	if in.MaxWords != nil {
		switch {
		case in.Type != ShortAnswer:
			errs.AddIn("maxWords", "Chỉ câu trả lời ngắn mới có giới hạn số từ.", "Only a short answer has a word limit.")
		case *in.MaxWords < 1 || *in.MaxWords > MaxWordsLimit:
			errs.AddIn("maxWords", "Giới hạn số từ phải từ 1 đến 100.", "The word limit must be from 1 to 100.")
		}
	}
	switch {
	case in.Type == Matching && in.MatchAllowReuse == nil:
		errs.AddIn("matchAllowReuse", "Hãy chọn một lựa chọn có được dùng cho nhiều mục hay không.",
			"Say whether one choice may answer more than one item.")
	case in.Type != Matching && in.MatchAllowReuse != nil:
		errs.AddIn("matchAllowReuse", "Chỉ câu nối mới có thiết lập này.", "Only a matching question has this setting.")
	}
	if in.Type == Matching {
		validateMatchItems(in, errs)
	} else if len(in.MatchItems) > 0 {
		errs.AddIn("matchItems", "Chỉ câu nối mới có các mục cần nối.", "Only a matching question has items to match.")
	}
	if in.Type == Essay {
		validateRubric(in, errs)
	} else if len(in.Rubric) > 0 {
		errs.AddIn("rubric", "Chỉ bài luận mới có tiêu chí chấm.", "Only an essay has rubric criteria.")
	}
}

func validateSelectCount(in Input, errs *ValidationError) {
	if in.SelectCount == nil {
		return
	}
	if in.Type != MultipleChoice {
		errs.AddIn("selectCount", "Chỉ câu nhiều đáp án mới có số đáp án cần chọn.",
			"Only a multiple choice question has a number of answers to choose.")
		return
	}
	n := *in.SelectCount
	if n < 2 || n > MaxSelectCount || n > len(in.Options)-1 {
		errs.AddIn("selectCount", "Số đáp án cần chọn phải từ 2 đến số phương án trừ 1.",
			"The number of answers to choose must be from 2 to one less than the number of options.")
		return
	}
	if correctOptions(in) != n {
		errs.AddIn("options", fmt.Sprintf("Cần đúng %d phương án đúng, bằng số đáp án cần chọn.", n),
			fmt.Sprintf("Mark exactly %d options correct, the number of answers to choose.", n))
	}
}

func correctOptions(in Input) int {
	correct := 0
	for _, o := range in.Options {
		if o.IsCorrect {
			correct++
		}
	}
	return correct
}

// validateTFNGOptions holds a tfng question to its three options with one
// correct. Their texts are not checked: the store writes TFNGTexts.
func validateTFNGOptions(in Input, errs *ValidationError) {
	if len(in.Options) != len(TFNGTexts) {
		errs.AddIn("options", "Câu Đúng/Sai/Không có thông tin phải có đúng ba phương án.",
			"A True / False / Not given question has exactly three options.")
	}
	if correctOptions(in) != 1 {
		errs.AddIn("options", "Câu Đúng/Sai/Không có thông tin cần đúng một phương án đúng.",
			"A True / False / Not given question has exactly one correct option.")
	}
}

func validateMatchChoices(in Input, errs *ValidationError) {
	if len(in.Options) < 2 || len(in.Options) > MaxMatchChoices {
		errs.AddIn("options", "Câu nối cần từ 2 đến 12 lựa chọn.", "A matching question has 2 to 12 choices.")
	}
	for i, o := range in.Options {
		validateOptionText(i, o, errs)
	}
	if correctOptions(in) != 0 {
		errs.AddIn("options", "Lựa chọn của câu nối không được đánh dấu đúng.",
			"A matching question's choices are not marked correct.")
	}
}

func validateMatchItems(in Input, errs *ValidationError) {
	if len(in.MatchItems) < MinMatchItems || len(in.MatchItems) > MaxMatchItems {
		errs.AddIn("matchItems", "Câu nối cần từ 2 đến 20 mục.", "A matching question has 2 to 20 items.")
	}
	reuse := in.MatchAllowReuse != nil && *in.MatchAllowReuse
	if !reuse && len(in.MatchItems) > len(in.Options) {
		errs.AddIn("matchItems", "Khi mỗi lựa chọn chỉ dùng một lần, số mục không được nhiều hơn số lựa chọn.",
			"When each choice answers one item, there cannot be more items than choices.")
	}
	taken := make(map[int]bool, len(in.MatchItems))
	for i, item := range in.MatchItems {
		switch length := utf8.RuneCountInString(item.Text); {
		case strings.TrimSpace(item.Text) == "":
			errs.AddIn(fmt.Sprintf("matchItems[%d].text", i), "Nội dung mục không được để trống.", "An item's text cannot be empty.")
		case length > MaxMatchItemLength:
			errs.AddIn(fmt.Sprintf("matchItems[%d].text", i), "Nội dung mục không được dài quá 500 ký tự.",
				"An item's text cannot be longer than 500 characters.")
		}
		field := fmt.Sprintf("matchItems[%d].correctOption", i)
		switch {
		case item.CorrectOption < 0 || item.CorrectOption >= len(in.Options):
			errs.AddIn(field, "Mục phải nối với một lựa chọn của câu này.", "An item must match one of this question's choices.")
		case !reuse && taken[item.CorrectOption]:
			errs.AddIn(field, "Lựa chọn này đã nối với một mục khác.", "This choice already answers another item.")
		}
		taken[item.CorrectOption] = true
	}
}

// validateRubric holds an essay to at most six criteria whose labels are
// distinct, ignoring case and the space around them.
func validateRubric(in Input, errs *ValidationError) {
	if len(in.Rubric) > MaxRubricCriteria {
		errs.AddIn("rubric", "Bài luận có tối đa sáu tiêu chí.", "An essay has at most six rubric criteria.")
	}
	seen := make(map[string]bool, len(in.Rubric))
	for i, criterion := range in.Rubric {
		field := fmt.Sprintf("rubric[%d].label", i)
		label := strings.ToLower(strings.TrimSpace(criterion.Label))
		switch {
		case label == "":
			errs.AddIn(field, "Tên tiêu chí không được để trống.", "A criterion's label cannot be empty.")
		case utf8.RuneCountInString(criterion.Label) > MaxRubricLabel:
			errs.AddIn(field, "Tên tiêu chí không được dài quá 80 ký tự.", "A criterion's label cannot be longer than 80 characters.")
		case seen[label]:
			errs.AddIn(field, "Hai tiêu chí không được trùng tên.", "Two criteria cannot have the same label.")
		}
		seen[label] = true
	}
}
