blueprint_sources = files('bootstrapper.blp', 'manager.blp')

blueprints = custom_target(
  'blueprints',
  input: blueprint_sources,
  output: ['bootstrapper.ui', 'manager.ui'],
  command: [
    find_program('blueprint-compiler'),
    'batch-compile', '@OUTDIR@', '@CURRENT_SOURCE_DIR@', '@INPUT@',
  ],
)
