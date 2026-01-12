package com.cribbagegame.dailychallenge.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * Represents a playing card with rank and suit.
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class Card {
    private Rank rank;
    private Suit suit;

    public enum Rank {
        ACE(1, 1),
        TWO(2, 2),
        THREE(3, 3),
        FOUR(4, 4),
        FIVE(5, 5),
        SIX(6, 6),
        SEVEN(7, 7),
        EIGHT(8, 8),
        NINE(9, 9),
        TEN(10, 10),
        JACK(11, 10),
        QUEEN(12, 10),
        KING(13, 10);

        private final int value;
        private final int pegValue;

        Rank(int value, int pegValue) {
            this.value = value;
            this.pegValue = pegValue;
        }

        public int getValue() {
            return value;
        }

        public int getPegValue() {
            return pegValue;
        }
    }

    public enum Suit {
        CLUBS, DIAMONDS, HEARTS, SPADES
    }

    public int getPegValue() {
        return rank.getPegValue();
    }

    public int getValue() {
        return rank.getValue();
    }

    @Override
    public String toString() {
        return rank.name().substring(0, 1) + suit.name().substring(0, 1);
    }

    /**
     * Parse a card from string format like "AC" (Ace of Clubs)
     */
    public static Card fromString(String cardStr) {
        if (cardStr == null || cardStr.length() < 2) {
            throw new IllegalArgumentException("Invalid card string: " + cardStr);
        }

        char rankChar = cardStr.charAt(0);
        char suitChar = cardStr.charAt(1);

        Rank rank = switch (rankChar) {
            case 'A' -> Rank.ACE;
            case '2' -> Rank.TWO;
            case '3' -> Rank.THREE;
            case '4' -> Rank.FOUR;
            case '5' -> Rank.FIVE;
            case '6' -> Rank.SIX;
            case '7' -> Rank.SEVEN;
            case '8' -> Rank.EIGHT;
            case '9' -> Rank.NINE;
            case 'T', '1' -> Rank.TEN;
            case 'J' -> Rank.JACK;
            case 'Q' -> Rank.QUEEN;
            case 'K' -> Rank.KING;
            default -> throw new IllegalArgumentException("Invalid rank: " + rankChar);
        };

        Suit suit = switch (suitChar) {
            case 'C' -> Suit.CLUBS;
            case 'D' -> Suit.DIAMONDS;
            case 'H' -> Suit.HEARTS;
            case 'S' -> Suit.SPADES;
            default -> throw new IllegalArgumentException("Invalid suit: " + suitChar);
        };

        return new Card(rank, suit);
    }
}
