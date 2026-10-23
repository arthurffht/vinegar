package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/pojntfx/go-gettext/pkg/i18n"
	"github.com/vinegarhq/vinegar/internal/gutil"
	"github.com/vinegarhq/vinegar/internal/sysinfo"
)

// Set at link time by the build system.
var (
	LocaleDir   = "/usr/share/locale"
	ResourceDir = "/usr/share/vinegar"
)

func main() {
	debug.SetPanicOnFault(true)

	// Probing Vulkan initializes GPU drivers, which may take a moment;
	// do so while GTK starts up.
	go sysinfo.VulkanDevices()

	// Vinegar for GNOME is Wayland-only: both GTK and Wine are driven
	// through the Wayland compositor, never through Xwayland.
	// Broadway is permitted for remote and headless testing.
	if os.Getenv("GDK_BACKEND") != "broadway" {
		if !waylandAvailable() {
			fmt.Fprintln(os.Stderr, "vinegar: a Wayland session is required")
			os.Exit(1)
		}
		os.Setenv("GDK_BACKEND", "wayland")
	}

	setDefaultEnv := func(key, value string) {
		if _, ok := os.LookupEnv(key); !ok {
			os.Setenv(key, value)
		}
	}
	if slices.ContainsFunc(sysinfo.Cards, func(c sysinfo.Card) bool {
		return strings.HasPrefix(c.Driver, "nvidia")
	}) {
		// Causes GTK panic with: "No provider of eglGetCurrentContext found."
		setDefaultEnv("GSK_RENDERER", "cairo")
	}
	// VK_SUBOPTIMAL_KHR
	setDefaultEnv("GDK_DISABLE", "vulkan")

	if err := gutil.LoadResources(ResourceDir); err != nil {
		fmt.Fprintln(os.Stderr, "vinegar:", err)
		os.Exit(1)
	}

	if err := i18n.InitI18n("vinegar", LocaleDir, slog.Default()); err != nil {
		slog.Error("Failed to set locale", "err", err)
	}

	if code := newApp().Run(int32(len(os.Args)), os.Args); code > 0 {
		os.Exit(int(code))
	}
}

// waylandAvailable reports whether a Wayland compositor socket can be
// found, following the same resolution rules as libwayland-client.
func waylandAvailable() bool {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		display = "wayland-0"
	}
	if !filepath.IsAbs(display) {
		display = filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), display)
	}
	_, err := os.Stat(display)
	return err == nil
}
