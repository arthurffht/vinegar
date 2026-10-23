# <img src="data/icons/vinegar.svg" alt="Vinegar logo" height="64px"> Vinegar for GNOME

[![Build][pipeline_img]][pipeline]

An open-source, configurable, fast bootstrapper for running Roblox Studio on
Linux — in an edition made exclusively for **GNOME 50 and 51 on Wayland**.

This is a fork of [VinegarHQ's Vinegar](https://github.com/vinegarhq/vinegar)
that trades portability for the tightest possible GNOME integration.

[pipeline]:     https://github.com/arthurffht/vinegar/actions/workflows/build.yml
[pipeline_img]: https://img.shields.io/github/actions/workflow/status/arthurffht/vinegar/build.yml?branch=gnome&style=flat-square&label=build%20%26%20tests

![Showcase of Vinegar's manager and bootstrapper window](data/screenshots/vinegar.png)

## What is different

* **Wayland only.** GTK runs on the Wayland backend and Studio is rendered by
  Wine's native Wayland driver. Nothing goes through Xwayland, and the Flatpak
  does not request X11 access at all.
* **Wayland-only Wine.** Wine builds come from
  [arthurffht/kombucha](https://github.com/arthurffht/kombucha)'s `gnome`
  flavor, built `--without-x` with `winewayland.drv` as the only graphics
  driver. Upstream Kombucha builds are used as a fallback.
* **Follows GNOME.** Wine and Studio follow the GNOME dark style *and* accent
  color, and use GNOME's grayscale font rendering.
* **Picks the right renderer for your GPU.** The Vulkan support of the GPU is
  probed at runtime: DXVK on Vulkan 1.3, DXVK (Legacy, Sarek) on Vulkan 1.1–1.2
  hardware such as Intel Gen7/8 or NVIDIA Kepler, WineD3D otherwise.
* **GNOME tooling.** The interface is written in
  [Blueprint](https://gnome.pages.gitlab.gnome.org/blueprint-compiler/), the
  build system is Meson, and the Flatpak targets the GNOME 51 runtime.
* **HIG polish.** Confirmation before destructive actions, <kbd>Ctrl</kbd>+<kbd>Q</kbd>
  / <kbd>Ctrl</kbd>+<kbd>W</kbd>, tooltips, accessible labels and complete
  Brazilian Portuguese translation.
* **Bug fixes**, including crashes when startup fails or when parsing Studio
  logs, Wine log lines split across pipe reads, broken Wine installs after an
  interrupted download, a minimum memory check that never fired and data races
  in the instance counter and download progress.

The core stays in Go, using [puregotk](https://codeberg.org/puregotk/puregotk)
to call GTK 4 and libadwaita without cgo, which keeps the binary small and
the startup fast.

## Installing

### Flatpak

Download the `.flatpak` bundle from the
[latest release](https://github.com/arthurffht/vinegar/releases/latest), then:

```sh
flatpak install --user vinegar-*.flatpak
```

This uses the same application ID as the Flathub release of Vinegar, so
uninstall that one first.

### From source

Requirements: Go ≥ 1.26, Meson, blueprint-compiler, GTK ≥ 4.20,
libadwaita ≥ 1.9, Vulkan headers and gettext.

```sh
meson setup build --prefix=/usr
meson compile -C build
sudo meson install -C build
```

## Development

```sh
meson setup build
meson compile -C build
meson test -C build
meson devenv -C build vinegar manage
```

`meson devenv` points Vinegar at the freshly compiled GResource bundle, so no
installation is needed to try changes. After changing Go dependencies, run
`build-aux/flatpak/update-go-sources.py` to refresh the Flatpak sources.

Branches: `gnome` is the default branch with this edition; `master` mirrors
upstream and is merged into `gnome` to pick up upstream changes:

```sh
git fetch upstream
git switch master && git merge --ff-only upstream/master && git push
git switch gnome && git merge master
```

# See Also
+ [Upstream Vinegar](https://github.com/vinegarhq/vinegar) and its [documentation](https://vinegarhq.github.io)
+ [Roblox-Studio-Mod-Manager](https://github.com/MaximumADHD/Roblox-Studio-Mod-Manager)
+ [Bloxstrap](https://github.com/pizzaboxer/bloxstrap)

# Acknowledgements
+ [VinegarHQ](https://github.com/vinegarhq) for Vinegar and Kombucha
+ Credits to
  + [pizzaboxer](https://github.com/pizzaboxer)
  + [MaximumADHD](https://github.com/MaximumADHD)
