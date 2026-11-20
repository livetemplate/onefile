package logviewer

// End-to-end test of the log viewer: builds tinkerdown from source, serves a
// copy of this app, and drives it in Docker Chrome. Set TINKERDOWN_SRC to the
// tinkerdown checkout to test against (default ../tinkerdown), or TINKERDOWN_BIN
// to a built binary to skip the build.

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		src = "../tinkerdown"
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

// copyApp copies the viewer into a temp dir, so the test can append to its
// log file without touching the repo.
func copyApp(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	for _, p := range []string{"index.md", "tinkerdown.yaml", "scripts/demo-logs.sh", "logs/app.log"} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dst, p)
		os.MkdirAll(filepath.Dir(target), 0o755)
		if err := os.WriteFile(target, data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// serve runs `tinkerdown serve --allow-exec` on dir and returns its URL and log.
func serve(t *testing.T, bin, dir string) (string, *syncBuffer) {
	t.Helper()
	port := freePort(t)
	logs := &syncBuffer{}
	cmd := exec.Command(bin, "serve", "--allow-exec", "--port", fmt.Sprint(port), dir)
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
	name := fmt.Sprintf("chrome-e2e-logviewer-%d", port)
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

func TestLogViewer(t *testing.T) {
	bin := buildTinkerdown(t)
	dir := copyApp(t)
	url, serverLog := serve(t, bin, dir)
	ctx := dockerChrome(t)
	logs := &browserLogs{}
	logs.listen(ctx)

	run := func(actions ...chromedp.Action) error {
		actx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		return chromedp.Run(actx, actions...)
	}
	dump := func() {
		var html string
		run(chromedp.OuterHTML("main", &html, chromedp.ByQuery))
		logs.mu.Lock()
		defer logs.mu.Unlock()
		t.Logf("console:\n%s", strings.Join(logs.console, "\n"))
		t.Logf("ws sent:\n%s", strings.Join(logs.sent, "\n"))
		t.Logf("ws received: %d frames, last: %s", len(logs.recv), strings.Join(logs.recv[max(0, len(logs.recv)-3):], "\n"))
		t.Logf("rendered html:\n%.4000s", html)
		s := serverLog.String()
		t.Logf("server log tail:\n%s", s[max(0, len(s)-3000):])
	}
	eval := func(js string, out interface{}) {
		t.Helper()
		if err := run(chromedp.Evaluate(js, out)); err != nil {
			dump()
			t.Fatalf("evaluate %s: %v", js, err)
		}
	}
	waitJS := func(what, js string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			var ok bool
			if run(chromedp.Evaluate(js, &ok)) == nil && ok {
				return
			}
			time.Sleep(150 * time.Millisecond)
		}
		dump()
		t.Fatalf("timed out waiting for %s (%s)", what, js)
	}
	exec := func(js string) {
		t.Helper()
		if err := run(chromedp.Evaluate(js, nil)); err != nil {
			dump()
			t.Fatalf("evaluate %s: %v", js, err)
		}
	}
	// everyRow is a JS check that the list has rows and all satisfy pred (row r).
	everyRow := func(pred string) string {
		return fmt.Sprintf(`(() => { const li = [...document.querySelectorAll("ol.log li")]; return li.length > 0 && li.every(r => %s); })()`, pred)
	}
	// setLevel picks a level as a user would. chromedp.SetValue("") leaves a
	// select with no option chosen, a state no user can reach.
	setLevel := func(v string) chromedp.Action {
		return chromedp.Evaluate(fmt.Sprintf(`(() => { const s = document.querySelector('[data-testid="level"]');
			s.value = %q; s.dispatchEvent(new Event("change", {bubbles: true})); })()`, v), nil)
	}
	shot := func(name string) {
		var buf []byte
		if err := run(chromedp.FullScreenshot(&buf, 90)); err == nil {
			path := filepath.Join(os.TempDir(), "logviewer-"+name+".png")
			os.WriteFile(path, buf, 0o644)
			t.Logf("screenshot: %s", path)
		}
	}

	// The first Run creates the tab and ties it to its context, so it must use
	// the long-lived ctx: a Run on a timeout context would close the tab.
	if err := chromedp.Run(ctx, network.Enable(), chromedp.EmulateViewport(1280, 900), chromedp.Navigate(url),
		chromedp.WaitVisible(`[data-testid="log"]`, chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}

	t.Run("the All tab interleaves every source", func(t *testing.T) {
		waitJS("rows from web, worker and app", `["web","worker","app"].every(s => document.querySelector("li.src-" + s))`)
		// Rows stay in arrival order across sources: web and worker alternate.
		waitJS("interleaved web and worker rows", `(() => {
			const srcs = [...document.querySelectorAll("ol.log li")].map(li => li.className.match(/src-(\w+)/)[1]).filter(s => s !== "app");
			let switches = 0; for (let i = 1; i < srcs.length; i++) if (srcs[i] !== srcs[i-1]) switches++;
			return switches >= 3; })()`)
		var ordered bool
		eval(`(() => { const k = [...document.querySelectorAll("ol.log li")].map(li => +li.dataset.key);
			return k.every((v, i) => i === 0 || v > k[i-1]); })()`, &ordered)
		if !ordered {
			dump()
			t.Fatal("rows are not in seq order")
		}
	})

	t.Run("stderr rows are marked", func(t *testing.T) {
		waitJS("a stderr row", `!!document.querySelector("li.stream-stderr")`)
		var text string
		eval(`document.querySelector("li.stream-stderr .std").textContent`, &text)
		if text != "stderr" {
			t.Fatalf("stderr marker = %q", text)
		}
	})

	t.Run("appending to the log file shows the line", func(t *testing.T) {
		f, err := os.OpenFile(filepath.Join(dir, "logs/app.log"), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(f, `{"level":"error","msg":"disk almost full"}`)
		f.Close()
		waitJS("the appended line", `[...document.querySelectorAll("li.src-app .msg")].some(m => m.textContent === "disk almost full")`)
	})

	t.Run("follows the tail, and stops following when scrolled up", func(t *testing.T) {
		atBottom := `(() => { const l = document.querySelector("ol.log"); return l.scrollHeight - l.scrollTop - l.clientHeight < 40; })()`
		waitJS("enough rows to scroll", `(() => { const l = document.querySelector("ol.log"); return l.scrollHeight > l.clientHeight + 200; })()`)
		waitJS("list pinned to the bottom", atBottom)
		var count int
		eval(`document.querySelectorAll("ol.log li").length`, &count)
		waitJS("more rows", fmt.Sprintf(`document.querySelectorAll("ol.log li").length > %d`, count))
		waitJS("still pinned after new rows", atBottom)

		exec(`document.querySelector("ol.log").scrollTop = 0`)
		eval(`document.querySelectorAll("ol.log li").length`, &count)
		waitJS("more rows while scrolled up", fmt.Sprintf(`document.querySelectorAll("ol.log li").length > %d`, count))
		var top float64
		eval(`document.querySelector("ol.log").scrollTop`, &top)
		if top > 40 {
			t.Fatalf("scrolled-up reader was moved to %v", top)
		}
		exec(`document.querySelector("ol.log").scrollTop = 1e9`)
	})

	t.Run("level filter still applies after switching to a source tab", func(t *testing.T) {
		if err := run(setLevel("warn")); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS("only warn rows", everyRow(`r.classList.contains("lvl-warn")`))
		if err := run(chromedp.Click(`button[role="tab"][data-filter="source = worker"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS("only worker warn rows", everyRow(`r.classList.contains("lvl-warn") && r.classList.contains("src-worker")`))
		shot("desktop-filtered")
	})

	t.Run("search narrows the rows", func(t *testing.T) {
		if err := run(chromedp.Click(`button[role="tab"][data-filter=""]`, chromedp.ByQuery),
			setLevel(""),
			chromedp.SendKeys(`[data-testid="search"]`, "failed job", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS("only matching rows", everyRow(`r.textContent.includes("failed job")`))
	})

	t.Run("phone layout", func(t *testing.T) {
		if err := run(chromedp.EmulateViewport(390, 844)); err != nil {
			t.Fatal(err)
		}
		var overflow bool
		eval(`document.documentElement.scrollWidth > window.innerWidth + 1`, &overflow)
		shot("phone")
		if overflow {
			t.Fatal("page scrolls horizontally at phone width")
		}
	})

	t.Run("filters went over the websocket", func(t *testing.T) {
		logs.mu.Lock()
		defer logs.mu.Unlock()
		if !strings.Contains(strings.Join(logs.sent, "\n"), `"action":"Filter"`) {
			t.Fatalf("no Filter action in the sent frames: %q", logs.sent)
		}
	})
}
