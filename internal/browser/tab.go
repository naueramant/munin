package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var FailedLoadReloadDelay = time.Second * 10

// actionTimeout bounds every chromedp call on a tab so a busy or unresponsive
// page (e.g. a live-updating dashboard) can never block a caller forever.
// Several of these calls run while BrowserManager's mutex is held, so a hang
// here would silently freeze the whole manager (rotation, reloads, config
// reloads) with no error logged.
var actionTimeout = 10 * time.Second

// runWithTimeout runs a chromedp action bounded by actionTimeout.
func runWithTimeout(parent context.Context, actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(parent, actionTimeout)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

type Tab struct {
	Browser *Browser
	Context context.Context

	Close func()

	isReloading bool

	// onDocLoadFailed is invoked when the top-level document fails to load.
	onDocLoadFailed func(errText string)

	css  string
	js   string
	zoom float64
}

type BasicAuthCredentials struct {
	Username string
	Password string
}

func newTab(b *Browser, parent context.Context, opts ...chromedp.ContextOption) *Tab {
	ctx, closeFunc := chromedp.NewContext(parent, opts...)

	t := &Tab{
		Browser: b,
		Context: ctx,
		Close:   closeFunc,
	}

	t.onDocLoadFailed = t.defaultLoadFailedHandler()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		// When the page is done loading, inject the CSS and JS
		if _, ok := ev.(*page.EventFrameStoppedLoading); ok {
			go t.injectCSS()
			go t.injectJS()
			go t.injectZoom()
		}

		if e, ok := ev.(*network.EventLoadingFailed); ok {
			if e.Type == network.ResourceTypeDocument && !e.Canceled && e.ErrorText != "net::ERR_ABORTED" {
				if cb := t.onDocLoadFailed; cb != nil {
					go cb(e.ErrorText)
				}
			}
		}
	})

	return t
}

// reset clears per-navigation state so a reused tab starts clean on the next
// config reload or takeover.
func (t *Tab) reset() {
	t.css = ""
	t.js = ""
	t.zoom = 0
	t.isReloading = false
	t.onDocLoadFailed = t.defaultLoadFailedHandler()
}

// ensureStarted forces allocation of the browser owned by this tab so that
// sibling tabs created afterwards attach to it instead of spawning their own.
// This deliberately does not use runWithTimeout: the context passed to the
// first-ever chromedp.Run becomes the OS process's owning context (chromedp
// launches Chrome via exec.CommandContext(ctx, ...)), so cancelling it - as
// runWithTimeout's deferred cancel does - would kill the browser right after
// it starts.
func (t *Tab) ensureStarted() {
	if err := chromedp.Run(t.Context); err != nil {
		slog.Warn("Failed to start browser", "error", err)
	}
}

// defaultLoadFailedHandler returns the standard document-load-failure behaviour:
// warn and schedule a delayed reload of the same content.
func (t *Tab) defaultLoadFailedHandler() func(errText string) {
	return func(errText string) {
		slog.Warn("Tab failed to load document, scheduling retry", "error", errText, "retry_in", FailedLoadReloadDelay)
		t.delayedReload()
	}
}

func (t *Tab) Reload() {
	if err := runWithTimeout(t.Context, chromedp.Reload()); err != nil {
		slog.Warn("Failed to reload tab", "error", err)
	}
}

func (t *Tab) Navigate(url string) {
	// Clear any Authorization header left over from a previous basic-auth
	// navigation, since tabs are reused across config reloads and takeovers.
	if err := runWithTimeout(t.Context, network.Enable(), network.SetExtraHTTPHeaders(network.Headers{}), navigateAction(url)); err != nil {
		slog.Warn("Failed to navigate tab", "url", url, "error", err)
	}
}

func (t *Tab) NavigateWithBasicAuth(url string, creds BasicAuthCredentials) {
	x := base64.StdEncoding.EncodeToString([]byte(creds.Username + ":" + creds.Password))
	headers := network.Headers{"Authorization": "Basic " + x}

	if err := runWithTimeout(
		t.Context,
		network.Enable(),
		network.SetExtraHTTPHeaders(headers),
		navigateAction(url),
	); err != nil {
		slog.Warn("Failed to navigate tab with basic auth", "url", url, "error", err)
	}
}

// navigateAction triggers navigation without waiting for the page's load event,
// so rotation is never stalled by a slow or streaming page (e.g. a live
// dashboard) that fires load late or not at all.
func navigateAction(url string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, errText, _, err := page.Navigate(url).Do(ctx)
		if err != nil {
			return err
		}
		if errText != "" {
			return fmt.Errorf("%s", errText)
		}
		return nil
	})
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
	_ = runWithTimeout(
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
	_ = runWithTimeout(
		t.Context,
		chromedp.EvaluateAsDevTools(t.js, &executed),
	)
}

func (t *Tab) SetZoom(zoom float64) {
	t.zoom = zoom
	t.injectZoom()
}

func (t *Tab) injectZoom() {
	if t.zoom <= 0 {
		return
	}
	script := "document.body.style.zoom='" + strconv.FormatFloat(t.zoom, 'f', -1, 64) + "'"
	var executed *runtime.RemoteObject
	_ = runWithTimeout(
		t.Context,
		chromedp.EvaluateAsDevTools(script, &executed),
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
