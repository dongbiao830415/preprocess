// Copyright (c) 2026 The preprocess Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package preprocess implements the %ifdef/%if/%ifndef/%elif/%else/%endif
// input preprocessor directives, ported from the SQLite lemon parser
// generator. Directives must start at a line start (no leading whitespace)
// and be followed by whitespace or end of line.
package preprocess

import (
	"fmt"
	"strings"
)

// Op is a single macro operation, mirroring the lemon command line options:
// Define is -D, Undefine is -U. Operations are applied in the order given, so
// an Undefine only removes macros defined by earlier Define operations.
type Op struct {
	Name  string
	Undef bool // false = -D (define), true = -U (undefine).
}

// Define returns the operation defining macro NAME (-D NAME[=VALUE]; for an
// entry with =, only NAME is defined).
func Define(name string) Op { return Op{Name: name} }

// Undefine returns the operation undefining macro NAME (-U NAME).
func Undefine(name string) Op { return Op{Name: name, Undef: true} }

// Process handles the %ifdef/%if/%ifndef/%elif/%else/%endif directives in
// src. ops are the macro operations (-D/-U semantics) applied in the order
// given. Directive lines and excluded text are replaced by spaces, newlines
// are kept, so line numbers are preserved. CRLF line separators are
// normalized to LF. On error a non-nil error with a line number is returned.
func Process(src []rune, ops ...Op) ([]rune, error) {
	return process(normalize(src), buildSet(ops...))
}

// ProcessString is the string version of Process.
func ProcessString(src string, ops ...Op) (string, error) {
	out, err := Process([]rune(src), ops...)
	return string(out), err
}

// buildSet returns the set of defined macros after applying ops in order: a
// Define adds its name (everything from = on is discarded), an Undefine
// removes its name.
func buildSet(ops ...Op) map[string]bool {
	set := map[string]bool{}
	for _, op := range ops {
		name := op.Name
		if !op.Undef {
			if i := strings.IndexByte(name, '='); i >= 0 {
				name = name[:i]
			}
		}
		if op.Undef {
			delete(set, name)
		} else {
			set[name] = true
		}
	}
	return set
}

// evalBool evaluates a preprocessor boolean expression: identifiers (macro
// names), !, &&, ||, parentheses. Precedence: ! > && > ||.
func evalBool(expr string, lineno int, set map[string]bool) (bool, error) {
	p := &boolParser{expr: expr, lineno: lineno, set: set}
	p.skip()
	v, err := p.orExpr()
	if err != nil {
		return false, err
	}
	p.skip()
	if p.pos != len(p.expr) {
		return false, p.syntaxErr()
	}
	return v, nil
}

type boolParser struct {
	expr   string
	pos    int
	lineno int
	set    map[string]bool
}

func (p *boolParser) syntaxErr() error {
	return fmt.Errorf("%%if syntax error on line %d", p.lineno)
}

func (p *boolParser) skip() {
	for p.pos < len(p.expr) && (p.expr[p.pos] == ' ' || p.expr[p.pos] == '\t') {
		p.pos++
	}
}

func (p *boolParser) orExpr() (bool, error) {
	v, err := p.andExpr()
	if err != nil {
		return false, err
	}
	for {
		p.skip()
		if p.pos+1 < len(p.expr) && p.expr[p.pos:p.pos+2] == "||" {
			p.pos += 2
			w, err := p.andExpr()
			if err != nil {
				return false, err
			}
			v = v || w
		} else {
			return v, nil
		}
	}
}

func (p *boolParser) andExpr() (bool, error) {
	v, err := p.unary()
	if err != nil {
		return false, err
	}
	for {
		p.skip()
		if p.pos+1 < len(p.expr) && p.expr[p.pos:p.pos+2] == "&&" {
			p.pos += 2
			w, err := p.unary()
			if err != nil {
				return false, err
			}
			v = v && w
		} else {
			return v, nil
		}
	}
}

func (p *boolParser) unary() (bool, error) {
	p.skip()
	if p.pos < len(p.expr) && p.expr[p.pos] == '!' {
		p.pos++
		v, err := p.unary()
		return !v, err
	}
	return p.primary()
}

func (p *boolParser) primary() (bool, error) {
	p.skip()
	if p.pos >= len(p.expr) {
		return false, p.syntaxErr()
	}
	switch {
	case p.expr[p.pos] == '(':
		p.pos++
		v, err := p.orExpr()
		if err != nil {
			return false, err
		}
		p.skip()
		if p.pos >= len(p.expr) || p.expr[p.pos] != ')' {
			return false, p.syntaxErr()
		}
		p.pos++
		return v, nil
	case isAlpha(p.expr[p.pos]):
		start := p.pos
		for p.pos < len(p.expr) && (isAlphaNum(p.expr[p.pos]) || p.expr[p.pos] == '_') {
			p.pos++
		}
		return p.set[p.expr[start:p.pos]], nil
	}
	return false, p.syntaxErr()
}

