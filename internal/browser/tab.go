package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var FailedLoadReloadDelay = time.Second * 10

type Tab struct {
	Browser *Browser
	Context context.Context

	Close func()

	isReloading bool

	css string
	js  string
}

type BasicAuthCredentials struct {
	Username string
	Password string
}

func newTab(b *Browser) *Tab {
	ctx, closeFunc := chromedp.NewContext(b.Context,
		chromedp.WithErrorf(func(format string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(format, args...))
		}),
	)

	t := &Tab{
		Browser: b,
		Context: ctx,
		Close:   closeFunc,
	}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		// When the page is done loading, inject the CSS and JS
		if _, ok := ev.(*page.EventFrameStoppedLoading); ok {
			go t.injectCSS()
			go t.injectJS()
		}
	})

	return t
}

func (t *Tab) Focus() {
	if err := chromedp.Run(t.Context, page.BringToFront()); err != nil {
		slog.Debug("Failed to bring tab to front", "error", err)
	}
}

func (t *Tab) Reload() {
	if err := chromedp.Run(t.Context, chromedp.Reload()); err != nil {
		slog.Debug("Failed to reload tab", "error", err)
	}
}

func (t *Tab) Navigate(url string) {
	if err := chromedp.Run(t.Context, network.Enable(), chromedp.Navigate(url)); err != nil {
		slog.Warn("Failed to navigate tab", "url", url, "error", err)
	}
}

func (t *Tab) NavigateWithBasicAuth(url string, creds BasicAuthCredentials) {
	x := base64.StdEncoding.EncodeToString([]byte(creds.Username + ":" + creds.Password))
	headers := network.Headers{"Authorization": "Basic " + x}

	if err := chromedp.Run(
		t.Context,
		network.Enable(),
		network.SetExtraHTTPHeaders(headers),
		chromedp.Navigate(url),
	); err != nil {
		slog.Warn("Failed to navigate tab with basic auth", "url", url, "error", err)
	}
}

func (t *Tab) SetCSS(css string) {
	t.css = css
	t.injectCSS()
}

func (t *Tab) injectCSS() {
	if t.css == "" {
		return
	}
	script := `
	(() => {
		const style = document.createElement('style');
		style.type = 'text/css';
		style.appendChild(document.createTextNode(` + "`" + t.css + "`" + `));
		document.head.appendChild(style);
	})()
	`
	var executed *runtime.RemoteObject
	_ = chromedp.Run(
		t.Context,
		chromedp.EvaluateAsDevTools(script, &executed),
	)
}

func (t *Tab) SetJS(js string) {
	t.js = js
	t.injectJS()
}

func (t *Tab) injectJS() {
	if t.js == "" {
		return
	}
	var executed *runtime.RemoteObject
	_ = chromedp.Run(
		t.Context,
		chromedp.EvaluateAsDevTools(t.js, &executed),
	)
}

func (t *Tab) delayedReload() {
	if !t.isReloading {
		t.isReloading = true
		time.Sleep(FailedLoadReloadDelay)
		t.isReloading = false
		t.Reload()
	}
}
