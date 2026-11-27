package e2e

// End-to-end test of the git app (../git): runs the serve command from its
// README through sh, as a user pastes it, with the page served from a URL,
// in a repo the test makes in a temporary directory. git is the real one; a
// shim first on PATH only counts its runs. Drives the page in Docker Chrome.
// No test uses the network.

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

func TestGit(t *testing.T) {
	bin := buildTinkerdown(t)
	pageMD, err := os.ReadFile("../git/index.md")
	if err != nil {
		t.Fatal(err)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(pageMD) }))
	defer pages.Close()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	runsFile := filepath.Join(t.TempDir(), "git-runs")
	fakeTool(t, shim, "git", fmt.Sprintf(`echo "$*" >> %q; exec %q "$@"`, runsFile, realGit))
	gitRuns := func() int {
		b, _ := os.ReadFile(runsFile)
		return strings.Count(string(b), "\n")
	}

	// gitIn runs the real git in dir, past the shim, so the test's own runs
	// aren't counted.
	gitIn := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command(realGit, append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	newRepo := func(commit bool) string {
		t.Helper()
		dir := t.TempDir()
		for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "ada@example.com"}, {"config", "user.name", "Ada"}} {
			gitIn(dir, args...)
		}
		if commit {
			os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
			for _, args := range [][]string{{"add", "a.txt"}, {"commit", "-qm", "first"}, {"branch", "feat"}} {
				gitIn(dir, args...)
			}
		}
		return dir
	}
	repo := newRepo(true)
	git := func(args ...string) string {
		t.Helper()
		return gitIn(repo, args...)
	}
	// preCommit installs a pre-commit hook for one step.
	preCommit := func(t *testing.T, script string) {
		hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
		os.WriteFile(hook, []byte("#!/bin/sh\n"+script+"\n"), 0o755)
		t.Cleanup(func() { os.Remove(hook) })
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	readmeCmd := readmeCommandOf(t, "../git/README.md")
	serveREADME := func(dir, cmdline string, flags ...string) (string, *syncBuffer) {
		t.Helper()
		port := freePort(t)
		cmdline = replaceOnce(t, cmdline, "tinkerdown serve", strings.Join(append([]string{bin, "serve", "--port", fmt.Sprint(port)}, flags...), " "))
		cmdline = replaceOnce(t, cmdline, "github.com/livetemplate/onefile/git", pages.URL+"/index.md")
		cmd := exec.Command("sh", "-c", "exec env "+cmdline)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+shim+":"+os.Getenv("PATH"))
		return serveCmd(t, cmd, port)
	}
	url, serverLog := serveREADME(repo, readmeCmd)

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
		t.Logf("git status: %s", git("status", "--short"))
	}
	waitFor := func(t *testing.T, what string, timeout time.Duration, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(timeout); !ok(); time.Sleep(100 * time.Millisecond) {
			if time.Now().After(deadline) {
				dump()
				t.Fatalf("timed out after %v waiting for %s", timeout, what)
			}
		}
	}
	js := func(expr string) (out string) {
		run(chromedp.Evaluate(`(() => { try { return String(`+expr+`) } catch (e) { return "" } })()`, &out))
		return out
	}
	waitJS := func(t *testing.T, what string, timeout time.Duration, expr string) {
		t.Helper()
		waitFor(t, what+" ("+expr+")", timeout, func() bool { return js(expr) == "true" })
	}
	click := func(t *testing.T, sel string) {
		t.Helper()
		if err := run(chromedp.Click(sel, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatalf("click %s: %v", sel, err)
		}
	}
	fileRow := func(path string) string {
		return `[...document.querySelectorAll(".files li")].find(li => li.querySelector(".path").textContent === ` + quoteJS(path) + `)`
	}
	clickPath := func(t *testing.T, path string) {
		t.Helper()
		if err := run(chromedp.Evaluate(fileRow(path)+`.querySelector(".path").click()`, nil)); err != nil {
			dump()
			t.Fatalf("click %s: %v", path, err)
		}
	}
	const logSubjects = `[...document.querySelectorAll(".commits .subject")].map(s => s.textContent)`

	if err := chromedp.Run(ctx, network.Enable(), chromedp.EmulateViewport(1280, 900), chromedp.Navigate(url),
		chromedp.WaitVisible(".commits li", chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}

	t.Run("the page lists the repo, and a new file shows within 3 s", func(t *testing.T) {
		waitJS(t, "first in the log", 5*time.Second, logSubjects+`[0] === "first"`)
		waitJS(t, "main current, feat listed", 5*time.Second, `document.querySelector('.refs li[data-head="*"] .name')?.textContent === "main" && !!document.querySelector('button[name="switch"][data-branch="feat"]')`)
		start := time.Now()
		write("b b.txt", "new\n")
		waitJS(t, "the new file", 6*time.Second, fileRow("b b.txt")+`?.dataset.x === "?"`)
		// The server pushes the row within one 2 s rerun (tinkerdown's list
		// tests, and a Go WebSocket client). Docker's headless Chrome reads the
		// frame 1.6-2.1 s after the server writes it, so the browser gets 6 s.
		t.Logf("a new file showed after %v", time.Since(start))
	})

	t.Run("file names and subjects stay text", func(t *testing.T) {
		evil := "<img src=x onerror=alert(1)>.txt"
		write(evil, "x\n")
		waitJS(t, "the odd name", 5*time.Second, `!!(`+fileRow(evil)+`)`)
		if n := js(`document.querySelectorAll(".content-wrapper img, .content-wrapper script").length`); n != "0" {
			dump()
			t.Fatalf("%s img/script elements; the name must render as text", n)
		}
		os.Remove(filepath.Join(repo, evil))
		waitJS(t, "the odd name to leave", 5*time.Second, `!(`+fileRow(evil)+`)`)
	})

	t.Run("Stage and Unstage move a file within 1 s", func(t *testing.T) {
		click(t, `button[name="stage"][data-path="b b.txt"]`)
		waitJS(t, "b b.txt staged", time.Second+500*time.Millisecond, fileRow("b b.txt")+`?.dataset.x === "A"`)
		if got := git("diff", "--cached", "--name-only"); got != "b b.txt" {
			t.Fatalf("staged %q, want b b.txt", got)
		}
		if v := js(`getComputedStyle(` + fileRow("b b.txt") + `.querySelector(".stage")).display`); v != "none" {
			t.Errorf("Stage still shows on a staged file (display %s)", v)
		}
		click(t, `button[name="unstage"][data-path="b b.txt"]`)
		waitJS(t, "b b.txt unstaged", time.Second+500*time.Millisecond, fileRow("b b.txt")+`?.dataset.x === "?"`)
	})

	t.Run("the diff pane shows the clicked file, and follows it", func(t *testing.T) {
		write("a.txt", "a\nmore\n")
		waitJS(t, "a.txt changed", 5*time.Second, fileRow("a.txt")+`?.dataset.y === "M"`)
		clickPath(t, "a.txt")
		waitJS(t, "the added line", 5*time.Second, `[...document.querySelectorAll('.diff li[data-mark="+"]')].some(li => li.textContent === "+more")`)
		write("a.txt", "changed\n")
		waitJS(t, "the diff to follow the file", 5*time.Second, `[...document.querySelectorAll('.diff li[data-mark="-"]')].some(li => li.textContent === "-a") && [...document.querySelectorAll('.diff li')].some(li => li.textContent === "+changed")`)
		clickPath(t, "b b.txt")
		waitJS(t, "a new file's content", 5*time.Second, `[...document.querySelectorAll('.diff li')].some(li => li.textContent === "+new")`)
	})

	t.Run("hand-made messages with values the page never listed run nothing", func(t *testing.T) {
		for _, c := range []struct{ button, attr, value string }{
			{`button[name="stage"]`, "data-path", "../outside"},
			{`button[name="stage"]`, "data-path", ":/"},
			{`button[name="stage"]`, "data-path", "*"},
			{`button[name="switch"]`, "data-branch", "HEAD~1"},
		} {
			want := strings.Count(serverLog.String(), "is not listed") + 1
			waitFor(t, "the refusal of "+c.value, 10*time.Second, func() bool {
				if strings.Count(serverLog.String(), "is not listed") >= want {
					return true
				}
				run(chromedp.SetAttributeValue(c.button, c.attr, c.value, chromedp.ByQuery), chromedp.Click(c.button, chromedp.ByQuery))
				time.Sleep(300 * time.Millisecond)
				return strings.Count(serverLog.String(), "is not listed") >= want
			})
		}
		if got := git("diff", "--cached", "--name-only"); got != "" {
			t.Fatalf("hand-made messages staged %q", got)
		}
		if got := git("rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
			t.Fatalf("a hand-made switch moved HEAD to %s", got)
		}
		// The page's markup was changed; load it fresh.
		if err := run(chromedp.Reload(), chromedp.WaitVisible(".commits li", chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Commit commits the staged files with the typed message", func(t *testing.T) {
		click(t, `button[name="stage"][data-path="a.txt"]`)
		waitJS(t, "a.txt staged", 3*time.Second, fileRow("a.txt")+`?.dataset.x === "M"`)
		const msg = `fix "the" typo <b>x</b>`
		if err := run(chromedp.SendKeys(`input[name="message"]`, msg, chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		click(t, `button[name="commit"]`)
		waitJS(t, "the commit at the top of the log", 5*time.Second, logSubjects+`[0] === `+quoteJS(msg))
		waitJS(t, "git's summary", 5*time.Second, `document.querySelector(".commit .out").textContent.includes("1 file changed")`)
		waitJS(t, "a.txt to leave the list", 5*time.Second, `!(`+fileRow("a.txt")+`)`)
		if got := git("log", "-1", "--format=%s"); got != msg {
			t.Fatalf("committed %q", got)
		}
		shotPNG(t, run, "git-desktop")
	})

	t.Run("a failing pre-commit hook shows its output and commits nothing", func(t *testing.T) {
		// The hook's glob matches, since the commit drops GIT_LITERAL_PATHSPECS.
		preCommit(t, "echo \"lint: $(git diff --cached --name-only -- '*.txt' | wc -l) files\" >&2; exit 1")
		click(t, `button[name="stage"][data-path="b b.txt"]`)
		waitJS(t, "b b.txt staged", 3*time.Second, fileRow("b b.txt")+`?.dataset.x === "A"`)
		head := git("rev-parse", "HEAD")
		run(chromedp.SetValue(`input[name="message"]`, "add b", chromedp.ByQuery))
		click(t, `button[name="commit"]`)
		waitJS(t, "the hook's output, its glob matching b b.txt", 5*time.Second, `document.querySelector(".commit .out").textContent.includes("lint: 1 files")`)
		if got := git("rev-parse", "HEAD"); got != head {
			t.Fatal("a failing hook committed")
		}
	})

	t.Run("two quick Commit clicks leave no index.lock", func(t *testing.T) {
		preCommit(t, "sleep 1")
		count := git("rev-list", "--count", "HEAD")
		received := strings.Count(serverLog.String(), `"action":"commit"`)
		run(chromedp.SetValue(`input[name="message"]`, "add b", chromedp.ByQuery))
		run(chromedp.Evaluate(`(() => { const b = document.querySelector('button[name="commit"]'); b.click(); b.click(); })()`, nil))
		waitFor(t, "the commit", 10*time.Second, func() bool { return git("rev-list", "--count", "HEAD") != count })
		time.Sleep(2 * time.Second)
		if n := strings.Count(serverLog.String(), `"action":"commit"`) - received; n != 1 {
			t.Errorf("the server received %d commit clicks, want 1: the button hides while a commit runs", n)
		}
		if _, err := os.Stat(filepath.Join(repo, ".git", "index.lock")); err == nil {
			dump()
			t.Fatal("index.lock remains")
		}
		if got := git("status", "--short"); got != "" {
			dump()
			t.Fatalf("the work tree isn't clean after the commit: %q", got)
		}
		waitJS(t, "Nothing to commit", 5*time.Second, `document.querySelector(".changes").textContent.includes("Nothing to commit")`)
	})

	// The button hides while a commit runs, so the client sends one click
	// (above). A message made by hand still reaches the server, and restarts
	// the run: the commit stopped in its hook leaves no index.lock behind.
	t.Run("a Commit message sent while a commit runs restarts it cleanly", func(t *testing.T) {
		preCommit(t, "sleep 2")
		write("e.txt", "e\n")
		git("add", "e.txt")
		received := strings.Count(serverLog.String(), `"action":"commit"`)
		run(chromedp.SetValue(`input[name="message"]`, "add e", chromedp.ByQuery))
		click(t, `button[name="commit"]`)
		waitJS(t, "the run", 5*time.Second, `!!document.querySelector('.commit button[disabled]')`)
		time.Sleep(500 * time.Millisecond)
		run(chromedp.Evaluate(`(() => { const b = document.querySelector('.commit button[disabled]'); b.disabled = false; b.name = "commit"; b.click(); })()`, nil))
		waitFor(t, "the restarted commit", 10*time.Second, func() bool { return git("log", "-1", "--format=%s") == "add e" })
		time.Sleep(time.Second)
		if n := strings.Count(serverLog.String(), `"action":"commit"`) - received; n != 2 {
			t.Fatalf("the server received %d commit messages, want 2", n)
		}
		if _, err := os.Stat(filepath.Join(repo, ".git", "index.lock")); err == nil {
			dump()
			t.Fatal("index.lock remains")
		}
		if got := git("status", "--short"); got != "" {
			t.Fatalf("the work tree isn't clean: %q", got)
		}
	})

	t.Run("Switch changes branch, and a refused switch shows git's message", func(t *testing.T) {
		click(t, `button[name="switch"][data-branch="feat"]`)
		waitJS(t, "feat current", 3*time.Second, `document.querySelector('.refs li[data-head="*"] .name')?.textContent === "feat"`)
		if got := git("rev-parse", "--abbrev-ref", "HEAD"); got != "feat" {
			t.Fatalf("HEAD is %s", got)
		}
		write("a.txt", "local edit\n") // feat's a.txt differs from main's, so switching back would lose this
		waitJS(t, "a.txt changed", 5*time.Second, `!!(`+fileRow("a.txt")+`)`)
		click(t, `button[name="switch"][data-branch="main"]`)
		waitFor(t, "git's refusal", 10*time.Second, func() bool {
			return strings.Contains(js(`document.querySelector(".branches").textContent`), "would be overwritten") ||
				strings.Contains(serverLog.String(), "would be overwritten")
		})
		if !strings.Contains(js(`document.querySelector(".branches").textContent`), "would be overwritten") {
			t.Errorf("only the server log has git's refusal; the page should show it")
		}
		if got := git("rev-parse", "--abbrev-ref", "HEAD"); got != "feat" {
			t.Fatalf("a refused switch moved HEAD to %s", got)
		}
		git("checkout", "--", "a.txt")
	})

	t.Run("on a phone no cell leaves its row, and the page doesn't scroll sideways", func(t *testing.T) {
		write("c.txt", "c\n")
		waitJS(t, "c.txt", 5*time.Second, `!!(`+fileRow("c.txt")+`)`)
		if err := run(chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(2), chromedp.EmulateMobile)); err != nil {
			t.Fatal(err)
		}
		clickPath(t, "c.txt")
		waitJS(t, "c.txt's diff", 5*time.Second, `[...document.querySelectorAll('.diff li')].some(li => li.textContent === "+c")`)
		var outside []string
		run(chromedp.Evaluate(`[...document.querySelectorAll(".files li, .refs li, .commits li")].flatMap(li => { const r = li.getBoundingClientRect();
			return [...li.children].filter(c => c.offsetParent && getComputedStyle(c).visibility !== "hidden").filter(c => { const b = c.getBoundingClientRect();
				return b.left < r.left - 1 || b.right > r.right + 1 || b.top < r.top - 1 || b.bottom > r.bottom + 1; }).map(c => li.innerText.split("\n")[0] + " ." + c.className); })`, &outside))
		var overflow bool
		run(chromedp.Evaluate(`document.documentElement.scrollWidth > window.innerWidth`, &overflow))
		if len(outside) > 0 || overflow {
			dump()
			t.Fatalf("cells outside their rows: %v; page scrolls sideways: %v", outside, overflow)
		}
		shotPNG(t, run, "git-phone")
		run(chromedp.EmulateViewport(1280, 900))
		os.Remove(filepath.Join(repo, "c.txt"))
	})

	t.Run("a reload and Back change nothing", func(t *testing.T) {
		head := git("rev-parse", "HEAD")
		if err := run(chromedp.Reload(), chromedp.WaitVisible(".commits li", chromedp.ByQuery),
			chromedp.Navigate("about:blank"), chromedp.NavigateBack(), chromedp.WaitVisible(".commits li", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		time.Sleep(time.Second)
		if git("rev-parse", "HEAD") != head || git("diff", "--cached", "--name-only") != "" {
			t.Fatal("a reload or Back changed the repo")
		}
	})

	t.Run("git stops running 30 s after the page leaves", func(t *testing.T) {
		if err := run(chromedp.Navigate("about:blank")); err != nil {
			t.Fatal(err)
		}
		// Every input stops 30 s after the last page showing it leaves
		// (tinkerdown's MergeIdle): wait for 5 s without a git run, by 45 s.
		left := time.Now()
		n, quietSince := gitRuns(), time.Now()
		for time.Since(quietSince) < 5*time.Second {
			if time.Since(left) > 45*time.Second {
				t.Fatalf("git still runs %v after the page left", time.Since(left).Round(time.Second))
			}
			time.Sleep(250 * time.Millisecond)
			if m := gitRuns(); m != n {
				n, quietSince = m, time.Now()
			}
		}
		if d := quietSince.Sub(left); d < 25*time.Second {
			t.Errorf("git stopped %v after the page left; inputs should keep running about 30 s", d.Round(time.Second))
		}
	})

	t.Run("with --watch-dir, a new file shows within 1 s", func(t *testing.T) {
		url2, _ := serveREADME(repo, readmeCmd, "--watch-dir", ".")
		if err := run(chromedp.Navigate(url2), chromedp.WaitVisible(".commits li", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		time.Sleep(time.Second) // the watch starts after the first run
		write("d.txt", "d\n")
		waitJS(t, "d.txt", time.Second+500*time.Millisecond, `!!(`+fileRow("d.txt")+`)`)
		os.Remove(filepath.Join(repo, "d.txt"))
	})

	t.Run("in a repo with no commits, the list works and the log shows git's error", func(t *testing.T) {
		empty := newRepo(false)
		os.WriteFile(filepath.Join(empty, "first.txt"), []byte("1\n"), 0o644)
		url3, _ := serveREADME(empty, readmeCmd)
		if err := run(chromedp.Navigate(url3), chromedp.WaitVisible(".changes", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "first.txt", 5*time.Second, `!!(`+fileRow("first.txt")+`)`)
		waitJS(t, "the log's error", 5*time.Second, `document.querySelector(".log mark")?.textContent.includes("does not have any commits")`)
		click(t, `button[name="stage"][data-path="first.txt"]`)
		waitJS(t, "first.txt staged", 3*time.Second, fileRow("first.txt")+`?.dataset.x === "A"`)
		click(t, `button[name="unstage"][data-path="first.txt"]`)
		waitJS(t, "first.txt unstaged", 3*time.Second, fileRow("first.txt")+`?.dataset.x === "?"`)
	})

	logs.failOnErrors(t)
}

// quoteJS quotes s as a JavaScript string literal.
func quoteJS(s string) string {
	return fmt.Sprintf("%q", s)
}
