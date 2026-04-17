project('apgbuild',
  version: '2.0.0',
  default_options: ['warning_level=0'],
  meson_version: '>= 0.60.0',
)

go     = find_program('go',      required: true)
pkgcfg = find_program('pkg-config', required: true)

self_cflags  = '-march=native -O3 -flto=thin -DNDEBUG'
self_ldflags = '-fuse-ld=mold -flto=thin'

# ── Resolve C library flags via pkg-config ────────────────────────────────────
# These are passed to CGO so the Go code can link against libarchive and libapg.

# libarchive — required for archive.go CGO
libarchive_cflags = run_command(pkgcfg, '--cflags', 'libarchive', check: false).stdout().strip()
libarchive_libs   = run_command(pkgcfg, '--libs',   'libarchive', check: false).stdout().strip()
if libarchive_libs == ''
  libarchive_libs = '-larchive'  # fallback if pkg-config not available
endif

# ── Stage 1: libapg (C library, built via its own meson.build) ───────────────

libapg_build = custom_target('libapg',
  output: 'libapg.stamp',
  build_by_default: true,
  command: [
    'sh', '-c',
    '''set -e
    cd "@0@/libapg"
    meson setup build --prefix=/usr --buildtype=release \
      -Dc_args="@1@" -Dc_link_args="@2@" 2>/dev/null || \
    meson setup --reconfigure build --prefix=/usr --buildtype=release \
      -Dc_args="@1@" -Dc_link_args="@2@"
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

# ── Stage 2: apgbuild CLI (Go + CGO) ─────────────────────────────────────────
# CGO_CFLAGS/CGO_LDFLAGS include pkg-config output so #include <archive.h>
# and -larchive are resolved correctly without hardcoding paths.

cgo_cflags  = self_cflags  + ' ' + libarchive_cflags
cgo_ldflags = self_ldflags + ' -lapg ' + libarchive_libs

go_env = environment()
go_env.set('CGO_CFLAGS',  cgo_cflags)
go_env.set('CGO_LDFLAGS', cgo_ldflags)
go_env.set('CGO_ENABLED', '1')
go_env.set('GOFLAGS',     '-mod=mod')

go_ldflags_str = '-s -w -extldflags "' + self_ldflags + ' -lapg ' + libarchive_libs + '"'

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
  'Binary'          : 'apgbuild',
  'libapg'          : 'built from submodule',
  'libarchive flags': libarchive_libs,
  'CGO_CFLAGS'      : cgo_cflags,
  'CGO_LDFLAGS'     : cgo_ldflags,
}, section: 'APGbuild')
