package e2e

// End-to-end test of the db app (../db): runs the SQLite serve command from
// its README through sh, as a user pastes it, with the page served from a URL
// and a stand-in sqlite3 (testdata/db/bin) first on PATH. Drives the page in
// Docker Chrome. No test uses the network.
//
// The test finds the page's parts through the markup the db plan sets (the
// execute button's data-entry, the f.fields.text search) and through the text
// it shows, not through test IDs: a fresh agent builds the page from the
// guide and the plan alone (plans/2026-11-25-db.md).

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
	"github.com/chromedp/chromedp/kb"
)

func TestDB(t *testing.T) {
	bin := buildTinkerdown(t)
	pageMD, err := os.ReadFile("../db/index.md")
	if err != nil {
		t.Fatal(err)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(pageMD) }))
	defer pages.Close()
	fakeBin, err := filepath.Abs("testdata/db/bin")
	if err != nil {
		t.Fatal(err)
	}

	// The user's directory: their .sql file, a folder of reports, and the
	// database the command names.
	dir := t.TempDir()
	queries, err := os.ReadFile("testdata/db/queries.sql")
	if err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile("testdata/db/reports/orders-by-day.sql")
	if err != nil {
		t.Fatal(err)
	}
	writeFile := func(name, text string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("queries.sql", string(queries))
	writeFile("reports/orders-by-day.sql", string(report))
	writeFile("app.db", "")

	// Each run of the fake sqlite3 appends its arguments here, each ended
	// by a NUL, and one more NUL ends the run.
	callsFile := filepath.Join(t.TempDir(), "calls")
	calls := func() [][]string {
		b, _ := os.ReadFile(callsFile)
		if len(b) == 0 {
			return nil
		}
		var out [][]string
		for _, call := range strings.Split(strings.TrimSuffix(string(b), "\x00\x00"), "\x00\x00") {
			out = append(out, strings.Split(call, "\x00"))
		}
		return out
	}
	serveREADME := func(cmdline string) (string, *syncBuffer) {
		t.Helper()
		port := freePort(t)
		cmdline = replaceOnce(t, cmdline, "tinkerdown serve", bin+" serve --port "+fmt.Sprint(port))
		cmdline = replaceOnce(t, cmdline, "github.com/livetemplate/onefile/db", pages.URL+"/index.md")
		cmd := exec.Command("sh", "-c", "exec "+cmdline)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "OFE2E_DB_CALLS="+callsFile)
		return serveCmd(t, cmd, port)
	}
	readmeCmd := readmeCommandOf(t, "../db/README.md")
	url, serverLog := serveREADME(readmeCmd)

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
		run(chromedp.OuterHTML(".content-wrapper", &html, chromedp.ByQuery))
		logs.mu.Lock()
		defer logs.mu.Unlock()
		t.Logf("console:\n%s", strings.Join(logs.console, "\n"))
		t.Logf("ws sent: %s", strings.Join(logs.sent[max(0, len(logs.sent)-5):], "\n"))
		t.Logf("ws received: %d frames, last: %.3000s", len(logs.recv), strings.Join(logs.recv[max(0, len(logs.recv)-3):], "\n"))
		t.Logf("rendered html:\n%.6000s", html)
		s := serverLog.String()
		t.Logf("server log tail:\n%s", s[max(0, len(s)-4000):])
		t.Logf("sqlite3 calls: %q", calls())
	}
	waitFor := func(t *testing.T, what string, timeout time.Duration, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(timeout); !ok(); time.Sleep(200 * time.Millisecond) {
			if time.Now().After(deadline) {
				dump()
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	waitJS := func(t *testing.T, what, js string) {
		t.Helper()
		waitFor(t, what+" ("+js+")", 30*time.Second, func() bool {
			var ok bool
			return run(chromedp.Evaluate(js, &ok)) == nil && ok
		})
	}
	runBtn := func(entry string) string { return `button[name="execute"][data-entry="` + entry + `"]` }
	const content = `document.querySelector(".content-wrapper")`
	const runButtons = `document.querySelectorAll('button[name="execute"]')`
	pageText := func() string {
		var s string
		run(chromedp.Evaluate(content+`.innerText`, &s))
		return s
	}
	// press clicks a query's Run, waits until the page shows want, and
	// returns the arguments of the one sqlite3 run it started.
	press := func(t *testing.T, entry, want string) []string {
		t.Helper()
		before := len(calls())
		if err := run(chromedp.Click(runBtn(entry), chromedp.ByQuery)); err != nil {
			dump()
			t.Fatalf("click Run %s: %v", entry, err)
		}
		waitFor(t, want, 30*time.Second, func() bool { return strings.Contains(pageText(), want) && len(calls()) > before })
		c := calls()
		if len(c) != before+1 {
			dump()
			t.Fatalf("sqlite3 ran %d times for one Run, want 1", len(c)-before)
		}
		return c[before]
	}

	if err := chromedp.Run(ctx, network.Enable(), chromedp.EmulateViewport(1280, 900), chromedp.Navigate(url),
		chromedp.WaitVisible(runBtn("queries/6"), chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}

	t.Run("the list shows the file's queries, and opening the page runs nothing", func(t *testing.T) {
		var n int
		run(chromedp.Evaluate(runButtons+`.length`, &n))
		got := pageText()
		for _, want := range []string{"users", "active-users", "nobody", "close-stale", "slow", "<img/src=x/onerror=alert(1)>"} {
			if !strings.Contains(got, want) {
				dump()
				t.Fatalf("the list lacks %q", want)
			}
		}
		if n != 6 {
			dump()
			t.Fatalf("%d Run buttons, want 6", n)
		}
		var kind string
		run(chromedp.Evaluate(`document.querySelector('`+runBtn("queries/4")+`').closest("li").innerText`, &kind))
		if !strings.Contains(strings.ToLower(kind), "update") {
			dump()
			t.Fatalf("close-stale's row %q doesn't show its kind", kind)
		}
		time.Sleep(time.Second)
		if c := calls(); len(c) != 0 {
			t.Fatalf("sqlite3 ran %d times before any Run", len(c))
		}
	})

	t.Run("Run shows the client's table, read-only and with the clicked query", func(t *testing.T) {
		args := press(t, "queries/1", "grace@example.com")
		want := []string{"-safe", "-readonly", "-box", "./app.db", " SELECT id, name, email FROM users LIMIT 100;"}
		if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("sqlite3 got %q, want %q", args, want)
		}
		if got := pageText(); !strings.Contains(got, "│ 2  │ Grace") || !strings.Contains(got, "┌────┬") {
			dump()
			t.Fatalf("the result isn't the client's table:\n%s", got)
		}
		shotPNG(t, run, "db-desktop")
	})

	t.Run("result and list text stays text", func(t *testing.T) {
		var tags int
		run(chromedp.Evaluate(content+`.querySelectorAll("img, script").length`, &tags))
		if tags != 0 || !strings.Contains(pageText(), "<img src=x onerror=alert(1)>") {
			dump()
			t.Fatalf("%d img/script elements in the page; want the value as text", tags)
		}
	})

	t.Run("a query that starts with a comment runs", func(t *testing.T) {
		args := press(t, "queries/2", "│ Grace                        │")
		if sql := args[len(args)-1]; !strings.HasPrefix(sql, " -- every active user") {
			t.Fatalf("sqlite3 got the SQL %q, want it after a space", sql)
		}
		if strings.Contains(pageText(), "unknown option") {
			dump()
			t.Fatal("sqlite3 read the query as an option")
		}
	})

	t.Run("an empty result says No rows, and only the last run shows", func(t *testing.T) {
		press(t, "queries/3", "No rows")
		if got := pageText(); strings.Contains(got, "grace@example.com") || strings.Contains(got, "│ Grace") {
			dump()
			t.Fatalf("an earlier result still shows:\n%s", got)
		}
	})

	t.Run("a write's error shows as stderr, with its exit status", func(t *testing.T) {
		press(t, "queries/4", "attempt to write a readonly database")
		waitJS(t, "the exit status", content+`.innerText.includes("exit status 8")`)
		var marked bool
		run(chromedp.Evaluate(`[...`+content+`.querySelectorAll("[class*=stderr]")].some(e => e.textContent.includes("readonly database"))`, &marked))
		if !marked {
			dump()
			t.Fatal("the error line isn't marked as stderr")
		}
	})

	t.Run("a hand-made click on an unlisted entry runs nothing", func(t *testing.T) {
		before := len(calls())
		if err := run(chromedp.Evaluate(`document.querySelector('`+runBtn("queries/1")+`').dataset.entry = "queries/99"`, nil),
			chromedp.Click(runBtn("queries/99"), chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the refusal in the server log", 10*time.Second, func() bool { return strings.Contains(serverLog.String(), "value entry is not listed") })
		if n := len(calls()); n != before {
			t.Fatalf("sqlite3 ran %d times after a hand-made click", n-before)
		}
	})

	t.Run("a reload and Back run nothing more", func(t *testing.T) {
		before := len(calls())
		if err := run(chromedp.Reload(), chromedp.WaitVisible(runBtn("queries/6"), chromedp.ByQuery),
			chromedp.Navigate("about:blank"), chromedp.NavigateBack(), chromedp.WaitVisible(runBtn("queries/6"), chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		time.Sleep(1500 * time.Millisecond)
		if n := len(calls()); n != before {
			t.Fatalf("sqlite3 ran %d times from a reload and Back", n-before)
		}
	})

	t.Run("search narrows by name and SQL", func(t *testing.T) {
		const search = `input[name="f.fields.text"]`
		for _, q := range []string{"close", "orders SET"} {
			if err := run(chromedp.SendKeys(search, q, chromedp.ByQuery)); err != nil {
				dump()
				t.Fatal(err)
			}
			waitJS(t, "only close-stale for "+q, runButtons+`.length === 1 && !!document.querySelector('`+runBtn("queries/4")+`')`)
			if err := run(chromedp.SendKeys(search, strings.Repeat(kb.Backspace, len(q)), chromedp.ByQuery)); err != nil {
				t.Fatal(err)
			}
			waitJS(t, "every query back", runButtons+`.length === 6`)
		}
	})

	t.Run("editing the file updates the list", func(t *testing.T) {
		writeFile("queries.sql", string(queries)+"\n-- name: added\nSELECT 2;\n")
		start := time.Now()
		waitJS(t, "a seventh query", `!!document.querySelector('`+runBtn("queries/7")+`')`)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("the new query took %v to show", d)
		}
		writeFile("queries.sql", string(queries))
		waitJS(t, "six queries again", runButtons+`.length === 6`)
	})

	t.Run("on a phone no cell leaves its row, and the page doesn't scroll sideways", func(t *testing.T) {
		if err := run(chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(2), chromedp.EmulateMobile)); err != nil {
			t.Fatal(err)
		}
		press(t, "queries/1", "grace@example.com")
		var outside []string
		run(chromedp.Evaluate(`[...document.querySelectorAll('button[name="execute"]')].map(b => b.closest("li")).flatMap(li => { const r = li.getBoundingClientRect();
			return [...li.children].filter(c => c.offsetParent).filter(c => { const b = c.getBoundingClientRect();
				return b.left < r.left - 1 || b.right > r.right + 1 || b.top < r.top - 1 || b.bottom > r.bottom + 1; }).map(c => li.innerText.split("\n")[0] + " ." + c.className); })`, &outside))
		var overflow bool
		run(chromedp.Evaluate(`document.documentElement.scrollWidth > window.innerWidth`, &overflow))
		if len(outside) > 0 || overflow {
			dump()
			t.Fatalf("cells outside their rows: %v; page scrolls sideways: %v", outside, overflow)
		}
		shotPNG(t, run, "db-phone")
		run(chromedp.EmulateViewport(1280, 900))
	})

	t.Run("no sqlite3 remains after the page leaves", func(t *testing.T) {
		if err := run(chromedp.Click(runBtn("queries/5"), chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		running := func() bool { return exec.Command("pgrep", "-f", fakeBin+"/sqlite3").Run() == nil }
		waitFor(t, "the slow query to start", 10*time.Second, running)
		if err := run(chromedp.Navigate("about:blank")); err != nil {
			t.Fatal(err)
		}
		// A params merge's commands run on for 30 s after the last page
		// showing them leaves (tinkerdown's MergeIdle), so allow 45.
		waitFor(t, "every sqlite3 the page started to exit", 45*time.Second, func() bool { return !running() })
	})

	t.Run("with a folder too, each Run runs its own file's query", func(t *testing.T) {
		url2, _ := serveREADME(readmeCmd + " --source sql:./reports/")
		if err := run(chromedp.Navigate(url2), chromedp.WaitVisible(runBtn("reports/1"), chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		args := press(t, "reports/1", "2026-11-25")
		if sql := args[len(args)-1]; sql != " SELECT day, count(*) AS n FROM orders GROUP BY day;" {
			t.Fatalf("Run on the report ran %q", sql)
		}
		args = press(t, "queries/1", "grace@example.com")
		if sql := args[len(args)-1]; sql != " SELECT id, name, email FROM users LIMIT 100;" {
			t.Fatalf("Run on users ran %q", sql)
		}
	})

	logs.failOnErrors(t)
}
