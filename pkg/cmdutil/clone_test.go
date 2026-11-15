package cmdutil

import "testing"

func TestParseCloneArgs(t *testing.T) {
	tests := []struct {
		name     string
		rest     []string
		wantID   int
		wantName string
		wantErr  bool
	}{
		{"nothing: next free ID, no name", nil, 0, "", false},
		{"id only", []string{"200"}, 200, "", false},
		{"name only", []string{"web2"}, 0, "web2", false},
		{"id and name", []string{"200", "web2"}, 200, "web2", false},
		{"a number then a number is id then name", []string{"200", "300"}, 200, "300", false},
		{"too many", []string{"200", "web2", "extra"}, 0, "", true},
		{"zero id", []string{"0"}, 0, "", true},
		{"negative id", []string{"-5"}, 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, name, err := ParseCloneArgs(tt.rest)
			if id != tt.wantID || name != tt.wantName || (err != nil) != tt.wantErr {
				t.Fatalf("ParseCloneArgs(%q) = %d, %q, %v; want %d, %q, err=%v", tt.rest, id, name, err, tt.wantID, tt.wantName, tt.wantErr)
			}
		})
	}
}
