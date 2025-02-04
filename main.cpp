/**
 * AnmiTali Package Builder (apgbuild)
 * A modern package building utility
 * 
 * Author: AnmiTaliDev
 * Created: 2025-02-04 18:27:31 UTC
 * License: MIT
 */

#include <iostream>
#include <string>
#include <vector>
#include <filesystem>
#include <fstream>
#include <cstdlib>
#include <map>
#include <nlohmann/json.hpp>
#include <openssl/md5.h>
#include <cstring>
#include <algorithm>
#include <sstream>
#include <iomanip>
#include <regex>
#include <chrono>

namespace fs = std::filesystem;
using json = nlohmann::json;

namespace Color {
    const std::string RESET = "\033[0m";
    const std::string RED = "\033[31m";
    const std::string GREEN = "\033[32m";
    const std::string CYAN = "\033[36m";
    const std::string BOLD = "\033[1m";
}

namespace ApgBuilderInfo {
    const std::string VERSION_STR = "1.0.0";
    const std::string BUILD_DATE = "2025-02-04 18:27:31";
    const std::string AUTHOR = "AnmiTaliDev";
    const std::string LICENSE = "MIT";
    const std::string COPYRIGHT = "Copyright (c) 2025 AnmiTaliDev";
    const std::string DESCRIPTION = "AnmiTali Package Builder - A modern package building utility";
    const std::string REPOSITORY = "https://github.com/NurOS-Linux/apgbuild";
}

class ApgBuilder {
private:
    struct ProgramOptions {
        bool makemetadata = false;
        bool open = false;
        bool makesums = false;
        bool version = false;
        std::string input_dir;
        std::string output_path;
    };

    std::string calculateMD5(const std::string& filepath) {
        std::ifstream file(filepath, std::ios::binary);
        if (!file.is_open()) {
            throw std::runtime_error("Failed to open file: " + filepath);
        }

        MD5_CTX md5Context;
        MD5_Init(&md5Context);
        char buf[1024 * 16];
        while (file.good()) {
            file.read(buf, sizeof(buf));
            MD5_Update(&md5Context, buf, file.gcount());
        }
        unsigned char result[MD5_DIGEST_LENGTH];
        MD5_Final(result, &md5Context);

        std::stringstream ss;
        for (int i = 0; i < MD5_DIGEST_LENGTH; i++) {
            ss << std::hex << std::setw(2) << std::setfill('0') << (int)result[i];
        }
        return ss.str();
    }

    void createMD5Sums(const std::string& directory, const std::string& output) {
        std::cout << Color::CYAN << "Creating MD5 checksums for directory: " << directory << Color::RESET << std::endl;

        if (!fs::exists(directory)) {
            throw std::runtime_error("Directory does not exist: " + directory);
        }

        std::ofstream md5file(output);
        if (!md5file.is_open()) {
            throw std::runtime_error("Failed to create md5sums file");
        }

        for (const auto& entry : fs::recursive_directory_iterator(directory)) {
            if (entry.is_regular_file()) {
                std::string relative_path = fs::relative(entry.path(), directory).string();
                std::string md5sum = calculateMD5(entry.path().string());
                md5file << md5sum << "  " << relative_path << "\n";
                std::cout << Color::GREEN << "✓ " << Color::RESET << relative_path << std::endl;
            }
        }

        std::cout << Color::GREEN << "md5sums file created successfully!" << Color::RESET << std::endl;
    }

    void createMetadata() {
        json metadata;
        std::string input;

        std::cout << Color::BOLD << Color::CYAN << "\nPackage Metadata Creation Wizard\n"
            << "==============================\n" << Color::RESET << std::endl;

        std::cout << "Package name: ";
        std::getline(std::cin, input);
        metadata["name"] = input;

        std::cout << "Version: ";
        std::getline(std::cin, input);
        metadata["version"] = input;

        std::cout << "Release number: ";
        std::getline(std::cin, input);
        metadata["release"] = std::stoi(input);

        std::cout << "Architecture (x86_64, aarch64, etc.): ";
        std::getline(std::cin, input);
        metadata["architecture"] = input;

        std::cout << "Description: ";
        std::getline(std::cin, input);
        metadata["description"] = input;

        std::cout << "Maintainer: ";
        std::getline(std::cin, input);
        metadata["maintainer"] = input;

        std::cout << "License: ";
        std::getline(std::cin, input);
        metadata["license"] = input;

        std::cout << "Homepage: ";
        std::getline(std::cin, input);
        metadata["homepage"] = input;

        metadata["dependencies"] = json::array();
        std::cout << "\nAdd dependencies? (y/n): ";
        std::getline(std::cin, input);
        while (input == "y" || input == "Y") {
            json dep;
            std::cout << "Dependency name: ";
            std::getline(std::cin, input);
            dep["name"] = input;

            std::cout << "Version: ";
            std::getline(std::cin, input);
            dep["version"] = input;

            std::cout << "Condition (>=, <=, =, >, <): ";
            std::getline(std::cin, input);
            dep["condition"] = input;

            metadata["dependencies"].push_back(dep);

            std::cout << "Add another dependency? (y/n): ";
            std::getline(std::cin, input);
        }

        metadata["conflicts"] = json::array();
        std::cout << "\nAdd conflicts? (y/n): ";
        std::getline(std::cin, input);
        while (input == "y" || input == "Y") {
            std::cout << "Conflict package name: ";
            std::getline(std::cin, input);
            metadata["conflicts"].push_back(input);

            std::cout << "Add another conflict? (y/n): ";
            std::getline(std::cin, input);
        }

        metadata["provides"] = json::array();
        std::cout << "\nAdd provides? (y/n): ";
        std::getline(std::cin, input);
        while (input == "y" || input == "Y") {
            std::cout << "Provide name: ";
            std::getline(std::cin, input);
            metadata["provides"].push_back(input);

            std::cout << "Add another provide? (y/n): ";
            std::getline(std::cin, input);
        }

        metadata["replaces"] = json::array();
        std::cout << "\nAdd replaces? (y/n): ";
        std::getline(std::cin, input);
        while (input == "y" || input == "Y") {
            std::cout << "Replace package name: ";
            std::getline(std::cin, input);
            metadata["replaces"].push_back(input);

            std::cout << "Add another replace? (y/n): ";
            std::getline(std::cin, input);
        }

        std::cout << Color::GREEN << "\nCreating metadata.json..." << Color::RESET << std::endl;
        std::ofstream file("metadata.json");
        if (!file.is_open()) {
            throw std::runtime_error("Failed to create metadata.json");
        }
        file << metadata.dump(4);
        std::cout << Color::GREEN << "✓ metadata.json created successfully!" << Color::RESET << std::endl;
    }