func isAlpha(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isAlphaNum(c byte) bool {
	return isAlpha(c) || c >= '0' && c <= '9'
}

// ppFrame is one stack frame of a nested conditional block.
type ppFrame struct {
	start    int  // Start of this level's excluded region (when !taken).
	lineno   int  // Line of this level's %if directive.
	taken    bool // Whether this if/elif/else chain already took a branch.
	elseSeen bool // %else seen; a later %elif is an error.
}

// process handles the %ifdef/%if/%ifndef/%elif/%else/%endif directives in
// src. Directives and excluded text are replaced by spaces, newlines are
// kept, so line numbers are preserved.
func process(src []rune, set map[string]bool) ([]rune, error) {
	res := append([]rune(nil), src...)
	var frames []ppFrame
	exclude := 0 // Number of nested levels whose branch was not taken.
	lineno := 1
	for i := 0; i < len(res); i++ {
		if res[i] == '\n' {
			lineno++
			continue
		}
		if res[i] != '%' || (i > 0 && res[i-1] != '\n') {
			continue
		}
		switch {
		case matchDirective(res, i, "%endif"):
			if len(frames) != 0 {
				f := frames[len(frames)-1]
				frames = frames[:len(frames)-1]
				if !f.taken {
					exclude--
					if exclude == 0 {
						blank(res, f.start, i)
					}
				}
			}
			i = blankLine(res, i) - 1
		case matchDirective(res, i, "%else"):
			if len(frames) == 0 {
				frames = append(frames, ppFrame{start: i, lineno: lineno})
				exclude = 1
			} else {
				f := &frames[len(frames)-1]
				f.elseSeen = true
				if f.taken {
					f.taken = false
					f.start = i
					f.lineno = lineno
					exclude++
				} else {
					f.taken = true
					exclude--
					if exclude == 0 {
						blank(res, f.start, i)
					}
				}
			}
			i = blankLine(res, i) - 1
		case matchDirective(res, i, "%elif"):
			if len(frames) == 0 {
				frames = append(frames, ppFrame{start: i, lineno: lineno})
				exclude = 1
			} else {
				f := &frames[len(frames)-1]
				if f.elseSeen {
					return nil, fmt.Errorf("%%elif after %%else on line %d", lineno)
				}
				if !f.taken {
					v, err := evalBool(lineText(res, i+5), lineno, set)
					if err != nil {
						return nil, err
					}
					if v {
						f.taken = true
						exclude--
						if exclude == 0 {
							blank(res, f.start, i)
						}
					}
				}
			}
			i = blankLine(res, i) - 1
		case matchDirective(res, i, "%ifndef"), matchDirective(res, i, "%ifdef"), matchDirective(res, i, "%if"):
			kwLen, isNot := 3, false
			switch {
			case matchDirective(res, i, "%ifdef"):
				kwLen = 6
			case matchDirective(res, i, "%ifndef"):
				kwLen = 7
				isNot = true
			}
			v, err := evalBool(lineText(res, i+kwLen), lineno, set)
			if err != nil {
				return nil, err
			}
			if isNot {
				v = !v
			}
			frames = append(frames, ppFrame{start: i, lineno: lineno, taken: v})
			if !v {
				exclude++
			}
			i = blankLine(res, i) - 1
		}
	}
	if exclude != 0 {
		return nil, fmt.Errorf("unterminated %%if starting on line %d", frames[len(frames)-1].lineno)
	}
	return res, nil
}

// matchDirective reports whether res[i:] starts with directive kw followed by
// whitespace or end of line.
func matchDirective(res []rune, i int, kw string) bool {
	if i+len(kw) > len(res) {
		return false
	}
	if string(res[i:i+len(kw)]) != kw {
		return false
	}
	if i+len(kw) == len(res) {
		return true
	}
	c := res[i+len(kw)]
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// blank replaces all non-newline runes in res[from:to] with spaces.
func blank(res []rune, from, to int) {
	for j := from; j < to; j++ {
		if res[j] != '\n' {
			res[j] = ' '
		}
	}
}

// blankLine replaces the directive line starting at i (up to, but excluding,
// the newline) with spaces and returns the newline index, or len(res) at EOF.
func blankLine(res []rune, i int) int {
	for ; i < len(res) && res[i] != '\n'; i++ {
		res[i] = ' '
	}
	return i
}

// lineText returns the text from start, skipping leading whitespace, up to
// the end of the line (excluding the newline).
func lineText(res []rune, start int) string {
	for start < len(res) && (res[start] == ' ' || res[start] == '\t') {
		start++
	}
	end := start
	for end < len(res) && res[end] != '\n' {
		end++
	}
	return string(res[start:end])
}

// normalize replaces \r\n with \n.
func normalize(src []rune) []rune {
	out := make([]rune, 0, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] == '\r' && i+1 < len(src) && src[i+1] == '\n' {
			out = append(out, '\n')
			i++
			continue
		}
		out = append(out, src[i])
	}
	return out
}
