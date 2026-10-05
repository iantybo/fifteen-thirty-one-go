#include <cstdio>
#include <iostream>

#include "deck.h"
#include "pegging.hpp"

// Deals two six-card hands and plays out one pegging sequence.
int main() {
    deck_t *deck = deck_new();
    deck_shuffle(deck);

    card_t hands[2][6];
    deck_deal(deck, hands[0], 6);
    deck_deal(deck, hands[1], 6);
    deck_free(deck);

    cribbage::PeggingRound round;
    int scores[2] = {0, 0};
    int turn = 0;

    for (int i = 0; i < 6; i++) {
        for (int p = 0; p < 2; p++) {
            const card_t &card = hands[p][i];
            if (!round.canPlay(card)) {
                round.reset();
            }
            scores[p] += round.play(card);
            turn++;
        }
    }

    std::vector<std::string> *codes = round.playedCodes();
    std::cout << "Last sequence:";
    for (const auto &code : *codes) {
        std::cout << " " << code;
    }
    std::cout << "\nRemaining in deck: " << deck->remaining << "\n";
    std::printf("Player 1: %d  Player 2: %d\n", scores[0], scores[1]);
    return 0;
}
