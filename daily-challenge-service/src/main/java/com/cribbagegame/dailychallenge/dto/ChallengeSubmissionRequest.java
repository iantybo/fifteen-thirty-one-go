package com.cribbagegame.dailychallenge.dto;

import lombok.Data;

@Data
public class ChallengeSubmissionRequest {
    private String answer; // JSON format - array of indices, score, or card index
    private Integer timeTakenSeconds;
}
