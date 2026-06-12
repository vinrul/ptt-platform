package gps

import "testing"

func TestQueryLimit(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int
	}{
		{name: "default", value: "", expected: 200},
		{name: "invalid", value: "invalid", expected: 200},
		{name: "negative", value: "-1", expected: 200},
		{name: "requested", value: "500", expected: 500},
		{name: "maximum", value: "2000", expected: 1000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := queryLimit(test.value); actual != test.expected {
				t.Fatalf("expected %d, got %d", test.expected, actual)
			}
		})
	}
}

func TestQueryHours(t *testing.T) {
	if since, err := queryHours(""); err != nil || since != nil {
		t.Fatalf("expected empty hours to keep default query, got since=%v err=%v", since, err)
	}
	if _, err := queryHours("0"); err == nil {
		t.Fatal("expected zero hours to be invalid")
	}
	if since, err := queryHours("24"); err != nil || since == nil {
		t.Fatalf("expected valid 24 hour window, got since=%v err=%v", since, err)
	}
}
