package e2e

// Shared helpers for the onefile apps' end-to-end tests: build tinkerdown,
// serve an app with it, and drive the page in Docker Chrome. Set
// TINKERDOWN_SRC to the tinkerdown checkout to test against (default
// ../../tinkerdown, a sibling of this repo), or TINKERDOWN_BIN to a built
// binary to skip the build. Relative paths are read from this e2e directory,
// where `go test` runs.

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// buildTinkerdown returns TINKERDOWN_BIN, or compiles the tinkerdown CLI from
// TINKERDOWN_SRC. The build embeds that checkout's committed client bundle, so
// client changes need `make build-client` there first.
func buildTinkerdown(t *testing.T) string {
	t.Helper()
	if bin := os.Getenv("TINKERDOWN_BIN"); bin != "" {
		return bin
	}
	src := os.Getenv("TINKERDOWN_SRC")
	if src == "" {
		src = "../../tinkerdown"
	}
	bin := filepath.Join(t.TempDir(), "tinkerdown")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/tinkerdown")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build tinkerdown from %s: %v\n%s", src, err, out)
	}
	return bin
}

// serve runs `tinkerdown serve --allow-exec` on dir with the given flags and
// returns its URL and log.
func serve(t *testing.T, bin, dir string, flags ...string) (string, *syncBuffer) {
	t.Helper()
	port := freePort(t)
	logs := &syncBuffer{}
	cmd := exec.Command(bin, append([]string{"serve", "--allow-exec", "--port", fmt.Sprint(port), dir}, flags...)...)
	cmd.Stdout, cmd.Stderr = logs, logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			cmd.Process.Kill()
		}
	})
	url := fmt.Sprintf("http://localhost:%d", port)
	for i := 0; i < 100; i++ {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return url, logs
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("server did not start:\n%s", logs.String())
	return "", nil
}

// dockerChrome starts headless Chrome in Docker on the host network.
func dockerChrome(t *testing.T) context.Context {
	t.Helper()
	if out, err := exec.Command("docker", "version").CombinedOutput(); err != nil {
		t.Fatalf("docker is required for the e2e test: %v\n%s", err, out)
	}
	port := freePort(t)
	name := fmt.Sprintf("chrome-e2e-onefile-%d", port)
	if out, err := exec.Command("docker", "run", "-d", "--rm", "--name", name, "--network", "host",
		"chromedp/headless-shell:stable", fmt.Sprintf("--remote-debugging-port=%d", port)).CombinedOutput(); err != nil {
		t.Fatalf("start chrome: %v\n%s", err, out)
	}
	t.Cleanup(func() { exec.Command("docker", "rm", "-f", name).Run() })
	for i := 0; i < 120; i++ {
		if resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/version", port)); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	alloc, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), fmt.Sprintf("http://localhost:%d", port))
	ctx, cancelCtx := chromedp.NewContext(alloc)
	ctx, cancelTimeout := context.WithTimeout(ctx, 3*time.Minute)
	t.Cleanup(func() { cancelTimeout(); cancelCtx(); cancelAlloc() })
	return ctx
}

type browserLogs struct {
	mu      sync.Mutex
	console []string
	recv    []string
	sent    []string
}

func (l *browserLogs) listen(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		l.mu.Lock()
		defer l.mu.Unlock()
		switch e := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, a := range e.Args {
				l.console = append(l.console, fmt.Sprintf("[%s] %s", e.Type, a.Value))
			}
		case *runtime.EventExceptionThrown:
			l.console = append(l.console, "[exception] "+e.ExceptionDetails.Text)
		case *network.EventWebSocketFrameReceived:
			l.recv = append(l.recv, e.Response.PayloadData)
		case *network.EventWebSocketFrameSent:
			l.sent = append(l.sent, e.Response.PayloadData)
		}
	})
}
