package browser

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/naueramant/munin/internal/assets"
	"github.com/naueramant/munin/internal/config"
	"github.com/naueramant/munin/internal/utils"
)

type BrowserManager struct {
	Browser      *Browser
	Config       *config.Configuration
	AssetsServer *assets.Server
	BaseDir      string
	extraFlags   []string

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	cycleCtx    context.Context
	cycleCancel context.CancelFunc

	takeoverGen uint64

	// currentIndex is the config.Tabs index currently focused by the rotation,
	// used by runTakeover to know which persistent tab to refocus afterwards.
	currentIndex int
}

func NewBrowserManager(c *config.Configuration, as *assets.Server, baseDir string, extraFlags ...string) *BrowserManager {
	ctx, cancel := context.WithCancel(context.Background())

	bm := &BrowserManager{
		Config:       c,
		AssetsServer: as,
		BaseDir:      baseDir,
		extraFlags:   extraFlags,
		ctx:          ctx,
		cancel:       cancel,
	}

	slog.Debug("Spawning chromium browser")
	bm.Browser = NewBrowser(extraFlags...)

	return bm
}

func (bm *BrowserManager) Start() {
	bm.ApplyConfig(bm.Config)
}

// teardownLocked cancels the active generation and clears takeover state. The
// browser and its owning first tab are normally kept alive across config
// reloads so we never respawn Chromium and race its singleton profile lock.
// But Chromium can also die on its own (crash, OOM, external kill) without
// bm.Browser.Context ever being cancelled, so browserAliveLocked probes the
// existing browser and discards+respawns it if that probe fails. Callers
// reconcile the tab set via reconcileTabs. Callers must hold bm.mu.
func (bm *BrowserManager) teardownLocked() {
	if bm.cancel != nil {
		bm.cancel()
	}
	bm.ctx, bm.cancel = context.WithCancel(context.Background())

	if !bm.browserAliveLocked() {
		slog.Debug("Spawning chromium browser")
		if bm.Browser != nil && bm.Browser.Close != nil {
			bm.Browser.Close()
		}
		bm.Browser = NewBrowser(bm.extraFlags...)
	}

	// Invalidate any in-flight takeover
	bm.takeoverGen++
}

// browserAliveLocked reports whether the current browser process is still
// responsive. It issues a real CDP round-trip on the owning tab (a missing
// browser/tab or a cancelled allocator context short-circuits to dead
// without needing one). Callers must hold bm.mu.
func (bm *BrowserManager) browserAliveLocked() bool {
	if bm.Browser == nil || bm.Browser.Context == nil || bm.Browser.Context.Err() != nil {
		return false
	}
	if len(bm.Browser.Tabs) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(bm.Browser.Tabs[0].Context, 3*time.Second)
	defer cancel()
	return chromedp.Run(ctx, network.Enable()) == nil
}

// ensureTab makes sure the browser holds its single persistent display tab,
// creating and bootstrapping it if necessary, and resets its per-navigation
// state. --kiosk appears to break activating a background tab, so the
// manager never keeps more than one tab: all content, including rotation and
// scheduled takeovers, is shown by navigating this same tab. Callers must
// hold bm.mu.
func (bm *BrowserManager) ensureTab() *Tab {
	if len(bm.Browser.Tabs) == 0 {
		created := bm.Browser.NewTab()
		created.ensureStarted()
		return created
	}
	t := bm.Browser.Tabs[0]
	t.reset()
	return t
}

// ShowError replaces whatever is on screen with a fullscreen error page. Used
// for conditions like an invalid screen configuration.
func (bm *BrowserManager) ShowError(title, detail string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.teardownLocked()

	slog.Warn("Displaying error screen", "title", title, "detail", detail)
	t := bm.ensureTab()
	t.Navigate(bm.errorPageURL(title, "", "", detail))
}

