// tools/sitecheck/sitecheck_test.go
package sitecheck

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// --- test entry ------------------------------------------------------------
func Test_NoRuntimeErrors(t *testing.T) {
	slog.Info("moving to git root")
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting current working directory: %v", err)
	}
	root, err := findGitRoot(pwd)
	if err != nil {
		t.Fatalf("finding git root: %v", err)
	}
	err = os.Chdir(root)
	if err != nil {
		t.Fatalf("changing directory to git root: %v", err)
	}

	slog.Info("changed directory to git root", "path", root)

	baseURL := "http://127.0.0.1:3000"

	// 1) serve static output
	cmd := exec.CommandContext(context.Background(),
		"hugo",
		"server",
		"--minify",
		"--quiet",
		"-p", "3000",
		"--bind", "127.0.0.1",
		"-b", baseURL,
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("hugo server: %v", err)
	}
	defer cmd.Process.Kill()

	// wait until the port answers
	waitFor := func() error {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := http.Get(baseURL + "/robots.txt"); err == nil {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		return fmt.Errorf("hugo server never became ready on :3000")
	}
	if err := waitFor(); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(baseURL + "/sitemap.xml")
	if err != nil {
		t.Fatalf("GET sitemap: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// URL represents one <url> entry in the sitemap.
	type URL struct {
		Loc string `xml:"loc"`
	}

	// URLSet wraps all <url> entries.
	// Note: Because the sitemap’s <urlset> uses a default namespace (xmlns="…"),
	// you must include that namespace URI in the struct tag in order for Unmarshal to match it.
	type URLSet struct {
		XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		URLs    []URL    `xml:"url"`
	}

	sm := URLSet{}
	if err := xml.Unmarshal(data, &sm); err != nil {
		t.Fatalf("unmarshal sitemap.xml: %v", err)
	}

	pages := make([]string, 0, len(sm.URLs))
	for _, e := range sm.URLs {
		slog.Info("found page in sitemap", "url", e.Loc)
		pages = append(pages, e.Loc)
	}

	// 2) headless Chrome context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	slog.Info("starting sitecheck", "pages", len(pages))

	for _, url := range pages {
		slog.Info("checking page", "url", url)

		var jsErrs, consoleErrs []string

		err := chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				chromedp.ListenTarget(ctx, func(ev any) {
					switch e := ev.(type) {
					case *runtime.EventExceptionThrown:
						d := e.ExceptionDetails
						// always include the top‐level message
						msg := d.Text
						if d.Exception != nil {
							// if there's an actual exception object, include its description
							msg = fmt.Sprintf("%s: %s", d.Text, d.Exception.Description)
						}

						if d.StackTrace != nil {
							// walk the stack frames
							for _, frame := range d.StackTrace.CallFrames {
								// CDP’s CallFrame.LineNumber is 0‐based, so you may want to +1 for readability
								msg += fmt.Sprintf(
									"\n\tat %s (%s:%d:%d)",
									frame.FunctionName,
									frame.URL,
									frame.LineNumber+1,
									frame.ColumnNumber+1,
								)
							}
						}
						jsErrs = append(jsErrs, msg)
					case *log.EventEntryAdded:
						// console.error, etc.
						if e.Entry.Level == "error" {
							// Some console errors may also include a stack trace
							// in e.Entry.StackTrace (if available), but it’s often nil.
							// You can at least grab URL + line:
							consoleErrs = append(
								consoleErrs,
								fmt.Sprintf(
									"%s (%s:%d)",
									e.Entry.Text,
									e.Entry.URL,
									e.Entry.LineNumber,
								),
							)
						}
					}
				})
				return nil
			}),
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("navigating %s: %v", url, err)
		}
		if len(jsErrs)+len(consoleErrs) > 0 {
			t.Errorf("runtime errors on %s\n  JS: %v\n  console: %v", url, jsErrs, consoleErrs)
		}
	}
}

// only need BaseURL here—any other keys in hugo.toml are ignored
type HugoConfig struct {
	BaseURL string `toml:"baseURL"`
}

func findGitRoot(start string) (string, error) {
	dir := start
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			return "", fmt.Errorf("no .git directory found")
		}
		dir = parent
	}
}
