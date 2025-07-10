package testing

import (
	"testing"
)

// TestThatPasses is a test that should pass to ensure we don't break all tests
func TestThatPasses(t *testing.T) {
	t.Parallel()
	
	expected := "pass"
	actual := "pass"
	
	if actual != expected {
		t.Errorf("Expected %s, but got %s", expected, actual)
	}
}