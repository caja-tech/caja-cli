package lsp

import (
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
)

func TestScanRegionMarkers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []regionMarker
	}{
		{
			name:  "basic region",
			input: "#region\nlet x = 1\n#endregion\n",
			want:  []regionMarker{{line: 0}, {line: 2, end: true}},
		},
		{
			name:  "named region",
			input: "#region Setup\nlet x = 1\n#endregion\n",
			want:  []regionMarker{{line: 0}, {line: 2, end: true}},
		},
		{
			name:  "indented region",
			input: "const f = fn() -> Nothing {\n\t#region Body\n\tlet x = 1\n\t#endregion\n}\n",
			want:  []regionMarker{{line: 1}, {line: 3, end: true}},
		},
		{
			name:  "not at line start is ignored",
			input: "let x = 1 # not a region\n",
			want:  nil,
		},
		{
			name:  "hash inside a string is not a marker",
			input: "let x = \"#region fake\"\n#region real\n#endregion\n",
			want:  []regionMarker{{line: 1}, {line: 2, end: true}},
		},
		{
			name:  "hash inside a date literal is not a marker",
			input: "let d = '#region fake'\n#region real\n#endregion\n",
			want:  []regionMarker{{line: 1}, {line: 2, end: true}},
		},
		{
			name:  "multi-line string keeps line numbers correct",
			input: "let s = \"line one\nstill string\"\n#region\n#endregion\n",
			want:  []regionMarker{{line: 2}, {line: 3, end: true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanRegionMarkers(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("scanRegionMarkers(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("marker %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFoldingRangeCapabilityAdvertised(t *testing.T) {
	h := NewCajaHandler()
	s := servertest.New(t, h)

	if s.InitResult == nil || s.InitResult.Capabilities.FoldingRangeProvider == nil || !*s.InitResult.Capabilities.FoldingRangeProvider {
		t.Fatalf("expected FoldingRangeProvider to be advertised, got %+v", s.InitResult.Capabilities)
	}
}

func TestFoldingRange(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []lsp.FoldingRange
	}{
		{
			name:  "single region",
			input: "#region Setup\nlet x = 1\n#endregion\nreturn x",
			want:  []lsp.FoldingRange{{StartLine: 0, EndLine: 2}},
		},
		{
			name:  "nested regions",
			input: "#region Outer\n#region Inner\nlet x = 1\n#endregion\nlet y = 2\n#endregion\n",
			want: []lsp.FoldingRange{
				{StartLine: 1, EndLine: 3},
				{StartLine: 0, EndLine: 5},
			},
		},
		{
			name:  "unmatched endregion is ignored",
			input: "#endregion\nlet x = 1\n",
			want:  nil,
		},
		{
			name:  "unmatched region is left open, no range",
			input: "#region Setup\nlet x = 1\n",
			want:  nil,
		},
		{
			name:  "no regions",
			input: "let x = 1\nreturn x\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewCajaHandler()
			s := servertest.New(t, h)

			uri := lsp.DocumentURI("file:///test_folding.caja")
			s.DidOpen(uri, "caja", tt.input)

			ranges, err := s.FoldingRange(uri)
			if err != nil {
				t.Fatalf("FoldingRange returned error: %v", err)
			}

			if len(ranges) != len(tt.want) {
				t.Fatalf("FoldingRange(%q) = %+v, want %+v", tt.input, ranges, tt.want)
			}
			for i := range ranges {
				if ranges[i].StartLine != tt.want[i].StartLine || ranges[i].EndLine != tt.want[i].EndLine {
					t.Errorf("range %d = {Start:%d End:%d}, want {Start:%d End:%d}",
						i, ranges[i].StartLine, ranges[i].EndLine, tt.want[i].StartLine, tt.want[i].EndLine)
				}
				if ranges[i].Kind == nil || *ranges[i].Kind != lsp.FoldingRangeKindRegion {
					t.Errorf("range %d Kind = %v, want %q", i, ranges[i].Kind, lsp.FoldingRangeKindRegion)
				}
			}
		})
	}
}
