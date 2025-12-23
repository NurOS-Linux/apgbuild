#pragma once

/**
 * AnmiTali Package Builder (apgbuild)
 * A package building utility
 *
 * Author: AnmiTaliDev, Ruzen42
 * Created: 2025-02-04 18:27:31 UTC
 * License: GPL 3.0
 */

#include <filesystem>
#include <nlohmann/json.hpp>
#include <string>
#include <vector>

class ApgBuilder {
public:
  void run(int argc, char **argv);
  void showHelp() const;

private:
  struct ProgramOptions {
    bool makemetadata = false;
    bool open = false;
    bool makesums = false;
    bool version = false;
    std::string input_dir;
    std::string output_path;
  };

  std::string calculateMD5(const std::string &filepath) const;
  void createMD5Sums(const std::string &directory,
                     const std::string &output) const;
  void createArchive(const std::string &archive_path,
                     const std::string &source_dir) const;
  void extractArchive(const std::string &archive_path) const;
  bool isPathSafe(const std::string &path,
                  const std::string &extract_dir) const;

  void validateDirectoryPath(const std::string &path) const;
  void validateOutputPath(const std::string &path) const;

  std::string getInputLine(const std::string &prompt) const;
  int getIntegerInput(const std::string &prompt) const;

  void createMetadata() const;
  void createPackage(const std::string &directory,
                     const std::string &output_path) const;
  void extractPackage(const std::string &package_path) const;
  void showVersion() const;

  ProgramOptions parseArguments(int argc, char **argv) const;
};
