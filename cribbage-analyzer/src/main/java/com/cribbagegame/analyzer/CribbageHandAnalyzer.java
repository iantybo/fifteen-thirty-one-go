package com.cribbagegame.analyzer;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Analyzes cribbage hands and calculates scores with detailed breakdowns.
 */
public class CribbageHandAnalyzer {

    /**
     * Score a cribbage hand (4 cards + starter card).
     *
     * @param hand    The 4 cards in the player's hand
     * @param starter The starter card (cut card)
     * @param isCrib  Whether this is scoring the crib (affects flush rules)
     * @return HandScore with detailed breakdown
     */
    public HandScore scoreHand(List<Card> hand, Card starter, boolean isCrib) {
        if (hand.size() != 4) {
            throw new IllegalArgumentException("Hand must contain exactly 4 cards");
        }

        List<HandScore.ScoreComponent> components = new ArrayList<>();
        List<Card> allCards = new ArrayList<>(hand);
        allCards.add(starter);

        // Score fifteens
        components.addAll(scoreFifteens(allCards));

        // Score pairs
        components.addAll(scorePairs(allCards));

        // Score runs
        components.addAll(scoreRuns(allCards));

        // Score flush
        components.addAll(scoreFlush(hand, starter, isCrib));

        // Score nobs (Jack of same suit as starter)
        components.addAll(scoreNobs(hand, starter));

        return new HandScore(components);
    }

    private List<HandScore.ScoreComponent> scoreFifteens(List<Card> cards) {
        List<HandScore.ScoreComponent> fifteens = new ArrayList<>();
        int n = cards.size();

        // Check all possible combinations
        for (int i = 1; i < (1 << n); i++) {
            List<Card> subset = new ArrayList<>();
            int sum = 0;

            for (int j = 0; j < n; j++) {
                if ((i & (1 << j)) != 0) {
                    Card card = cards.get(j);
                    subset.add(card);
                    sum += card.getCribbageValue();
                }
            }

            if (sum == 15) {
                String desc = subset.stream()
                        .map(Card::toString)
                        .collect(Collectors.joining(" + "));
                fifteens.add(new HandScore.ScoreComponent(
                        HandScore.ScoreType.FIFTEEN,
                        2,
                        desc
                ));
            }
        }

        return fifteens;
    }

    private List<HandScore.ScoreComponent> scorePairs(List<Card> cards) {
        List<HandScore.ScoreComponent> pairs = new ArrayList<>();
        Map<Card.Rank, List<Card>> rankGroups = new HashMap<>();

        // Group cards by rank
        for (Card card : cards) {
            rankGroups.computeIfAbsent(card.getRank(), k -> new ArrayList<>()).add(card);
        }

        // Score pairs, triple pairs, and double pairs
        for (Map.Entry<Card.Rank, List<Card>> entry : rankGroups.entrySet()) {
            List<Card> group = entry.getValue();
            int count = group.size();

            if (count >= 2) {
                // Number of pairs = C(n, 2) = n * (n-1) / 2
                int numPairs = count * (count - 1) / 2;
                String desc = String.format("%d × %s", count, entry.getKey().getSymbol());

                pairs.add(new HandScore.ScoreComponent(
                        HandScore.ScoreType.PAIR,
                        numPairs * 2,
                        desc
                ));
            }
        }

        return pairs;
    }

    private List<HandScore.ScoreComponent> scoreRuns(List<Card> cards) {
        List<HandScore.ScoreComponent> runs = new ArrayList<>();

        // Sort cards by rank
        List<Card> sortedCards = new ArrayList<>(cards);
        sortedCards.sort(Comparator.comparingInt(c -> c.getRank().getNumericValue()));

        // Try to find longest run
        int longestRunLength = 0;
        List<List<Card>> runsFound = new ArrayList<>();

        // Check for runs of length 5, 4, then 3
        for (int length = 5; length >= 3; length--) {
            if (length > cards.size()) continue;

            runsFound = findRunsOfLength(sortedCards, length);
            if (!runsFound.isEmpty()) {
                longestRunLength = length;
                break;
            }
        }

        // Add run components
        for (List<Card> run : runsFound) {
            String desc = run.stream()
                    .map(Card::toString)
                    .collect(Collectors.joining("-"));
            runs.add(new HandScore.ScoreComponent(
                    HandScore.ScoreType.RUN,
                    longestRunLength,
                    desc
            ));
        }

        return runs;
    }

    private List<List<Card>> findRunsOfLength(List<Card> sortedCards, int length) {
        List<List<Card>> runs = new ArrayList<>();

        // Generate all combinations of the specified length
        generateCombinations(sortedCards, length, 0, new ArrayList<>(), runs);

        // Filter to only actual runs (consecutive ranks)
        return runs.stream()
                .filter(this::isRun)
                .collect(Collectors.toList());
    }

    private void generateCombinations(List<Card> cards, int length, int start,
                                      List<Card> current, List<List<Card>> result) {
        if (current.size() == length) {
            result.add(new ArrayList<>(current));
            return;
        }

        for (int i = start; i < cards.size(); i++) {
            current.add(cards.get(i));
            generateCombinations(cards, length, i + 1, current, result);
            current.remove(current.size() - 1);
        }
    }

    private boolean isRun(List<Card> cards) {
        List<Card> sorted = new ArrayList<>(cards);
        sorted.sort(Comparator.comparingInt(c -> c.getRank().getNumericValue()));

        for (int i = 1; i < sorted.size(); i++) {
            int prevValue = sorted.get(i - 1).getRank().getNumericValue();
            int currValue = sorted.get(i).getRank().getNumericValue();
            if (currValue != prevValue + 1) {
                return false;
            }
        }
        return true;
    }

    private List<HandScore.ScoreComponent> scoreFlush(List<Card> hand, Card starter, boolean isCrib) {
        List<HandScore.ScoreComponent> flushes = new ArrayList<>();

        // Check if all hand cards are same suit
        Card.Suit handSuit = hand.get(0).getSuit();
        boolean allHandSameSuit = hand.stream().allMatch(c -> c.getSuit() == handSuit);

        if (!allHandSameSuit) {
            return flushes;
        }

        // In crib, all 5 cards must match for flush
        if (isCrib) {
            if (starter.getSuit() == handSuit) {
                flushes.add(new HandScore.ScoreComponent(
                        HandScore.ScoreType.FLUSH,
                        5,
                        "5-card flush in " + handSuit.getSymbol()
                ));
            }
        } else {
            // In hand, 4-card flush scores 4, 5-card flush scores 5
            if (starter.getSuit() == handSuit) {
                flushes.add(new HandScore.ScoreComponent(
                        HandScore.ScoreType.FLUSH,
                        5,
                        "5-card flush in " + handSuit.getSymbol()
                ));
            } else {
                flushes.add(new HandScore.ScoreComponent(
                        HandScore.ScoreType.FLUSH,
                        4,
                        "4-card flush in " + handSuit.getSymbol()
                ));
            }
        }

        return flushes;
    }

    private List<HandScore.ScoreComponent> scoreNobs(List<Card> hand, Card starter) {
        List<HandScore.ScoreComponent> nobs = new ArrayList<>();

        for (Card card : hand) {
            if (card.getRank() == Card.Rank.JACK && card.getSuit() == starter.getSuit()) {
                nobs.add(new HandScore.ScoreComponent(
                        HandScore.ScoreType.NOBS,
                        1,
                        "Jack of " + starter.getSuit().getSymbol()
                ));
            }
        }

        return nobs;
    }
}
