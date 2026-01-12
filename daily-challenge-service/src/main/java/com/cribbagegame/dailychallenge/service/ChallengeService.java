package com.cribbagegame.dailychallenge.service;

import com.cribbagegame.dailychallenge.dto.ChallengeSubmissionRequest;
import com.cribbagegame.dailychallenge.dto.ChallengeSubmissionResponse;
import com.cribbagegame.dailychallenge.dto.LeaderboardEntry;
import com.cribbagegame.dailychallenge.entity.ChallengeSubmission;
import com.cribbagegame.dailychallenge.entity.UserChallengeStats;
import com.cribbagegame.dailychallenge.model.Card;
import com.cribbagegame.dailychallenge.model.ChallengeType;
import com.cribbagegame.dailychallenge.model.DailyChallenge;
import com.cribbagegame.dailychallenge.repository.ChallengeSubmissionRepository;
import com.cribbagegame.dailychallenge.repository.UserChallengeStatsRepository;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import java.util.stream.Collectors;

@Service
public class ChallengeService {

    private final ChallengeGenerator generator;
    private final CribbageScorer scorer;
    private final ChallengeSubmissionRepository submissionRepository;
    private final UserChallengeStatsRepository statsRepository;
    private final ObjectMapper objectMapper;

    public ChallengeService(ChallengeGenerator generator,
                          CribbageScorer scorer,
                          ChallengeSubmissionRepository submissionRepository,
                          UserChallengeStatsRepository statsRepository) {
        this.generator = generator;
        this.scorer = scorer;
        this.submissionRepository = submissionRepository;
        this.statsRepository = statsRepository;
        this.objectMapper = new ObjectMapper();
    }

    /**
     * Get today's challenge
     */
    public DailyChallenge getTodaysChallenge() {
        return generator.generateChallenge(LocalDate.now(ZoneOffset.UTC));
    }

    /**
     * Get challenge for specific date
     */
    public DailyChallenge getChallenge(LocalDate date) {
        return generator.generateChallenge(date);
    }

    /**
     * Submit a solution to today's challenge
     */
    @Transactional
    public ChallengeSubmissionResponse submitSolution(Long userId, ChallengeSubmissionRequest request) {
        DailyChallenge challenge = getTodaysChallenge();

        // Check if user already submitted
        Optional<ChallengeSubmission> existing = submissionRepository
                .findByUserIdAndChallengeId(userId, challenge.getChallengeId());
        if (existing.isPresent()) {
            throw new IllegalStateException("Already submitted solution for today's challenge");
        }

        // Evaluate answer
        boolean isCorrect = evaluateAnswer(challenge, request.getAnswer());
        int pointsEarned = calculatePoints(isCorrect, request.getTimeTakenSeconds(), challenge.getMaxPoints());

        // Save submission
        ChallengeSubmission submission = ChallengeSubmission.builder()
                .userId(userId)
                .challengeId(challenge.getChallengeId())
                .answer(request.getAnswer())
                .pointsEarned(pointsEarned)
                .correct(isCorrect)
                .timeTakenSeconds(request.getTimeTakenSeconds())
                .submittedAt(Instant.now())
                .build();
        submissionRepository.save(submission);

        // Update user stats
        updateUserStats(userId, isCorrect, pointsEarned);

        // Get score breakdown if correct
        String explanation = isCorrect ? getExplanation(challenge) : null;

        return ChallengeSubmissionResponse.builder()
                .correct(isCorrect)
                .pointsEarned(pointsEarned)
                .correctAnswer(isCorrect ? null : challenge.getCorrectAnswer())
                .explanation(explanation)
                .build();
    }

    /**
     * Get leaderboard for today's challenge
     */
    public List<LeaderboardEntry> getTodaysLeaderboard() {
        DailyChallenge challenge = getTodaysChallenge();
        List<ChallengeSubmission> submissions = submissionRepository
                .findTopSolversByChallengeId(challenge.getChallengeId());

        return submissions.stream()
                .limit(100)
                .map(s -> LeaderboardEntry.builder()
                        .userId(s.getUserId())
                        .pointsEarned(s.getPointsEarned())
                        .timeTakenSeconds(s.getTimeTakenSeconds())
                        .submittedAt(s.getSubmittedAt())
                        .build())
                .collect(Collectors.toList());
    }

    /**
     * Get global leaderboard (all-time)
     */
    public List<LeaderboardEntry> getGlobalLeaderboard() {
        List<UserChallengeStats> topStats = statsRepository.findTop10ByOrderByTotalPointsDesc();

        return topStats.stream()
                .map(s -> LeaderboardEntry.builder()
                        .userId(s.getUserId())
                        .pointsEarned(s.getTotalPoints())
                        .challengesCompleted(s.getTotalChallengesCompleted())
                        .currentStreak(s.getCurrentStreak())
                        .longestStreak(s.getLongestStreak())
                        .build())
                .collect(Collectors.toList());
    }

