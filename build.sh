clang++ -std=c++17 -Wall -Wextra -O3 -flto -march=native \
main.cpp -o apgbuild \
-lssl -lcrypto