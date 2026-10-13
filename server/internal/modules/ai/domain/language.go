package domain

// Language is the language the text an operation writes for a person is written in.
type Language string

// The two languages of the interface.
const (
	Vietnamese Language = "vi"
	English    Language = "en"
)

// Valid reports whether l is one of the two languages.
func (l Language) Valid() bool { return l == Vietnamese || l == English }

// Name is l in English, as a prompt names it. An empty or unknown Language names Vietnamese, the product's first.
func (l Language) Name() string {
	if l == English {
		return "English"
	}
	return "Vietnamese"
}

// Usage is what one operation cost and what remains: Credits is what the call was charged, never more than its ceiling, and Left the least of what remains of the teacher's allowance and of the pool after it.
type Usage struct {
	Credits int64
	Left    int64
}
