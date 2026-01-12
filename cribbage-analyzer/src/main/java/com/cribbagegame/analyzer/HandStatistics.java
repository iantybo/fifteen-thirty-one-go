package com.cribbagegame.analyzer;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Provides fun statistics and insights about cribbage hands.
 */
public class HandStatistics {

    /**
     * Get interesting statistics and fun facts about a hand.
     */
    public static Statistics analyze(List<Card> hand, Card starter, HandScore score) {
        return new Statistics(hand, starter, score);
    }

    public static class Statistics {
        private final List<Card> hand;
        private final Card starter;
        private final HandScore score;

        public Statistics(List<Card> hand, Card starter, HandScore score) {
            this.hand = hand;
            this.starter = starter;
            this.score = score;
        }

        /**
         * Get a fun description of the hand quality.
         */
        public String getHandQuality() {
            int totalScore = score.getTotalScore();

            if (totalScore == 29) return "🎉 PERFECT HAND! (Extremely rare - 1 in 216,580!)";
            if (totalScore >= 24) return "🔥 Exceptional! (Top 0.1% of hands)";
            if (totalScore >= 20) return "⭐ Outstanding! (Top 1% of hands)";
            if (totalScore >= 16) return "✨ Excellent! (Top 10% of hands)";
            if (totalScore >= 12) return "👍 Very Good! (Above average)";
            if (totalScore >= 8) return "😊 Good (Solid hand)";
            if (totalScore >= 4) return "🙂 Average (Typical hand)";
            if (totalScore >= 2) return "😐 Below Average";
            if (totalScore == 0) return "😢 Zero points (About 10% of hands score nothing!)";
            return "🤔 Unusual";
        }

        /**
         * Get probability information about the hand score.
         */
        public String getProbabilityInfo() {
            int totalScore = score.getTotalScore();

            if (totalScore == 29) {
                return "A 29-point hand requires J-5-5-5 with the starter being the 5 of the same suit as the Jack. " +
                       "Probability: 0.000463% or 1 in 216,580 hands!";
            } else if (totalScore == 28) {
                return "28 points is IMPOSSIBLE in cribbage! The highest score after 29 is 24.";
            } else if (totalScore == 27 || totalScore == 26 || totalScore == 25) {
                return totalScore + " points is impossible to achieve in cribbage due to the scoring combinations.";
            } else if (totalScore >= 24) {
                return "Scoring 24+ points happens in less than 0.2% of hands. You're extremely lucky!";
            } else if (totalScore >= 20) {
                return "20+ point hands occur in about 1% of deals. This is a rare treat!";
            } else if (totalScore >= 16) {
                return "16+ point hands happen in roughly 5% of deals. Well done!";
            } else if (totalScore >= 12) {
                return "12+ point hands occur in about 15% of deals. This is above average.";
            } else if (totalScore >= 8) {
                return "8+ point hands happen in about 35% of deals. A solid result.";
            } else if (totalScore >= 4) {
                return "4-7 point hands are the most common, occurring in about 40% of deals.";
            } else if (totalScore == 0) {
                return "Zero-point hands occur in about 10% of deals. Don't worry, it happens to everyone!";
            }
            return "Average expected score per hand is around 8 points.";
        }

        /**
         * Get interesting patterns in the hand.
         */
        public List<String> getInterestingPatterns() {
            List<String> patterns = new ArrayList<>();

            // Check for rare patterns
            if (hasAllSameSuit()) {
                patterns.add("🃏 Suited hand - All cards are " + hand.get(0).getSuit().getSymbol());
            }

            if (hasSequentialRanks()) {
                patterns.add("📊 Sequential ranks - Cards form a natural sequence");
            }

            if (hasManyFaceCards()) {
                patterns.add("👑 Royal hand - Loaded with face cards");
            }

            if (hasAllLowCards()) {
                patterns.add("🎲 Low cards - All cards 5 or under");
            }

            if (hasThreeOfAKind()) {
                patterns.add("🎯 Three of a kind - Rare triple!");
            }

            if (hasFourOfAKind()) {
                patterns.add("💎 Four of a kind - Extremely rare quad!");
            }

            if (hasPerfectFifteenHand()) {
                patterns.add("🎰 Fifteen heaven - Multiple fifteen combinations!");
            }

            if (isBalancedHand()) {
                patterns.add("⚖️ Balanced hand - Good mix of high and low cards");
            }

            return patterns;
        }

