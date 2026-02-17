package i2ptunnel

import (
	"testing"
)

func TestMergeI2CPOptions(t *testing.T) {
	tests := []struct {
		name     string
		i2cp     map[string]interface{}
		existing map[string]string
		want     map[string]string
	}{
		{
			name: "encrypted LeaseSet options",
			i2cp: map[string]interface{}{
				"leaseSetEncType":  "4,0",
				"leaseSetType":     "3",
				"leaseSetAuthType": "0",
			},
			existing: map[string]string{"name": "test"},
			want: map[string]string{
				"name":                  "test",
				"i2cp.leaseSetEncType":  "4,0",
				"i2cp.leaseSetType":     "3",
				"i2cp.leaseSetAuthType": "0",
			},
		},
		{
			name:     "nil i2cp map",
			i2cp:     nil,
			existing: map[string]string{"name": "test"},
			want:     map[string]string{"name": "test"},
		},
		{
			name:     "empty i2cp map",
			i2cp:     map[string]interface{}{},
			existing: map[string]string{"name": "test"},
			want:     map[string]string{"name": "test"},
		},
		{
			name:     "integer value converted to string",
			i2cp:     map[string]interface{}{"leaseSetType": 3},
			existing: map[string]string{},
			want:     map[string]string{"i2cp.leaseSetType": "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeI2CPOptions(tt.i2cp, tt.existing)
			if len(tt.existing) != len(tt.want) {
				t.Errorf("got %d options, want %d", len(tt.existing), len(tt.want))
			}
			for k, v := range tt.want {
				if got, ok := tt.existing[k]; !ok {
					t.Errorf("missing key %q", k)
				} else if got != v {
					t.Errorf("key %q = %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestExtractI2CPOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    map[string]string
		wantNil bool
		want    map[string]interface{}
	}{
		{
			name: "extracts i2cp keys",
			opts: map[string]string{
				"name":                  "test",
				"port":                  "8080",
				"i2cp.leaseSetEncType":  "4,0",
				"i2cp.leaseSetType":     "3",
				"i2cp.leaseSetAuthType": "2",
			},
			want: map[string]interface{}{
				"leaseSetEncType":  "4,0",
				"leaseSetType":     "3",
				"leaseSetAuthType": "2",
			},
		},
		{
			name:    "no i2cp keys returns nil",
			opts:    map[string]string{"name": "test", "port": "8080"},
			wantNil: true,
		},
		{
			name:    "empty map returns nil",
			opts:    map[string]string{},
			wantNil: true,
		},
		{
			name: "empty values are skipped",
			opts: map[string]string{
				"i2cp.leaseSetEncType": "4,0",
				"i2cp.leaseSetType":    "",
			},
			want: map[string]interface{}{
				"leaseSetEncType": "4,0",
			},
		},
		{
			name: "all empty values returns nil",
			opts: map[string]string{
				"i2cp.leaseSetType":    "",
				"i2cp.leaseSetEncType": "",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractI2CPOptions(tt.opts)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if len(got) != len(tt.want) {
				t.Errorf("got %d entries, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if gv, ok := got[k]; !ok {
					t.Errorf("missing key %q", k)
				} else if gv != v {
					t.Errorf("key %q = %v, want %v", k, gv, v)
				}
			}
		})
	}
}

func TestRoundTripI2CPOptions(t *testing.T) {
	original := map[string]interface{}{
		"leaseSetEncType":  "4,0",
		"leaseSetType":     "3",
		"leaseSetAuthType": "1",
		"leaseSetPrivKey":  "base64encodedkey==",
	}

	opts := make(map[string]string)
	opts["name"] = "test-tunnel"
	opts["port"] = "8080"
	MergeI2CPOptions(original, opts)

	extracted := ExtractI2CPOptions(opts)
	if extracted == nil {
		t.Fatal("expected non-nil extracted options")
	}

	for k, v := range original {
		ev, ok := extracted[k]
		if !ok {
			t.Errorf("round trip lost key %q", k)
			continue
		}
		wantStr := v.(string)
		if ev != wantStr {
			t.Errorf("key %q: got %v, want %v", k, ev, wantStr)
		}
	}
}
