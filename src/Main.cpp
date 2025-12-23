/**
 * AnmiTali Package Builder (apgbuild)
 * A package building utility
 *
 * Author: AnmiTaliDev, Ruzen42
 * Created: 2025-02-04 18:27:31 UTC
 * License: GPL 3.0
 */

#include <../include/ApgBuilder.hpp>
#include <../include/Helper.hpp>
#include <iostream>

int main(int argc, char *argv[]) {
  if (argc > 1 && std::string(argv[1]) == "--help") {
    const ApgBuilder builder;
    builder.showHelp();
    return EXIT_SUCCESS;
  }

  try {
    ApgBuilder builder;
    builder.run(argc, argv);
    return EXIT_SUCCESS;
  } catch (const std::exception &e) {
    std::cerr << Color::RED << "Fatal error: " << e.what() << Color::RESET
              << std::endl;
    return EXIT_FAILURE;
  }
}
