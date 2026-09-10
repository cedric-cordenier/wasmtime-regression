package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	var (
		n       = flag.Int("n", 20, "number of unique wasm modules to load")
		guest   = flag.String("guest", "guest.wasm", "wasm module to make unique copies of")
		dir     = flag.String("modules", "modules", "directory for the module copies")
		threads = flag.String("threads", "4", "RAYON_NUM_THREADS in the container: wasmtime's compile parallelism")
		hold    = flag.Duration("hold", 30*time.Second, "how long the container holds every module loaded")
		image   = flag.String("image", "golang:1.26.4-bookworm", "image with a Go toolchain and glibc")
	)
	flag.Parse()

	if err := buildGuest(*guest); err != nil {
		fail(err)
	}
	if err := writeModules(*guest, *dir, *n); err != nil {
		fail(err)
	}
	fmt.Printf("%d unique modules in %s/\nthreads=%s hold=%s\n\n", *n, *dir, *threads, *hold)

	for _, v := range []string{"47", "48"} {
		if err := measure(v, *dir, *threads, *image, *hold); err != nil {
			fail(err)
		}
	}
}

func buildGuest(guest string) error {
	if st, err := os.Stat(guest); err == nil && st.Size() > 0 {
		return nil
	}

	out, err := filepath.Abs(guest)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = "guest"
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "GOFLAGS=")
	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("building guest: %v: %s", err, o)
	}
	return nil
}

func writeModules(guest, dir string, n int) error {
	wasm, err := os.ReadFile(guest)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Stale copies from a larger -n would otherwise still be loaded.
	stale, err := filepath.Glob(filepath.Join(dir, "*.wasm"))
	if err != nil {
		return err
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	for i := range n {
		name := fmt.Sprintf("uniq%08d", i)
		section := append([]byte{0x00, byte(len(name) + 1), byte(len(name))}, name...)

		out := filepath.Join(dir, fmt.Sprintf("module-%04d.wasm", i))
		if err := os.WriteFile(out, append(wasm, section...), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func measure(version, dir, threads, image string, hold time.Duration) error {
	loader, err := filepath.Abs("loader" + version)
	if err != nil {
		return err
	}
	modules, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	gomodcache, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		return err
	}

	name := "wasmtime-mem-v" + version
	_ = exec.Command("docker", "rm", "-f", name).Run()

	out, err := exec.Command("docker", "run", "-d", "--name", name,
		"-e", "RAYON_NUM_THREADS="+threads,
		"-e", "GOPROXY=off", "-e", "GOTOOLCHAIN=local", "-e", "HOME=/tmp",
		"-e", "GOMODCACHE=/hostmod", "-e", "GOCACHE=/tmp/gocache",
		"-v", loader+":/src:ro",
		"-v", modules+":/modules:ro",
		"-v", strings.TrimSpace(string(gomodcache))+":/hostmod:ro",
		"-w", "/src", image,
		"go", "run", ".", "/modules", hold.String(),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("v%s docker run: %v: %s", version, err, out)
	}
	defer exec.Command("docker", "rm", "-f", name).Run()

	loaded, err := waitForLoad(name)
	if err != nil {
		return fmt.Errorf("v%s: %w", version, err)
	}

	mem, err := exec.Command("docker", "stats", "--no-stream", "--format", "{{.MemUsage}}", name).Output()
	if err != nil {
		return fmt.Errorf("v%s docker stats: %w", version, err)
	}

	fmt.Printf("v%s  %s  docker mem %s\n", version, strings.TrimSpace(loaded), strings.TrimSpace(string(mem)))
	return nil
}

func waitForLoad(name string) (string, error) {
	deadline := time.Now().Add(20 * time.Minute)

	for time.Now().Before(deadline) {
		logs, _ := exec.Command("docker", "logs", name).CombinedOutput()
		for _, line := range strings.Split(string(logs), "\n") {
			if strings.HasPrefix(line, "LOADED") {
				return line, nil
			}
		}

		running, _ := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", name).Output()
		if strings.TrimSpace(string(running)) != "true" {
			return "", fmt.Errorf("container exited before loading:\n%s", logs)
		}

		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("timed out waiting for modules to load")
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