        /**
         * Get a fun fact about the specific cards.
         */
        public String getFunFact() {
            List<String> facts = new ArrayList<>();

            // Card-specific facts
            if (hasCard(Card.Rank.FIVE)) {
                facts.add("💡 Fives are the most valuable cards in cribbage due to their fifteen-making potential!");
            }

            if (hasCard(Card.Rank.JACK) && hasCard(Card.Rank.FIVE)) {
                facts.add("🎴 Jack-Five combo: The foundation of the perfect 29-point hand!");
            }

            if (score.getFifteensCount() >= 8) {
                facts.add("🎊 Amazing! You found " + score.getFifteensCount() + " different fifteen combinations!");
            }

            if (score.getPairsCount() >= 3) {
                facts.add("👥 Multiple pairs! Each pair combination is worth 2 points.");
            }

            if (score.getRunsScore() >= 12) {
                facts.add("🏃 Long runs are valuable! Yours scored " + score.getRunsScore() + " points!");
            }

            Map<Card.Rank, Long> rankCounts = hand.stream()
                    .collect(Collectors.groupingBy(Card::getRank, Collectors.counting()));
            if (rankCounts.values().stream().anyMatch(c -> c == 4)) {
                facts.add("🌟 Four of a kind scores 12 points just from pairs alone!");
            }

            if (facts.isEmpty()) {
                facts.add("📚 The average cribbage hand scores about 8 points.");
            }

            return facts.get(new Random().nextInt(facts.size()));
        }

        /**
         * Suggest what cards would have made the hand better.
         */
        public String getWhatIfSuggestion() {
            int totalScore = score.getTotalScore();

            if (totalScore == 29) {
                return "Your hand is perfect! Nothing could make it better!";
            }

            if (totalScore >= 20) {
                return "Your hand is already exceptional. Small improvements might add 1-4 points.";
            }

            if (score.getFifteensCount() == 0) {
                return "Adding cards that sum to 15 would significantly improve this hand.";
            }

            if (score.getPairsCount() == 0 && !hasMultipleSameRank()) {
                return "A matching rank would add pairs and potentially boost your score by 2-6 points.";
            }

            if (score.getRunsScore() == 0) {
                return "Cards forming a sequence (run) would add 3+ points.";
            }

            return "With the right cards, most hands can improve by 4-8 points on average.";
        }

        // Helper methods for pattern detection
        private boolean hasAllSameSuit() {
            Card.Suit firstSuit = hand.get(0).getSuit();
            return hand.stream().allMatch(c -> c.getSuit() == firstSuit);
        }

        private boolean hasSequentialRanks() {
            List<Integer> values = hand.stream()
                    .map(c -> c.getRank().getNumericValue())
                    .sorted()
                    .collect(Collectors.toList());

            for (int i = 1; i < values.size(); i++) {
                if (values.get(i) != values.get(i - 1) + 1) {
                    return false;
                }
            }
            return true;
        }

        private boolean hasManyFaceCards() {
            long faceCount = hand.stream()
                    .filter(c -> c.getRank().getCribbageValue() == 10 && c.getRank().getNumericValue() >= 11)
                    .count();
            return faceCount >= 3;
        }

        private boolean hasAllLowCards() {
            return hand.stream().allMatch(c -> c.getRank().getNumericValue() <= 5);
        }

        private boolean hasThreeOfAKind() {
            Map<Card.Rank, Long> rankCounts = hand.stream()
                    .collect(Collectors.groupingBy(Card::getRank, Collectors.counting()));
            return rankCounts.values().stream().anyMatch(c -> c == 3);
        }

        private boolean hasFourOfAKind() {
            Map<Card.Rank, Long> rankCounts = hand.stream()
                    .collect(Collectors.groupingBy(Card::getRank, Collectors.counting()));
            return rankCounts.values().stream().anyMatch(c -> c == 4);
        }

        private boolean hasPerfectFifteenHand() {
            return score.getFifteensCount() >= 6;
        }

        private boolean isBalancedHand() {
            int sum = hand.stream().mapToInt(Card::getCribbageValue).sum();
            return sum >= 16 && sum <= 24; // Reasonable middle range
        }

        private boolean hasCard(Card.Rank rank) {
            return hand.stream().anyMatch(c -> c.getRank() == rank) ||
                   starter.getRank() == rank;
        }

        private boolean hasMultipleSameRank() {
            Map<Card.Rank, Long> rankCounts = hand.stream()
                    .collect(Collectors.groupingBy(Card::getRank, Collectors.counting()));
            return rankCounts.values().stream().anyMatch(c -> c >= 2);
        }
    }
}
