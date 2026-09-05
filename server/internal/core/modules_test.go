package core

import "testing"

// A disabled object store must leave every media-facing port a nil interface,
// not an interface wrapping a nil pointer, or the 501s become nil-pointer 500s.
func TestADisabledObjectStoreLeavesEveryMediaPortNil(t *testing.T) {
	if mediaTransport(nil) != nil || questionsMedia(nil) != nil || testsMedia(nil) != nil || attemptsMedia(nil) != nil {
		t.Fatal("a media port wraps a typed nil; handlers would 500 rather than 501")
	}
}
