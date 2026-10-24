package config

import (
	"reflect"
	"testing"
)

func TestParsePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		want    []segment
		wantErr bool
	}{
		{name: "single key", path: "tiers", want: []segment{{key: "tiers", isKey: true}}},
		{
			name: "nested keys with index",
			path: "representation.styles.tiers[0]",
			want: []segment{
				{key: "representation", isKey: true},
				{key: "styles", isKey: true},
				{key: "tiers", isKey: true},
				{index: 0},
			},
		},
		{
			name: "index then key",
			path: "tiers[1].display-name",
			want: []segment{{key: "tiers", isKey: true}, {index: 1}, {key: "display-name", isKey: true}},
		},
		{name: "repeated index", path: "a[1][2]", want: []segment{{key: "a", isKey: true}, {index: 1}, {index: 2}}},
		{name: "empty", path: "", wantErr: true},
		{name: "empty key", path: "a..b", wantErr: true},
		{name: "leading index", path: "[0]", wantErr: true},
		{name: "negative index", path: "a[-1]", wantErr: true},
		{name: "unclosed index", path: "a[1", wantErr: true},
		{name: "junk after index", path: "a[1]x", wantErr: true},
		{name: "bad key character", path: "a b", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePath(%q) = %+v, want %+v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLoadBuiltIn(t *testing.T) {
	t.Parallel()

	cfg, diags := Load(nil)
	if cfg == nil || len(diags) != 0 {
		t.Fatalf("built-in config must load cleanly, got diagnostics %+v", diags)
	}

	if got := cfg.Tiers[0].ID; got != "Off" {
		t.Errorf("tier 0 = %q, want Off (the default base)", got)
	}

	if _, ok := cfg.Tier("mEdIuM"); !ok {
		t.Error("tier lookup must be case-insensitive")
	}

	if got, want := cfg.Style(len(cfg.Representation.Styles.Tiers)), cfg.Style(0); got != want {
		t.Errorf("styles must repeat round robin: got %+v, want %+v", got, want)
	}
}
