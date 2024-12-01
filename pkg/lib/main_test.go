package lib

import "testing"

func TestValidateService(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantErr bool
	}{
		{
			name:    "valid service",
			arg:     "reportd",
			wantErr: false,
		},
		{
			name:    "valid service 2",
			arg:     "reportd23",
			wantErr: false,
		},
		{
			name:    "invalid service",
			arg:     "reportd-",
			wantErr: true,
		},
		{
			name:    "newline",
			arg:     "reportd\n",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateService(tt.arg); (err != nil) != tt.wantErr {
				t.Errorf("ValidateService(%q) error = %v, wantErr %v", tt.arg, err, tt.wantErr)
			}
		})
	}
}
