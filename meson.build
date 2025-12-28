# apgbuild - AnmiTali Package Builder
# Author: AnmiTaliDev <anmitali198@gmail.com>
# Copyright (c) 2026 NurOS
# License: GPL 3.0

project('apgbuild',
  version: '2.0.0',
  license: 'GPL-3.0',
  meson_version: '>= 0.60.0'
)

go = find_program('go', required: true)
source_root = meson.project_source_root()
build_root = meson.project_build_root()

# Build apgbuild binary
apgbuild = custom_target('apgbuild',
  output: 'apgbuild',
  command: [
    go, 'build',
    '-C', source_root,
    '-ldflags', '-s -w',
    '-o', build_root / 'apgbuild',
    './cmd/apgbuild'
  ],
  build_by_default: true,
  install: true,
  install_dir: get_option('bindir')
)
