project(
  'vinegar',
  'cpp',
  version: '1.9.4.1',
  license: 'GPL-3.0-only',
  meson_version: '>= 1.4.0',
  default_options: ['warning_level=2', 'cpp_std=c++17', 'buildtype=release'],
)

app_id = 'org.vinegarhq.Vinegar'

gnome = import('gnome')
i18n = import('i18n')

prefix = get_option('prefix')
bindir = prefix / get_option('bindir')
datadir = prefix / get_option('datadir')
localedir = prefix / get_option('localedir')
pkgdatadir = datadir / meson.project_name()

# GTK and libadwaita are loaded at runtime through puregotk, but are
# checked here so that a misconfigured system fails early. GNOME 50
# ships libadwaita 1.9, GNOME 51 ships 1.10.
dependency('gtk4', version: '>= 4.20')
dependency('libadwaita-1', version: '>= 1.9')

subdir('data')
subdir('po')
subdir('layer')

go = find_program('go')
go_ldflags = ' '.join([
  '-s', '-w',
  '-X', 'main.LocaleDir=' + localedir,
  '-X', 'main.ResourceDir=' + pkgdatadir,
])

custom_target(
  'vinegar',
  output: 'vinegar',
  command: [
    go, 'build', '-C', meson.project_source_root(),
    '-trimpath', '-buildvcs=false',
    '-ldflags', go_ldflags,
    '-o', meson.current_build_dir() / 'vinegar',
    './cmd/vinegar',
  ],
  build_by_default: true,
  build_always_stale: true,
  console: true,
  install: true,
  install_dir: bindir,
)

test(
  'go-test',
  go,
  args: ['test', '-C', meson.project_source_root(), './...'],
  timeout: 300,
)

devenv = environment()
devenv.set('VINEGAR_RESOURCE', meson.current_build_dir() / 'data' / 'vinegar.gresource')
devenv.prepend('PATH', meson.current_build_dir())
meson.add_devenv(devenv)

gnome.post_install(
  gtk_update_icon_cache: true,
  update_desktop_database: true,
  update_mime_database: true,
)

summary({
  'prefix': prefix,
  'resources': pkgdatadir,
  'locales': localedir,
}, section: 'Directories')
