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
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	ReadyStdout = "stdout"
	ReadyTCP    = "tcp"
	ReadyHTTP   = "http"
)

const ReadyKinds = ReadyStdout + ", " + ReadyTCP + ", " + ReadyHTTP

const probeTimeout = 2 * time.Second

type Ready struct {
	Kind   string
	Port   string
	Path   string
	Status int
}

func (r Ready) Validate(ports map[string]int) error {
	switch r.Kind {
	case ReadyStdout:
		return r.refuseFields("kind stdout", r.Port != "", r.Path != "", r.Status != 0)
	case ReadyTCP:
		if err := r.refuseFields("kind tcp", false, r.Path != "", r.Status != 0); err != nil {
			return err
		}

		return r.requirePort(ports)
	case ReadyHTTP:
		if r.Path == "" {
			return fmt.Errorf("readiness kind http: path is required and names the path to get")
		}

		if r.Status == 0 {
			return fmt.Errorf("readiness kind http: status is required and names the status the get must answer")
		}

		return r.requirePort(ports)
	default:
		return fmt.Errorf("readiness kind %q: not a kind; the kinds are %s", r.Kind, ReadyKinds)
	}
}

func (r Ready) requirePort(ports map[string]int) error {
	if r.Port == "" {
		return fmt.Errorf("readiness kind %s: port is required and names one of the declared ports %s", r.Kind, PortNames(ports))
	}

	if _, ok := ports[r.Port]; !ok {
		return fmt.Errorf("readiness kind %s: port %q is not declared by this service; the declared ports are %s", r.Kind, r.Port, PortNames(ports))
	}

	return nil
}

func (r Ready) refuseFields(label string, port bool, path bool, status bool) error {
	if port {
		return fmt.Errorf("readiness %s: port takes no effect and is refused", label)
	}

	if path {
		return fmt.Errorf("readiness %s: path takes no effect and is refused", label)
	}

	if status {
		return fmt.Errorf("readiness %s: status takes no effect and is refused", label)
	}

	return nil
}

func awaitTCP(ctx context.Context, address string, timeout time.Duration, exited <-chan error) error {
	return await(ctx, timeout, exited, fmt.Sprintf("no tcp connection to %s within %s", address, timeout), func() bool {
		connection, err := net.DialTimeout("tcp", address, probeTimeout)
		if err != nil {
			return false
		}

		_ = connection.Close()

		return true
	})
}

func awaitHTTP(ctx context.Context, url string, status int, timeout time.Duration, exited <-chan error) error {
	client := &http.Client{Timeout: probeTimeout}

	return await(ctx, timeout, exited, fmt.Sprintf("no %d from %s within %s", status, url, timeout), func() bool {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return false
		}

		response, err := client.Do(request)
		if err != nil {
			return false
		}

		_ = response.Body.Close()

		return response.StatusCode == status
	})
}

func await(ctx context.Context, timeout time.Duration, exited <-chan error, timeoutReason string, probe func() bool) error {
	deadline := time.After(timeout)

	for {
		if probe() {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("cancelled: %w", ctx.Err())
		case err := <-exited:
			return fmt.Errorf("the process exited before answering the probe: %v", err)
		case <-deadline:
			return errors.New(timeoutReason)
		case <-time.After(pollInterval):
		}
	}
}

func probeAddress(ports map[string]int, name string) string {
	return LoopbackHost + ":" + strconv.Itoa(ports[name])
}
