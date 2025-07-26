project('apgbuild', 'cpp',
  version : '0.2.0',
  default_options : [
    'warning_level=3',
    'werror=false',
    'cpp_std=c++17',
    'optimization=3',
    'b_lto=true'
  ])

# Add native optimization flag if supported
native_opt = []
if meson.get_compiler('cpp').has_argument('-march=native')
  native_opt = ['-march=native']
endif

# Find dependencies
openssl_dep = dependency('openssl')
nlohmann_json_dep = dependency('nlohmann_json')
libarchive_dep = dependency('libarchive')

# Define the executable
apgbuild_exe = executable('apgbuild',
  'src/main.cpp',
  dependencies : [openssl_dep, nlohmann_json_dep, libarchive_dep],
  cpp_args : native_opt,
  install : true)