func (bm *BrowserManager) ApplyConfig(c *config.Configuration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.teardownLocked()
	bm.Config = c

	if bm.Config == nil || bm.Config.Syntax == "" {
		bm.showNotConfiguredScreen()
		slog.Warn("No configuration syntax found")
		return
	}

	if len(bm.Config.Tabs) == 0 {
		bm.showNotConfiguredScreen()
		slog.Warn("No tabs configured")
		return
	}

	// A single persistent tab is reused for every config entry: rotating just
	// navigates it to the next content. Per-tab state (scroll position,
	// in-page JS, etc.) no longer survives rotation, since there is no way to
	// show a tab without navigating it there - see ensureTab.
	t := bm.ensureTab()
	bm.currentIndex = 0
	bm.showTab(t, bm.Config.Tabs[0].Content)
	slog.Debug("Initialized display tab", "tabs", len(bm.Config.Tabs))

	bm.cycleCtx, bm.cycleCancel = context.WithCancel(bm.ctx)
	go bm.startCycle(bm.cycleCtx)
	go bm.startScheduler(bm.ctx)
}

// showTab configures the display tab to show the given content: resets the
// failure handler to the default, applies the CSS/JS/zoom to inject once the
// page loads, and navigates (with basic auth when configured).
func (bm *BrowserManager) showTab(t *Tab, c config.Content) {
	t.onDocLoadFailed = t.defaultLoadFailedHandler()
	t.css = bm.resolveExtra(c.CSS)
	t.js = bm.resolveExtra(c.JS)
	t.zoom = c.Zoom

	target, creds := bm.contentTarget(c)
	if creds != nil {
		t.NavigateWithBasicAuth(target, *creds)
	} else {
		t.Navigate(target)
	}
}

func (bm *BrowserManager) Close() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	slog.Debug("Closing browser")
	if bm.cancel != nil {
		bm.cancel()
	}
	if bm.Browser != nil && bm.Browser.Close != nil {
		bm.Browser.Close()
	}
}

// resolveExtra reads a CSS/JS asset referenced by a tab or schedule and returns
// its contents, or an empty string when unset or unreadable.
func (bm *BrowserManager) resolveExtra(path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) && bm.BaseDir != "" {
		path = filepath.Join(bm.BaseDir, path)
	}
	s, err := utils.ReadFileToString(path)
	if err != nil {
		slog.Error("Failed to read extra file", "path", path, "error", err)
		return ""
	}
	return s
}

func (bm *BrowserManager) showNotConfiguredScreen() {
	t := bm.ensureTab()

	url := fmt.Sprintf(
		"%s/static/not_configured.html?ip=%s",
		bm.AssetsServer.Host(),
		utils.GetLocalIp(),
	)

	t.Navigate(url)
}

func (bm *BrowserManager) startCycle(ctx context.Context) {
	tabs := bm.Config.Tabs
	if len(tabs) == 0 {
		return
	}

	i := 0
	for {
		// A zero duration (or a single tab) means "stay here"; stop rotating.
		if tabs[i].Duration.Duration() == 0 {
			slog.Debug("Stopping tab rotation", "reason", "zero duration", "index", i)
			return
		}

		select {
		case <-ctx.Done():
			slog.Debug("Stopping tab rotation", "reason", "context done")
			return
		case <-time.After(tabs[i].Duration.Duration()):
		}

		i = (i + 1) % len(tabs)

		bm.mu.Lock()
		// Record the advance before the cancellation check: a takeover can race
		// this tick and cancel ctx right after the timer fires, and if that
		// happens currentIndex must still reflect the tab rotation was moving
		// to, otherwise the takeover restores the tab it was leaving instead.
		bm.currentIndex = i
		if ctx.Err() != nil || len(bm.Browser.Tabs) == 0 {
			slog.Debug("Stopping tab rotation", "reason", "context done or tab missing", "index", i)
			bm.mu.Unlock()
			return
		}
		bm.showTab(bm.Browser.Tabs[0], tabs[i].Content)
		bm.mu.Unlock()
		slog.Debug("Switched active content", "index", i)
	}
}

// startScheduler launches one goroutine per configured schedule. Each goroutine
// waits for its next trigger time and then performs a timed page takeover.
func (bm *BrowserManager) startScheduler(ctx context.Context) {
	if bm.Config == nil {
		return
	}
	for i := range bm.Config.Schedules {
		go bm.runScheduleLoop(ctx, bm.Config.Schedules[i])
	}
}

