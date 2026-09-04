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

package rustname_test

import (
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

func TestNamesFollowRustCasing(t *testing.T) {
	tests := []struct {
		in     string
		snake  string
		pascal string
		upper  string
	}{
		{"createGreeting", "create_greeting", "CreateGreeting", "CREATE_GREETING"},
		{"GreetingStore", "greeting_store", "GreetingStore", "GREETING_STORE"},
		{"player-session", "player_session", "PlayerSession", "PLAYER_SESSION"},
		{"songe-hello", "songe_hello", "SongeHello", "SONGE_HELLO"},
		{"HTTPServer", "httpserver", "HTTPServer", "HTTPSERVER"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := rustname.Snake(tt.in); got != tt.snake {
				t.Errorf("Snake(%q) = %q, want %q", tt.in, got, tt.snake)
			}

			if got := rustname.Pascal(tt.in); got != tt.pascal {
				t.Errorf("Pascal(%q) = %q, want %q", tt.in, got, tt.pascal)
			}

			if got := rustname.Upper(tt.in); got != tt.upper {
				t.Errorf("Upper(%q) = %q, want %q", tt.in, got, tt.upper)
			}
		})
	}
}

func TestAModuleNameIsASnakeIdentThatIsNoKeyword(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"rest", true},
		{"udp_v1", true},
		{"Rest", false},
		{"rest-cell", false},
		{"1rest", false},
		{"mod", false},
		{"type", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := rustname.IsModuleName(tt.in); got != tt.want {
				t.Errorf("IsModuleName(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestAKeywordIdentIsSpelledWithARawPrefix(t *testing.T) {
	if got := rustname.RustIdent("type"); got != "r#type" {
		t.Errorf("RustIdent(%q) = %q, want %q", "type", got, "r#type")
	}

	if got := rustname.RustIdent("sessionId"); got != "session_id" {
		t.Errorf("RustIdent(%q) = %q, want %q", "sessionId", got, "session_id")
	}
}
