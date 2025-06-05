// tools/sitecheck/sitecheck_test.go
package sitecheck

import (
	"context"
	"encoding/xml"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// --- helpers ---------------------------------------------------------------
func listPages() ([]string, error) {
	// Prefer sitemap if it exists
	data, err := os.ReadFile("public/sitemap.xml")
	if err != nil {
		return filepath.Glob("public/**/*.html") // fallback: walk disk
	}
	var sm struct {
		URLs []struct {
			Loc string `xml:"loc"`
		}
	}
	if err := xml.Unmarshal(data, &sm); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sm.URLs))
	for _, u := range sm.URLs {
		out = append(out, u.Loc)
	}
	return out, nil
}

// --- test entry ------------------------------------------------------------
func Test_NoRuntimeErrors(t *testing.T) {
	pages, err := listPages()
	if err != nil {
		t.Fatalf("listing pages: %v", err)
	}

	// 1) serve static output
	srv := &http.Server{Addr: ":1313", Handler: http.FileServer(http.Dir("public"))}
	go srv.ListenAndServe()
	defer srv.Shutdown(context.Background())

	// 2) headless Chrome context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	ctx, _ = context.WithTimeout(ctx, 30*time.Second)

	for _, url := range pages {
		var jsErrs, consoleErrs []string

		err := chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				chromedp.ListenTarget(ctx, func(ev any) {
					switch e := ev.(type) {
					case *runtime.EventExceptionThrown:
						jsErrs = append(jsErrs, e.ExceptionDetails.Text)
					case *log.EventEntryAdded:
						if e.Entry.Level == "error" {
							consoleErrs = append(consoleErrs, e.Entry.Text)
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
