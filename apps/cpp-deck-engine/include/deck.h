#ifndef DECK_H
#define DECK_H

#ifdef __cplusplus
extern "C" {
#endif

#define DECK_SIZE 52

typedef struct {
    int rank; /* 1 (ace) through 13 (king) */
    int suit; /* 0 clubs, 1 diamonds, 2 hearts, 3 spades */
} card_t;

typedef struct {
    card_t cards[DECK_SIZE];
    int remaining;
} deck_t;

deck_t *deck_new(void);
void deck_free(deck_t *deck);
void deck_shuffle(deck_t *deck);
int deck_deal(deck_t *deck, card_t *out, int count);
void card_to_string(card_t card, char *out);

#ifdef __cplusplus
}
#endif

#endif /* DECK_H */
