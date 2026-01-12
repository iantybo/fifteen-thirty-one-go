package com.cribbagegame.dailychallenge.service;

import com.cribbagegame.dailychallenge.model.Card;
import com.cribbagegame.dailychallenge.model.ChallengeType;
import com.cribbagegame.dailychallenge.model.DailyChallenge;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;

import java.time.LocalDate;
import java.util.*;
import java.util.stream.Collectors;

/**
 * Service for generating daily challenges with deterministic randomness
 */
@Service
public class ChallengeGenerator {

    private final CribbageScorer scorer;
    private final ObjectMapper objectMapper;

    public ChallengeGenerator(CribbageScorer scorer) {
        this.scorer = scorer;
        this.objectMapper = new ObjectMapper();
    }

    /**
     * Generate a challenge for a specific date (deterministic)
     */
    public DailyChallenge generateChallenge(LocalDate date) {
        // Use date as seed for deterministic randomness
        long seed = date.toEpochDay();
        Random random = new Random(seed);

        // Rotate challenge types based on day of week
        ChallengeType type = selectChallengeType(date);

        return switch (type) {
            case OPTIMAL_DISCARD -> generateOptimalDiscardChallenge(date, random);
            case MAX_SCORE_HUNT -> generateMaxScoreChallenge(date, random);
            case BEST_PEG_PLAY -> generateBestPegPlayChallenge(date, random);
        };
    }

    /**
     * Select challenge type based on day of week
     */
    private ChallengeType selectChallengeType(LocalDate date) {
        int dayOfWeek = date.getDayOfWeek().getValue();
        return switch (dayOfWeek % 3) {
            case 0 -> ChallengeType.OPTIMAL_DISCARD;
            case 1 -> ChallengeType.MAX_SCORE_HUNT;
            default -> ChallengeType.BEST_PEG_PLAY;
        };
    }

    /**
     * Generate an optimal discard challenge
     */
    private DailyChallenge generateOptimalDiscardChallenge(LocalDate date, Random random) {
        List<Card> fullDeck = createShuffledDeck(random);

        // Deal 6 cards to player, 1 starter card
        List<Card> sixCards = fullDeck.subList(0, 6);
        Card starter = fullDeck.get(6);

        // Find optimal discard (keep best 4)
        OptimalDiscard optimal = findOptimalDiscard(sixCards, starter);

        String challengeId = "OD-" + date.toString();

        return DailyChallenge.builder()
                .challengeId(challengeId)
                .date(date)
                .type(ChallengeType.OPTIMAL_DISCARD)
                .cards(new ArrayList<>(sixCards))
                .starterCard(starter)
                .correctAnswer(serializeIntList(optimal.discardIndices))
                .maxPoints(100)
                .hint(generateOptimalDiscardHint(optimal))
                .difficulty(calculateDifficulty(optimal.optimalScore, 12))
                .build();
    }

    /**
     * Generate a max score hunt challenge
     */
    private DailyChallenge generateMaxScoreChallenge(LocalDate date, Random random) {
        List<Card> fullDeck = createShuffledDeck(random);

        // Keep generating until we get an interesting hand (score 12-24)
        List<Card> hand = null;
        Card starter = null;
        int score = 0;
        int attempts = 0;

        while ((score < 10 || score > 24) && attempts < 100) {
            hand = fullDeck.subList(attempts * 5, attempts * 5 + 4);
            starter = fullDeck.get(attempts * 5 + 4);
            score = scorer.scoreHand(hand, starter, false);
            attempts++;
        }

        String challengeId = "MS-" + date.toString();

        return DailyChallenge.builder()
                .challengeId(challengeId)
                .date(date)
                .type(ChallengeType.MAX_SCORE_HUNT)
                .cards(new ArrayList<>(hand))
                .starterCard(starter)
                .correctAnswer(String.valueOf(score))
                .maxPoints(100)
                .hint(generateMaxScoreHint(score))
                .difficulty(calculateDifficulty(score, 20))
                .build();
    }

    /**
     * Generate a best peg play challenge
     */
    private DailyChallenge generateBestPegPlayChallenge(LocalDate date, Random random) {
        List<Card> fullDeck = createShuffledDeck(random);

        // Create a pegging scenario
        List<Card> hand = fullDeck.subList(0, 4);
        List<Card> peggedCards = new ArrayList<>();
        peggedCards.add(fullDeck.get(10)); // Opponent played a card

        int pegCount = peggedCards.stream().mapToInt(Card::getPegValue).sum();

        // Find best card to play
        int bestCardIndex = findBestPegPlay(hand, peggedCards, pegCount);

        String challengeId = "BP-" + date.toString();

        return DailyChallenge.builder()
                .challengeId(challengeId)
                .date(date)
                .type(ChallengeType.BEST_PEG_PLAY)
                .cards(new ArrayList<>(hand))
                .pegCount(pegCount)
                .peggedCards(peggedCards)
                .correctAnswer(String.valueOf(bestCardIndex))
                .maxPoints(100)
                .hint("Think about making 15, pairs, or runs!")
                .difficulty(3)
                .build();
    }

