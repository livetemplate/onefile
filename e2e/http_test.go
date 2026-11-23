package e2e

// End-to-end test of the http app (../http): runs the serve commands from its
// README through sh, as a user pastes them, with the page served from a URL
// and the fixture .http file pointed at a local API server that records every
// request. Drives the page in Docker Chrome. No test uses the network.

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// apiHit is one request the API server received.
type apiHit struct {
	method, path, body string
	header             http.Header
}

func TestHTTP(t *testing.T) {
	bin := buildTinkerdown(t)
	pageMD, err := os.ReadFile("../http/index.md")
	if err != nil {
		t.Fatal(err)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(pageMD) }))
	defer pages.Close()

	var mu sync.Mutex
	var hits []apiHit
	var api *httptest.Server
	api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		hits = append(hits, apiHit{r.Method, r.URL.Path, string(body), r.Header.Clone()})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/users" && r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":42,"url":"%s/users/42","note":"<script>alert(1)</script>","img":"<img src=x onerror=alert(1)>","link":"javascript:alert(1)"}`, api.URL)
		case r.URL.Path == "/users":
			fmt.Fprint(w, `[{"id":1,"name":"Ada"}]`)
		case r.URL.Path == "/big": // about 200 KiB of minified JSON, on one line
			fmt.Fprint(w, `{"items":[`)
			for i := 0; i < 4000; i++ {
				fmt.Fprintf(w, `{"n":%d,"s":"%s"},`, i, strings.Repeat("x", 40))
			}
			fmt.Fprint(w, `{"last":true}]}`)
		case r.URL.Path == "/redirect":
			http.Redirect(w, r, "/target", http.StatusFound)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer api.Close()
	count := func(method, path string) int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, h := range hits {
			if h.method == method && h.path == path {
				n++
			}
		}
		return n
	}
	total := func() int { mu.Lock(); defer mu.Unlock(); return len(hits) }
	last := func(method, path string) apiHit {
		mu.Lock()
		defer mu.Unlock()
		for i := len(hits) - 1; i >= 0; i-- {
			if hits[i].method == method && hits[i].path == path {
				return hits[i]
			}
		}
		return apiHit{}
	}

	// The user's directory: their .http file, and the token in the
	// environment.
	const token = "s3cret-ofe2e-token"
	dir := t.TempDir()
	fixture, err := os.ReadFile("testdata/http/api.http")
	if err != nil {
		t.Fatal(err)
	}
	httpFile := replaceOnce(t, string(fixture), "@host = HOST", "@host = "+api.URL)
	apiPath := filepath.Join(dir, "api.http")
	writeFile := func(text string) {
		t.Helper()
		if err := os.WriteFile(apiPath, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(httpFile)
	serveREADME := func(cmdline string) (string, *syncBuffer) {
		t.Helper()
		port := freePort(t)
		cmdline = replaceOnce(t, cmdline, "tinkerdown serve", bin+" serve --port "+fmt.Sprint(port))
		cmdline = replaceOnce(t, cmdline, "https://raw.githubusercontent.com/livetemplate/onefile/main/http/index.md", pages.URL+"/index.md")
		cmd := exec.Command("sh", "-c", "exec "+cmdline)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "OFE2E_TOKEN="+token)
		return serveCmd(t, cmd, port)
	}
	url, serverLog := serveREADME(readmeCommandOf(t, "../http/README.md"))

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
	const resp = `document.querySelector('[data-testid="response"]')`
	// send presses a request's Send and waits until a new run's summary line
	// for url has arrived and the run has ended. Rows are keyed by seq, which
	// never repeats, so the rows already shown mark the old runs. The summary
	// is hidden by CSS, so it is matched by textContent, as jsonindent
	// prints it.
	send := func(t *testing.T, entry, url string) {
		t.Helper()
		if err := run(chromedp.Evaluate(`window.oldRows = new Set([...`+resp+`.children].map(li => li.dataset.key))`, nil),
			chromedp.Click(`[data-testid="send-`+entry+`"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatalf("click Send %s: %v", entry, err)
		}
		waitJS(t, "the response to "+url, `[...`+resp+`.children].some(li => li.dataset.exit !== "" && !oldRows.has(li.dataset.key) && li.textContent.includes('"url_effective": "`+url+`"')) && !document.querySelector('[data-testid="state"][aria-busy="true"]')`)
	}
	respText := func() string {
		var s string
		run(chromedp.Evaluate(resp+`.innerText`, &s))
		return s
	}

	if err := chromedp.Run(ctx, network.Enable(), chromedp.EmulateViewport(1280, 900), chromedp.Navigate(url),
		chromedp.WaitVisible(`[data-testid="send-6"]`, chromedp.ByQuery)); err != nil {
		dump()
		t.Fatal(err)
	}

	t.Run("the list shows the file's requests, and opening the page sends nothing", func(t *testing.T) {
		var names string
		run(chromedp.Evaluate(`[...document.querySelectorAll(".reqs .name")].map(n => n.textContent).join(",")`, &names))
		if names != "List users,Create user,Big,Redirect,Unset,Refused" {
			dump()
			t.Fatalf("requests = %s", names)
		}
		var urls string
		run(chromedp.Evaluate(`[...document.querySelectorAll(".reqs .url")].map(n => n.textContent).join(" ")`, &urls))
		if urls != "{{host}}/users {{host}}/users {{host}}/big {{host}}/redirect {{host}}/users/{{nope}} http://127.0.0.1:1/" {
			dump()
			t.Fatalf("urls = %q, want each as written", urls)
		}
		var sent string
		run(chromedp.Evaluate(`document.querySelector('[data-testid="req-2"] .sent pre').textContent`, &sent))
		if !strings.Contains(sent, "Authorization: Bearer {{token}}") || strings.Contains(sent, "a comment") {
			dump()
			t.Fatalf("request 2 shows %q; want the header as written, without the comment", sent)
		}
		time.Sleep(time.Second)
		if n := total(); n != 0 {
			t.Fatalf("the API got %d requests before any Send", n)
		}
	})

	t.Run("Send on a POST shows the status, headers, time and indented JSON", func(t *testing.T) {
		send(t, "2", api.URL+"/users")
		got := respText()
		for _, want := range []string{"HTTP/1.1 201 Created", "Content-Type: application/json", "\"id\": 42,", "201 · 0.0"} {
			if !strings.Contains(got, want) {
				dump()
				t.Fatalf("response lacks %q:\n%s", want, got)
			}
		}
		if n := count("POST", "/users"); n != 1 {
			t.Fatalf("the API got %d POSTs, want 1", n)
		}
		h := last("POST", "/users")
		if h.body != "{\n  \"name\": \"Ada <script>alert(1)</script>\"\n}" || h.header.Get("Authorization") != "Bearer "+token || h.header.Get("Content-Type") != "application/json" {
			t.Fatalf("the POST arrived as body %q, Authorization %q, Content-Type %q", h.body, h.header.Get("Authorization"), h.header.Get("Content-Type"))
		}
		// The summary row shows first, though curl prints it last.
		var first string
		run(chromedp.Evaluate(`[...`+resp+`.children].sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)[0].dataset.exit`, &first))
		if first != "0" {
			dump()
			t.Fatalf("the top row's exitcode = %q, want curl's summary on top", first)
		}
		shotPNG(t, run, "http-desktop")
	})

	t.Run("response text stays text, and only http links are links", func(t *testing.T) {
		var tags, bad, links int
		run(chromedp.Evaluate(resp+`.querySelectorAll("script, img").length`, &tags),
			chromedp.Evaluate(resp+`.querySelectorAll('a[href^="javascript"]').length`, &bad),
			chromedp.Evaluate(resp+`.querySelectorAll('a[href="`+api.URL+`/users/42"]').length`, &links))
		got := respText()
		if tags != 0 || bad != 0 || links != 1 || !strings.Contains(got, "<script>alert(1)</script>") || !strings.Contains(got, "javascript:alert(1)") {
			dump()
			t.Fatalf("script/img elements %d, javascript: links %d, links to the user %d; text:\n%s", tags, bad, links, got)
		}
	})

	t.Run("the token never reaches the browser", func(t *testing.T) {
		logs.mu.Lock()
		frames := strings.Join(logs.recv, "\n")
		logs.mu.Unlock()
		var html string
		run(chromedp.OuterHTML("html", &html, chromedp.ByQuery))
		if strings.Contains(frames, token) || strings.Contains(html, token) {
			t.Fatal("the token is in a WebSocket frame or the page's HTML")
		}
	})

	t.Run("Send on a GET sends no body", func(t *testing.T) {
		send(t, "1", api.URL+"/users")
		h := last("GET", "/users")
		if h.body != "" || h.header.Get("Content-Length") != "" || h.header.Get("Accept") != "application/json" {
			t.Fatalf("the GET arrived with body %q, Content-Length %q, Accept %q", h.body, h.header.Get("Content-Length"), h.header.Get("Accept"))
		}
		if got := respText(); !strings.Contains(got, "HTTP/1.1 200 OK") || strings.Contains(got, "201") {
			dump()
			t.Fatalf("the pane doesn't show only the GET's response:\n%s", got)
		}
	})

	t.Run("a 200 KiB minified JSON body arrives whole and indented", func(t *testing.T) {
		send(t, "3", api.URL+"/big")
		got := respText()
		if !strings.Contains(got, "\"n\": 3999,") || !strings.Contains(got, "\"last\": true") || strings.Count(got, "\n") < 4000 {
			dump()
			t.Fatalf("the big body isn't whole and indented: %d bytes, %d lines", len(got), strings.Count(got, "\n"))
		}
	})

	t.Run("a redirect isn't followed", func(t *testing.T) {
		send(t, "4", api.URL+"/redirect")
		if !strings.Contains(respText(), "HTTP/1.1 302 Found") || count("GET", "/target") != 0 {
			dump()
			t.Fatalf("the redirect target got %d requests, want 0", count("GET", "/target"))
		}
	})

	t.Run("an unset variable sends nothing and names it", func(t *testing.T) {
		before := total()
		if err := run(chromedp.Click(`[data-testid="send-5"]`, chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "the refusal", `(document.querySelector('[data-testid="resp-error"]')?.textContent || "").includes("unset variable nope")`)
		time.Sleep(500 * time.Millisecond)
		if n := total(); n != before {
			t.Fatalf("the API got %d requests after a refused Send", n-before)
		}
	})

	t.Run("a refused connection shows curl's error", func(t *testing.T) {
		send(t, "6", "http://127.0.0.1:1/")
		if got := respText(); !strings.Contains(got, "Failed to connect") || !strings.Contains(got, "0 · 0.0") {
			dump()
			t.Fatalf("the pane doesn't show curl's error:\n%s", got)
		}
	})

	t.Run("a hand-made click on an unlisted entry sends nothing", func(t *testing.T) {
		before := total()
		if err := run(chromedp.Evaluate(`document.querySelector('[data-testid="send-1"]').dataset.entry = "99"`, nil),
			chromedp.Click(`[data-testid="send-1"]`, chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for !strings.Contains(serverLog.String(), "value entry is not listed") && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if !strings.Contains(serverLog.String(), "value entry is not listed") {
			dump()
			t.Fatal("the server log doesn't show the refusal")
		}
		if n := total(); n != before {
			t.Fatalf("the API got %d requests after a hand-made click", n-before)
		}
		run(chromedp.Evaluate(`document.querySelector('[data-testid="send-1"]').dataset.entry = "1"`, nil))
	})

	t.Run("a reload and Back send nothing more", func(t *testing.T) {
		before := total()
		if err := run(chromedp.Reload(), chromedp.WaitVisible(`[data-testid="send-6"]`, chromedp.ByQuery),
			chromedp.Navigate("about:blank"), chromedp.NavigateBack(), chromedp.WaitVisible(`[data-testid="send-6"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		time.Sleep(1500 * time.Millisecond)
		if n := total(); n != before {
			t.Fatalf("the API got %d requests from a reload and Back", n-before)
		}
	})

	t.Run("search narrows by method", func(t *testing.T) {
		if err := run(chromedp.SendKeys(`[data-testid="search"]`, "POST", chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		waitJS(t, "only Create user", `[...document.querySelectorAll(".reqs .name")].map(n => n.textContent).join() === "Create user"`)
		if err := run(chromedp.SendKeys(`[data-testid="search"]`, strings.Repeat(kb.Backspace, 4), chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		waitJS(t, "every request back", `document.querySelectorAll(".reqs li").length === 6`)
	})

	t.Run("editing the file updates the list", func(t *testing.T) {
		writeFile(httpFile + "\n### Seventh\nDELETE {{host}}/users/1\n")
		start := time.Now()
		waitJS(t, "a seventh request", `!!document.querySelector('[data-testid="send-7"]')`)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("the new request took %v to show", d)
		}
		writeFile(httpFile + "\n### Broken\nGET http://x\nnot a header\n")
		waitJS(t, "the parse error", `/line \d+:/.test(document.querySelector('[data-testid="error"]')?.textContent || "")`)
		if err := run(chromedp.WaitVisible(`[data-testid="send-7"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal("a parse error dropped the last good list")
		}
		writeFile(httpFile)
		waitJS(t, "six requests and no error", `!document.querySelector('[data-testid="send-7"]') && !document.querySelector('[data-testid="error"]') && !!document.querySelector('[data-testid="send-6"]')`)
	})

	t.Run("on a phone no cell leaves its row", func(t *testing.T) {
		if err := run(chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(2), chromedp.EmulateMobile)); err != nil {
			t.Fatal(err)
		}
		send(t, "2", api.URL+"/users")
		var outside []string
		run(chromedp.Evaluate(`[...document.querySelectorAll(".reqs li, .body li")].flatMap(li => { const r = li.getBoundingClientRect();
			return [...li.children].filter(c => c.offsetParent).filter(c => { const b = c.getBoundingClientRect();
				return b.left < r.left - 1 || b.right > r.right + 1 || b.top < r.top - 1 || b.bottom > r.bottom + 1; }).map(c => li.dataset.key + " ." + c.className); })`, &outside))
		var overflow bool
		run(chromedp.Evaluate(`document.documentElement.scrollWidth > window.innerWidth`, &overflow))
		if len(outside) > 0 || overflow {
			dump()
			t.Fatalf("cells outside their rows: %v; page scrolls sideways: %v", outside, overflow)
		}
		shotPNG(t, run, "http-phone")
		run(chromedp.EmulateViewport(1280, 900))
	})

	t.Run("no curl remains after the page leaves", func(t *testing.T) {
		if err := run(chromedp.Navigate("about:blank")); err != nil {
			t.Fatal(err)
		}
		port := regexp.MustCompile(`:(\d+)$`).FindStringSubmatch(api.URL)[1]
		deadline := time.Now().Add(30 * time.Second)
		for exec.Command("pgrep", "-f", "curl.*127.0.0.1:"+port).Run() == nil {
			if time.Now().After(deadline) {
				t.Fatal("a curl the page started is still running 30 s after it left")
			}
			time.Sleep(500 * time.Millisecond)
		}
	})

	t.Run("the README's --config form lists the same requests and sends the same POST", func(t *testing.T) {
		readme, err := os.ReadFile("../http/README.md")
		if err != nil {
			t.Fatal(err)
		}
		yaml := regexp.MustCompile("(?s)```yaml\n(# http.yaml\n.*?)```").FindStringSubmatch(string(readme))
		cmds := regexp.MustCompile("(?s)```sh\n(tinkerdown serve [^\n]*--config http.yaml)\n```").FindStringSubmatch(string(readme))
		if yaml == nil || cmds == nil {
			t.Fatal("the README has no http.yaml block, or no serve command that uses it")
		}
		if err := os.WriteFile(filepath.Join(dir, "http.yaml"), []byte(yaml[1]), 0o644); err != nil {
			t.Fatal(err)
		}
		url2, _ := serveREADME(cmds[1])
		if err := run(chromedp.Navigate(url2), chromedp.WaitVisible(`[data-testid="send-6"]`, chromedp.ByQuery)); err != nil {
			dump()
			t.Fatal(err)
		}
		before := count("POST", "/users")
		send(t, "2", api.URL+"/users")
		if n := count("POST", "/users"); n != before+1 || last("POST", "/users").header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("the --config form sent %d POSTs, want 1, with the token", n-before)
		}
	})

	logs.mu.Lock()
	defer logs.mu.Unlock()
	for _, c := range logs.console {
		if strings.HasPrefix(c, "[error]") || strings.HasPrefix(c, "[exception]") {
			t.Errorf("browser console: %s", c)
		}
	}
}
