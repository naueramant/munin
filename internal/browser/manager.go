package browser

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
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

	ctx    context.Context
	cancel context.CancelFunc
}

func NewBrowserManager(c *config.Configuration, as *assets.Server, baseDir string, extraFlags ...string) *BrowserManager {
	ctx, cancel := context.WithCancel(context.Background())

	bm := BrowserManager{
		Config:       c,
		AssetsServer: as,
		BaseDir:      baseDir,
		ctx:          ctx,
		cancel:       cancel,
	}

	slog.Debug("Spawning chromium browser")
	bm.Browser = NewBrowser(extraFlags...)

	if len(c.Tabs) == 0 {
		return &bm
	}

	for _, tabCon := range c.Tabs {
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
		slog.Debug("Initialized tabs", "count", len(c.Tabs))
	}

	return &bm
}

func (bm *BrowserManager) Start() {
	if bm.Config.Syntax == "" {
		bm.showNoConfigScreen()
		slog.Warn("No configuration syntax found")
		return
	}

	if len(bm.Config.Tabs) == 0 {
		bm.showNoTabsScreen()
		slog.Warn("No tabs configured")
		return
	}

	bm.startCycle(bm.ctx)
}

func (bm *BrowserManager) Close() {
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

func (bm *BrowserManager) showNoTabsScreen() {
	t := bm.Browser.NewTab()

	url := fmt.Sprintf(
		"%s/static/notabs.html?ip=%s",
		bm.AssetsServer.Host(),
		utils.GetLocalIp(),
	)

	t.Navigate(url)
}

func (bm *BrowserManager) showNoConfigScreen() {
	t := bm.Browser.NewTab()

	url := fmt.Sprintf(
		"%s/static/noconfig.html?ip=%s",
		bm.AssetsServer.Host(),
		utils.GetLocalIp(),
	)

	t.Navigate(url)
}

func (bm *BrowserManager) startCycle(ctx context.Context) {
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
