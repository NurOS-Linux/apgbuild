#pragma once

/**
 * AnmiTali Package Builder (apgbuild)
 * A package building utility
 *
 * Author: AnmiTaliDev, Ruzen42
 * Created: 2025-02-04 18:27:31 UTC
 * License: GPL 3.0
 */

namespace Color {
const std::string RESET = "\033[0m";
const std::string RED = "\033[31m";
const std::string GREEN = "\033[32m";
const std::string CYAN = "\033[36m";
const std::string BOLD = "\033[1m";
} // namespace Color

namespace ApgBuilderInfo {
const std::string VERSION_STR = "v0.2.1";
const std::string AUTHOR = "AnmiTaliDev, Ruzen42";
const std::string LICENSE = "GPL 3.0";
const std::string COPYRIGHT = "Copyright (c) 2025 AnmiTaliDev";
const std::string DESCRIPTION =
    "AnmiTali Package Builder - A  package building utility";
const std::string REPOSITORY = "https://github.com/NurOS-Linux/apgbuild";
} // namespace ApgBuilderInfo
