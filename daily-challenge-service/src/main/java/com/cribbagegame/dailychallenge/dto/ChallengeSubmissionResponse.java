package com.cribbagegame.dailychallenge.dto;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class ChallengeSubmissionResponse {
    private boolean correct;
    private int pointsEarned;
    private String correctAnswer; // Only shown if incorrect
    private String explanation;
}
