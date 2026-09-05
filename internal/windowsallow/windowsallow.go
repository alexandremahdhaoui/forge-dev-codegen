package windowsallow

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	dosHeaderMinLen   = 64
	elfanewOffset     = 0x3c
	peSignatureLen    = 4
	timeDateStampGap  = 8
	timeDateStampSize = 4
)

type Probe struct {
	Args           []string
	Expect         string
	TimeoutSeconds int
}

type Request struct {
	Source      string
	Destination string
	Name        string
	Commit      string
	Attempts    int
	Keep        int
	Probe       Probe
}

type Runner func(dir, exe string, args []string, timeout time.Duration) (string, error)

type Outcome struct {
	AllowedName string
	AllowedPath string
	Attempts    int
	Pruned      []string
}

func belongsToThisBinary(entry, name string) bool {
	return strings.HasPrefix(entry, name+"-") && strings.HasSuffix(entry, ".exe")
}

func pruneOldBuilds(destination, name, allowedName string, keep int) ([]string, error) {
	entries, err := os.ReadDir(destination)
	if err != nil {
		return nil, fmt.Errorf("reading the destination %q to prune old builds: %w", destination, err)
	}
	type build struct {
		name    string
		modTime time.Time
	}
	var builds []build
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == allowedName || !belongsToThisBinary(entry.Name(), name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		builds = append(builds, build{name: entry.Name(), modTime: info.ModTime()})
	}
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].modTime.After(builds[j].modTime)
	})
	keepOthers := keep - 1
	if keepOthers < 0 {
		keepOthers = 0
	}
	var pruned []string
	for i, b := range builds {
		if i < keepOthers {
			continue
		}
		if err := os.Remove(filepath.Join(destination, b.name)); err != nil {
			return pruned, fmt.Errorf("pruning the old build %q: %w", b.name, err)
		}
		pruned = append(pruned, b.name)
	}
	return pruned, nil
}

func timeDateStampOffset(pe []byte) (int, error) {
	if len(pe) < dosHeaderMinLen {
		return 0, fmt.Errorf("reading the source, it is %d bytes and a PE file needs at least %d", len(pe), dosHeaderMinLen)
	}
	if pe[0] != 'M' || pe[1] != 'Z' {
		return 0, fmt.Errorf("reading the source, it does not start with the MZ magic of a PE file")
	}
	elfanew := int(binary.LittleEndian.Uint32(pe[elfanewOffset : elfanewOffset+4]))
	stamp := elfanew + timeDateStampGap
	if elfanew <= 0 || elfanew+peSignatureLen > len(pe) {
		return 0, fmt.Errorf("reading the source, its PE header offset %d points outside the %d byte file", elfanew, len(pe))
	}
	if pe[elfanew] != 'P' || pe[elfanew+1] != 'E' || pe[elfanew+2] != 0 || pe[elfanew+3] != 0 {
		return 0, fmt.Errorf("reading the source, the offset %d does not carry the PE signature", elfanew)
	}
	if stamp+timeDateStampSize > len(pe) {
		return 0, fmt.Errorf("reading the source, the timestamp field at %d falls outside the %d byte file", stamp, len(pe))
	}
	return stamp, nil
}

func moveHash(pe []byte, stamp, attempt int) []byte {
	out := make([]byte, len(pe))
	copy(out, pe)
	out[stamp] = byte(int(out[stamp]) + attempt)
	return out
}

func shortSum(pe []byte) string {
	sum := md5.Sum(pe)
	return hex.EncodeToString(sum[:])[:8]
}

func candidateName(name, commit, sum string) string {
	return fmt.Sprintf("%s-%s-%s.exe", name, commit, sum)
}

func blocked(output string) bool {
	return strings.Contains(output, "Invalid argument")
}

func allowed(output, expect string) bool {
	return strings.Contains(output, expect)
}

func Allow(req Request, run Runner) (Outcome, error) {
	info, err := os.Stat(req.Destination)
	if err != nil {
		return Outcome{}, fmt.Errorf("reading the destination %q: %w", req.Destination, err)
	}
	if !info.IsDir() {
		return Outcome{}, fmt.Errorf("the destination %q is not a directory", req.Destination)
	}
	if req.Attempts < 1 {
		return Outcome{}, fmt.Errorf("the attempts count %d is below one", req.Attempts)
	}
	original, err := os.ReadFile(req.Source)
	if err != nil {
		return Outcome{}, fmt.Errorf("reading the source %q: %w", req.Source, err)
	}
	stamp, err := timeDateStampOffset(original)
	if err != nil {
		return Outcome{}, err
	}
	timeout := time.Duration(req.Probe.TimeoutSeconds) * time.Second

	var lastOutput string
	for attempt := 1; attempt <= req.Attempts; attempt++ {
		bytes := original
		if attempt > 1 {
			bytes = moveHash(original, stamp, attempt)
		}
		name := candidateName(req.Name, req.Commit, shortSum(bytes))
		path := filepath.Join(req.Destination, name)

		if _, err := os.Stat(path); err != nil {
			if err := os.WriteFile(path, bytes, 0o755); err != nil {
				return Outcome{}, fmt.Errorf("writing the candidate %q: %w", path, err)
			}
		}

		output, _ := run(req.Destination, name, req.Probe.Args, timeout)
		lastOutput = output

		if allowed(output, req.Probe.Expect) {
			marker := filepath.Join(req.Destination, req.Name+"-allowed.txt")
			if err := os.WriteFile(marker, []byte(name+"\n"), 0o644); err != nil {
				return Outcome{}, fmt.Errorf("writing the allowed marker %q: %w", marker, err)
			}
			keep := req.Keep
			if keep < 1 {
				keep = 1
			}
			pruned, err := pruneOldBuilds(req.Destination, req.Name, name, keep)
			if err != nil {
				return Outcome{}, err
			}
			return Outcome{AllowedName: name, AllowedPath: path, Attempts: attempt, Pruned: pruned}, nil
		}

		if blocked(output) {
			_ = os.Remove(path)
		}
	}

	return Outcome{}, fmt.Errorf("smart app control refused every one of %d builds of %q, run it again because roughly one in two clears, the last probe said %q", req.Attempts, req.Name, strings.TrimSpace(lastOutput))
}

func RealRunner(dir, exe string, args []string, timeout time.Duration) (string, error) {
	cmd := exec.Command(filepath.Join(dir, exe), args...)
	cmd.Dir = dir
	done := make(chan struct{})
	var out []byte
	var runErr error
	go func() {
		out, runErr = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
		return string(out), runErr
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return string(out), nil
	}
}
