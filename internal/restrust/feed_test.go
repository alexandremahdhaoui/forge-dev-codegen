package restrust_test

import (
	"os"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const chatLineFeedFixture = "testdata/chat_line_feed.openapi.yaml"

func chatLineFeedSpec(t *testing.T) string {
	t.Helper()

	doc, err := os.ReadFile(chatLineFeedFixture)
	if err != nil {
		t.Fatalf("reading the fixture %s: %v", chatLineFeedFixture, err)
	}

	return string(doc)
}

func TestAStoreFeedingAStreamOfItsOwnTypeImportsEachTypeOnceInBothStoreAdapters(t *testing.T) {
	files := generateSpec(t, chatLineFeedSpec(t), restrust.Options{Service: "songe-social"})

	for _, path := range []string{"adapter/zz_generated_chat_line_sqlite.rs", "adapter/zz_generated_chat_line_memory.rs"} {
		file, emitted := files[path]
		if !emitted {
			t.Fatalf("%s was not emitted, the emitted paths are %q", path, paths(files))
		}

		if !strings.Contains(file.Content, "use crate::rest::types::chat_line::ChatLine;\n") {
			t.Errorf("%s never imports ChatLine\n%s", path, file.Content)
		}

		seen := map[string]bool{}

		for _, line := range strings.Split(file.Content, "\n") {
			if !strings.HasPrefix(line, "use ") {
				continue
			}

			if seen[line] {
				t.Errorf("%s carries %q twice\n%s", path, line, file.Content)
			}

			seen[line] = true
		}
	}
}

func TestTheStoreMemoryModuleAndTheFeedMemoryModuleOfOneTypeLandOnTwoPaths(t *testing.T) {
	files := generateSpec(t, chatLineFeedSpec(t), restrust.Options{Service: "songe-social"})

	for path, holds := range map[string]string{
		"adapter/zz_generated_chat_line_memory.rs":      "pub struct ChatLineMemoryStore {",
		"adapter/zz_generated_chat_line_memory_feed.rs": "pub struct ChatLineMemoryFeed {",
	} {
		file, emitted := files[path]
		if !emitted {
			t.Fatalf("%s was not emitted, the emitted paths are %q", path, paths(files))
		}

		if !strings.Contains(file.Content, holds) {
			t.Errorf("%s lacks %q\n%s", path, holds, file.Content)
		}
	}

	mod := files["adapter/mod.rs"].Content

	for _, want := range []string{
		"pub use zz_generated_chat_line_memory as chat_line_memory;",
		"pub use zz_generated_chat_line_memory_feed as chat_line_memory_feed;",
	} {
		if !strings.Contains(mod, want) {
			t.Errorf("the adapter mod.rs lacks %q\n%s", want, mod)
		}
	}
}

func TestGenerateRefusesTwoEmittedFilesOnOnePathNamingBothTemplatesAndThePath(t *testing.T) {
	spec := strings.Replace(queriedSpec, "name: GreetingClock", "name: Greeting", 1)
	spec = strings.Replace(spec, "x-ports: [GreetingStore, GreetingClock]", "x-ports: [GreetingStore, Greeting]", 1)

	_, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-hello"})

	want := "emitting the rest cell: the memory_store template and the clock_memory template both write adapter/zz_generated_greeting_memory.rs, one path holds one file"
	if err == nil || err.Error() != want {
		t.Fatalf("two files on one path were not refused by name\n got %v\nwant %s", err, want)
	}
}
