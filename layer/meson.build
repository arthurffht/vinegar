vulkan = dependency('vulkan')

shared_module(
  'VkLayer_VINEGAR_VinegarLayer',
  'vinegar_layer.cpp',
  dependencies: vulkan.partial_dependency(compile_args: true, includes: true),
  name_prefix: 'lib',
  override_options: ['warning_level=1'],
  install: true,
  install_dir: get_option('prefix') / get_option('libdir'),
)

install_data(
  'VkLayer_VINEGAR_VinegarLayer.json',
  install_dir: datadir / 'vulkan' / 'explicit_layer.d',
)
