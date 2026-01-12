package com.cribbagegame.dailychallenge.dto;

import lombok.Builder;
import lombok.Data;

import java.time.Instant;

@Data
@Builder
public class LeaderboardEntry {
    private Long userId;
    private int pointsEarned;
    private Integer timeTakenSeconds;
    private Instant submittedAt;
    private Integer challengesCompleted;
    private Integer currentStreak;
    private Integer longestStreak;
}
