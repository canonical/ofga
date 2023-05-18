package hello

import "testing"

func TestHello(t *testing.T) {
	expected := "Hello World"
	msg := Hello()
	if msg != expected {
		t.Fatalf(`Hello() = %v, want %v`, msg, expected)
	}
}