func (bm *BrowserManager) runScheduleLoop(ctx context.Context, s config.Schedule) {
	full := s.Duration.Duration()

	// Catch-up: if we start inside an active window, show the remaining time.
	if remaining, active := utils.ActiveWindowRemaining(s.When, full, time.Now()); active {
		slog.Info("Resuming active scheduled takeover", "when", s.When, "remaining", remaining.Round(time.Second))
		bm.runTakeover(ctx, s, remaining)
	}

	for {
		delay := utils.ComputeNextCronDelay(s.When, "", time.Now())
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		bm.runTakeover(ctx, s, full)
	}
}

// runTakeover interrupts the tab rotation and navigates the display tab to
// the schedule's content for `hold`, then navigates it back to whichever
// rotation content was active beforehand. A newer takeover firing during the
// hold window preempts this one (last one wins).
func (bm *BrowserManager) runTakeover(ctx context.Context, s config.Schedule, hold time.Duration) {
	bm.mu.Lock()
	if ctx.Err() != nil || len(bm.Browser.Tabs) == 0 {
		bm.mu.Unlock()
		return
	}

	bm.takeoverGen++
	gen := bm.takeoverGen

	// Stop the rotation while the takeover is on screen.
	if bm.cycleCancel != nil {
		bm.cycleCancel()
	}

	t := bm.Browser.Tabs[0]
	target, creds := bm.contentTarget(s.Content)

	t.onDocLoadFailed = func(errText string) {
		slog.Warn("Scheduled takeover failed to load, showing error page", "when", s.When, "url", target, "error", errText)
		t.css, t.js = "", ""
		t.Navigate(bm.errorPageURL("Scheduled Page Unavailable", "The scheduled page could not be loaded.", target, errText))
	}
	t.css = bm.resolveExtra(s.CSS)
	t.js = bm.resolveExtra(s.JS)
	t.zoom = s.Zoom

	if creds != nil {
		t.NavigateWithBasicAuth(target, *creds)
	} else {
		t.Navigate(target)
	}

	slog.Debug("Showing scheduled takeover", "when", s.When, "hold", hold.Round(time.Second))
	bm.mu.Unlock()

	select {
	case <-ctx.Done():
		return
	case <-time.After(hold):
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Only restore if we are still the active takeover and the config is current.
	if ctx.Err() != nil || gen != bm.takeoverGen {
		return
	}

	if bm.Config != nil && bm.currentIndex < len(bm.Config.Tabs) && len(bm.Browser.Tabs) > 0 {
		bm.showTab(bm.Browser.Tabs[0], bm.Config.Tabs[bm.currentIndex].Content)
	}

	bm.cycleCtx, bm.cycleCancel = context.WithCancel(bm.ctx)
	slog.Debug("Resuming tab rotation after takeover")
	go bm.startCycle(bm.cycleCtx)
}

// contentTarget resolves the URL to display for a tab or schedule and any
// basic-auth credentials. Inline messages are rendered via the local assets
// server.
func (bm *BrowserManager) contentTarget(c config.Content) (string, *BasicAuthCredentials) {
	if c.HasURL() {
		if c.Auth.Username != "" && c.Auth.Password != "" {
			return c.URL, &BasicAuthCredentials{
				Username: c.Auth.Username,
				Password: c.Auth.Password,
			}
		}
		return c.URL, nil
	}
	return bm.messageURL(c), nil
}

func (bm *BrowserManager) messageURL(c config.Content) string {
	q := url.Values{}
	q.Set("msg", c.Message)
	if c.FontSize > 0 {
		q.Set("fontSize", strconv.FormatUint(c.FontSize, 10))
	}
	if c.TextColor != "" {
		q.Set("textColor", c.TextColor)
	}
	if c.BackgroundColor != "" {
		q.Set("backgroundColor", c.BackgroundColor)
	}
	if c.Blink {
		q.Set("blink", "1")
	}
	return fmt.Sprintf("%s/static/message.html?%s", bm.AssetsServer.Host(), q.Encode())
}

func (bm *BrowserManager) errorPageURL(title, message, target, detail string) string {
	q := url.Values{}
	if title != "" {
		q.Set("title", title)
	}
	if message != "" {
		q.Set("message", message)
	}
	if target != "" {
		q.Set("url", target)
	}
	if detail != "" {
		q.Set("error", detail)
	}
	return fmt.Sprintf("%s/static/error.html?%s", bm.AssetsServer.Host(), q.Encode())
}
