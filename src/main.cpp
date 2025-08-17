/**
 * AnmiTali Package Builder (apgbuild)
 * A package building utility
 * 
 * Author: AnmiTaliDev
 * Created: 2025-02-04 18:27:31 UTC
 * License: GPL 3.0
 */

#include <iostream>
#include <string>
#include <vector>
#include <filesystem>
#include <fstream>
#include <cstdlib>
#include <map>
#include <nlohmann/json.hpp>
#include <openssl/evp.h>
#include <cstring>
#include <algorithm>
#include <sstream>
#include <iomanip>
#include <regex>
#include <chrono>
#include <stdexcept>
#include <memory>
#include <optional>
#include <archive.h>
#include <archive_entry.h>
#include <sys/stat.h>

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
    const std::string VERSION_STR = "v0.2.0";
    const std::string AUTHOR = "AnmiTaliDev";
    const std::string LICENSE = "GPL 3.0";
    const std::string COPYRIGHT = "Copyright (c) 2025 AnmiTaliDev";
    const std::string DESCRIPTION = "AnmiTali Package Builder - A  package building utility";
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

    std::string calculateMD5(const std::string& filepath) const {
        if (filepath.empty()) {
            throw std::invalid_argument("File path cannot be empty");
        }
        
        if (!fs::exists(filepath)) {
            throw std::runtime_error("File does not exist: " + filepath);
        }
        
        std::ifstream file(filepath, std::ios::binary);
        if (!file.is_open()) {
            throw std::runtime_error("Failed to open file: " + filepath);
        }

        EVP_MD_CTX* md5Context = EVP_MD_CTX_new();
        if (!md5Context) {
            throw std::runtime_error("Failed to create MD5 context");
        }

        if (EVP_DigestInit_ex(md5Context, EVP_md5(), nullptr) != 1) {
            EVP_MD_CTX_free(md5Context);
            throw std::runtime_error("Failed to initialize MD5 context");
        }
        
        constexpr size_t BUFFER_SIZE = 16 * 1024;
        std::vector<char> buffer(BUFFER_SIZE);
        
        while (file.good()) {
            file.read(buffer.data(), BUFFER_SIZE);
            const auto bytes_read = file.gcount();
            if (bytes_read > 0) {
                if (EVP_DigestUpdate(md5Context, buffer.data(), static_cast<size_t>(bytes_read)) != 1) {
                    EVP_MD_CTX_free(md5Context);
                    throw std::runtime_error("Failed to update MD5 hash");
                }
            }
        }
        
        if (file.bad()) {
            EVP_MD_CTX_free(md5Context);
            throw std::runtime_error("Error reading file: " + filepath);
        }
        
        unsigned char result[EVP_MAX_MD_SIZE];
        unsigned int result_len = 0;
        if (EVP_DigestFinal_ex(md5Context, result, &result_len) != 1) {
            EVP_MD_CTX_free(md5Context);
            throw std::runtime_error("Failed to finalize MD5 hash");
        }

        EVP_MD_CTX_free(md5Context);

        std::ostringstream ss;
        for (unsigned int i = 0; i < result_len; ++i) {
            ss << std::hex << std::setw(2) << std::setfill('0') << static_cast<unsigned int>(result[i]);
        }
        return ss.str();
    }

    void createMD5Sums(const std::string& directory, const std::string& output) const {
        validateDirectoryPath(directory);
        validateOutputPath(output);
        
        std::cout << Color::CYAN << "Creating MD5 checksums for directory: " << directory << Color::RESET << std::endl;

        if (!fs::exists(directory)) {
            throw std::runtime_error("Directory does not exist: " + directory);
        }
        
        if (!fs::is_directory(directory)) {
            throw std::runtime_error("Path is not a directory: " + directory);
        }

        std::ofstream md5file(output);
        if (!md5file.is_open()) {
            throw std::runtime_error("Failed to create md5sums file: " + output);
        }

        size_t files_processed = 0;
        std::error_code ec;
        
        for (const auto& entry : fs::recursive_directory_iterator(directory, ec)) {
            if (ec) {
                throw std::runtime_error("Error iterating directory: " + ec.message());
            }
            
            if (entry.is_regular_file(ec) && !ec) {
                try {
                    const std::string relative_path = fs::relative(entry.path(), directory).string();
                    const std::string md5sum = calculateMD5(entry.path().string());
                    md5file << md5sum << "  " << relative_path << "\n";
                    std::cout << Color::GREEN << "✓ " << Color::RESET << relative_path << std::endl;
                    ++files_processed;
                } catch (const std::exception& e) {
                    std::cerr << Color::RED << "Warning: Failed to process " << entry.path() 
                             << ": " << e.what() << Color::RESET << std::endl;
                }
            }
        }
        
        if (!md5file.good()) {
            throw std::runtime_error("Error writing to md5sums file");
        }

        std::cout << Color::GREEN << "md5sums file created successfully! Processed " 
                  << files_processed << " files." << Color::RESET << std::endl;
    }

    void validateDirectoryPath(const std::string& path) const {
        if (path.empty()) {
            throw std::invalid_argument("Directory path cannot be empty");
        }
        if (path.find("\0") != std::string::npos) {
            throw std::invalid_argument("Directory path contains null characters");
        }
    }
    
    void createArchive(const std::string& archive_path, const std::string& source_dir) const {
        struct archive *a = nullptr;
        struct archive_entry *entry = nullptr;
        struct stat st;
        constexpr size_t BUFFER_SIZE = 64 * 1024;
        std::vector<char> buffer(BUFFER_SIZE);
        FILE *fd = nullptr;
        size_t files_added = 0;

        try {
            a = archive_write_new();
            if (!a) {
                throw std::runtime_error("Failed to create archive object");
            }

            if (archive_write_add_filter_xz(a) != ARCHIVE_OK) {
                throw std::runtime_error("Failed to set XZ compression: " + std::string(archive_error_string(a)));
            }
            
            if (archive_write_set_format_ustar(a) != ARCHIVE_OK) {
                throw std::runtime_error("Failed to set TAR format: " + std::string(archive_error_string(a)));
            }

            if (archive_write_open_filename(a, archive_path.c_str()) != ARCHIVE_OK) {
                throw std::runtime_error("Failed to open archive file '" + archive_path + "': " + std::string(archive_error_string(a)));
            }

            std::error_code ec;
            for (const auto& dir_entry : fs::recursive_directory_iterator(source_dir, ec)) {
                if (ec) {
                    throw std::runtime_error("Error iterating directory '" + source_dir + "': " + ec.message());
                }

                const std::string full_path = dir_entry.path().string();
                const std::string relative_path = fs::relative(dir_entry.path(), source_dir).string();

                if (stat(full_path.c_str(), &st) != 0) {
                    std::cerr << Color::RED << "Warning: Cannot stat file, skipping: " << full_path << Color::RESET << std::endl;
                    continue;
                }

                entry = archive_entry_new();
                if (!entry) {
                    throw std::runtime_error("Failed to create archive entry");
                }

                archive_entry_set_pathname(entry, relative_path.c_str());
                archive_entry_copy_stat(entry, &st);

                if (archive_write_header(a, entry) != ARCHIVE_OK) {
                    std::cerr << Color::RED << "Warning: Failed to write header for: " << relative_path 
                             << " - " << archive_error_string(a) << Color::RESET << std::endl;
                    archive_entry_free(entry);
                    entry = nullptr;
                    continue;
                }

                if (S_ISREG(st.st_mode)) {
                    fd = fopen(full_path.c_str(), "rb");
                    if (!fd) {
                        std::cerr << Color::RED << "Warning: Cannot open file, skipping: " << full_path << Color::RESET << std::endl;
                        archive_entry_free(entry);
                        entry = nullptr;
                        continue;
                    }

                    size_t bytes_read;
                    while ((bytes_read = fread(buffer.data(), 1, BUFFER_SIZE, fd)) > 0) {
                        if (archive_write_data(a, buffer.data(), bytes_read) < 0) {
                            fclose(fd);
                            fd = nullptr;
                            archive_entry_free(entry);
                            entry = nullptr;
                            throw std::runtime_error("Failed to write data for '" + relative_path + "': " + std::string(archive_error_string(a)));
                        }
                    }

                    if (ferror(fd)) {
                        fclose(fd);
                        fd = nullptr;
                        archive_entry_free(entry);
                        entry = nullptr;
                        throw std::runtime_error("Error reading file: " + full_path);
                    }

                    fclose(fd);
                    fd = nullptr;
                }

                archive_entry_free(entry);
                entry = nullptr;
                ++files_added;
            }

            if (archive_write_close(a) != ARCHIVE_OK) {
                throw std::runtime_error("Failed to close archive: " + std::string(archive_error_string(a)));
            }

            std::cout << Color::GREEN << "✓ Archive created with " << files_added << " files" << Color::RESET << std::endl;

        } catch (...) {
            if (fd) fclose(fd);
            if (entry) archive_entry_free(entry);
            if (a) {
                archive_write_close(a);
                archive_write_free(a);
            }
            throw;
        }

        if (a) archive_write_free(a);
    }

    bool isPathSafe(const std::string& path, const std::string& extract_dir) const {
        if (path.empty()) return false;
        
        if (path.find("..") != std::string::npos) return false;
        
        if (path[0] == '/' || path[0] == '\\') return false;
        
        if (path.find('\0') != std::string::npos) return false;
        
        try {
            const fs::path full_path = fs::canonical(fs::path(extract_dir) / path);
            const fs::path base_path = fs::canonical(extract_dir);
            
            const std::string full_str = full_path.string();
            const std::string base_str = base_path.string();
            
            return full_str.substr(0, base_str.length()) == base_str &&
                   (full_str.length() == base_str.length() || 
                    full_str[base_str.length()] == fs::path::preferred_separator);
        } catch (const fs::filesystem_error&) {
            return false;
        }
    }

    void extractArchive(const std::string& archive_path) const {
        struct archive *a;
        struct archive *ext;
        struct archive_entry *entry;
        int r;
        
        constexpr size_t MAX_ARCHIVE_SIZE = 1024 * 1024 * 1024;
        constexpr size_t MAX_FILES = 10000;
        constexpr size_t MAX_FILE_SIZE = 100 * 1024 * 1024;
        
        size_t total_size = 0;
        size_t file_count = 0;
        
        const std::string extract_dir = fs::current_path().string();

        a = archive_read_new();
        ext = archive_write_disk_new();
        
        if (!a || !ext) {
            if (a) archive_read_free(a);
            if (ext) archive_write_free(ext);
            throw std::runtime_error("Failed to create archive objects");
        }

        archive_read_support_format_all(a);
        archive_read_support_filter_all(a);
        archive_write_disk_set_options(ext, ARCHIVE_EXTRACT_TIME | ARCHIVE_EXTRACT_PERM | ARCHIVE_EXTRACT_SECURE_NOABSOLUTEPATHS | ARCHIVE_EXTRACT_SECURE_NODOTDOT);

        if ((r = archive_read_open_filename(a, archive_path.c_str(), 10240))) {
            archive_read_free(a);
            archive_write_free(ext);
            throw std::runtime_error("Failed to open archive: " + archive_path);
        }

        while (true) {
            r = archive_read_next_header(a, &entry);
            if (r == ARCHIVE_EOF) {
                break;
            }
            if (r != ARCHIVE_OK) {
                archive_read_free(a);
                archive_write_free(ext);
                throw std::runtime_error("Archive read error: " + std::string(archive_error_string(a)));
            }

            const char* pathname = archive_entry_pathname(entry);
            if (!pathname) {
                std::cerr << Color::RED << "Warning: Entry with null pathname skipped" << Color::RESET << std::endl;
                continue;
            }

            if (!isPathSafe(pathname, extract_dir)) {
                std::cerr << Color::RED << "Warning: Unsafe path detected and skipped: " << pathname << Color::RESET << std::endl;
                continue;
            }

            const la_int64_t file_size = archive_entry_size(entry);
            if (file_size > static_cast<la_int64_t>(MAX_FILE_SIZE)) {
                std::cerr << Color::RED << "Warning: File too large, skipped: " << pathname << Color::RESET << std::endl;
                continue;
            }

            total_size += static_cast<size_t>(file_size);
            if (total_size > MAX_ARCHIVE_SIZE) {
                archive_read_free(a);
                archive_write_free(ext);
                throw std::runtime_error("Archive too large (>1GB limit)");
            }

            if (++file_count > MAX_FILES) {
                archive_read_free(a);
                archive_write_free(ext);
                throw std::runtime_error("Too many files in archive (>10000 limit)");
            }

            r = archive_write_header(ext, entry);
            if (r != ARCHIVE_OK) {
                std::cerr << Color::RED << "Warning: Failed to write header for: " << pathname << Color::RESET << std::endl;
                continue;
            }

            if (archive_entry_size(entry) > 0) {
                const void *buff;
                size_t size;
                la_int64_t offset;

                while (true) {
                    r = archive_read_data_block(a, &buff, &size, &offset);
                    if (r == ARCHIVE_EOF) {
                        break;
                    }
                    if (r != ARCHIVE_OK) {
                        std::cerr << Color::RED << "Warning: Data read error for: " << pathname << Color::RESET << std::endl;
                        break;
                    }
                    
                    if (archive_write_data_block(ext, buff, size, offset) != ARCHIVE_OK) {
                        std::cerr << Color::RED << "Warning: Data write error for: " << pathname << Color::RESET << std::endl;
                        break;
                    }
                }
            }

            if (archive_write_finish_entry(ext) != ARCHIVE_OK) {
                std::cerr << Color::RED << "Warning: Failed to finish entry for: " << pathname << Color::RESET << std::endl;
            }
        }

        archive_read_close(a);
        archive_read_free(a);
        archive_write_close(ext);
        archive_write_free(ext);
    }
    
    void validateOutputPath(const std::string& path) const {
        if (path.empty()) {
            throw std::invalid_argument("Output path cannot be empty");
        }
        if (path.find("\0") != std::string::npos) {
            throw std::invalid_argument("Output path contains null characters");
        }
    }
    
    std::string getInputLine(const std::string& prompt) const {
        std::string input;
        std::cout << prompt;
        if (!std::getline(std::cin, input)) {
            throw std::runtime_error("Failed to read user input");
        }
        return input;
    }
    
    int getIntegerInput(const std::string& prompt) const {
        const std::string input = getInputLine(prompt);
        try {
            size_t pos;
            const int value = std::stoi(input, &pos);
            if (pos != input.length()) {
                throw std::invalid_argument("Invalid integer format");
            }
            return value;
        } catch (const std::exception&) {
            throw std::invalid_argument("Invalid integer: " + input);
        }
    }

    void createMetadata() const {
        json metadata;

        std::cout << Color::BOLD << Color::CYAN << "\nPackage Metadata Creation Wizard\n"
            << "==============================\n" << Color::RESET << std::endl;

        metadata["name"] = getInputLine("Package name: ");
        metadata["version"] = getInputLine("Version: ");
        metadata["release"] = getIntegerInput("Release number: ");

        metadata["architecture"] = getInputLine("Architecture (x86_64, aarch64, etc.): ");
        metadata["description"] = getInputLine("Description: ");
        metadata["maintainer"] = getInputLine("Maintainer: ");
        metadata["license"] = getInputLine("License: ");
        metadata["homepage"] = getInputLine("Homepage: ");

        metadata["dependencies"] = json::array();
        std::string input = getInputLine("\nAdd dependencies? (y/n): ");
        while (input == "y" || input == "Y") {
            json dep;
            dep["name"] = getInputLine("Dependency name: ");
            dep["version"] = getInputLine("Version: ");
            dep["condition"] = getInputLine("Condition (>=, <=, =, >, <): ");
            metadata["dependencies"].push_back(dep);
            input = getInputLine("Add another dependency? (y/n): ");
        }

        metadata["conflicts"] = json::array();
        input = getInputLine("\nAdd conflicts? (y/n): ");
        while (input == "y" || input == "Y") {
            const std::string conflict = getInputLine("Conflict package name: ");
            metadata["conflicts"].push_back(conflict);
            input = getInputLine("Add another conflict? (y/n): ");
        }

        metadata["provides"] = json::array();
        input = getInputLine("\nAdd provides? (y/n): ");
        while (input == "y" || input == "Y") {
            const std::string provide = getInputLine("Provide name: ");
            metadata["provides"].push_back(provide);
            input = getInputLine("Add another provide? (y/n): ");
        }

        metadata["replaces"] = json::array();
        input = getInputLine("\nAdd replaces? (y/n): ");
        while (input == "y" || input == "Y") {
            const std::string replace = getInputLine("Replace package name: ");
            metadata["replaces"].push_back(replace);
            input = getInputLine("Add another replace? (y/n): ");
        }

        std::cout << Color::GREEN << "\nCreating metadata.json..." << Color::RESET << std::endl;
        std::ofstream file("metadata.json");
        if (!file.is_open()) {
            throw std::runtime_error("Failed to create metadata.json");
        }
        
        try {
            file << metadata.dump(4);
            if (!file.good()) {
                throw std::runtime_error("Error writing metadata to file");
            }
        } catch (const json::exception& e) {
            throw std::runtime_error("JSON serialization error: " + std::string(e.what()));
        }
        
        std::cout << Color::GREEN << "✓ metadata.json created successfully!" << Color::RESET << std::endl;
    }

    void createPackage(const std::string& directory, const std::string& output_path) const {
        validateDirectoryPath(directory);
        validateOutputPath(output_path);
        
        std::cout << Color::CYAN << "Creating package from directory: " << directory << Color::RESET << std::endl;

        if (!fs::exists(directory)) {
            throw std::runtime_error("Directory does not exist: " + directory);
        }
        
        if (!fs::is_directory(directory)) {
            throw std::runtime_error("Path is not a directory: " + directory);
        }

        const std::string data_dir = directory + "/data";
        if (fs::exists(data_dir) && fs::is_directory(data_dir)) {
            createMD5Sums(data_dir, directory + "/md5sums");
        }

        createArchive(output_path, directory);

        std::cout << Color::GREEN << "✓ Package created successfully: " << output_path << Color::RESET << std::endl;
    }

    void extractPackage(const std::string& package_path) const {
        if (package_path.empty()) {
            throw std::invalid_argument("Package path cannot be empty");
        }
        
        std::cout << Color::CYAN << "Extracting package: " << package_path << Color::RESET << std::endl;

        if (!fs::exists(package_path)) {
            throw std::runtime_error("Package file does not exist: " + package_path);
        }
        
        if (!fs::is_regular_file(package_path)) {
            throw std::runtime_error("Path is not a regular file: " + package_path);
        }

        extractArchive(package_path);

        std::cout << Color::GREEN << "✓ Package extracted successfully" << Color::RESET << std::endl;
    }

    void showVersion() const {
        std::cout << Color::BOLD << Color::CYAN << ApgBuilderInfo::DESCRIPTION << Color::RESET << std::endl << std::endl;

        std::cout << "Version:     " << ApgBuilderInfo::VERSION_STR << std::endl;
        std::cout << "Author:      " << ApgBuilderInfo::AUTHOR << std::endl;
        std::cout << "License:     " << ApgBuilderInfo::LICENSE << std::endl;
        std::cout << "Repository:  " << ApgBuilderInfo::REPOSITORY << std::endl;
        std::cout << ApgBuilderInfo::COPYRIGHT << std::endl;
    }
    

    ProgramOptions parseArguments(int argc, char* argv[]) const {
        ProgramOptions options;

        for (int i = 1; i < argc; ++i) {
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
            else if (arg == "-o") {
                if (i + 1 >= argc) {
                    throw std::invalid_argument("Option -o requires an argument");
                }
                options.output_path = argv[++i];
            }
            else if (arg[0] != '-') {
                options.input_dir = arg;
            }
        }

        return options;
    }

