package openapi

import (
	"reflect"
	"strings"
	"testing"
)

// The Go-side half of the student-payload rule (spec §13.5).
//
// api/contract_check.py asserts this over the OpenAPI document; this asserts it
// over the types actually generated from it, so a generator change cannot
// quietly reintroduce a field the contract forbids. T-3.5 adds the third layer:
// walking a live JSON response.
//
// If this fails, do not edit the generated file. Fix api/openapi.yaml and rerun
// `make gen`.

var forbidden = []string{"isCorrect", "sampleAnswer", "acceptedAnswers", "transcript"}

// jsonFieldNames walks a struct type and returns every JSON field name reachable
// from it, following pointers, slices, maps and nested structs.
func jsonFieldNames(t reflect.Type, seen map[reflect.Type]bool, out map[string]string, path string) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() == reflect.Map {
		jsonFieldNames(t.Elem(), seen, out, path+"[]")
		return
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			name = f.Name
		}
		out[name] = path + "." + name
		jsonFieldNames(f.Type, seen, out, path+"."+name)
	}
}

func TestStudentQuestionCarriesNoGradingKey(t *testing.T) {
	got := map[string]string{}
	jsonFieldNames(reflect.TypeOf(StudentQuestion{}), map[reflect.Type]bool{}, got, "StudentQuestion")
	for _, bad := range forbidden {
		if where, ok := got[bad]; ok {
			t.Errorf("StudentQuestion exposes %q at %s — forbidden by spec §13.5", bad, where)
		}
	}
}

func TestAttemptSessionCarriesNoGradingKey(t *testing.T) {
	got := map[string]string{}
	jsonFieldNames(reflect.TypeOf(AttemptSession{}), map[reflect.Type]bool{}, got, "AttemptSession")
	for _, bad := range forbidden {
		if where, ok := got[bad]; ok {
			t.Errorf("AttemptSession exposes %q at %s — this is the take-test payload (§13.5)", bad, where)
		}
	}
}

// The inverse. If AdminQuestion ever stopped carrying the grading key, grading
// would break silently rather than failing to compile.
func TestAdminQuestionStillCarriesGradingKey(t *testing.T) {
	got := map[string]string{}
	jsonFieldNames(reflect.TypeOf(AdminQuestion{}), map[reflect.Type]bool{}, got, "AdminQuestion")
	for _, needed := range []string{"isCorrect", "sampleAnswer", "acceptedAnswers", "transcript"} {
		if _, ok := got[needed]; !ok {
			t.Errorf("AdminQuestion is missing %q — grading needs it", needed)
		}
	}
}

// ResultQuestion may carry transcript (gated by showTranscriptAfterSubmit,
// §11.3) but never the other three.
func TestResultQuestionLeaksOnlyTranscript(t *testing.T) {
	got := map[string]string{}
	jsonFieldNames(reflect.TypeOf(ResultQuestion{}), map[reflect.Type]bool{}, got, "ResultQuestion")
	for _, bad := range []string{"isCorrect", "sampleAnswer", "acceptedAnswers"} {
		if where, ok := got[bad]; ok {
			t.Errorf("ResultQuestion exposes %q at %s — only transcript is permitted here", bad, where)
		}
	}
}
