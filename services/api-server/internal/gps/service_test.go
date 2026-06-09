package gps

import "testing"

func TestValidateLocation(t *testing.T) {
	validSpeed := 1.5
	validHeading := 359.9
	validAccuracy := 4.2

	tests := []struct {
		name    string
		update  Update
		wantErr bool
	}{
		{
			name: "valid location",
			update: Update{
				Lat:      -6.2,
				Lng:      106.8,
				Speed:    &validSpeed,
				Heading:  &validHeading,
				Accuracy: &validAccuracy,
			},
		},
		{name: "latitude too high", update: Update{Lat: 91, Lng: 106.8}, wantErr: true},
		{name: "longitude too low", update: Update{Lat: -6.2, Lng: -181}, wantErr: true},
		{name: "negative speed", update: Update{Lat: -6.2, Lng: 106.8, Speed: floatPointer(-1)}, wantErr: true},
		{name: "invalid heading", update: Update{Lat: -6.2, Lng: 106.8, Heading: floatPointer(360)}, wantErr: true},
		{name: "negative accuracy", update: Update{Lat: -6.2, Lng: 106.8, Accuracy: floatPointer(-1)}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validate(test.update)
			if test.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("expected valid location, got %v", err)
			}
		})
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
