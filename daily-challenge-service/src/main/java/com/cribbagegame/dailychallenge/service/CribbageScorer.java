package com.cribbagegame.dailychallenge.service;

import com.cribbagegame.dailychallenge.model.Card;
import org.springframework.stereotype.Service;

import java.util.*;

/**
 * Service for calculating cribbage hand scores
 */
@Service
public class CribbageScorer {

    /**
     * Calculate the total score for a hand with starter card
     */
    public int scoreHand(List<Card> hand, Card starter, boolean isCrib) {
        if (hand.size() != 4) {
            throw new IllegalArgumentException("Hand must contain exactly 4 cards");
        }

        List<Card> allCards = new ArrayList<>(hand);
        allCards.add(starter);

        int score = 0;
        score += scoreFifteens(allCards);
        score += scorePairs(allCards);
        score += scoreRuns(allCards);
        score += scoreFlush(hand, starter, isCrib);
        score += scoreNobs(hand, starter);

        return score;
    }

    /**
     * Score all combinations that sum to 15 (2 points each)
     */
    private int scoreFifteens(List<Card> cards) {
        int count = 0;
        int n = cards.size();

        // Check all possible combinations
        for (int i = 1; i < (1 << n); i++) {
            int sum = 0;
            for (int j = 0; j < n; j++) {
                if ((i & (1 << j)) != 0) {
                    sum += cards.get(j).getPegValue();
                }
            }
            if (sum == 15) {
                count++;
            }
        }

        return count * 2;
    }

    /**
     * Score pairs (2 points per pair)
     */
    private int scorePairs(List<Card> cards) {
        int score = 0;
        for (int i = 0; i < cards.size(); i++) {
            for (int j = i + 1; j < cards.size(); j++) {
                if (cards.get(i).getRank() == cards.get(j).getRank()) {
                    score += 2;
                }
            }
        }
        return score;
    }

    /**
     * Score runs (1 point per card in the run)
     */
    private int scoreRuns(List<Card> cards) {
        // Sort cards by rank value
        List<Card> sorted = new ArrayList<>(cards);
        sorted.sort(Comparator.comparingInt(c -> c.getRank().getValue()));

        // Try to find runs of 5, 4, or 3 cards
        for (int runLength = 5; runLength >= 3; runLength--) {
            int runsFound = findRuns(sorted, runLength);
            if (runsFound > 0) {
                return runsFound * runLength;
            }
        }

        return 0;
    }

    /**
     * Find number of runs of specified length
     */
    private int findRuns(List<Card> sortedCards, int length) {
        if (length > sortedCards.size()) {
            return 0;
        }

        // For runs of length 5, just check if all cards form a sequence
        if (length == 5) {
            if (isSequence(sortedCards)) {
                return 1;
            }
            return 0;
        }

        // For shorter runs, check all combinations
        List<List<Card>> combinations = generateCombinations(sortedCards, length);
        int runCount = 0;

        for (List<Card> combo : combinations) {
            List<Card> sorted = new ArrayList<>(combo);
            sorted.sort(Comparator.comparingInt(c -> c.getRank().getValue()));
            if (isSequence(sorted)) {
                runCount++;
            }
        }

        return runCount;
    }

    /**
     * Check if cards form a sequence
     */
    private boolean isSequence(List<Card> sortedCards) {
        for (int i = 1; i < sortedCards.size(); i++) {
            if (sortedCards.get(i).getRank().getValue() !=
                sortedCards.get(i - 1).getRank().getValue() + 1) {
                return false;
            }
        }
        return true;
    }

    /**
     * Generate all combinations of specified length
     */
    private List<List<Card>> generateCombinations(List<Card> cards, int length) {
        List<List<Card>> result = new ArrayList<>();
        generateCombinationsHelper(cards, length, 0, new ArrayList<>(), result);
        return result;
    }

    private void generateCombinationsHelper(List<Card> cards, int length, int start,
                                           List<Card> current, List<List<Card>> result) {
        if (current.size() == length) {
            result.add(new ArrayList<>(current));
            return;
        }

        for (int i = start; i < cards.size(); i++) {
            current.add(cards.get(i));
            generateCombinationsHelper(cards, length, i + 1, current, result);
            current.remove(current.size() - 1);
        }
    }

    /**
     * Score flush (4 or 5 points)
     */
    private int scoreFlush(List<Card> hand, Card starter, boolean isCrib) {
        Card.Suit suit = hand.get(0).getSuit();
        boolean allSameSuit = hand.stream().allMatch(c -> c.getSuit() == suit);

        if (!allSameSuit) {
            return 0;
        }

        // 4 points for 4-card flush
        if (!isCrib) {
            // For non-crib hands, 4 cards same suit = 4 points
            if (starter.getSuit() == suit) {
                return 5; // All 5 cards same suit
            }
            return 4;
        } else {
            // For crib, all 5 cards must be same suit
            if (starter.getSuit() == suit) {
                return 5;
            }
            return 0;
        }
    }

    /**
     * Score nobs (1 point for Jack of same suit as starter)
     */
    private int scoreNobs(List<Card> hand, Card starter) {
        for (Card card : hand) {
            if (card.getRank() == Card.Rank.JACK && card.getSuit() == starter.getSuit()) {
                return 1;
            }
        }
        return 0;
    }

    /**
     * Calculate score breakdown for detailed display
     */
    public Map<String, Integer> getScoreBreakdown(List<Card> hand, Card starter, boolean isCrib) {
        if (hand.size() != 4) {
            throw new IllegalArgumentException("Hand must contain exactly 4 cards");
        }

        List<Card> allCards = new ArrayList<>(hand);
        allCards.add(starter);

        Map<String, Integer> breakdown = new LinkedHashMap<>();
        breakdown.put("fifteens", scoreFifteens(allCards));
        breakdown.put("pairs", scorePairs(allCards));
        breakdown.put("runs", scoreRuns(allCards));
        breakdown.put("flush", scoreFlush(hand, starter, isCrib));
        breakdown.put("nobs", scoreNobs(hand, starter));

        int total = breakdown.values().stream().mapToInt(Integer::intValue).sum();
        breakdown.put("total", total);

        return breakdown;
    }
}