public:
    void showHelp() const {
        std::cout << Color::BOLD << Color::CYAN << ApgBuilderInfo::DESCRIPTION << Color::RESET << std::endl;
        std::cout << "\nUsage:\n  apgbuild [options] [directory]\n" << std::endl;

        std::cout << "Options:\n";
        std::cout << "  --makemetadata, -m  Create package metadata file\n";
        std::cout << "  --open              Extract package\n";
        std::cout << "  --makesums          Create MD5 checksums\n";
        std::cout << "  --version, -v       Show version information\n";
        std::cout << "  -o <path>           Specify output path\n";
    }

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
                const std::string output = options.output_path.empty() ?
                    fs::current_path().string() + "/package.apg" :
                    options.output_path;
                createPackage(options.input_dir, output);
                return;
            }

            showHelp();
        }
        catch (const std::exception& e) {
            std::cerr << Color::RED << "Error: " << e.what() << Color::RESET << std::endl;
            std::exit(EXIT_FAILURE);
        }
    }
};

int main(int argc, char* argv[]) {
    std::cout << "Пер ты играеш стендоф :steamhappy:" << std::endl;
    if (argc > 1 && std::string(argv[1]) == "--help") {
        const ApgBuilder builder;
        builder.showHelp();
        return EXIT_SUCCESS;
    }

    try {
        ApgBuilder builder;
        builder.run(argc, argv);
        return EXIT_SUCCESS;
    }
    catch (const std::exception& e) {
        std::cerr << Color::RED << "Fatal error: " << e.what() << Color::RESET << std::endl;
        return EXIT_FAILURE;
    }
}