    void createPackage(const std::string& directory, const std::string& output_path) {
        std::cout << Color::CYAN << "Creating package from directory: " << directory << Color::RESET << std::endl;

        if (!fs::exists(directory)) {
            throw std::runtime_error("Directory does not exist: " + directory);
        }

        std::string data_dir = directory + "/data";
        if (fs::exists(data_dir)) {
            createMD5Sums(data_dir, directory + "/md5sums");
        }

        std::string cmd = "tar -cJf \"" + output_path + "\" -C \"" + directory + "\" .";
        int result = system(cmd.c_str());
        if (result != 0) {
            throw std::runtime_error("Failed to create package archive");
        }

        std::cout << Color::GREEN << "✓ Package created successfully: " << output_path << Color::RESET << std::endl;
    }

    void extractPackage(const std::string& package_path) {
        std::cout << Color::CYAN << "Extracting package: " << package_path << Color::RESET << std::endl;

        if (!fs::exists(package_path)) {
            throw std::runtime_error("Package file does not exist: " + package_path);
        }

        std::string cmd = "tar -xJf \"" + package_path + "\"";
        int result = system(cmd.c_str());
        if (result != 0) {
            throw std::runtime_error("Failed to extract package");
        }

        std::cout << Color::GREEN << "✓ Package extracted successfully" << Color::RESET << std::endl;
    }

    void showVersion() {
        std::cout << Color::BOLD << Color::CYAN << ApgBuilderInfo::DESCRIPTION << Color::RESET << std::endl << std::endl;

        std::cout << "Version:     " << ApgBuilderInfo::VERSION_STR << std::endl;
        std::cout << "Built on:    " << ApgBuilderInfo::BUILD_DATE << std::endl;
        std::cout << "Author:      " << ApgBuilderInfo::AUTHOR << std::endl;
        std::cout << "License:     " << ApgBuilderInfo::LICENSE << std::endl;
        std::cout << "Repository:  " << ApgBuilderInfo::REPOSITORY << std::endl;
        std::cout << ApgBuilderInfo::COPYRIGHT << std::endl;

        std::cout << "\nUsage:\n  apgbuild [options] [directory]\n" << std::endl;

        std::cout << "Options:\n";
        std::cout << "  --makemetadata, -m  Create package metadata file\n";
        std::cout << "  --open              Extract package\n";
        std::cout << "  --makesums          Create MD5 checksums\n";
        std::cout << "  --version, -v       Show version information\n";
        std::cout << "  -o <path>           Specify output path\n";
    }

    ProgramOptions parseArguments(int argc, char* argv[]) {
        ProgramOptions options;

        for (int i = 1; i < argc; i++) {
            std::string arg = argv[i];
            if (arg == "--makemetadata" || arg == "-m") {
                options.makemetadata = true;
            }
            else if (arg == "--open") {
                options.open = true;
            }
            else if (arg == "--makesums") {
                options.makesums = true;
            }
            else if (arg == "--version" || arg == "-v") {
                options.version = true;
            }
            else if (arg == "-o" && i + 1 < argc) {
                options.output_path = argv[++i];
            }
            else if (arg[0] != '-') {
                options.input_dir = arg;
            }
        }

        return options;
    }

public:
    void run(int argc, char* argv[]) {
        try {
            ProgramOptions options = parseArguments(argc, argv);

            if (options.version) {
                showVersion();
                return;
            }

            if (options.makemetadata) {
                createMetadata();
                return;
            }

            if (options.makesums) {
                if (options.input_dir.empty()) {
                    throw std::runtime_error("Directory not specified for --makesums");
                }
                createMD5Sums(options.input_dir, "md5sums");
                return;
            }

            if (options.open) {
                if (options.input_dir.empty()) {
                    throw std::runtime_error("Package not specified for --open");
                }
                extractPackage(options.input_dir);
                return;
            }

            if (!options.input_dir.empty()) {
                std::string output = options.output_path.empty() ?
                    fs::current_path().string() + "/package.apg" :
                    options.output_path;
                createPackage(options.input_dir, output);
                return;
            }

            showVersion();
        }
        catch (const std::exception& e) {
            std::cerr << Color::RED << "Error: " << e.what() << Color::RESET << std::endl;
            exit(1);
        }
    }
};

int main(int argc, char* argv[]) {
    if (argc > 1 && std::string(argv[1]) == "--help") {
        ApgBuilder builder;
        builder.run(argc, argv);
        return 0;
    }

    try {
        ApgBuilder builder;
        builder.run(argc, argv);
        return 0;
    }
    catch (const std::exception& e) {
        std::cerr << Color::RED << "Fatal error: " << e.what() << Color::RESET << std::endl;
        return 1;
    }
}