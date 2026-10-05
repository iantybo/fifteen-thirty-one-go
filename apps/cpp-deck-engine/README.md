# cpp-deck-engine

A small C/C++ engine for cribbage: a C deck module (create, shuffle, deal)
and a C++ pegging tracker that scores fifteens, 31s, pairs, and runs.

## Build and run

```bash
cd apps/cpp-deck-engine
cmake -S . -B build
cmake --build build
./build/deck-engine
```