    /**
     * Get user's challenge statistics
     */
    public UserChallengeStats getUserStats(Long userId) {
        return statsRepository.findByUserId(userId)
                .orElse(UserChallengeStats.builder()
                        .userId(userId)
                        .totalChallengesCompleted(0)
                        .totalChallengesCorrect(0)
                        .currentStreak(0)
                        .longestStreak(0)
                        .totalPoints(0)
                        .createdAt(Instant.now())
                        .updatedAt(Instant.now())
                        .build());
    }

    /**
     * Evaluate if the answer is correct
     */
    private boolean evaluateAnswer(DailyChallenge challenge, String answer) {
        try {
            if (challenge.getType() == ChallengeType.OPTIMAL_DISCARD) {
                List<Integer> userAnswer = objectMapper.readValue(answer, new TypeReference<>() {});
                List<Integer> correctAnswer = objectMapper.readValue(
                        challenge.getCorrectAnswer(), new TypeReference<>() {});
                return userAnswer.equals(correctAnswer);
            } else if (challenge.getType() == ChallengeType.MAX_SCORE_HUNT) {
                int userScore = Integer.parseInt(answer);
                int correctScore = Integer.parseInt(challenge.getCorrectAnswer());
                return userScore == correctScore;
            } else if (challenge.getType() == ChallengeType.BEST_PEG_PLAY) {
                int userIndex = Integer.parseInt(answer);
                int correctIndex = Integer.parseInt(challenge.getCorrectAnswer());
                return userIndex == correctIndex;
            }
        } catch (JsonProcessingException | NumberFormatException e) {
            return false;
        }
        return false;
    }

    /**
     * Calculate points earned based on correctness and speed
     */
    private int calculatePoints(boolean isCorrect, Integer timeTaken, int maxPoints) {
        if (!isCorrect) {
            return 0;
        }

        // Base points for correct answer
        int points = maxPoints;

        // Bonus for speed (up to 50% extra)
        if (timeTaken != null && timeTaken > 0) {
            if (timeTaken < 30) {
                points += (int) (maxPoints * 0.5); // 50% bonus
            } else if (timeTaken < 60) {
                points += (int) (maxPoints * 0.3); // 30% bonus
            } else if (timeTaken < 120) {
                points += (int) (maxPoints * 0.1); // 10% bonus
            }
        }

        return points;
    }

    /**
     * Update user statistics after submission
     */
    private void updateUserStats(Long userId, boolean isCorrect, int pointsEarned) {
        UserChallengeStats stats = statsRepository.findByUserId(userId)
                .orElse(UserChallengeStats.builder()
                        .userId(userId)
                        .totalChallengesCompleted(0)
                        .totalChallengesCorrect(0)
                        .currentStreak(0)
                        .longestStreak(0)
                        .totalPoints(0)
                        .createdAt(Instant.now())
                        .updatedAt(Instant.now())
                        .build());

        stats.setTotalChallengesCompleted(stats.getTotalChallengesCompleted() + 1);
        stats.setTotalPoints(stats.getTotalPoints() + pointsEarned);

        if (isCorrect) {
            stats.setTotalChallengesCorrect(stats.getTotalChallengesCorrect() + 1);

            // Update streak
            Instant lastCompleted = stats.getLastCompletedDate();
            LocalDate today = LocalDate.now(ZoneOffset.UTC);

            if (lastCompleted == null) {
                stats.setCurrentStreak(1);
            } else {
                LocalDate lastDate = LocalDate.ofInstant(lastCompleted, ZoneOffset.UTC);
                long daysBetween = Duration.between(
                        lastDate.atStartOfDay().toInstant(ZoneOffset.UTC),
                        today.atStartOfDay().toInstant(ZoneOffset.UTC)
                ).toDays();

                if (daysBetween == 1) {
                    // Consecutive day
                    stats.setCurrentStreak(stats.getCurrentStreak() + 1);
                } else if (daysBetween > 1) {
                    // Streak broken
                    stats.setCurrentStreak(1);
                }
            }

            stats.setLastCompletedDate(Instant.now());

            // Update longest streak
            if (stats.getCurrentStreak() > stats.getLongestStreak()) {
                stats.setLongestStreak(stats.getCurrentStreak());
            }
        }

        statsRepository.save(stats);
    }

    /**
     * Generate explanation for the correct answer
     */
    private String getExplanation(DailyChallenge challenge) {
        if (challenge.getType() == ChallengeType.MAX_SCORE_HUNT) {
            var breakdown = scorer.getScoreBreakdown(
                    challenge.getCards(),
                    challenge.getStarterCard(),
                    false
            );
            return String.format("Score breakdown: Fifteens: %d, Pairs: %d, Runs: %d, Flush: %d, Nobs: %d = Total: %d",
                    breakdown.get("fifteens"),
                    breakdown.get("pairs"),
                    breakdown.get("runs"),
                    breakdown.get("flush"),
                    breakdown.get("nobs"),
                    breakdown.get("total"));
        }
        return "Great job!";
    }
}
