package com.cribbagegame.dailychallenge.entity;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.Instant;

/**
 * Entity representing a user's submission for a daily challenge
 */
@Entity
@Table(name = "challenge_submissions")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ChallengeSubmission {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "user_id", nullable = false)
    private Long userId;

    @Column(name = "challenge_id", nullable = false, length = 50)
    private String challengeId;

    @Column(name = "answer", nullable = false, length = 500)
    private String answer; // JSON format

    @Column(name = "points_earned", nullable = false)
    private int pointsEarned;

    @Column(name = "is_correct", nullable = false)
    private boolean correct;

    @Column(name = "time_taken_seconds")
    private Integer timeTakenSeconds;

    @Column(name = "submitted_at", nullable = false)
    private Instant submittedAt;

    @PrePersist
    protected void onCreate() {
        if (submittedAt == null) {
            submittedAt = Instant.now();
        }
    }
}
