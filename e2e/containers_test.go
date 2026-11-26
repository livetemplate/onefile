package e2e

// End-to-end test of the containers app (../containers): runs the serve
// command from its README through bash, as a user pastes it, with the page
// served from a URL and the Docker commands narrowed to this test's
// containers. Drives the page in Docker Chrome against real containers.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

func TestContainers(t *testing.T) {
	bin := buildTinkerdown(t)
	pageMD, err := os.ReadFile("../containers/index.md")
	if err != nil {
		t.Fatal(err)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(pageMD) }))
	defer pages.Close()

	// Every container this test makes carries the label and an ofe2e-<pid>-
	// name, and the feed only sees the label.
	label := fmt.Sprintf("ofe2e=%d", os.Getpid())
	prefix := fmt.Sprintf("ofe2e-%d-", os.Getpid())
	docker := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	t.Cleanup(func() {
		ids, _ := exec.Command("docker", "ps", "-aq", "--filter", "label="+label).Output()
		if f := strings.Fields(string(ids)); len(f) > 0 {
			exec.Command("docker", append([]string{"rm", "-f"}, f...)...).Run()
		}
	})
	// A container that prints a numbered line a second and stops on SIGTERM
	// within a second, so Stop doesn't wait out Docker's 10s timeout.
	runLogger := func(name string) {
		docker("run", "-d", "--label", label, "--name", prefix+name, "busybox:1.36", "sh", "-c",
			`trap "exit 0" TERM; i=0; while true; do i=$((i+1)); echo "`+name+` line $i"; sleep 1; done`)
	}
	runLogger("web")
	runLogger("db")

	port := freePort(t)
	cmdline := readmeCommandOf(t, "../containers/README.md")
	cmdline = replaceOnce(t, cmdline, "tinkerdown serve", bin+" serve --port "+fmt.Sprint(port))
	cmdline = replaceOnce(t, cmdline, "github.com/livetemplate/onefile/containers", pages.URL+"/index.md")
	cmdline = replaceOnce(t, cmdline, "docker ps -a ", "docker ps -a --filter label="+label+" ")
	cmdline = replaceOnce(t, cmdline, "exec docker events ", "exec docker events --filter label="+label+" ")
	url, serverLog := serveCmd(t, exec.Command("bash", "-c", "exec "+cmdline), port)

	ctx := dockerChrome(t)
	logs := &browserLogs{}
	logs.listen(ctx)
	// Kill asks first; the test answers the browser's confirm dialog, and
	// learns when the dialog has closed.
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
		t.Logf("ws received: %d frames, last: %s", len(logs.recv), strings.Join(logs.recv[max(0, len(logs.recv)-3):], "\n"))
		t.Logf("rendered html:\n%.5000s", html)
		s := serverLog.String()
		t.Logf("server log tail:\n%s", s[max(0, len(s)-4000):])
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
	const logMsgs = `[...document.querySelectorAll('[data-testid="logs"] .msg')]`
	row := func(name string) string {
		return `[...document.querySelectorAll("ol.ctr li")].find(li => li.querySelector(".name")?.textContent === "` + prefix + name + `")`
	}
	visibleButtons := func(t *testing.T, name string) string {
		t.Helper()
		var got string
		if err := run(chromedp.Evaluate(`[...`+row(name)+`.querySelectorAll("button")].filter(b => b.offsetParent).map(b => b.textContent).join(",")`, &got)); err != nil {
			dump()
			t.Fatalf("buttons of %s: %v", name, err)
		}
		return got
	}
	click := func(t *testing.T, name, button string) {
		t.Helper()
		if err := run(chromedp.Click(`ol.ctr button[name="`+button+`"][data-name="`+prefix+name+`"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatalf("click %s on %s: %v", button, name, err)
		}
	}

	if err := chromedp.Run(ctx, network.Enable(), chromedp.EmulateViewport(1280, 900), chromedp.Navigate(url),
		chromedp.WaitVisible(`[data-testid="containers"]`, chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}

	t.Run("running containers show running, with their buttons", func(t *testing.T) {
		waitJS(t, "web and db running", row("web")+`?.classList.contains("st-running") && `+row("db")+`?.classList.contains("st-running")`)
		if got := visibleButtons(t, "web"); got != "Stop,Restart,Kill" {
			dump()
			t.Fatalf("a running row shows %q, want Stop,Restart,Kill", got)
		}
		var others int
		run(chromedp.Evaluate(`[...document.querySelectorAll("ol.ctr li .name")].filter(n => !n.textContent.startsWith("`+prefix+`")).length`, &others))
		if others != 0 {
			t.Fatalf("%d rows aren't this test's containers; the README command's filter didn't apply", others)
		}
		run(chromedp.Evaluate(`window.__samePage = true`, nil))
	})

	t.Run("a container that exits with code 3 shows exited (3) in the Exited tab", func(t *testing.T) {
		docker("run", "-d", "--label", label, "--name", prefix+"job", "busybox:1.36", "sh", "-c", "exit 3")
		waitJS(t, "job exited (3)", row("job")+`?.querySelector(".status").textContent === "exited (3)"`)
		if got := visibleButtons(t, "job"); got != "Start" {
			dump()
			t.Fatalf("an exited row shows %q, want Start", got)
		}
		if err := run(chromedp.Click(`button[role="tab"][data-filter="fields.state = exited"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "only exited rows", `(() => { const r = [...document.querySelectorAll("ol.ctr li")]; return r.length === 1 && r[0].classList.contains("st-exited"); })()`)
		if err := run(chromedp.Click(`button[role="tab"][data-filter=""]`, chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "all rows back", `document.querySelectorAll("ol.ctr li").length === 3`)
	})

	t.Run("Stop moves a row to exited, and Start brings it back", func(t *testing.T) {
		// The button spins from the click until docker stop returns.
		var busy string
		if err := run(chromedp.Evaluate(`(b => (b.click(), b.getAttribute("aria-busy")))(document.querySelector('ol.ctr button[name="stop"][data-name="`+prefix+`db"]'))`, &busy)); err != nil || busy != "true" {
			dump()
			t.Fatalf("Stop's aria-busy right after the click = %q (err %v), want true", busy, err)
		}
		waitJS(t, "db exited", row("db")+`?.classList.contains("st-exited")`)
		click(t, "db", "start")
		waitJS(t, "db running again", row("db")+`?.classList.contains("st-running")`)
	})

	t.Run("clicking a name follows its logs, without a page load", func(t *testing.T) {
		if err := run(chromedp.Click(`ol.ctr a[href="?name=`+prefix+`web"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "web's log lines", logMsgs+`.some(m => m.textContent.startsWith("web line"))`)
		waitJS(t, "the pane naming web", `document.querySelector('[data-testid="log-title"]').textContent === "`+prefix+`web · Logs"`)
		mark := fmt.Sprintf("marker-%d", time.Now().UnixNano())
		docker("exec", prefix+"web", "sh", "-c", "echo "+mark+" > /proc/1/fd/1")
		waitJS(t, "a line printed after opening", logMsgs+`.some(m => m.textContent === "`+mark+`")`)
		var same bool
		var search string
		run(chromedp.Evaluate(`window.__samePage === true`, &same), chromedp.Evaluate(`location.search`, &search))
		if !same || search != "?name="+prefix+"web" {
			dump()
			t.Fatalf("after the link: same page %v, location.search %q", same, search)
		}
		// The pane follows: new lines keep it at the bottom.
		waitJS(t, "the logs pane at its bottom", `(o => o.scrollHeight - o.scrollTop - o.clientHeight < 40)(document.querySelector('[data-testid="logs"]'))`)
		var stray bool
		run(chromedp.Evaluate(logMsgs+`.some(m => m.textContent.startsWith("db line"))`, &stray))
		if stray {
			t.Fatal("db's lines appeared in web's logs")
		}
	})

	t.Run("after Restart the logs show the new run without repeating the old one", func(t *testing.T) {
		click(t, "web", "restart")
		// The new run starts its count again: a second "web line 1".
		waitJS(t, "the new run's first line", logMsgs+`.filter(m => m.textContent === "web line 1").length === 2`)
		waitJS(t, "the new run's second line", logMsgs+`.filter(m => m.textContent === "web line 2").length === 2`)
		var repeated []string
		run(chromedp.Evaluate(`(() => { const seen = new Set(), dup = [];
			for (const li of document.querySelectorAll('[data-testid="logs"] li')) { const k = li.textContent; if (seen.has(k)) dup.push(k); seen.add(k); }
			return dup; })()`, &repeated))
		if len(repeated) > 0 {
			dump()
			t.Fatalf("lines shown twice after the restart: %v", repeated)
		}
	})

	shotPNG(t, run, "containers-desktop")

	t.Run("Kill asks first", func(t *testing.T) {
		acceptDialog.Store(false)
		click(t, "web", "kill")
		select {
		case <-dialogClosed:
		case <-time.After(10 * time.Second):
			dump()
			t.Fatal("Kill sent without asking")
		}
		// The client sends the action right after the dialog, or never.
		run(chromedp.Evaluate(`0`, nil))
		logs.mu.Lock()
		sent := strings.Join(logs.sent, "\n")
		logs.mu.Unlock()
		if strings.Contains(sent, `"action":"kill"`) {
			t.Fatal("cancelling the dialog still sent the kill")
		}
		acceptDialog.Store(true)
		click(t, "web", "kill")
		waitJS(t, "web killed", row("web")+`?.querySelector(".status").textContent === "exited (137)"`)
	})

	t.Run("docker rm -f removes the row", func(t *testing.T) {
		docker("rm", "-f", prefix+"job")
		waitJS(t, "job's row gone", `!`+row("job"))
	})

	t.Run("search narrows by name", func(t *testing.T) {
		if err := run(chromedp.SendKeys(`[data-testid="search"]`, "db", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "only db", `(() => { const n = [...document.querySelectorAll("ol.ctr li .name")].map(e => e.textContent); return n.length === 1 && n[0] === "`+prefix+`db"; })()`)
		if err := run(chromedp.SendKeys(`[data-testid="search"]`, kb.Backspace+kb.Backspace, chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "both rows back", `document.querySelectorAll("ol.ctr li").length === 2`)
	})

	t.Run("on a phone no cell leaves its row", func(t *testing.T) {
		if err := run(chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(2), chromedp.EmulateMobile)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "the phone layout", `matchMedia("(max-width: 700px)").matches && getComputedStyle(document.querySelector("ol.ctr li")).gridTemplateColumns.split(" ").length === 2`)
		var outside []string
		run(chromedp.Evaluate(`[...document.querySelectorAll("ol.ctr li")].flatMap(li => { const r = li.getBoundingClientRect();
			return [...li.children].filter(c => c.offsetParent).filter(c => { const b = c.getBoundingClientRect();
				return b.left < r.left - 1 || b.right > r.right + 1 || b.top < r.top - 1 || b.bottom > r.bottom + 1; }).map(c => li.querySelector(".name").textContent + " ." + c.className); })`, &outside))
		var overflow bool
		run(chromedp.Evaluate(`document.documentElement.scrollWidth > window.innerWidth`, &overflow))
		if len(outside) > 0 || overflow {
			dump()
			t.Fatalf("cells outside their rows: %v; page scrolls sideways: %v", outside, overflow)
		}
		shotPNG(t, run, "containers-phone")
	})

	logs.failOnErrors(t)
}
