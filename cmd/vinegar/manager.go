package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"codeberg.org/puregotk/puregotk/v4/adw"
	"codeberg.org/puregotk/puregotk/v4/gio"
	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gobject"
	"codeberg.org/puregotk/puregotk/v4/gtk"
	"github.com/google/go-github/v80/github"
	"github.com/vinegarhq/vinegar/internal/config"
	"github.com/vinegarhq/vinegar/internal/dirs"
	"github.com/vinegarhq/vinegar/internal/gutil"

	. "github.com/pojntfx/go-gettext/pkg/i18n"
)

type manager struct {
	*app

	builder *gtk.Builder
	win     adw.ApplicationWindow
}

func (a *app) newManager() *manager {
	m := manager{
		app: a,
	}

	m.builder = gtk.NewBuilderFromResource(gutil.Resource("ui/manager.ui"))
	m.builder.GetObject("window").Cast(&m.win)
	m.win.SetApplication(&a.Application.Application)

	cmd := gutil.GetObject[gtk.Entry](m.builder, "cmd")
	cb := m.runWineCmd
	cmd.ConnectActivate(&cb)

	m.connectElements()
	for name, fn := range map[string]any{
		"save":  m.saveConfig,
		"about": m.showAbout,
		"open-data": func() {
			gtk.ShowUri(&m.win.Window, "file://"+dirs.Data, 0)
		},
		"open-logs": func() {
			gtk.ShowUri(&m.win.Window, "file://"+dirs.Logs, 0)
		},

		"prefix-kill": m.killPrefix,
		"delete-prefix": func() {
			m.confirm(L("Delete Wine Data?"),
				L("Studio settings, logins and installed components inside the Wineprefix will be permanently lost."),
				L("Delete"), m.deletePrefixes)
		},
		"delete-studio": func() {
			m.confirm(L("Uninstall Studio?"),
				L("Studio will be downloaded again the next time it is launched."),
				L("Uninstall"), m.deleteDeployments)
		},
		"clear-cache": m.clearCache,
		"update":      m.updateWine,
		"restore":     m.boot.restoreSettings,

		"winecfg": func() {
			cmd.SetText("winecfg")
			gobject.SignalEmitByName(&cmd.Object, "activate")
		},
	} {
		action := gio.NewSimpleAction(name, nil)
		activate := func(_ gio.SimpleAction, p uintptr) {
			switch v := fn.(type) {
			case func() error:
				m.app.errThread(func() error {
					return v()
				})
			case func():
				v()
			default:
				panic(fmt.Sprintf("unhandled function type: %T", fn))
			}
		}
		action.ConnectActivate(&activate)
		m.win.AddAction(action)
		action.Unref()
	}

	tags := gutil.GetObject[gtk.StringList](m.builder, "wine_tags")
	selectedTag := gutil.GetObject[gtk.DropDown](m.builder, "wine_tag")
	gutil.ConnectBuilderSimple(m.builder, "wine_updater", "show", func() {
		if tags.GetNItems() > 1 {
			return
		}
		slog.Info("Fetching releases")

		tag, err := filepath.EvalSymlinks(dirs.WinePath)
		if err == nil {
			tag, _ = strings.CutPrefix(filepath.Base(tag), dirs.TagPrefix)
			tags.Append(tag)
			selectedTag.SetSelected(1)
		}

		m.app.errThread(func() error {
			client := github.NewClient(nil)
			ctx := context.Background()

			seen := map[string]bool{tag: true}
			var errs []error
			for _, repo := range config.KombuchaRepos {
				releases, _, err := client.Repositories.ListReleases(
					ctx, repo.Owner, repo.Name, nil)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				for _, release := range releases {
					name := release.GetTagName()
					if seen[name] {
						continue
					}
					seen[name] = true
					gutil.IdleAdd(func() {
						tags.Append(name)
					})
				}
			}

			// Only fail if no repository could be reached at all.
			if len(errs) == len(config.KombuchaRepos) {
				return errors.Join(errs...)
			}
			return nil
		})
	})

	wineRow := gutil.GetObject[adw.ActionRow](m.builder, "wine_row")
	updateWine := gutil.GetObject[gtk.Button](m.builder, "wine_confirm")

	cnfcb := func(_ gtk.Button) {
		spin := adw.NewSpinner()
		wineRow.SetSensitive(false)
		wineRow.AddSuffix(&spin.Widget)
		sel := gtk.StringObjectNewFromInternalPtr(
			selectedTag.GetSelectedItem().Ptr)
		m.app.errThread(func() error {
			defer gutil.IdleAdd(func() {
				wineRow.SetSensitive(true)
				wineRow.Remove(&spin.Widget)
			})
			return a.updateWine(sel.GetString())
		})
	}
	updateWine.ConnectClicked(&cnfcb)

	return &m
}

func (m *manager) showToast(s string) {
	if m == nil {
		return
	}
	overlay := gutil.GetObject[adw.ToastOverlay](m.builder, "overlay")
	slog.Info(s)

	gutil.IdleAdd(func() {
		toast := adw.NewToast(s)
		toast.SetPriority(adw.ToastPriorityHighValue)
		overlay.AddToast(toast)
	})
}

// confirm presents a destructive confirmation dialog, running fn in
// a separate thread only once the user has accepted it.
func (m *manager) confirm(heading, body, accept string, fn func() error) {
	d := adw.NewAlertDialog(heading, body)
	d.AddResponses("cancel", L("Cancel"), "accept", accept)
	d.SetCloseResponse("cancel")
	d.SetDefaultResponse("cancel")
	d.SetResponseAppearance("accept", adw.ResponseDestructiveValue)

	var ccb gio.AsyncReadyCallback
	ccb = func(_ uintptr, resPtr uintptr, _ uintptr) {
		defer glib.UnrefCallback(&ccb)
		res := gio.SimpleAsyncResultNewFromInternalPtr(resPtr)
		if d.ChooseFinish(res) == "accept" {
			m.app.errThread(fn)
		}
	}
	d.Choose(&m.win.Widget, nil, &ccb, 0)
}
