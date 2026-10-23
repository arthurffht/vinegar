subdir('ui')

gresource = gnome.compile_resources(
  'vinegar',
  'vinegar.gresource.xml',
  source_dir: [meson.current_source_dir(), meson.current_build_dir() / 'ui'],
  dependencies: blueprints,
  gresource_bundle: true,
  install: true,
  install_dir: pkgdatadir,
)

desktop_file = files(app_id + '.desktop')
install_data(desktop_file, install_dir: datadir / 'applications')

metainfo_file = files(app_id + '.metainfo.xml')
install_data(metainfo_file, install_dir: datadir / 'metainfo')

install_data(app_id + '-studio.xml', install_dir: datadir / 'mime' / 'packages')

install_data(
  'icons' / 'vinegar.svg',
  rename: app_id + '.svg',
  install_dir: datadir / 'icons' / 'hicolor' / 'scalable' / 'apps',
)
install_data(
  'icons' / 'roblox-studio.svg',
  rename: app_id + '.studio.svg',
  install_dir: datadir / 'icons' / 'hicolor' / 'scalable' / 'apps',
)

desktop_utils = find_program('desktop-file-validate', required: false)
if desktop_utils.found()
  test('validate-desktop', desktop_utils, args: [desktop_file])
endif

appstreamcli = find_program('appstreamcli', required: false)
if appstreamcli.found()
  test(
    'validate-metainfo',
    appstreamcli,
    args: ['validate', '--no-net', '--explain', metainfo_file],
  )
endif
