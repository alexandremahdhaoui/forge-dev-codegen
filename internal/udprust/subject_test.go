package udprust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
)

func sessionHolding(t *testing.T, ports ...udprust.PortSpec) map[string]string {
	t.Helper()

	opts := sessionOptions()
	opts.Ports = ports

	files, err := udprust.Generate([]byte(sessionProto), opts)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	return byPath
}

func carries(t *testing.T, byPath map[string]string, path string, wants ...string) {
	t.Helper()

	content, ok := byPath[path]
	if !ok {
		t.Fatalf("no file at %s", path)
	}

	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Errorf("%s lacks %q:\n%s", path, want, content)
		}
	}
}

func TestEveryPortErrorAnswersTheIdentifierItCarriesThroughSubject(t *testing.T) {
	byPath := sessionHolding(t)

	t.Run("client", func(t *testing.T) {
		carries(t, byPath, "port/zz_generated_hello_datagram_client.rs",
			"impl HelloDatagramClientError {\n    pub fn subject(&self) -> String {\n        match *self {\n            Self::Hello { .. } => \"songe.hello.udp.v1.HelloDatagram/Hello\".to_string(),\n            Self::Echo { .. } => \"songe.hello.udp.v1.HelloDatagram/Echo\".to_string(),\n        }\n    }\n}",
		)
	})

	t.Run("broadcast", func(t *testing.T) {
		carries(t, byPath, "port/zz_generated_hello_datagram_broadcast.rs",
			"impl HelloDatagramBroadcastError {\n    pub fn subject(&self) -> String {\n        match self {\n            Self::Encode { session_id, .. }\n            | Self::UnknownSession { session_id, .. }\n            | Self::PeerTable { session_id, .. } => format!(\"{session_id:?}\"),\n        }\n    }\n}",
		)
	})
}

func TestAUdpControllerErrorCarriesOneArmPerHeldPortAndTheDriverAnswersItAsRuntime(t *testing.T) {
	byPath := sessionHolding(t, counterPort(), udprust.PortSpec{Name: "Clock"})

	controllerPath := "controller/zz_generated_hello_datagram_controller.rs"

	for _, port := range []struct{ name, snake string }{{"HelloDatagramBroadcast", "hello_datagram_broadcast"}, {"Clock", "clock"}} {
		carries(t, byPath, controllerPath,
			"    #[error(\"calling the "+port.snake+" port for {id:?}\")]\n    "+port.name+" {\n        id: String,\n        #[source]\n        source: "+port.name+"Error,\n    },",
			"impl From<"+port.name+"Error> for HelloDatagramControllerError {\n    fn from(source: "+port.name+"Error) -> Self {\n        Self::"+port.name+" {\n            id: source.subject(),\n            source,\n        }\n    }\n}",
		)
	}

	controller := byPath[controllerPath]

	if strings.Contains(controller, "TickCounterError") || strings.Contains(controller, "    TickCounter {") {
		t.Errorf("a counter port answers no error and still got an arm:\n%s", controller)
	}

	if strings.Contains(controller, "kind: String") {
		t.Errorf("the broadcast error still has a second arm keyed by the push kind:\n%s", controller)
	}

	driverPath := "driver/zz_generated_hello_datagram_udp_driver.rs"

	carries(t, byPath, driverPath,
		"Err(error) => {\n                            eprintln!(\n                                \"dropping a datagram from {peer}: running songe.hello.udp.v1.HelloDatagram/Echo: {}\",\n                                error_chain(&error)\n                            );\n\n                            None\n                        }",
	)

	if strings.Contains(byPath[driverPath], "HelloDatagramControllerError::") {
		t.Errorf("the udp driver answers one controller error arm apart from the others:\n%s", byPath[driverPath])
	}
}
