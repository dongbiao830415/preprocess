// Copyright (c) 2026 The preprocess Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package preprocess

import "testing"

func TestBuildSet(t *testing.T) {
	tests := []struct {
		name string
		ops  []Op
		want map[string]bool
	}{
		{"define value truncated", []Op{Define("FOO=bar")}, map[string]bool{"FOO": true}},
		{"undef then define", []Op{Undefine("A"), Define("A")}, map[string]bool{"A": true}},
		{"define then undef", []Op{Define("A"), Undefine("A")}, map[string]bool{}},
		{"redefine", []Op{Define("A"), Undefine("A"), Define("A")}, map[string]bool{"A": true}},
		{"undefine no-op", []Op{Undefine("MISSING")}, map[string]bool{}},
	}
	for _, tt := range tests {
		got := buildSet(tt.ops...)
		if len(got) != len(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
			continue
		}
		for k := range tt.want {
			if !got[k] {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
			}
		}
	}
}

func TestEvalBool(t *testing.T) {
	set := buildSet(Define("FOO"))
	tests := []struct {
		expr string
		want bool
	}{
		{"FOO", true},
		{"MISSING", false},
		{"!FOO", false},
		{"!!FOO", true},
		{"FOO && MISSING", false},
		{"FOO || MISSING", true},
		{"MISSING || FOO", true},
		{"MISSING && FOO || FOO", true},
		{"(FOO || MISSING) && !MISSING", true},
		{"((FOO))", true},
		{"MISSING && MISSING2 || FOO && FOO", true},
	}
	for _, tt := range tests {
		got, err := evalBool(tt.expr, 1, set)
		if err != nil {
			t.Errorf("%q: %v", tt.expr, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q: got %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestProcessString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no directives", "plain text\n", "plain text\n"},
		{"crlf normalized", "a\r\nb\r\n", "a\nb\n"},
		{"lone cr kept", "a\rb\n", "a\rb\n"},
	}
	for _, tt := range tests {
		got, err := ProcessString(tt.in)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func pp(t *testing.T, in string, ops ...Op) string {
	t.Helper()
	out, err := Process([]rune(in), ops...)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	return string(out)
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ops  []Op
		want string
	}{
		{"ifdef true",
			"%ifdef FOO\ntext\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"          \ntext\n      \n"},
		{"ifdef false",
			"%ifdef MISSING\ntext\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"              \n    \n      \n"},
		{"ifndef false",
			"%ifndef FOO\ntext\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"           \n    \n      \n"},
		{"ifndef true",
			"%ifndef MISSING\ntext\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"               \ntext\n      \n"},
		{"if expr",
			"%if FOO && BAR\ntext\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"              \ntext\n      \n"},
		{"if not",
			"%if !MISSING\ntext\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"            \ntext\n      \n"},
		{"if parens",
			"%if (FOO || MISSING) && BAR\ntext\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"                           \ntext\n      \n"},
		{"define then undef",
			"%ifdef FOO\ntext\n%endif\n",
			[]Op{Define("FOO"), Undefine("FOO")},
			"          \n    \n      \n"},
		{"undef then define",
			"%ifdef FOO\ntext\n%endif\n",
			[]Op{Undefine("FOO"), Define("FOO")},
			"          \ntext\n      \n"},
		{"else taken",
			"%ifdef FOO\na\n%else\nb\n%endif\n",
			[]Op{Define("FOO")},
			"          \na\n     \n \n      \n"},
		{"else not taken",
			"%ifdef MISSING\na\n%else\nb\n%endif\n",
			[]Op{Define("FOO")},
			"              \n \n     \nb\n      \n"},
		{"elif chain",
			"%ifdef MISSING\na\n%elif BAR\nb\n%else\nc\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"              \n \n         \nb\n     \n \n      \n"},
		{"elif taken",
			"%ifdef FOO\na\n%elif BAR\nb\n%endif\n",
			[]Op{Define("FOO"), Define("BAR")},
			"          \na\n         \nb\n      \n"},
		{"elif not taken",
			"%ifdef MISSING\na\n%elif MISSING2\nb\n%endif\n",
			[]Op{Define("FOO")},
			"              \n \n              \n \n      \n"},
		{"nested",
			"%ifdef FOO\n%ifdef MISSING\na\n%endif\n%endif\nx\n",
			[]Op{Define("FOO")},
			"          \n              \n \n      \n      \nx\n"},
		{"not line start",
			"x %ifdef FOO\ny\n",
			[]Op{Define("FOO")},
			"x %ifdef FOO\ny\n"},
		{"indented not directive",
			"  %ifdef FOO\ny\n",
			[]Op{Define("FOO")},
			"  %ifdef FOO\ny\n"},
		{"unmatched endif ignored",
			"%endif\ntext\n",
			nil,
			"      \ntext\n"},
		{"crlf input",
			"%ifdef FOO\r\ntext\r\n%endif\r\n",
			[]Op{Define("FOO")},
			"          \ntext\n      \n"},
	}
	for _, tt := range tests {
		if got := pp(t, tt.in, tt.ops...); got != tt.want {
			t.Errorf("%s:\nin  %q\nout %q\nwant%q", tt.name, tt.in, got, tt.want)
		}
	}
}

func TestProcessErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"unterminated",
			"%ifdef FOO\ntext\n",
			"unterminated %if starting on line 1"},
		{"nested unterminated",
			"%ifdef FOO\n%ifdef BAR\ntext\n",
			"unterminated %if starting on line 2"},
		{"syntax error",
			"%if FOO &&\ntext\n%endif\n",
			"%if syntax error on line 1"},
		{"syntax error paren",
			"%if (FOO\ntext\n%endif\n",
			"%if syntax error on line 1"},
		{"elif after else",
			"%ifdef FOO\n%else\n%elif BAR\n%endif\n",
			"%elif after %else on line 3"},
	}
	for _, tt := range tests {
		_, err := Process([]rune(tt.in))
		if err == nil {
			t.Errorf("%s: expected error, got none", tt.name)
			continue
		}
		if got := err.Error(); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
