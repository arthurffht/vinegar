package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"codeberg.org/puregotk/puregotk/v4/glib"
	. "github.com/pojntfx/go-gettext/pkg/i18n"
	"github.com/sewnie/wine"
	"github.com/vinegarhq/vinegar/internal/gutil"
)

func (b *bootstrapper) commandPath() string {
	return filepath.Join(b.dir, "RobloxStudioBeta.exe")
}

func (b *bootstrapper) command(args ...string) (*wine.Cmd, error) {
	cmd := b.pfx.Wine(b.commandPath(), args...)
	if cmd.Err != nil {
		return nil, cmd.Err
	}

	// This is an authentication call, which is ran to the main Studio instance,
	// no point to run this with the launcher or seperate desktop.
	if len(args) > 0 && strings.HasPrefix(args[0], "roblox-studio-auth:") {
		return cmd, nil
	}

	// I was called a "noob" for my implementation by someone who creates
	// an entirely new Wineprefix for this feature.
	if d := string(b.cfg.Studio.Desktop); d != "" {
		cmd.Args = append([]string{
			cmd.Args[0],
			"explorer", "/desktop=" + glib.UuidStringRandom() + "," + d,
		}, cmd.Args[1:]...)
	}

	launcher := strings.Fields(b.cfg.Studio.Launcher)
	if len(launcher) >= 1 {
		cmd.Args = append(launcher, cmd.Args...)
		p, err := b.cfg.Studio.LauncherPath()
		if err != nil {
			return nil, fmt.Errorf("bad launcher: %w", err)
		}
		cmd.Path = p
	}

	return cmd, nil
}

func (b *bootstrapper) execute(args ...string) error {
	cmd, err := b.command(args...)
	if err != nil {
		return err
	}

	b.message(L("Launching Studio"), "cmd", cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(c)
		close(done)
	}()
	go func() {
		select {
		case s := <-c:
			slog.Warn("Received signal", "signal", s)
			slog.Debug("Killing Roblox", "pid", cmd.Process.Pid)
			_ = cmd.Process.Kill()
		case <-done:
		}
	}()

	b.count.Add(1)
	defer b.count.Add(-1)

	gutil.IdleAdd(func() {
		b.win.SetVisible(false)
	})

	cmd.Process.WithHandle(func(h uintptr) {
		if err := b.registerGame(h); err != nil {
			slog.Error("Failed to register with GameMode", "err", err)
		}
	})

	err = cmd.Wait()

	if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == -1 {
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			slog.Warn("Roblox was killed!", "signal", ws.Signal())
		}
		return nil
	}

	return err
}
