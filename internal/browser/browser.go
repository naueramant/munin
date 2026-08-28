package browser

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type Browser struct {
	Context context.Context
	Tabs    []*Tab

	Close func()
}

func NewBrowser(extraFlags ...string) *Browser {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("start-fullscreen", true),
		chromedp.Flag("kiosk", true),
		chromedp.Flag("noerrdialogs", true),
		chromedp.Flag("disable-session-crashed-bubble", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("check-for-update-interval", "31536000"),
		chromedp.Flag("user-data-dir", filepath.Join(os.TempDir(), "chromium-munin")),
		chromedp.Flag("incognito", true),
	)

	for _, flag := range extraFlags {
		opts = append(opts, chromedp.Flag(flag, true))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

	return &Browser{
		Context: allocCtx,
		Close:   cancel,
	}
}

func (b *Browser) NewTab() *Tab {
	t := newTab(b)

	chromedp.ListenTarget(t.Context, func(ev interface{}) {
		if e, ok := ev.(*network.EventLoadingFailed); ok {
			if e.Type == network.ResourceTypeDocument {
				slog.Warn("Tab failed to load document, scheduling retry", "retry_in", FailedLoadReloadDelay)
				go t.delayedReload()
			}
		}

		if _, ok := ev.(*target.EventTargetDestroyed); ok {
			b.removeTab(t)
		}
	})

	origClose := t.Close
	t.Close = func() {
		b.removeTab(t)
		if origClose != nil {
			origClose()
		}
	}

	b.Tabs = append(b.Tabs, t)
	return t
}

func (b *Browser) removeTab(t *Tab) {
	for i, tab := range b.Tabs {
		if tab.Context == t.Context {
			b.Tabs = append(b.Tabs[:i], b.Tabs[i+1:]...)
			return
		}
	}
}
