package browser

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type Browser struct {
	Context context.Context
	Tabs    []*Tab

	Close func()

	// browserCtx is the context of the first tab, which owns the live browser.
	// Additional tabs are created as its children so they join the same browser
	// process instead of spawning a new one.
	browserCtx context.Context
}

func NewBrowser(extraFlags ...string) *Browser {
	// A unique per-instance profile dir avoids Chromium's singleton lock: a
	// shared fixed dir makes a new instance forward to (and exit through) any
	// leftover or user-owned Chromium using the same profile, which crashes the
	// allocator with "Opening in existing browser session".
	userDataDir, err := os.MkdirTemp("", "chromium-munin-")
	if err != nil {
		userDataDir = filepath.Join(os.TempDir(), "chromium-munin")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-extensions", true),
		// --kiosk hides the toolbar/tab strip entirely, giving a true
		// fullscreen chrome-less window without relying on the window manager
		// to honor a fullscreen state request (it doesn't, on some setups).
		// It does appear to break Page.bringToFront for background tabs, which
		// is why the manager keeps and reuses a single display tab instead of
		// switching between several.
		chromedp.Flag("kiosk", true),
		chromedp.Flag("noerrdialogs", true),
		chromedp.Flag("disable-session-crashed-bubble", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("check-for-update-interval", "31536000"),
		// A dedicated temp profile dir already makes this a clean, throwaway
		// session; --incognito is omitted because it leaves the browser
		// without a regular BrowserContext, which makes Target.createTarget
		// for additional tabs fail with "no browser is open" (-32000).
		chromedp.Flag("user-data-dir", userDataDir),
	)

	if customPath := os.Getenv("CHROME_BIN"); customPath != "" {
		opts = append(opts, chromedp.ExecPath(customPath))
	} else if customPath := os.Getenv("CHROMIUM_PATH"); customPath != "" {
		opts = append(opts, chromedp.ExecPath(customPath))
	}

	for _, flag := range extraFlags {
		opts = append(opts, chromedp.Flag(flag, true))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

	return &Browser{
		Context: allocCtx,
		Close: func() {
			cancel()
			os.RemoveAll(userDataDir)
		},
	}
}

func (b *Browser) NewTab() *Tab {
	t := newTab(b, b.tabParent(), b.newTabOptions()...)

	chromedp.ListenTarget(t.Context, func(ev interface{}) {
		// EventTargetDestroyed fires for any target in the browser (service
		// workers, prerenders, etc.), not just this tab's own - it must be
		// filtered by TargetID, otherwise unrelated targets closing would
		// make this tab wrongly appear gone.
		e, ok := ev.(*target.EventTargetDestroyed)
		if !ok {
			return
		}
		c := chromedp.FromContext(t.Context)
		if c == nil || c.Target == nil || e.TargetID != c.Target.TargetID {
			return
		}
		b.removeTab(t)
	})

	origClose := t.Close
	t.Close = func() {
		b.removeTab(t)
		if origClose != nil {
			origClose()
		}
	}

	b.Tabs = append(b.Tabs, t)
	if b.browserCtx == nil {
		b.browserCtx = t.Context
	}
	return t
}

// tabParent returns the parent context for a new tab: the live browser's context
// once it exists, otherwise the allocator context to bootstrap the browser.
func (b *Browser) tabParent() context.Context {
	if b.browserCtx != nil {
		return b.browserCtx
	}
	return b.Context
}

// newTabOptions returns chromedp context options for a new tab. Browser-level
// options (like WithErrorf) are only valid when bootstrapping the browser, so
// they are omitted once a browser context already exists.
func (b *Browser) newTabOptions() []chromedp.ContextOption {
	if b.browserCtx != nil {
		return nil
	}
	return []chromedp.ContextOption{
		chromedp.WithErrorf(func(format string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(format, args...))
		}),
	}
}

func (b *Browser) removeTab(t *Tab) {
	for i, tab := range b.Tabs {
		if tab.Context == t.Context {
			b.Tabs = append(b.Tabs[:i], b.Tabs[i+1:]...)
			return
		}
	}
}
