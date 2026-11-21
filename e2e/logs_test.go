package e2e

// End-to-end test of the log viewer (../logs): serves a copy of it with demo
// sources passed as --source flags, and drives it in Docker Chrome.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// logsDir is the log viewer app, next to this e2e directory.
const logsDir = "../logs"

// copyApp copies the viewer and its demo inputs into a temp dir, so the test
// can append to the log file without touching the repo.
func copyApp(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	for from, to := range map[string]string{
		filepath.Join(logsDir, "index.md"): "index.md",
		"testdata/logs/demo-logs.sh":       "scripts/demo-logs.sh",
		"testdata/logs/app.log":            "logs/app.log",
	} {
		data, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dst, to)
		os.MkdirAll(filepath.Dir(target), 0o755)
		if err := os.WriteFile(target, data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// demoSources are the --source flags the test serves the viewer with: two
// demo processes and a log file, as a user would pass their own.
var demoSources = []string{
	"--source", "web=./scripts/demo-logs.sh web",
	"--source", "worker=./scripts/demo-logs.sh worker",
	"--source", "app=file:./logs/app.log",
}

func TestLogViewer(t *testing.T) {
	bin := buildTinkerdown(t)
	dir := copyApp(t)
	url, serverLog := serve(t, bin, dir, demoSources...)
	emptyURL, _ := serve(t, bin, copyApp(t))
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
			path := filepath.Join(os.TempDir(), "onefile-logs-"+name+".png")
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

	pauseResumeClear := func(t *testing.T) {
		click := func(id string) {
			t.Helper()
			if err := run(chromedp.Click(`[data-testid="`+id+`"]`, chromedp.ByQuery)); err != nil {
				dump()
				t.Fatalf("click %s: %v", id, err)
			}
		}
		lastKey := `(() => { const l = document.querySelectorAll("ol.log li"); return l.length ? +l[l.length-1].dataset.key : 0; })()`
		click("pause")
		waitJS("the Resume button", `!!document.querySelector('[data-testid="resume"]')`)
		var before, after int
		eval(lastKey, &before)
		time.Sleep(1500 * time.Millisecond) // the demo inputs write several lines a second
		eval(lastKey, &after)
		if after != before {
			dump()
			t.Fatalf("rows arrived while paused: last key %d -> %d", before, after)
		}
		click("resume")
		waitJS("rows after resume", fmt.Sprintf(`%s > %d`, lastKey, before))

		eval(lastKey, &before)
		click("clear")
		waitJS("old rows hidden", fmt.Sprintf(`[...document.querySelectorAll("ol.log li")].every(r => +r.dataset.key > %d)`, before))
		waitJS("new rows after clear", fmt.Sprintf(`%s > %d`, lastKey, before))
	}
	t.Run("pause freezes the list, resume catches up, clear empties it", pauseResumeClear)

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

	t.Run("pause, resume and clear work on a filtered tab", pauseResumeClear)

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
		// Each toolbar button must be the element a tap on its centre reaches.
		var covered []string
		eval(`["pause", "clear", "search", "level"].filter(id => {
			const el = document.querySelector('[data-testid="' + id + '"]');
			el.scrollIntoView({block: "center"});
			const r = el.getBoundingClientRect();
			const hit = document.elementFromPoint(r.x + r.width/2, r.y + r.height/2);
			return !el.contains(hit);
		}).map(id => { const el = document.querySelector('[data-testid="' + id + '"]'), r = el.getBoundingClientRect();
			const hit = document.elementFromPoint(r.x + r.width/2, r.y + r.height/2);
			return id + " covered by " + (hit ? hit.outerHTML.slice(0, 120) : "nothing"); })`, &covered)
		if len(covered) > 0 {
			t.Errorf("toolbar controls a tap can't reach: %q", covered)
		}
	})

	t.Run("with no sources the page says what to pass", func(t *testing.T) {
		if err := run(chromedp.EmulateViewport(1280, 900), chromedp.Navigate(emptyURL),
			chromedp.WaitVisible(`[data-testid="log"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS("the empty state", `(document.querySelector('[data-testid="empty"]')?.textContent || "").includes("Logs to follow")`)
		var tabs int
		eval(`document.querySelectorAll('button[role="tab"]').length`, &tabs)
		if tabs != 1 {
			t.Fatalf("%d tabs with no sources, want only All", tabs)
		}
	})
}
