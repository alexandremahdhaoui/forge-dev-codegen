package restrust_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const combatAddressSpec = `
openapi: 3.1.0
info:
  title: World API
  version: 1.0.0
paths:
  /combat/address:
    get:
      operationId: getCombatAddress
      x-controller: address
      x-ports:
        - kind: string
          name: CombatAddress
          value: Address
          adapters: [setting]
      responses:
        "200":
          description: The combat address
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Address"
components:
  schemas:
    Address:
      type: object
      required: [address]
      properties:
        address:
          type: string
`

const combatAddressCrateManifest = `[workspace]

[package]
name = "songe-world"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
thiserror = "2"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockall = "0.15"
`

const combatAddressCrateLib = `pub mod rest;
`

const addressControllerImpl = `use crate::rest::controller::{
    AddressController, AddressControllerError, AddressControllerImpl,
};
use crate::rest::types::address::Address;

impl AddressController for AddressControllerImpl {
    fn get_combat_address(&self) -> Result<Address, AddressControllerError> {
        self.combat_address
            .get()
            .map_err(|source| AddressControllerError::CombatAddress {
                id: "combat".to_string(),
                source,
            })
    }
}
`

const combatAddressTest = `use songe_world::rest::adapter::combat_address_setting::{
    CombatAddressSetting, CombatAddressSettingConfig,
};
use songe_world::rest::port::combat_address::CombatAddress;
use songe_world::rest::types::address::Address;

#[test]
fn the_setting_adapter_answers_the_address_its_configuration_named_on_every_call() {
    let adapter = CombatAddressSetting::new(CombatAddressSettingConfig {
        address: "combat.songe.test:7000".to_string(),
    });
    let named = Address {
        address: "combat.songe.test:7000".to_string(),
    };

    assert_eq!(adapter.get().unwrap(), named);
    assert_eq!(adapter.get().unwrap(), named);
}
`

var eightPortNames = []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel"}

func eightPortSpec() string {
	declarations := ""
	for _, name := range eightPortNames {
		declarations += "        - kind: string\n          name: " + name + "Address\n          value: Address\n          adapters: [setting]\n"
	}

	return strings.Replace(combatAddressSpec, "        - kind: string\n          name: CombatAddress\n          value: Address\n          adapters: [setting]\n", declarations, 1)
}

const eightPortControllerImpl = `use crate::rest::controller::{
    AddressController, AddressControllerError, AddressControllerImpl,
};
use crate::rest::types::address::Address;

impl AddressController for AddressControllerImpl {
    fn get_combat_address(&self) -> Result<Address, AddressControllerError> {
        self.alpha_address
            .get()
            .map_err(|source| AddressControllerError::AlphaAddress {
                id: "alpha".to_string(),
                source,
            })
    }
}
`

func TestARestControllerHoldingEightPortsPassesClippyDenyingWarningsBecauseItsConstructorCarriesTheTooManyArgumentsAllow(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	spec := eightPortSpec()
	if spec == combatAddressSpec {
		t.Fatal("the combat address declaration was never replaced, the fixture moved")
	}

	root := writeAddressCrate(t, spec, eightPortControllerImpl)

	runCargo(t, cargo, root, "clippy", "--all-targets", "--", "-D", "warnings")
}

func writeAddressCrate(t *testing.T, spec, controllerImpl string) string {
	t.Helper()

	files, err := restrust.Generate([]byte(spec), restrust.Options{Service: "songe-world"})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	root := t.TempDir()

	write := func(rel, content string) {
		t.Helper()

		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("making %s: %v", filepath.Dir(p), err)
		}

		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", p, err)
		}
	}

	write("Cargo.toml", combatAddressCrateManifest)
	write("src/lib.rs", combatAddressCrateLib)

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".rs") {
			continue
		}

		write(filepath.Join("src", "rest", f.Path), f.Content)
	}

	write("src/rest/controller/address_controller.rs", controllerImpl)

	return root
}

func TestTheSettingStringAdapterAnswersTheValueItsConfigurationNamed(t *testing.T) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("cargo is not on PATH")
	}

	root := writeAddressCrate(t, combatAddressSpec, addressControllerImpl)

	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatalf("making the tests directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "tests", "string.rs"), []byte(combatAddressTest), 0o644); err != nil {
		t.Fatalf("writing the string test: %v", err)
	}

	runCargo(t, cargo, root, "clippy", "--all-targets", "--", "-D", "warnings")
	runCargo(t, cargo, root, "test", "--test", "string")
}