    /**
     * Find optimal discard from 6 cards
     */
    private OptimalDiscard findOptimalDiscard(List<Card> sixCards, Card starter) {
        int bestScore = -1;
        List<Integer> bestDiscard = null;

        // Try all combinations of discarding 2 cards
        for (int i = 0; i < 6; i++) {
            for (int j = i + 1; j < 6; j++) {
                List<Card> kept = new ArrayList<>();
                for (int k = 0; k < 6; k++) {
                    if (k != i && k != j) {
                        kept.add(sixCards.get(k));
                    }
                }

                int score = scorer.scoreHand(kept, starter, false);
                if (score > bestScore) {
                    bestScore = score;
                    bestDiscard = Arrays.asList(i, j);
                }
            }
        }

        return new OptimalDiscard(bestDiscard, bestScore);
    }

    /**
     * Find best card to play during pegging
     */
    private int findBestPegPlay(List<Card> hand, List<Card> peggedCards, int currentCount) {
        int bestScore = -1;
        int bestIndex = 0;

        for (int i = 0; i < hand.size(); i++) {
            Card card = hand.get(i);
            int newCount = currentCount + card.getPegValue();

            if (newCount > 31) {
                continue; // Can't play this card
            }

            int score = scorePegPlay(card, peggedCards, newCount);
            if (score > bestScore) {
                bestScore = score;
                bestIndex = i;
            }
        }

        return bestIndex;
    }

    /**
     * Score a peg play (15s, pairs, runs, 31)
     */
    private int scorePegPlay(Card card, List<Card> peggedCards, int newCount) {
        int score = 0;

        // 15 or 31
        if (newCount == 15) score += 2;
        if (newCount == 31) score += 2;

        // Pair with last card
        if (!peggedCards.isEmpty()) {
            Card lastCard = peggedCards.get(peggedCards.size() - 1);
            if (card.getRank() == lastCard.getRank()) {
                score += 2;
            }
        }

        return score;
    }

    /**
     * Create a shuffled deck
     */
    private List<Card> createShuffledDeck(Random random) {
        List<Card> deck = new ArrayList<>();
        for (Card.Suit suit : Card.Suit.values()) {
            for (Card.Rank rank : Card.Rank.values()) {
                deck.add(new Card(rank, suit));
            }
        }
        Collections.shuffle(deck, random);
        return deck;
    }

    /**
     * Calculate difficulty (1-5 stars)
     */
    private int calculateDifficulty(int score, int medianScore) {
        if (score < medianScore - 6) return 1;
        if (score < medianScore - 3) return 2;
        if (score <= medianScore + 3) return 3;
        if (score <= medianScore + 6) return 4;
        return 5;
    }

    /**
     * Generate hint for optimal discard
     */
    private String generateOptimalDiscardHint(OptimalDiscard optimal) {
        if (optimal.optimalScore >= 20) {
            return "Look for high-scoring combinations like runs and fifteens!";
        } else if (optimal.optimalScore >= 12) {
            return "Balance keeping runs and pairs with fifteens.";
        } else {
            return "This is a tough hand - minimize your losses!";
        }
    }

    /**
     * Generate hint for max score
     */
    private String generateMaxScoreHint(int score) {
        if (score >= 20) {
            return "This is a monster hand! Look for multiple scoring categories.";
        } else if (score >= 12) {
            return "Count carefully - fifteens, pairs, and runs!";
        } else {
            return "Every point counts. Check all combinations.";
        }
    }

    /**
     * Serialize list of integers to JSON
     */
    private String serializeIntList(List<Integer> list) {
        try {
            return objectMapper.writeValueAsString(list);
        } catch (JsonProcessingException e) {
            throw new RuntimeException("Failed to serialize list", e);
        }
    }

    /**
     * Helper class for optimal discard result
     */
    private static class OptimalDiscard {
        List<Integer> discardIndices;
        int optimalScore;

        OptimalDiscard(List<Integer> discardIndices, int optimalScore) {
            this.discardIndices = discardIndices;
            this.optimalScore = optimalScore;
        }
    }
}
