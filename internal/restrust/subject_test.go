package restrust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/taxonomy"
)

func clientSubjectArms() string {
	arms := []string{"Self::" + taxonomy.WireRuntime().Variant + " { operation, .. }"}

	for _, member := range taxonomy.WireDetailed() {
		arms = append(arms, "Self::"+member.Variant+" { operation, .. }")
	}

	return strings.Join(arms, " | ") + " => operation.clone(),"
}

func TestEveryPortErrorAnswersTheIdentifierItCarriesThroughSubject(t *testing.T) {
	guarded, err := generatedByPath(t, strings.Replace(guardedSpec, "answers: page", "answers: one", 1))
	if err != nil {
		t.Fatalf("generating the guarded spec: %v", err)
	}

	queried := queriedFiles(t)

	cases := []struct {
		kind  string
		files map[string]string
		path  string
		want  string
	}{
		{
			kind:  "store",
			files: guarded,
			path:  "port/zz_generated_greeting_store.rs",
			want:  "impl GreetingStoreError {\n    pub fn subject(&self) -> String {\n        match self {\n            Self::Put { id, .. } | Self::Get { id, .. } | Self::Delete { id, .. } => id.clone(),\n            Self::Lookup { value, .. } => value.clone(),\n            Self::Missing { key } => key.clone(),\n            Self::Duplicate { values, .. } => values.clone(),\n        }\n    }\n}",
		},
		{
			kind:  "subscribe",
			files: guarded,
			path:  "port/zz_generated_greeting_event_subscribe.rs",
			want:  "impl GreetingEventSubscribeError {\n    pub fn subject(&self) -> String {\n        match self {\n            Self::Subscribe { key, .. } | Self::Publish { key, .. } => key.clone(),\n        }\n    }\n}",
		},
		{
			kind:  "declared",
			files: queried,
			path:  "port/zz_generated_greeting_clock.rs",
			want:  "impl GreetingClockError {\n    pub fn subject(&self) -> String {\n        match self {\n            Self::Refused { method, .. } | Self::Call { method, .. } => method.clone(),\n        }\n    }\n}",
		},
		{
			kind:  "client",
			files: guarded,
			path:  "port/zz_generated_greeting_client.rs",
			want:  "impl GreetingClientError {\n    pub fn subject(&self) -> String {\n        match self {\n            " + clientSubjectArms() + "\n        }\n    }\n}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			wantIn(t, tc.files, tc.path, tc.want)
		})
	}
}

func TestEveryHeldPortErrorConvertsIntoEachRestControllerErrorThatHoldsThePort(t *testing.T) {
	spec := strings.Replace(helloSpec, "      operationId: getGreeting\n      x-controller: greeting\n      x-ports: [GreetingStore]", "      operationId: getGreeting\n      x-controller: lookup\n      x-ports: [GreetingStore, HelloClient]", 1)

	files, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	conversion := func(port, controller string) string {
		return "impl From<" + port + "Error> for " + controller + "ControllerError {\n    fn from(source: " + port + "Error) -> Self {\n        Self::" + port + " {\n            id: source.subject(),\n            source,\n        }\n    }\n}"
	}

	wantIn(t, byPath, "controller/zz_generated_greeting_controller.rs", conversion("GreetingStore", "Greeting"))
	wantIn(t, byPath, "controller/zz_generated_lookup_controller.rs",
		conversion("GreetingStore", "Lookup"),
		conversion("HelloClient", "Lookup"),
		"use crate::port::hello_client::{HelloClient, HelloClientError};",
	)

	if strings.Contains(byPath["controller/zz_generated_greeting_controller.rs"], "HelloClientError") {
		t.Errorf("the greeting controller holds no hello client and still converts its error:\n%s", byPath["controller/zz_generated_greeting_controller.rs"])
	}
}
