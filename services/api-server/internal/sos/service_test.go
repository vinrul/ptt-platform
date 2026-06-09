package sos

import "testing"

func TestValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateInput
		wantErr bool
	}{
		{name: "message only", input: CreateInput{Message: "Emergency"}},
		{name: "valid coordinates", input: CreateInput{Lat: floatPointer(-6.2), Lng: floatPointer(106.8)}},
		{name: "latitude without longitude", input: CreateInput{Lat: floatPointer(-6.2)}, wantErr: true},
		{name: "invalid latitude", input: CreateInput{Lat: floatPointer(91), Lng: floatPointer(106.8)}, wantErr: true},
		{name: "invalid longitude", input: CreateInput{Lat: floatPointer(-6.2), Lng: floatPointer(181)}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCreate(test.input)
			if test.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("expected valid input, got %v", err)
			}
		})
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
