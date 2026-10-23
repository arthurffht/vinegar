package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"codeberg.org/puregotk/puregotk/v4/adw"
	"github.com/adrg/xdg"
	"github.com/google/go-github/v80/github"
	"github.com/sewnie/wine"
	"github.com/vinegarhq/vinegar/internal/config"
	"github.com/vinegarhq/vinegar/internal/dirs"
	"github.com/vinegarhq/vinegar/internal/gutil"
	"github.com/vinegarhq/vinegar/internal/logging"
	"github.com/vinegarhq/vinegar/internal/netutil"

	. "github.com/pojntfx/go-gettext/pkg/i18n"
)

// Reports whether a Wine Prefix was initialized.
func (a *app) prepareWine() (bool, error) {
	firstRun := !a.pfx.Exists()

	a.boot.message(L("Setting up Wine"), "first-time", firstRun)

	cmd := a.pfx.Wine("")
	if string(a.cfg.Studio.WineRoot) == dirs.WinePath && cmd.Err != nil {
		if err := a.updateWine("Latest"); err != nil {
			return false, fmt.Errorf("dl: %w", err)
		}
	}
	if a.pfx.Running() {
		return false, nil
	}

	if err := a.pfx.Prepare(); err != nil {
		return firstRun, err
	}
	a.updateWineTheme()

	// Do _not_ do this on prefixes that already exist, only new ones,
	// as to not discard the existing appdata directory.
	if !firstRun {
		return false, nil
	}

	folders := wine.NewRegistryKey(
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`)
	folders.SetValue("Local AppData", dirs.Windows(dirs.AppDataPath))
	folders.SetValue("Documents", dirs.Windows(xdg.UserDirs.Documents))
	folders.SetValue("My Pictures", dirs.Windows(xdg.UserDirs.Pictures))

	a.boot.message(L("Updating Paths"))
	err := a.pfx.RegistryImportKey(folders)
	if err != nil {
		return true, fmt.Errorf("paths: %w", err)
	}

	// Restart wineserver for wineboot to update app data internally.
	if err := a.pfx.Boot(wine.BootRestart).Run(); err != nil {
		return true, fmt.Errorf("restart: %w", err)
	}

	if err := a.boot.restoreSettings(); err != nil {
		return true, fmt.Errorf("restore: %w", err)
	}

	return true, nil
}

func (a *app) updateWine(needle string) error {
	release, err := kombuchaRelease(needle)
	if err != nil {
		return fmt.Errorf("release: %w", err)
	}

	tag := release.GetTagName()
	dir := filepath.Join(dirs.Data, dirs.TagPrefix+tag)

	log := slog.With("tag", tag, "released", release.PublishedAt.Time)

	if _, err := os.Stat(dirs.WinePath); err == nil {
		path, err := filepath.EvalSymlinks(dirs.WinePath)
		if err != nil {
			return fmt.Errorf("readlink: %w", err)
		}
		name := filepath.Base(path)
		if name == filepath.Base(dir) {
			log.Info("Wine build up to date")
			return nil
		}

		slog.Info("Removing old Wine build", "name", name)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove build: %w", err)
		}

		if err := os.Remove(dirs.WinePath); err != nil {
			return fmt.Errorf("remove link: %w", err)
		}
	}

	if _, err := os.Stat(dir); err == nil {
		goto install
	}

	if len(release.Assets) != 1 ||
		release.Assets[0].GetContentType() != "application/x-xz" {
		return errors.New("expected single .tar.xz release")
	}

	log.Info("Fetching Wine build")
	if err := netutil.ExtractURL(
		release.Assets[0].GetBrowserDownloadURL(), dirs.Data,
	); err != nil {
		return err
	}

install:
	if err := os.Symlink("kombucha-"+tag, dirs.WinePath); err != nil {
		return fmt.Errorf("create link: %w", err)
	}

	log.Info("Updated Wine build configuration")

	// No need to save wineroot is already set
	if a.cfg.Studio.WineRoot == dirs.WinePath {
		return nil
	}

	if path := dirs.WinePath; a.mgr != nil {
		// Thanks to the signals setup in the bindings procedure, this
		// will also handle setting the wine root as well.
		gutil.IdleAdd(func() {
			row := gutil.GetObject[adw.ActionRow](a.mgr.builder, "wine_row")
			row.SetSubtitle(path)
		})
		return nil
	}
	a.cfg.Studio.WineRoot = dirs.WinePath

	// Reload current configuration and save
	a.ActivateAction("win.save", nil)

	return nil
}

// kombuchaRelease finds the Kombucha release with the given tag, or the
// latest release if the needle is "Latest". The Wayland-only GNOME builds
// are preferred, falling back to the upstream builds.
func kombuchaRelease(needle string) (*github.RepositoryRelease, error) {
	client := github.NewClient(nil)
	ctx := context.Background()

	var errs []error
	for _, repo := range config.KombuchaRepos {
		var release *github.RepositoryRelease
		var err error
		if needle == "Latest" {
			release, _, err = client.Repositories.GetLatestRelease(ctx, repo.Owner, repo.Name)
		} else {
			release, _, err = client.Repositories.GetReleaseByTag(ctx, repo.Owner, repo.Name, needle)
		}
		if err == nil {
			slog.Info("Using Kombucha release", "repo", repo, "tag", release.GetTagName())
			return release, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", repo, err))
	}
	return nil, errors.Join(errs...)
}

// Implements io.Writer for reading the log from Wine. Wine's output
// may arrive in arbitrarily sized chunks, so incomplete lines are kept
// until their terminating newline arrives.
func (a *app) Write(b []byte) (int, error) {
	a.logMu.Lock()
	a.logBuf = append(a.logBuf, b...)
	i := bytes.LastIndexByte(a.logBuf, '\n')
	if i < 0 {
		a.logMu.Unlock()
		return len(b), nil
	}
	lines := string(a.logBuf[:i])
	a.logBuf = append(a.logBuf[:0], a.logBuf[i+1:]...)
	a.logMu.Unlock()

	for line := range strings.SplitSeq(lines, "\n") {
		// On older versions of Roblox, the channel used to be 'debugstr'.
		// XXXX:warn:seh:OutputDebugStringA "2026-04-27T14:57:01.636Z ... [FLog::Foo] Message"
		if a.boot != nil && len(line) >= 14 && line[10:13] == "seh" {
			// Ignore short messages such as newlines
			if len(line) < 39 {
				continue
			}
			from := line[14 : 14+18]
			if from != "OutputDebugStringA" {
				continue
			}
			a.boot.handleRobloxLog(line[34 : len(line)-1])
			continue
		}

		a.handleWineLog(line)
	}
	return len(b), nil
}

func (a *app) handleWineLog(line string) {
	if strings.Contains(line, "to unimplemented function advapi32.dll.SystemFunction036") {
		err := errors.New("Your Wineprefix is corrupt! Please delete all data in Vinegar's settings.")
		gutil.IdleAdd(func() {
			a.pfx.Server(wine.ServerKill, "9")
			a.showError(err)
		})
	}

	slog.Log(context.Background(), logging.LevelWine.Level(), line)
}

// updateWineTheme mirrors the GNOME desktop preferences (dark style,
// accent color) into Wine, and ensures Wine renders through Wayland.
func (a *app) updateWineTheme() {
	// If the studio theme is "Default", the wine theme change will effect
	// studio as well.
	if a.pfx == nil || !a.pfx.Running() {
		slog.Debug("Not changing theme: Wine is not running")
		return
	}

	root := wine.RegistryKey{Name: "HKEY_CURRENT_USER"}
	v := root.Add(`Software\Microsoft\Windows\CurrentVersion`)
	pz := v.Add(`Themes\Personalize`)
	mgr := v.Add(`ThemeManager`)

	// Wayland-only: never fall back to winex11 through Xwayland.
	root.Add(`Software\Wine\Drivers`).SetValue("Graphics", "wayland")

	// GNOME renders fonts with grayscale antialiasing by default.
	desktop := root.Add(`Control Panel\Desktop`)
	desktop.SetValue("FontSmoothing", "2")
	desktop.SetValue("FontSmoothingType", uint32(1))
	desktop.SetValue("FontSmoothingGamma", uint32(1400))

	mgr.SetValue("ColorName", nil)
	mgr.SetValue("DllName", nil)
	mgr.SetValue("LoadedBefore", nil)
	mgr.SetValue("SizeName", nil)

	// Reloading an MSStyle requires calling windows API, hence why
	// I try to stick to only changing the current theme to none
	// and change the colors. This means that the light theme's clean
	// look will be gone in the light theme.
	mgr.SetValue("LoadedBefore", "0")
	mgr.SetValue("ThemeActive", "0")

	sm := a.GetStyleManager()
	dark := sm.GetDark()
	colors := lightThemeValues
	light := uint32(1)
	if dark {
		colors = darkThemeValues
		light = 0
	}
	colors = withAccent(colors, accentColor(sm))
	root.Add(`Control Panel\Colors`).Values = colors
	pz.SetValue("AppsUseLightTheme", light)
	pz.SetValue("SystemUsesLightTheme", light)

	slog.Debug("Changing Wine's theme", "dark", dark)
	if err := a.pfx.RegistryImportKey(&root); err != nil {
		slog.Error("Failed to change Wine's theme", "err", err)
	}

	// Modifying the registry manually triggers no events. To make Studio
	// refresh its theme, run a small application to open a GUI for just a
	// second. Yes this results in a flicker and requires the user to
	// focus on Studio, but there's no other way to allow live updates :/
	if a.boot != nil && a.boot.count.Load() > 0 {
		_ = a.pfx.Wine("start", "cmd", "/c", "exit").Run()
	}
}

// accentColor returns the GNOME accent color as a Windows registry
// color value, falling back to the Adwaita blue.
func accentColor(sm *adw.StyleManager) string {
	const fallback = "53 132 228" // #3584e4
	if !sm.GetSystemSupportsAccentColors() {
		return fallback
	}
	c := sm.GetAccentColorRgba()
	if c == nil {
		return fallback
	}
	defer c.Free()
	return fmt.Sprintf("%d %d %d",
		uint8(c.Red*255+0.5), uint8(c.Green*255+0.5), uint8(c.Blue*255+0.5))
}

// withAccent returns a copy of values with the selection and highlight
// colors replaced by the given accent color.
func withAccent(values []wine.RegistryValue, accent string) []wine.RegistryValue {
	out := slices.Clone(values)
	for i, v := range out {
		switch v.Name {
		case "ButtonAlternativeFace", "Hilight", "HotTrackingColor", "MenuHilight":
			out[i].Data = accent
		}
	}
	return out
}

var darkThemeValues = []wine.RegistryValue{
	{"ActiveBorder", "34 34 38"},            // #222226 window_bg_color
	{"ActiveTitle", "46 46 50"},             // #2e2e32 headerbar_bg_color
	{"AppWorkSpace", "45 45 49"},            // midpoint between window/view background
	{"Background", "29 29 32"},              // #1d1d20 view_bg_color
	{"ButtonAlternativeFace", "98 160 234"}, // accent blue from Adwaita
	{"ButtonDkShadow", "22 22 25"},          // slightly darker than view_bg
	{"ButtonFace", "46 46 50"},              // #2e2e32 headerbar
	{"ButtonHilight", "57 57 61"},           // lifted tone from card_bg_color overlay
	{"ButtonLight", "53 53 57"},
	{"ButtonShadow", "25 25 28"},
	{"ButtonText", "255 255 255"},
	{"GradientActiveTitle", "46 46 50"},
	{"GradientInactiveTitle", "40 40 44"}, // sidebar_backdrop_color #28282c
	{"GrayText", "136 136 136"},
	{"Hilight", "98 160 234"}, // accent blue
	{"HilightText", "255 255 255"},
	{"InactiveBorder", "40 40 44"}, // sidebar_backdrop
	{"InactiveTitle", "40 40 44"},
	{"InactiveTitleText", "211 211 211"},
	{"InfoText", "255 255 255"},
	{"InfoWindow", "54 54 58"}, // dialog_bg_color #36363a
	{"Menu", "54 54 58"},       // dialog/popover background
	{"MenuBar", "46 46 50"},    // keep menu slightly lighter
	{"MenuHilight", "98 160 234"},
	{"MenuText", "255 255 255"},
	{"Scrollbar", "46 46 50"},
	{"TitleText", "255 255 255"},
	{"Window", "29 29 32"},      // view_bg_color
	{"WindowFrame", "46 46 50"}, // subtle outline color
	{"WindowText", "255 255 255"},
}

var lightThemeValues = []wine.RegistryValue{
	{"ActiveBorder", "255 255 255"},
	{"ActiveTitle", "50 150 250"},
	{"AppWorkSpace", "128 128 128"},
	{"Background", "37 111 149"},
	{"ButtonAlternateFace", "255 255 255"},
	{"ButtonAlternativeFace", "98 160 234"},
	{"ButtonDkShadow", "106 106 106"},
	{"ButtonFace", "245 245 245"},
	{"ButtonHilight", "255 255 255"},
	{"ButtonLight", "227 227 227"},
	{"ButtonShadow", "166 166 166"},
	{"ButtonText", "0 0 0"},
	{"GradientActiveTitle", "50 150 250"},
	{"GradientInactiveTitle", "128 128 128"},
	{"GrayText", "106 106 106"},
	{"Hilight", "48 150 250"},
	{"HilightText", "255 255 255"},
	{"HotTrackingColor", "48 150 250"},
	{"InactiveBorder", "255 255 255"},
	{"InactiveTitle", "128 128 128"},
	{"InactiveTitleText", "200 200 200"},
	{"InfoText", "0 0 0"},
	{"InfoWindow", "255 255 255"},
	{"Menu", "255 255 255"},
	{"MenuBar", "255 255 255"},
	{"MenuHilight", "48 150 250"},
	{"MenuText", "0 0 0"},
	{"Scrollbar", "255 255 255"},
	{"TitleText", "0 0 0"},
	{"Window", "255 255 255"},
	{"WindowFrame", "158 158 158"},
	{"WindowText", "0 0 0"},
}
