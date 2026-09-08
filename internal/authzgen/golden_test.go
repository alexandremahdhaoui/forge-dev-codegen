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

package authzgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/authzgen"
)

const demoDir = "../../demo/authz-gen"

func readDemoFile(t *testing.T, module, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(demoDir, module, rel))
	if err != nil {
		t.Fatalf("reading the demo file %s of %s: %v", rel, module, err)
	}

	return string(data)
}

func TestTheDemoModulesRegenerateExactlyTheCommittedGoldenFiles(t *testing.T) {
	for _, module := range []string{"session", "chat", "play"} {
		t.Run(module, func(t *testing.T) {
			files, err := authzgen.Generate([]byte(readDemoFile(t, module, "authz.yaml")))
			if err != nil {
				t.Fatalf("generating %s: %v", module, err)
			}

			for _, f := range files {
				if want := readDemoFile(t, module, f.Path); f.Content != want {
					t.Errorf("%s/%s drifted from the committed golden file\n got:\n%s\nwant:\n%s", module, f.Path, f.Content, want)
				}
			}
		})
	}
}

func TestThePlayGoldenModuleExtendsSessionAndGuardsAttackWithACondition(t *testing.T) {
	model := readDemoFile(t, "play", "zz_generated_play.fga")

	for _, want := range []string{
		"module play\n",
		"extend type session\n  relations\n    define turn_owner: [character]\n",
		"define alive: [character:* with monster_alive]\n",
		"define attack: alive and turn_owner from session and member from session\n",
		"condition monster_alive(hp: int) {\n  hp > 0\n}\n",
	} {
		if !strings.Contains(model, want) {
			t.Errorf("the play module never held\n%s", want)
		}
	}
}

func TestTheChatGoldenModuleReadsMembershipThroughTheSessionType(t *testing.T) {
	model := readDemoFile(t, "chat", "zz_generated_chat.fga")

	if !strings.Contains(model, "define member: member from session\n") {
		t.Fatalf("the chat module\n%s", model)
	}
}
