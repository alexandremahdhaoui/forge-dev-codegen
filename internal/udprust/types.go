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

package udprust

import "github.com/alexandremahdhaoui/forge-dev-codegen/internal/layoutports"

type Options struct {
	Service string
	Cell    string
	Hello   string
	Push    []string
	Ports   []PortSpec
	Gate    GateSpec
}

type GateSpec struct {
	Field    string
	Adapters *[]string
}

const GateAdapterSecret = "secret"

func GateAdapterKinds() []string {
	return []string{GateAdapterSecret}
}

type PortSpec = layoutports.Spec

const CounterPortKind = "counter"

const CounterAdapterMemory = "memory"

func PortKinds() []string {
	return []string{CounterPortKind}
}

func CounterAdapterKinds() []string {
	return []string{CounterAdapterMemory}
}

type File struct {
	Path    string
	Content string
}
