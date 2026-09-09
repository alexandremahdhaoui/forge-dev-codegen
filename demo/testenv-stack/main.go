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

package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const dialTimeout = 2 * time.Second

const verbs = "serve, token, check"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("reading the verb: none was given; the verbs are %s", verbs)
	}

	switch args[0] {
	case "serve":
		return serve(args[1:])
	case "token":
		return token()
	case "check":
		return check(args[1:])
	default:
		return fmt.Errorf("reading the verb %q: not a verb; the verbs are %s", args[0], verbs)
	}
}

func serve(args []string) error {
	address, err := flagValue(args, "--listen")
	if err != nil {
		return fmt.Errorf("reading the address to serve: %w", err)
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("binding %s: %w", address, err)
	}

	for {
		connection, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accepting on %s: %w", address, err)
		}

		_ = connection.Close()
	}
}

func token() error {
	if _, err := fmt.Println(`{"token":{"id":"demo-token"}}`); err != nil {
		return fmt.Errorf("answering the token: %w", err)
	}

	return nil
}

func check(keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("checking the stack: no key was given; a bare KEY must hold an address that accepts a connection and KEY=VALUE must hold VALUE")
	}

	for _, key := range keys {
		if name, want, found := strings.Cut(key, "="); found {
			if err := checkValue(name, want); err != nil {
				return err
			}

			continue
		}

		if err := checkAddress(key); err != nil {
			return err
		}
	}

	return nil
}

func checkValue(name string, want string) error {
	got, ok := os.LookupEnv(name)
	if !ok {
		return fmt.Errorf("reading %s: the stack exported no such variable", name)
	}

	if got != want {
		return fmt.Errorf("reading %s: the stack exported %q and the check wants %q", name, got, want)
	}

	return nil
}

func checkAddress(name string) error {
	address, ok := os.LookupEnv(name)
	if !ok {
		return fmt.Errorf("reading %s: the stack exported no such variable", name)
	}

	connection, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return fmt.Errorf("dialing %s at %s: %w", name, address, err)
	}

	return connection.Close()
}

func flagValue(args []string, flag string) (string, error) {
	for i, arg := range args {
		if arg != flag {
			continue
		}

		if i+1 >= len(args) {
			return "", fmt.Errorf("the flag %s carries no value", flag)
		}

		return args[i+1], nil
	}

	return "", fmt.Errorf("the flag %s is missing from %v", flag, args)
}
