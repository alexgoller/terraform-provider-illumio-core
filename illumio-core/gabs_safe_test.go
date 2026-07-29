// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"testing"

	"github.com/Jeffail/gabs/v2"
)

// The upstream form panicked with "interface conversion: interface {} is nil,
// not string" whenever the PCE omitted a field, crashing the provider
// mid-apply.
func TestGabsStringDoesNotPanicOnMissingField(t *testing.T) {
	c, err := gabs.ParseJSON([]byte(`{"href":"/orgs/1/labels/1","nested":{"href":"/orgs/1/x"}}`))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path []string
		want string
	}{
		{"present", []string{"href"}, "/orgs/1/labels/1"},
		{"nested present", []string{"nested", "href"}, "/orgs/1/x"},
		{"absent", []string{"name"}, ""},
		{"absent nested", []string{"nested", "missing"}, ""},
		{"absent parent", []string{"nope", "href"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gabsString(c, tt.path...); got != tt.want {
				t.Errorf("gabsString(%v) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGabsStringHandlesNilAndNonStrings(t *testing.T) {
	if got := gabsString(nil, "href"); got != "" {
		t.Errorf("nil container = %q, want empty", got)
	}

	c, _ := gabs.ParseJSON([]byte(`{"count":7,"flag":true,"null":null}`))
	for _, key := range []string{"count", "flag", "null"} {
		if got := gabsString(c, key); got != "" {
			t.Errorf("gabsString(%q) = %q, want empty for a non-string", key, got)
		}
	}
}

// A bare container, with no path.
func TestGabsStringNoPath(t *testing.T) {
	c, _ := gabs.ParseJSON([]byte(`"just-a-string"`))
	if got := gabsString(c); got != "just-a-string" {
		t.Errorf("got %q, want just-a-string", got)
	}
}
