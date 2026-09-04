// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpcrust

import (
	"strings"
	"unicode"
)

func Snake(s string) string {
	var b strings.Builder

	previousLower := false

	for _, r := range s {
		switch {
		case r == '-' || r == '_' || r == '.' || r == ' ':
			b.WriteRune('_')
			previousLower = false
		case unicode.IsUpper(r):
			if previousLower {
				b.WriteRune('_')
			}

			b.WriteRune(unicode.ToLower(r))
			previousLower = false
		default:
			b.WriteRune(r)
			previousLower = unicode.IsLower(r) || unicode.IsDigit(r)
		}
	}

	return b.String()
}

func Pascal(s string) string {
	var b strings.Builder

	up := true

	for _, r := range s {
		switch {
		case r == '-' || r == '_' || r == '.' || r == ' ':
			up = true
		case up:
			b.WriteRune(unicode.ToUpper(r))
			up = false
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func Upper(s string) string {
	return strings.ToUpper(Snake(s))
}

var rustKeywords = map[string]bool{
	"as": true, "break": true, "const": true, "continue": true, "crate": true, "else": true, "enum": true,
	"extern": true, "false": true, "fn": true, "for": true, "if": true, "impl": true, "in": true, "let": true,
	"loop": true, "match": true, "mod": true, "move": true, "mut": true, "pub": true, "ref": true, "return": true,
	"self": true, "static": true, "struct": true, "super": true, "trait": true, "true": true, "type": true,
	"unsafe": true, "use": true, "where": true, "while": true, "async": true, "await": true, "dyn": true,
}

func rustIdent(name string) string {
	ident := Snake(name)
	if rustKeywords[ident] {
		return "r#" + ident
	}

	return ident
}
