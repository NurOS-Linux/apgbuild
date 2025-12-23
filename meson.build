project('apgbuild', 'cpp',
  version : '0.2.1',
  default_options : [
    'warning_level=3',
    'werror=false',
    'cpp_std=c++23',
    'optimization=2',
    'b_lto=true'
  ])


openssl_dep = dependency('openssl')
nlohmann_json_dep = dependency('nlohmann_json')
libarchive_dep = dependency('libarchive')

sources = ['src/Main.cpp', 'src/ApgBuilder.cpp']

apgbuild_exe = executable('apgbuild', sources,
  dependencies : [openssl_dep, nlohmann_json_dep, libarchive_dep],
  install : true)
