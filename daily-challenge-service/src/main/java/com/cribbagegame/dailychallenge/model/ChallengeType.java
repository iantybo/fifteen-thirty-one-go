package com.cribbagegame.dailychallenge.model;

/**
 * Types of daily challenges available
 */
public enum ChallengeType {
    /**
     * Given 6 cards, find the optimal 2 cards to discard for maximum expected score
     */
    OPTIMAL_DISCARD("Find the best 2 cards to discard", "Choose wisely to maximize your hand score!"),

    /**
     * Given a hand and starter card, identify the maximum possible score
     */
    MAX_SCORE_HUNT("Find all scoring combinations", "How high can you score this hand?"),

    /**
     * Given game state during pegging, find the best card to play
     */
    BEST_PEG_PLAY("Choose the optimal card to play", "Make the smartest play in this pegging situation!");

    private final String title;
    private final String description;

    ChallengeType(String title, String description) {
        this.title = title;
        this.description = description;
    }

    public String getTitle() {
        return title;
    }

    public String getDescription() {
        return description;
    }
}
