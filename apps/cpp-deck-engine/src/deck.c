#include "deck.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

static const char *RANK_LABELS[] = {
    "", "A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K",
};
static const char SUIT_LABELS[] = {'C', 'D', 'H', 'S'};

deck_t *deck_new(void) {
    deck_t *deck = malloc(sizeof(deck_t));
    int i = 0;
    for (int suit = 0; suit < 4; suit++) {
        for (int rank = 1; rank <= 13; rank++) {
            deck->cards[i].rank = rank;
            deck->cards[i].suit = suit;
            i++;
        }
    }
    deck->remaining = DECK_SIZE;
    return deck;
}

void deck_free(deck_t *deck) {
    free(deck);
}

/* Fisher-Yates shuffle of the cards still in the deck. */
void deck_shuffle(deck_t *deck) {
    srand((unsigned)time(NULL));
    for (int i = deck->remaining - 1; i > 0; i--) {
        int j = rand() % i;
        card_t tmp = deck->cards[i];
        deck->cards[i] = deck->cards[j];
        deck->cards[j] = tmp;
    }
}

/* Deals `count` cards from the top of the deck into `out`.
 * Returns the number of cards dealt. */
int deck_deal(deck_t *deck, card_t *out, int count) {
    for (int i = 0; i < count; i++) {
        deck->remaining--;
        out[i] = deck->cards[deck->remaining];
    }
    return count;
}

/* Writes a card code such as "AS" or "10H" into `out`. */
void card_to_string(card_t card, char *out) {
    char buf[3];
    strcpy(buf, RANK_LABELS[card.rank]);
    buf[strlen(buf)] = SUIT_LABELS[card.suit];
    strcpy(out, buf);
}
