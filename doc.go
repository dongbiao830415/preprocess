// Copyright (c) 2026 The preprocess Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Preprocessor directives
//
// Before further processing, an input can be filtered with the directives
// below. They must start at a line start:
//
//	%if EXPR        include the block if EXPR is true
//	%ifdef NAME     include the block if NAME is a defined macro
//	%ifndef NAME    include the block if NAME is not a defined macro
//	%elif EXPR      alternative condition, evaluated if the previous ones were false
//	%else           include the block if no previous condition was true
//	%endif          end of a conditional block
//
// EXPR is a boolean expression of macro names with !, &&, || and parentheses.
// Excluded text is replaced by spaces, keeping line numbers intact.
// Directives nest. CRLF line separators are normalized to LF.
package preprocess
