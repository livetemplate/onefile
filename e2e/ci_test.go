package e2e

// End-to-end test of the CI app (../ci): runs the serve command from its
// README through bash, as a user pastes it, with a fake gh first on PATH that
// answers from fixture files (testdata/ci/gh) and never calls GitHub. The
// page is served from a URL and driven in Docker Chrome.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// ghRun is one run as `gh run list --json` prints it.
type ghRun struct {
	DatabaseID   int64  `json:"databaseId"`
	Number       int    `json:"number"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	WorkflowName string `json:"workflowName"`
	DisplayTitle string `json:"displayTitle"`
	HeadBranch   string `json:"headBranch"`
	Event        string `json:"event"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	URL          string `json:"url"`
	Attempt      int    `json:"attempt"`
}

func TestCI(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Fatal("TestCI needs jq: the fake gh applies the commands' --jq with it")
	}
	bin := buildTinkerdown(t)
	pageMD, err := os.ReadFile("../ci/index.md")
	if err != nil {
		t.Fatal(err)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(pageMD) }))
	defer pages.Close()

	ghDir, _ := filepath.Abs("testdata/ci")
	fixtures := t.TempDir()
	write := func(name string, v interface{}) {
		t.Helper()
		var b []byte
		if s, ok := v.(string); ok {
			b = []byte(s)
		} else {
			b, _ = json.Marshal(v)
		}
		if err := os.WriteFile(filepath.Join(fixtures, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(repo string, id int64, n int, status, conclusion, title, created string, attempt int) ghRun {
		return ghRun{DatabaseID: id, Number: n, Status: status, Conclusion: conclusion, WorkflowName: "CI", DisplayTitle: title,
			HeadBranch: "main", Event: "push", CreatedAt: created, UpdatedAt: created,
			URL: fmt.Sprintf("https://github.com/%s/actions/runs/%d", repo, id), Attempt: attempt}
	}
	const xss = `<img src=x onerror=alert(1)>`
	apiRuns := func(r13status, r13conclusion string, r12 ghRun) []ghRun {
		return []ghRun{
			run("acme/api", 1003, 13, r13status, r13conclusion, "add search", "2026-11-22T12:00:00Z", 1),
			r12,
			run("acme/api", 1001, 11, "completed", "success", xss, "2026-11-22T10:00:00Z", 1),
		}
	}
	r12 := run("acme/api", 1002, 12, "completed", "failure", "deploy api", "2026-11-22T11:00:00Z", 1)
	write("runs-acme_api.json", apiRuns("queued", "", r12))
	write("runs-acme_web.json", []ghRun{run("acme/web", 2001, 21, "completed", "failure", "web build", "2026-11-22T09:00:00Z", 1)})
	write("view-1002.json", map[string]interface{}{"status": "completed", "conclusion": "failure", "jobs": []map[string]interface{}{
		{"name": "build", "status": "completed", "conclusion": "success", "steps": []map[string]string{{"name": "Build", "conclusion": "success"}}},
		{"name": "test", "status": "completed", "conclusion": "failure", "steps": []map[string]string{{"name": "Run tests", "conclusion": "failure"}}},
	}})
	write("view-1001.json", map[string]interface{}{"status": "completed", "conclusion": "success", "jobs": []map[string]interface{}{
		{"name": "build", "status": "completed", "conclusion": "success", "steps": []map[string]string{}},
	}})
	var log strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&log, "test\tUNKNOWN STEP\t2026-11-22T11:00:%02d.0000000Z setup line %d\n", i%60, i)
	}
	log.WriteString("test\tUNKNOWN STEP\t2026-11-22T11:00:59.0000000Z \x1b[36;1mgo test ./...\x1b[0m\n")
	log.WriteString("test\tUNKNOWN STEP\t2026-11-22T11:01:00.0000000Z --- FAIL: TestDeploy (0.01s)\n")
	log.WriteString("test\tUNKNOWN STEP\t2026-11-22T11:01:00.1000000Z    deploy_test.go:9: got <script>alert(1)</script>\n")
	log.WriteString("test\tUNKNOWN STEP\t2026-11-22T11:01:00.2000000Z ##[error]Process completed with exit code 1.\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&log, "test\tUNKNOWN STEP\t2026-11-22T11:01:01.0000000Z Post job cleanup %d\n", i)
	}
	write("log-1002.txt", log.String())
	write("log-1003.txt", log.String())
	write("view-1003.json", map[string]interface{}{"status": "completed", "conclusion": "failure", "jobs": []map[string]interface{}{
		{"name": "test", "status": "completed", "conclusion": "failure", "steps": []map[string]string{{"name": "Run tests", "conclusion": "failure"}}},
	}})
	calls := func() string {
		b, _ := os.ReadFile(filepath.Join(fixtures, "calls.txt"))
		return string(b)
	}

	// The README's command, with this test's binary, port and page URL, the
	// placeholder repo filled in, a second repo, and one-second polls.
	port := freePort(t)
	cmdline := readmeCommandOf(t, "../ci/README.md")
	cmdline = replaceOnce(t, cmdline, "tinkerdown serve", bin+" serve --port "+fmt.Sprint(port))
	cmdline = replaceOnce(t, cmdline, "https://raw.githubusercontent.com/livetemplate/onefile/main/ci/index.md", pages.URL+"/index.md")
	cmdline = replaceOnce(t, cmdline, "'myapp=", "'api=")
	cmdline = replaceOnce(t, cmdline, "OWNER/REPO", "acme/api")
	feed := regexp.MustCompile(`--source 'api=[^']*'`).FindString(cmdline)
	cmdline += " " + strings.NewReplacer("'api=", "'web=", "acme/api", "acme/web").Replace(feed)
	if n := strings.Count(cmdline, "sleep 15"); n != 2 {
		t.Fatalf("want sleep 15 once per repo, found %d", n)
	}
	if n := strings.Count(cmdline, "sleep 10"); n != 2 {
		t.Fatalf("want sleep 10 twice (the jobs loop), found %d", n)
	}
	if n := strings.Count(cmdline, "sleep 30"); n != 1 {
		t.Fatalf("want sleep 30 once (the log's wait), found %d", n)
	}
	cmdline = strings.NewReplacer("sleep 15", "sleep 1", "sleep 10", "sleep 1", "sleep 30", "sleep 1").Replace(cmdline)
	srv := exec.Command("bash", "-c", "exec "+cmdline)
	srv.Env = append(os.Environ(), "PATH="+ghDir+":"+os.Getenv("PATH"), "FAKE_GH_DIR="+fixtures)
	url, serverLog := serveCmd(t, srv, port)

	ctx := dockerChrome(t)
	logs := &browserLogs{}
	logs.listen(ctx)
	var acceptDialog atomic.Bool
	dialogClosed := make(chan struct{}, 4)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev.(type) {
		case *page.EventJavascriptDialogOpening:
			go chromedp.Run(ctx, page.HandleJavaScriptDialog(acceptDialog.Load()))
		case *page.EventJavascriptDialogClosed:
			dialogClosed <- struct{}{}
		}
	})
	runCtx := func(actions ...chromedp.Action) error {
		actx, cancel := chromedpTimeout(ctx)
		defer cancel()
		return chromedp.Run(actx, actions...)
	}
	dump := func() {
		var html string
		runCtx(chromedp.OuterHTML(".content-wrapper", &html, chromedp.ByQuery))
		logs.mu.Lock()
		defer logs.mu.Unlock()
		t.Logf("console:\n%s", strings.Join(logs.console, "\n"))
		t.Logf("ws sent: %s", strings.Join(logs.sent[max(0, len(logs.sent)-5):], "\n"))
		t.Logf("ws received: %d frames, last: %s", len(logs.recv), strings.Join(logs.recv[max(0, len(logs.recv)-3):], "\n"))
		t.Logf("rendered html:\n%.5000s", html)
		s := serverLog.String()
		t.Logf("server log tail:\n%s", s[max(0, len(s)-4000):])
		t.Logf("gh calls:\n%s", calls())
	}
	waitJS := func(t *testing.T, what, js string) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			var ok bool
			if runCtx(chromedp.Evaluate(js, &ok)) == nil && ok {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		dump()
		t.Fatalf("timed out waiting for %s (%s)", what, js)
	}
	eval := func(t *testing.T, js string, out interface{}) {
		t.Helper()
		if err := runCtx(chromedp.Evaluate(js, out)); err != nil {
			dump()
			t.Fatalf("evaluate %s: %v", js, err)
		}
	}
	const numbers = `[...document.querySelectorAll("ol.runs li:not(.stream-stderr) .run")].map(a => a.textContent).join(",")`
	row := func(n int) string {
		return fmt.Sprintf(`[...document.querySelectorAll("ol.runs li")].find(li => li.querySelector(".run")?.textContent === "#%d")`, n)
	}
	buttons := func(t *testing.T, n int) string {
		t.Helper()
		var got string
		eval(t, `[...`+row(n)+`.querySelectorAll("button")].filter(b => b.offsetParent).map(b => b.textContent).join(",")`, &got)
		return got
	}
	click := func(t *testing.T, n int, sel string) {
		t.Helper()
		if err := runCtx(chromedp.Click(row(n)+`.querySelector('`+sel+`')`, chromedp.ByJSPath)); err != nil {
			dump()
			t.Fatalf("click %s on #%d: %v", sel, n, err)
		}
	}
	logCount := func(t *testing.T) int {
		t.Helper()
		var n int
		eval(t, `document.querySelectorAll('[data-testid="log"] li').length`, &n)
		return n
	}
	waitCall := func(t *testing.T, want string) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); !strings.Contains(calls(), want); time.Sleep(100 * time.Millisecond) {
			if time.Now().After(deadline) {
				dump()
				t.Fatalf("gh never ran %q", want)
			}
		}
	}

	if err := chromedp.Run(ctx, network.Enable(), chromedp.EmulateViewport(1280, 900), chromedp.Navigate(url),
		chromedp.WaitVisible(`[data-testid="runs"]`, chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}

	t.Run("runs from both repos, newest first, with titles as text", func(t *testing.T) {
		waitJS(t, "four runs, newest first", numbers+` === "#13,#12,#11,#21"`)
		var title string
		var imgs int
		eval(t, row(11)+`.querySelector(".title").textContent`, &title)
		eval(t, `document.querySelectorAll("ol.runs img").length`, &imgs)
		if title != xss || imgs != 0 {
			t.Fatalf("an HTML title must render as text: title %q, img elements %d", title, imgs)
		}
		var meta string
		eval(t, row(21)+`.querySelector(".meta").textContent`, &meta)
		if !strings.Contains(meta, "web") {
			t.Fatalf("a run from the second repo should name it: %q", meta)
		}
	})

	t.Run("each run shows only the buttons that fit it", func(t *testing.T) {
		for n, want := range map[int]string{13: "Cancel", 12: "Rerun failed", 11: ""} {
			if got := buttons(t, n); got != want {
				dump()
				t.Fatalf("#%d shows %q, want %q", n, got, want)
			}
		}
	})

	t.Run("tabs and search narrow the list", func(t *testing.T) {
		for filter, want := range map[string]string{
			"fields.status != completed":  "#13",
			"fields.conclusion = failure": "#12,#21",
			"fields.conclusion = success": "#11",
		} {
			if err := runCtx(chromedp.Click(`button[role="tab"][data-filter="`+filter+`"]`, chromedp.ByQuery)); err != nil {
				dump()
				t.Fatal(err)
			}
			waitJS(t, filter, numbers+` === "`+want+`"`)
		}
		if err := runCtx(chromedp.Click(`button[role="tab"][data-filter=""]`, chromedp.ByQuery),
			chromedp.SendKeys(`[data-testid="search"]`, "deploy", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "only the deploy run", numbers+` === "#12"`)
		if err := runCtx(chromedp.Evaluate(`(i => { i.value = ""; i.dispatchEvent(new Event("input", {bubbles: true})); })(document.querySelector('[data-testid="search"]'))`, nil)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "all runs back", numbers+` === "#13,#12,#11,#21"`)
	})

	t.Run("a queued run turns running, then failed, without a page load", func(t *testing.T) {
		runCtx(chromedp.Evaluate(`window.__samePage = true`, nil))
		write("runs-acme_api.json", apiRuns("in_progress", "", r12))
		waitJS(t, "#13 running", row(13)+`?.classList.contains("st-in_progress")`)
		write("runs-acme_api.json", apiRuns("completed", "failure", r12))
		waitJS(t, "#13 failed", row(13)+`?.classList.contains("c-failure")`)
		if got := buttons(t, 13); got != "Rerun failed" {
			t.Fatalf("a failed run shows %q", got)
		}
	})

	t.Run("a run's number shows its jobs and the failing log", func(t *testing.T) {
		click(t, 12, `.run`)
		waitJS(t, "the jobs", `[...document.querySelectorAll('[data-testid="jobs"] li')].map(l => l.textContent.trim()).join("|") === "build|test Run tests"`)
		waitJS(t, "the failed log", `[...document.querySelectorAll('[data-testid="log"] li')].some(l => l.textContent.includes("##[error]Process completed"))`)
		var title, logText string
		var lines, scripts int
		eval(t, `document.querySelector('[data-testid="pane-title"]').textContent`, &title)
		eval(t, `[...document.querySelectorAll('[data-testid="log"] li')].map(l => l.textContent).join("\n")`, &logText)
		eval(t, `document.querySelectorAll('[data-testid="log"] li').length`, &lines)
		eval(t, `document.querySelectorAll('[data-testid="log"] script').length`, &scripts)
		if title != "Run 1002" {
			t.Fatalf("pane title %q", title)
		}
		if !strings.Contains(logText, "got <script>alert(1)</script>") || scripts != 0 {
			t.Fatalf("log HTML must render as text (script elements: %d)", scripts)
		}
		if strings.Contains(logText, "\x1b") || !strings.Contains(logText, "go test ./...") {
			t.Fatalf("the log should keep the text and drop terminal colour codes:\n%q", logText)
		}
		if strings.Contains(logText, "UNKNOWN STEP") || strings.Contains(logText, "Post job cleanup") || !strings.HasPrefix(logText, "test\t") {
			t.Fatalf("the log should keep the job, drop the step column and the cleanup after the error:\n%s", logText)
		}
		if lines != 31 {
			t.Fatalf("the error and the 30 lines before it: %d lines", lines)
		}
		var same bool
		eval(t, `window.__samePage === true`, &same)
		if !same {
			t.Fatal("opening a run loaded the page again")
		}
	})

	t.Run("Rerun runs gh, and the rerun shows within one poll", func(t *testing.T) {
		click(t, 12, `button[name="rerun"]`)
		waitCall(t, "rerun 1002 --failed --repo acme/api")
		r12b := r12
		r12b.Status, r12b.Conclusion, r12b.Attempt = "queued", "", 2
		write("runs-acme_api.json", apiRuns("completed", "failure", r12b))
		waitJS(t, "#12 queued again", row(12)+`?.classList.contains("st-queued")`)
		waitJS(t, "the open pane on attempt 1 empties", `document.querySelectorAll('[data-testid="jobs"] li, [data-testid="log"] li').length === 0`)
		click(t, 21, `button[name="rerun"]`)
		waitCall(t, "rerun 2001 --failed --repo acme/web")
	})

	t.Run("Cancel asks first", func(t *testing.T) {
		acceptDialog.Store(false)
		click(t, 12, `button[name="cancel"]`)
		select {
		case <-dialogClosed:
		case <-time.After(10 * time.Second):
			dump()
			t.Fatal("Cancel ran without asking")
		}
		runCtx(chromedp.Evaluate(`0`, nil))
		if strings.Contains(calls(), "cancel") {
			t.Fatal("cancelling the dialog still cancelled the run")
		}
		acceptDialog.Store(true)
		click(t, 12, `button[name="cancel"]`)
		waitCall(t, "cancel 1002 --repo acme/api")
	})

	t.Run("hand-made action values run nothing", func(t *testing.T) {
		// The page shows each refusal as the block's error, but the next
		// stream push clears it within a second (G5 in the ci plan), so the
		// server log is what this checks.
		before := calls()
		for _, c := range []struct{ data, want string }{
			{`{run: "12;x", repo: "acme/api"}`, "not a run ID"},
			{`{run: "1002", repo: "a b"}`, "not a repo"},
			{`{run: "1002", repo: "a//b"}`, "not a repo"},
			{`{run: "1002", repo: "-x/y"}`, "starts with -"},
		} {
			seen := strings.Count(serverLog.String(), c.want)
			eval(t, `(() => { const id = document.querySelector(".runs").closest("[data-block-id]").dataset.blockId; window.tinkerdownClient.send(id, "rerun", `+c.data+`); return true; })()`, new(bool))
			for deadline := time.Now().Add(10 * time.Second); strings.Count(serverLog.String(), c.want) == seen; time.Sleep(100 * time.Millisecond) {
				if time.Now().After(deadline) {
					dump()
					t.Fatalf("no %q refusal for %s", c.want, c.data)
				}
			}
		}
		if calls() != before {
			t.Fatalf("a bad value reached gh:\n%s", strings.TrimPrefix(calls(), before))
		}
	})

	t.Run("a failed poll shows gh's message until the next good one", func(t *testing.T) {
		write("fail", "")
		waitJS(t, "the error row", `[...document.querySelectorAll("ol.runs li.stream-stderr")].some(li => li.textContent.includes("HTTP 403: API rate limit exceeded Try again later."))`)
		var errRows int
		eval(t, `[...document.querySelectorAll("ol.runs li.stream-stderr")].filter(li => li.textContent.includes("api:")).length`, &errRows)
		if errRows != 1 {
			t.Fatalf("want one error row for the repo, got %d", errRows)
		}
		os.Remove(filepath.Join(fixtures, "fail"))
		waitJS(t, "the error row gone", `document.querySelectorAll("ol.runs li.stream-stderr").length === 0`)
		waitJS(t, "the runs still listed", numbers+` === "#13,#12,#11,#21"`)
	})

	t.Run("no command from an earlier run lingers, and reopening a run shows its log once", func(t *testing.T) {
		click(t, 13, `.run`)
		waitJS(t, "#13's log", `document.querySelectorAll('[data-testid="log"] li').length === 31`)
		click(t, 11, `.run`)
		waitJS(t, "#11's jobs", `[...document.querySelectorAll('[data-testid="jobs"] li')].map(l => l.textContent.trim()).join("|") === "build"`)
		// The log command has seen #11 complete and pass: it shows nothing,
		// and doesn't fetch a log.
		waitCall(t, "view 1001 --repo acme/api --attempt 1 --json status,conclusion")
		time.Sleep(500 * time.Millisecond)
		if n := logCount(t); n != 0 {
			dump()
			t.Fatalf("a passing run shows %d log lines, want none", n)
		}
		if strings.Contains(calls(), "view 1001 --repo acme/api --attempt 1 --log-failed") {
			t.Fatal("a passing run's log was fetched")
		}
		// #13's commands stop 30s after its pane closed: only #11's two
		// tail processes remain, and no fake gh call is left running.
		var procs []string
		for deadline := time.Now().Add(45 * time.Second); ; time.Sleep(time.Second) {
			out, _ := exec.Command("ps", "-eo", "args").Output()
			procs = procs[:0]
			tails := 0
			for _, l := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(l) == "tail -f /dev/null" {
					tails++ // a command's own text also holds the words, so match whole
				} else if f := strings.Fields(l); len(f) > 1 && strings.HasSuffix(f[0], "bash") && strings.HasSuffix(f[1], "/testdata/ci/gh") {
					procs = append(procs, l)
				}
			}
			if tails == 2 && len(procs) == 0 {
				break
			}
			if time.Now().After(deadline) {
				dump()
				t.Fatalf("%d tail processes (want #11's two); fake gh still running: %v", tails, procs)
			}
		}
		// #13's commands have stopped; opening it again runs them again,
		// and its log must not show twice.
		click(t, 13, `.run`)
		const fetch13 = "view 1003 --repo acme/api --attempt 1 --log-failed"
		for deadline := time.Now().Add(10 * time.Second); strings.Count(calls(), fetch13) < 2; time.Sleep(100 * time.Millisecond) {
			if time.Now().After(deadline) {
				dump()
				t.Fatal("#13's log command didn't run again")
			}
		}
		waitJS(t, "#13's log again", `document.querySelectorAll('[data-testid="log"] li').length >= 31`)
		time.Sleep(time.Second)
		if n := logCount(t); n != 31 {
			dump()
			t.Fatalf("reopening #13 shows %d log lines, want 31", n)
		}
	})

	shotPNG(t, runCtx, "ci-desktop")

	t.Run("on a phone no cell leaves its row", func(t *testing.T) {
		if err := runCtx(chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(2), chromedp.EmulateMobile)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "the phone layout", `matchMedia("(max-width: 700px)").matches`)
		var outside []string
		eval(t, `[...document.querySelectorAll("ol.runs li")].flatMap(li => { const r = li.getBoundingClientRect();
			return [...li.children].filter(c => c.offsetParent).filter(c => { const b = c.getBoundingClientRect();
				return b.left < r.left - 1 || b.right > r.right + 1 || b.top < r.top - 1 || b.bottom > r.bottom + 1; }).map(c => { const b = c.getBoundingClientRect();
				return li.querySelector(".run")?.textContent + " ." + c.className + " " + [b.left, b.top, b.right, b.bottom].map(Math.round) + " in " + [r.left, r.top, r.right, r.bottom].map(Math.round); }); })`, &outside)
		var overflow bool
		eval(t, `document.documentElement.scrollWidth > window.innerWidth`, &overflow)
		if len(outside) > 0 || overflow {
			dump()
			t.Fatalf("cells outside their rows: %v; page scrolls sideways: %v", outside, overflow)
		}
		shotPNG(t, runCtx, "ci-phone")
	})

	logs.mu.Lock()
	defer logs.mu.Unlock()
	for _, c := range logs.console {
		if strings.HasPrefix(c, "[error]") || strings.HasPrefix(c, "[exception]") {
			t.Errorf("browser console: %s", c)
		}
	}
}
