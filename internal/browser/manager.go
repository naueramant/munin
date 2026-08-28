package browser

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

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

func (bm *BrowserManager) ApplyConfig(c *config.Configuration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Cancel previous cycling loop
	if bm.cancel != nil {
		bm.cancel()
	}
	bm.ctx, bm.cancel = context.WithCancel(context.Background())
	bm.Config = c

	// Ensure browser is running
	if bm.Browser == nil || bm.Browser.Context == nil || bm.Browser.Context.Err() != nil {
		slog.Debug("Spawning chromium browser")
		bm.Browser = NewBrowser(bm.extraFlags...)
	}

	// Close existing tabs cleanly
	tabsToClose := make([]*Tab, len(bm.Browser.Tabs))
	copy(tabsToClose, bm.Browser.Tabs)
	for _, t := range tabsToClose {
		if t.Close != nil {
			t.Close()
		}
	}
	bm.Browser.Tabs = nil

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

	for _, tabCon := range bm.Config.Tabs {
		tab := bm.Browser.NewTab()

		if tabCon.Auth.Username != "" && tabCon.Auth.Password != "" {
			tab.NavigateWithBasicAuth(
				tabCon.URL,
				BasicAuthCredentials{
					Username: tabCon.Auth.Username,
					Password: tabCon.Auth.Password,
				},
			)
		} else {
			tab.Navigate(tabCon.URL)
		}

		bm.applyTabExtras(tab, tabCon)
	}

	if len(bm.Browser.Tabs) > 0 {
		bm.Browser.Tabs[0].Focus()
		slog.Debug("Initialized tabs", "count", len(bm.Config.Tabs))
	}

	go bm.startCycle(bm.ctx)
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

func (bm *BrowserManager) applyTabExtras(t *Tab, tc config.Tab) {
	if tc.CSS != "" {
		cssPath := tc.CSS
		if !filepath.IsAbs(cssPath) && bm.BaseDir != "" {
			cssPath = filepath.Join(bm.BaseDir, cssPath)
		}
		cssStr, err := utils.ReadFileToString(cssPath)
		if err != nil {
			slog.Error("Failed to read CSS file", "path", cssPath, "error", err)
		} else {
			go t.SetCSS(cssStr)
		}
	}

	if tc.JS != "" {
		jsPath := tc.JS
		if !filepath.IsAbs(jsPath) && bm.BaseDir != "" {
			jsPath = filepath.Join(bm.BaseDir, jsPath)
		}
		jsStr, err := utils.ReadFileToString(jsPath)
		if err != nil {
			slog.Error("Failed to read JS file", "path", jsPath, "error", err)
		} else {
			go t.SetJS(jsStr)
		}
	}
}

func (bm *BrowserManager) showNotConfiguredScreen() {
	t := bm.Browser.NewTab()

	url := fmt.Sprintf(
		"%s/static/not_configured.html?ip=%s",
		bm.AssetsServer.Host(),
		utils.GetLocalIp(),
	)

	t.Navigate(url)
}

func (bm *BrowserManager) startCycle(ctx context.Context) {
	if len(bm.Browser.Tabs) == 0 {
		return
	}

	for {
		for i, tab := range bm.Browser.Tabs {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if i < len(bm.Config.Tabs) && bm.Config.Tabs[i].Reload {
				tab.Reload()
				bm.applyTabExtras(tab, bm.Config.Tabs[i])
			}

			tab.Focus()
			slog.Debug("Switched active tab", "index", i)

			if i >= len(bm.Config.Tabs) || bm.Config.Tabs[i].Duration == 0 {
				return
			}

			delay := time.Duration(bm.Config.Tabs[i].Duration) * time.Second
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}
