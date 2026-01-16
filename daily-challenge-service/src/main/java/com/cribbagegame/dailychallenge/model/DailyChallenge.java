package com.cribbagegame.dailychallenge.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.LocalDate;
import java.util.List;

/**
 * Represents a daily challenge puzzle
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class DailyChallenge {
    private String challengeId;
    private LocalDate date;
    private ChallengeType type;
    private List<Card> cards;
    private Card starterCard; // For MAX_SCORE_HUNT
    private Integer pegCount;  // For BEST_PEG_PLAY
    private List<Card> peggedCards; // For BEST_PEG_PLAY
    private String correctAnswer; // JSON array of card indices or score
    private int maxPoints;
    private String hint;
    private int difficulty; // 1-5 stars
}
