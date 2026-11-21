package e2e

// End-to-end test of the test runner (../tests): serves the page from a URL,
// as users run it, with `go test -json` on a small sample module passed as a
// --source flag, and drives it in Docker Chrome.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestTests(t *testing.T) {
	bin := buildTinkerdown(t)
	page, err := os.ReadFile("../tests/index.md")
	if err != nil {
		t.Fatal(err)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(page) }))
	defer pages.Close()
	sample, _ := filepath.Abs("testdata/tests/sample")
	url, serverLog := serve(t, bin, pages.URL+"/index.md",
		"--source", "go=cd "+sample+" && GOWORK=off GOFLAGS=-count=1 go test -json ./...")

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
		t.Logf("console:\n%s", strings.Join(logs.console, "\n"))
		t.Logf("ws received: %d frames, last: %s", len(logs.recv), strings.Join(logs.recv[max(0, len(logs.recv)-3):], "\n"))
		t.Logf("rendered html:\n%.4000s", html)
		s := serverLog.String()
		t.Logf("server log tail:\n%s", s[max(0, len(s)-3000):])
	}
	waitJS := func(t *testing.T, what, js string) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			var ok bool
			if run(chromedp.Evaluate(js, &ok)) == nil && ok {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		dump()
		t.Fatalf("timed out waiting for %s (%s)", what, js)
	}
	row := func(test string) string {
		return `[...document.querySelectorAll("ol.tests li")].find(li => li.querySelector(".name").textContent === "` + test + `")`
	}

	if err := chromedp.Run(ctx, network.Enable(), chromedp.EmulateViewport(1280, 900), chromedp.Navigate(url),
		chromedp.WaitVisible(`[data-testid="tests"]`, chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}

	t.Run("one row per test with its status", func(t *testing.T) {
		waitJS(t, "every test finished", row("TestCreateUser")+`?.classList.contains("act-pass") && `+
			row("TestDeleteUser")+`?.classList.contains("act-fail") && `+row("TestPostgres")+`?.classList.contains("act-skip")`)
		var names []string
		run(chromedp.Evaluate(`[...document.querySelectorAll("ol.tests li .name")].map(n => n.textContent).filter(n => n)`, &names))
		if got := strings.Join(names, ","); got != "TestCreateUser,TestDeleteUser,TestPostgres,TestLineSplitterTruncatesHugeLineAcrossChunksWithoutLosingTheNextLine" {
			dump()
			t.Fatalf("test rows = %s; want one per test, in run order", got)
		}
		var missing string
		run(chromedp.Evaluate(`[...document.querySelectorAll("ol.tests li")].map(li => li.textContent).find(s => s.includes("no value")) || ""`, &missing))
		if missing != "" {
			t.Fatalf("a missing field rendered as text: %q", missing)
		}
	})

	t.Run("a failing test shows its output", func(t *testing.T) {
		var out string
		run(chromedp.Evaluate(row("TestDeleteUser")+`.querySelector(".out").innerText`, &out))
		if !strings.Contains(out, "want 204, got 500") {
			dump()
			t.Fatalf("failing test output = %q", out)
		}
		if n := strings.Count(out, "=== RUN"); n != 1 {
			t.Fatalf("output holds %d runs; collect_reset should keep only the latest:\n%s", n, out)
		}
		var passOutVisible bool
		run(chromedp.Evaluate(`getComputedStyle(`+row("TestCreateUser")+`.querySelector(".out")).display !== "none"`, &passOutVisible))
		if passOutVisible {
			t.Fatal("a passing test's output should stay hidden")
		}
	})

	shotPNG(t, run, "tests-desktop")

	t.Run("the Failed tab shows only failures", func(t *testing.T) {
		if err := run(chromedp.Click(`button[role="tab"][data-filter="fields.Action = fail"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "only failing rows", `(() => { const r = [...document.querySelectorAll("ol.tests li")]; return r.length > 0 && r.every(li => li.classList.contains("act-fail")); })()`)
		if err := run(chromedp.Click(`button[role="tab"][data-filter=""]`, chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("search narrows by test name", func(t *testing.T) {
		if err := run(chromedp.SendKeys(`[data-testid="search"]`, "user", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "only matching tests", `(() => { const n = [...document.querySelectorAll("ol.tests li .name")].map(e => e.textContent); return n.length === 2 && n.every(x => x.toLowerCase().includes("user")); })()`)
	})

	t.Run("phone layout", func(t *testing.T) {
		// Clear the search so every row, the long names too, is on screen.
		if err := run(chromedp.Evaluate(`(() => { const s = document.querySelector('[data-testid="search"]');
			s.value = ""; s.dispatchEvent(new Event("input", {bubbles: true})); })()`, nil),
			chromedp.EmulateViewport(390, 844)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "every test back", `document.querySelectorAll("ol.tests li .name").length >= 4`)
		var overflow bool
		run(chromedp.Evaluate(`document.documentElement.scrollWidth > window.innerWidth + 1`, &overflow))
		shotPNG(t, run, "tests-phone")
		if overflow {
			t.Fatal("page scrolls horizontally at phone width")
		}
		// No row may be wider than its box: a long test name or package
		// path must wrap or be cut, not run past the card.
		var wide []string
		run(chromedp.Evaluate(`[...document.querySelectorAll("ol.tests li > *")].filter(e => e.getBoundingClientRect().right > e.closest("li").getBoundingClientRect().right + 1).map(e => e.className + ": " + e.textContent.slice(0, 40))`, &wide))
		if len(wide) > 0 {
			t.Fatalf("cells overflow their row at phone width: %q", wide)
		}
	})
}
