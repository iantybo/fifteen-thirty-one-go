package com.cribbagegame.dailychallenge.entity;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.Instant;

/**
 * Entity tracking user statistics for daily challenges
 */
@Entity
@Table(name = "user_challenge_stats")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class UserChallengeStats {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "user_id", nullable = false, unique = true)
    private Long userId;

    @Column(name = "total_challenges_completed", nullable = false)
    private int totalChallengesCompleted;

    @Column(name = "total_challenges_correct", nullable = false)
    private int totalChallengesCorrect;

    @Column(name = "current_streak", nullable = false)
    private int currentStreak;

    @Column(name = "longest_streak", nullable = false)
    private int longestStreak;

    @Column(name = "total_points", nullable = false)
    private int totalPoints;

    @Column(name = "last_completed_date")
    private Instant lastCompletedDate;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt;

    @PrePersist
    protected void onCreate() {
        Instant now = Instant.now();
        if (createdAt == null) {
            createdAt = now;
        }
        if (updatedAt == null) {
            updatedAt = now;
        }
    }

    @PreUpdate
    protected void onUpdate() {
        updatedAt = Instant.now();
    }
}
