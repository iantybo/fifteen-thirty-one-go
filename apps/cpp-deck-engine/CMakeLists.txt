cmake_minimum_required(VERSION 3.20)
project(cpp_deck_engine C CXX)

set(CMAKE_C_STANDARD 11)
set(CMAKE_CXX_STANDARD 17)

add_executable(deck-engine
    src/deck.c
    src/pegging.cpp
    src/main.cpp
)
target_include_directories(deck-engine PRIVATE include)
