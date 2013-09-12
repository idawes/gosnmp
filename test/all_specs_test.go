package test

import (
	. "launchpad.net/gocheck"
	"testing"
)

func TestAllSpecs(t *testing.T) {
	s, err := NewRequestResponseTestSuite()
	if err != nil {
		t.Fatal(err)
	}
	Suite(s)

	TestingT(t)
}
