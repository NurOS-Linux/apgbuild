project('apgbuild',
  version: '2.0.0',
  default_options: ['warning_level=0'],
  meson_version: '>= 0.60.0',
)

go = find_program('go', required: true)

self_cflags  = '-march=native -O3 -flto=thin -DNDEBUG'
self_ldflags = '-fuse-ld=mold -flto=thin'

# ── Stage 1: libapg (C library) ───────────────────────────────────────────────
# libapg must be built and installed before apgbuild CLI can link against it.

libapg_build = custom_target('libapg',
  output: 'libapg.stamp',
  build_by_default: true,
  command: [
    'sh', '-c',
    '''set -e
    cd "@0@/libapg"
    meson setup build --prefix=/usr --buildtype=release \
      -Dc_args="@1@" -Dc_link_args="@2@" 2>/dev/null || true
    ninja -C build
    ninja -C build install
    touch "@3@/libapg.stamp"
    '''.format(
      meson.current_source_dir(),
      self_cflags,
      self_ldflags,
      meson.current_build_dir(),
    ),
  ],
)

# ── Stage 2: apgbuild CLI (Go) ────────────────────────────────────────────────

go_env = environment()
go_env.set('CGO_CFLAGS',  self_cflags)
go_env.set('CGO_LDFLAGS', self_ldflags + ' -lapg')
go_env.set('CGO_ENABLED', '1')
go_env.set('GOFLAGS',     '-mod=mod')

go_ldflags_str = '-s -w -extldflags "' + self_ldflags + ' -lapg"'

go_mod_dir = meson.current_source_dir()

go_mod_download = custom_target('apgbuild-go-mod-download',
  output: 'apgbuild-go-mod-download.stamp',
  depends: [libapg_build],
  command: [
    'sh', '-c',
    'cd "@0@" && @1@ mod download && touch "@2@/apgbuild-go-mod-download.stamp"'.format(
      go_mod_dir,
      go.full_path(),
      meson.current_build_dir(),
    ),
  ],
  env: go_env,
  build_by_default: true,
)

apgbuild_exe = custom_target('apgbuild',
  output: 'apgbuild',
  depends: [go_mod_download],
  build_by_default: true,
  install: true,
  install_dir: get_option('bindir'),
  env: go_env,
  command: [
    go, 'build',
    '-ldflags', go_ldflags_str,
    '-o', '@OUTPUT@',
    join_paths(go_mod_dir, 'cmd', 'apgbuild'),
  ],
)

summary({
  'Binary'  : 'apgbuild',
  'libapg'  : 'built from submodule',
  'CFLAGS'  : self_cflags,
  'LDFLAGS' : self_ldflags,
}, section: 'APGbuild')
