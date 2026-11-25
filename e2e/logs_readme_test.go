package e2e

// End-to-end test of the logs README's command, run as written: the page by
// its short name (served locally), a log file, and `docker logs` after "--".
// A fake docker on PATH prints two lines, so no container is needed.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestLogsREADMECommand(t *testing.T) {
	bin := buildTinkerdown(t)
	page, err := os.ReadFile("../logs/index.md")
	if err != nil {
		t.Fatal(err)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(page) }))
	defer pages.Close()

	dir, tools := t.TempDir(), t.TempDir()
	logFile := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logFile, []byte("app one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeTool(t, tools, "docker", `printf '%s\n' "api one" "api two"; exec tail -f /dev/null`)

	port := freePort(t)
	cmdline := readmeCommandOf(t, "../logs/README.md")
	cmdline = replaceOnce(t, cmdline, "tinkerdown serve", bin+" serve --port "+fmt.Sprint(port))
	cmdline = replaceOnce(t, cmdline, "github.com/livetemplate/onefile/logs", pages.URL+"/index.md")
	cmdline = replaceOnce(t, cmdline, "/var/log/app.log", logFile)
	cmd := exec.Command("sh", "-c", "exec "+cmdline)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+tools+":"+os.Getenv("PATH"))
	url, serverLog := serveCmd(t, cmd, port)

	ctx := dockerChrome(t)
	logs := &browserLogs{}
	logs.listen(ctx)
	run := func(actions ...chromedp.Action) error {
		actx, cancel := chromedpTimeout(ctx)
		defer cancel()
		return chromedp.Run(actx, actions...)
	}
	dump := func() {
		var html string
		run(chromedp.OuterHTML("main", &html, chromedp.ByQuery))
		logs.mu.Lock()
		defer logs.mu.Unlock()
		t.Logf("command: %s", cmdline)
		t.Logf("console:\n%s", strings.Join(logs.console, "\n"))
		t.Logf("ws received: %d frames, last: %s", len(logs.recv), strings.Join(logs.recv[max(0, len(logs.recv)-3):], "\n"))
		t.Logf("rendered html:\n%.4000s", html)
		s := serverLog.String()
		t.Logf("server log tail:\n%s", s[max(0, len(s)-3000):])
	}
	// The first Run creates the tab and ties it to its context, so it uses
	// the long-lived ctx, as TestLogViewer does.
	if err := chromedp.Run(ctx, network.Enable(), chromedp.Navigate(url), chromedp.WaitVisible("ol.log", chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}
	var tabs string
	if err := run(chromedp.Evaluate(`[...document.querySelectorAll('button[role="tab"]')].map(b => b.textContent.trim()).join("|")`, &tabs)); err != nil || tabs != "All|api|app" {
		dump()
		t.Fatalf("tabs = %q (err %v), want All|api|app: the README's sources, labelled from the command and the file", tabs, err)
	}
	rows := `document.querySelectorAll("li.src-api").length === 2 && document.querySelectorAll("li.src-app").length === 1`
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		var ok bool
		if run(chromedp.Evaluate(rows, &ok)) == nil && ok {
			break
		}
		if time.Now().After(deadline) {
			dump()
			t.Fatal("want two api rows and one app row")
		}
	}
	logs.mu.Lock()
	defer logs.mu.Unlock()
	for _, line := range logs.console {
		if strings.Contains(strings.ToLower(line), "error") {
			t.Errorf("browser console error: %s", line)
		}
	}
}
