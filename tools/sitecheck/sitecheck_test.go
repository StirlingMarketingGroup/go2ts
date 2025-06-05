// tools/sitecheck/sitecheck_test.go
package sitecheck

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// --- helpers ---------------------------------------------------------------
func listPages() ([]string, error) {
	// Prefer sitemap if it exists
	data, err := os.ReadFile("public/sitemap.xml")
	if err != nil {
		slog.Warn("sitemap.xml not found, falling back to file system", "error", err)
		return filepath.Glob("public/**/*.html") // fallback: walk disk
	}
	slog.Info("using sitemap.xml to list pages", "count", len(data))
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
		return nil, err
	}

	var cfg HugoConfig
	if _, err := toml.DecodeFile("./hugo.toml", &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse hugo.toml: %v", err)
	}

	out := make([]string, 0, len(sm.URLs))
	for _, u := range sm.URLs {
		u := strings.TrimPrefix(u.Loc, cfg.BaseURL) // remove base URL prefix

		slog.Info("found URL in sitemap", "url", u)
		out = append(out, u)
	}
	return out, nil
}

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

	pages, err := listPages()
	if err != nil {
		t.Fatalf("listing pages: %v", err)
	}

	// 1) serve static output
	srv := &http.Server{Addr: ":3000", Handler: http.FileServer(http.Dir("public"))}
	go srv.ListenAndServe()
	defer srv.Shutdown(context.Background())

	baseURL := "http://localhost:3000/"

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
			chromedp.Navigate(baseURL+url),
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
