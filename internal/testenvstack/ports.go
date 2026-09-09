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

package testenvstack

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

const LoopbackHost = "127.0.0.1"

const (
	placeholderDelimiter  = "@"
	portPlaceholderPrefix = "port."
	tmpDirPlaceholder     = "tmpDir"
)

type Placeholders struct {
	Ports  map[string]int
	TmpDir string
}

func AllocatePorts(names []string) (map[string]int, error) {
	allocated := make(map[string]int, len(names))
	holders := make([]net.Listener, 0, len(names))

	defer func() {
		for _, holder := range holders {
			_ = holder.Close()
		}
	}()

	for _, name := range names {
		if name == "" {
			return nil, fmt.Errorf("allocating a port: the name is empty")
		}

		if _, taken := allocated[name]; taken {
			return nil, fmt.Errorf("allocating port %q: the name is declared twice", name)
		}

		listener, err := net.Listen("tcp", LoopbackHost+":0")
		if err != nil {
			return nil, fmt.Errorf("allocating port %q: %w", name, err)
		}

		holders = append(holders, listener)

		address, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			return nil, fmt.Errorf("allocating port %q: the listener answered %T and not a tcp address", name, listener.Addr())
		}

		allocated[name] = address.Port
	}

	return allocated, nil
}

func (p Placeholders) Substitute(text string) (string, error) {
	var out strings.Builder

	rest := text

	for {
		open := strings.Index(rest, placeholderDelimiter)
		if open < 0 {
			out.WriteString(rest)

			return out.String(), nil
		}

		end := strings.Index(rest[open+1:], placeholderDelimiter)
		if end < 0 {
			return "", fmt.Errorf("substituting %q: an opening %s has no closing %s", text, placeholderDelimiter, placeholderDelimiter)
		}

		value, err := p.resolve(rest[open+1 : open+1+end])
		if err != nil {
			return "", fmt.Errorf("substituting %q: %w", text, err)
		}

		out.WriteString(rest[:open])
		out.WriteString(value)

		rest = rest[open+end+2:]
	}
}

func (p Placeholders) SubstituteAll(values []string) ([]string, error) {
	if values == nil {
		return nil, nil
	}

	out := make([]string, 0, len(values))

	for i, value := range values {
		substituted, err := p.Substitute(value)
		if err != nil {
			return nil, fmt.Errorf("reading item %d: %w", i, err)
		}

		out = append(out, substituted)
	}

	return out, nil
}

func (p Placeholders) SubstituteMap(values map[string]string) (map[string]string, error) {
	if values == nil {
		return nil, nil
	}

	out := make(map[string]string, len(values))

	for key, value := range values {
		substituted, err := p.Substitute(value)
		if err != nil {
			return nil, fmt.Errorf("reading key %q: %w", key, err)
		}

		out[key] = substituted
	}

	return out, nil
}

func (p Placeholders) resolve(inner string) (string, error) {
	if inner == tmpDirPlaceholder {
		return p.TmpDir, nil
	}

	if !strings.HasPrefix(inner, portPlaceholderPrefix) {
		return "", fmt.Errorf("placeholder %q names nothing; the placeholders are @tmpDir@ and @port.<name>@ over %s", inner, PortNames(p.Ports))
	}

	name := strings.TrimPrefix(inner, portPlaceholderPrefix)

	port, ok := p.Ports[name]
	if !ok {
		return "", fmt.Errorf("port %q is not declared by this service; the declared ports are %s", name, PortNames(p.Ports))
	}

	return strconv.Itoa(port), nil
}

func PortNames(ports map[string]int) string {
	if len(ports) == 0 {
		return "no port at all"
	}

	names := make([]string, 0, len(ports))
	for name := range ports {
		names = append(names, name)
	}

	sort.Strings(names)

	return strings.Join(names, ", ")
}

func PortAddresses(addrEnv string, ports map[string]int) map[string]string {
	out := make(map[string]string, len(ports))

	for name, port := range ports {
		out[addrEnv+"_"+strings.ToUpper(name)] = LoopbackHost + ":" + strconv.Itoa(port)
	}

	return out
}
