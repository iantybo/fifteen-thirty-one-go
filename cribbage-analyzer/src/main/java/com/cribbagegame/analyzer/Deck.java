package com.cribbagegame.analyzer;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Random;

/**
 * Represents a standard 52-card deck.
 */
public class Deck {
    private final List<Card> cards;
    private final Random random;

    public Deck() {
        this(new Random());
    }

    public Deck(Random random) {
        this.random = random;
        this.cards = new ArrayList<>();
        initializeDeck();
    }

    private void initializeDeck() {
        for (Card.Suit suit : Card.Suit.values()) {
            for (Card.Rank rank : Card.Rank.values()) {
                cards.add(new Card(rank, suit));
            }
        }
    }

    public void shuffle() {
        Collections.shuffle(cards, random);
    }

    public Card draw() {
        if (cards.isEmpty()) {
            throw new IllegalStateException("Deck is empty");
        }
        return cards.remove(cards.size() - 1);
    }

    public List<Card> drawMultiple(int count) {
        if (count > cards.size()) {
            throw new IllegalStateException("Not enough cards in deck");
        }
        List<Card> drawn = new ArrayList<>();
        for (int i = 0; i < count; i++) {
            drawn.add(draw());
        }
        return drawn;
    }

    public int size() {
        return cards.size();
    }

    public boolean isEmpty() {
        return cards.isEmpty();
    }

    public void reset() {
        cards.clear();
        initializeDeck();
    }
}
