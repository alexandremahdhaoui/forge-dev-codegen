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

package taxonomy_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/taxonomy"
)

func TestEveryMemberNamesAWireTypeTheClientEnumDeclares(t *testing.T) {
	declared := map[string]bool{taxonomy.WireRuntime().Type: true}
	for _, w := range taxonomy.WireDetailed() {
		declared[w.Type] = true
	}

	for _, m := range taxonomy.All("greeting") {
		if !declared[m.Wire] {
			t.Errorf("member %s names wire type %q and no client variant reads it back", m.Variant, m.Wire)
		}
	}
}

func TestEveryMemberCarriesARestStatusAGrpcMethodAndAGrpcCode(t *testing.T) {
	for _, m := range taxonomy.All("greeting") {
		if m.RestStatusExpr == "" {
			t.Errorf("member %s carries no rest status expression", m.Variant)
		}

		if m.GrpcMethod == "" || m.GrpcCode == "" {
			t.Errorf("member %s carries no grpc status, got method %q and code %q", m.Variant, m.GrpcMethod, m.GrpcCode)
		}

		if len(m.Fields) == 0 {
			t.Errorf("member %s declares no field, a vector cannot spell it", m.Variant)
		}
	}
}

func TestNoTwoMembersShareAGrpcCode(t *testing.T) {
	seen := map[string]string{}

	for _, m := range taxonomy.All("greeting") {
		if other, taken := seen[m.GrpcCode]; taken {
			t.Errorf("members %s and %s both map to tonic::Code::%s, a caller cannot tell them apart", other, m.Variant, m.GrpcCode)
		}

		seen[m.GrpcCode] = m.Variant
	}
}

func TestRuntimeIsTheOnlyMemberThatHidesItsDetail(t *testing.T) {
	if !taxonomy.Runtime("greeting").Generic {
		t.Fatal("a runtime failure must answer a generic message")
	}

	for _, m := range taxonomy.Detailed("greeting") {
		if m.Generic {
			t.Errorf("member %s hides its detail and a player mistake gets a detailed message", m.Variant)
		}
	}
}

func TestTheOwnerFillsTheNotFoundDisplayAndNoOtherSlotRemains(t *testing.T) {
	for _, m := range taxonomy.All("greeting") {
		if strings.Contains(m.Display, taxonomy.OwnerSlot) {
			t.Errorf("member %s still carries the owner slot in %q", m.Variant, m.Display)
		}
	}

	for _, m := range taxonomy.Detailed("greeting") {
		if m.Variant != "NotFound" {
			continue
		}

		if want := "finding greeting {id:?}: not found"; m.Display != want {
			t.Fatalf("NotFound reads %q, want %q", m.Display, want)
		}

		return
	}

	t.Fatal("NotFound is a member")
}

func TestByVariantAnswersEveryMemberAndRefusesANameNobodyDeclares(t *testing.T) {
	for _, name := range taxonomy.VariantNames() {
		if _, ok := taxonomy.ByVariant(name); !ok {
			t.Errorf("ByVariant does not answer %q and VariantNames lists it", name)
		}
	}

	if _, ok := taxonomy.ByVariant("Bogus"); ok {
		t.Fatal("ByVariant answered a member nobody declares")
	}
}

func TestByRestStatusAnswersTheStaticStatusesAndLeavesInvalidToTheOperation(t *testing.T) {
	for _, m := range taxonomy.Detailed("greeting") {
		if m.RestStatus == 0 {
			continue
		}

		got, ok := taxonomy.ByRestStatus(m.RestStatus)
		if !ok || got.Variant != m.Variant {
			t.Errorf("status %d answers %q, want %s", m.RestStatus, got.Variant, m.Variant)
		}
	}

	if _, ok := taxonomy.ByRestStatus(500); ok {
		t.Fatal("500 is the runtime status the driver answers itself, no controller error mocks it")
	}

	if taxonomy.Invalid().RestStatus != 0 {
		t.Fatal("Invalid reads the status the operation declares, never a fixed one")
	}
}

func TestTheLiteralPutsTheSubjectInTheFirstFieldAndTheFillerInTheRest(t *testing.T) {
	tests := []struct {
		variant string
		want    string
	}{
		{
			variant: "NotFound",
			want:    `GreetingControllerError::NotFound { id: "g1".to_string() }`,
		},
		{
			variant: "Invalid",
			want:    `GreetingControllerError::Invalid { field: "g1".to_string(), reason: "filler".to_string() }`,
		},
		{
			variant: "RateLimited",
			want:    `GreetingControllerError::RateLimited { subject: "g1".to_string(), reason: "filler".to_string() }`,
		},
	}

	for _, tc := range tests {
		m, ok := taxonomy.ByVariant(tc.variant)
		if !ok {
			t.Fatalf("%s is a member", tc.variant)
		}

		if got := m.Literal("GreetingControllerError", "g1", "filler"); got != tc.want {
			t.Errorf("the literal reads %q, want %q", got, tc.want)
		}
	}
}

func TestTheDetailExpressionHidesARuntimeFailureAndShowsAPlayerMistake(t *testing.T) {
	if got := taxonomy.Runtime("greeting").DetailExpr(); got != `"internal error".to_string()` {
		t.Fatalf("a runtime failure answers %s, want a generic message", got)
	}

	if got := taxonomy.Invalid().DetailExpr(); got != "error.to_string()" {
		t.Fatalf("a player mistake answers %s, want the detail", got)
	}
}

func TestTheFieldListSpellsEveryFieldWithItsRustType(t *testing.T) {
	m, ok := taxonomy.ByVariant("Authentication")
	if !ok {
		t.Fatal("Authentication is a member")
	}

	if want := "subject: String, reason: String"; m.FieldList() != want {
		t.Fatalf("the field list reads %q, want %q", m.FieldList(), want)
	}
}

func TestTheRestStatusListNamesEveryDetailedMemberOnce(t *testing.T) {
	list := taxonomy.RestStatusList()

	for _, name := range taxonomy.DetailedVariantNames() {
		if !strings.Contains(list, name) {
			t.Errorf("the refusal list lacks %s, an author cannot learn the status it answers", name)
		}
	}

	if strings.Contains(list, "Runtime") {
		t.Fatal("a controller never answers Runtime by name, the port arms carry it")
	}
}
