package com.cribbagegame.dailychallenge.controller;

import com.cribbagegame.dailychallenge.dto.ChallengeSubmissionRequest;
import com.cribbagegame.dailychallenge.dto.ChallengeSubmissionResponse;
import com.cribbagegame.dailychallenge.dto.LeaderboardEntry;
import com.cribbagegame.dailychallenge.entity.UserChallengeStats;
import com.cribbagegame.dailychallenge.model.DailyChallenge;
import com.cribbagegame.dailychallenge.security.JwtUtil;
import com.cribbagegame.dailychallenge.service.ChallengeService;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.LocalDate;
import java.util.List;

@RestController
@RequestMapping("/api/challenges")
@CrossOrigin(origins = "${cors.allowed.origins}")
public class ChallengeController {

    private final ChallengeService challengeService;
    private final JwtUtil jwtUtil;

    public ChallengeController(ChallengeService challengeService, JwtUtil jwtUtil) {
        this.challengeService = challengeService;
        this.jwtUtil = jwtUtil;
    }

    /**
     * GET /api/challenges/today
     * Get today's challenge
     */
    @GetMapping("/today")
    public ResponseEntity<DailyChallenge> getTodaysChallenge() {
        DailyChallenge challenge = challengeService.getTodaysChallenge();
        // Remove correct answer from response
        challenge.setCorrectAnswer(null);
        return ResponseEntity.ok(challenge);
    }

    /**
     * GET /api/challenges/{date}
     * Get challenge for specific date
     */
    @GetMapping("/{date}")
    public ResponseEntity<DailyChallenge> getChallenge(@PathVariable String date) {
        LocalDate localDate = LocalDate.parse(date);
        DailyChallenge challenge = challengeService.getChallenge(localDate);
        // Remove correct answer from response
        challenge.setCorrectAnswer(null);
        return ResponseEntity.ok(challenge);
    }

    /**
     * POST /api/challenges/submit
     * Submit solution to today's challenge
     */
    @PostMapping("/submit")
    public ResponseEntity<ChallengeSubmissionResponse> submitSolution(
            @RequestHeader("Authorization") String authHeader,
            @RequestBody ChallengeSubmissionRequest request) {

        Long userId = extractUserId(authHeader);
        ChallengeSubmissionResponse response = challengeService.submitSolution(userId, request);
        return ResponseEntity.ok(response);
    }

    /**
     * GET /api/challenges/leaderboard/today
     * Get leaderboard for today's challenge
     */
    @GetMapping("/leaderboard/today")
    public ResponseEntity<List<LeaderboardEntry>> getTodaysLeaderboard() {
        List<LeaderboardEntry> leaderboard = challengeService.getTodaysLeaderboard();
        return ResponseEntity.ok(leaderboard);
    }

    /**
     * GET /api/challenges/leaderboard/global
     * Get global all-time leaderboard
     */
    @GetMapping("/leaderboard/global")
    public ResponseEntity<List<LeaderboardEntry>> getGlobalLeaderboard() {
        List<LeaderboardEntry> leaderboard = challengeService.getGlobalLeaderboard();
        return ResponseEntity.ok(leaderboard);
    }

    /**
     * GET /api/challenges/stats/me
     * Get current user's statistics
     */
    @GetMapping("/stats/me")
    public ResponseEntity<UserChallengeStats> getMyStats(
            @RequestHeader("Authorization") String authHeader) {

        Long userId = extractUserId(authHeader);
        UserChallengeStats stats = challengeService.getUserStats(userId);
        return ResponseEntity.ok(stats);
    }

    /**
     * GET /api/challenges/stats/{userId}
     * Get specific user's statistics
     */
    @GetMapping("/stats/{userId}")
    public ResponseEntity<UserChallengeStats> getUserStats(@PathVariable Long userId) {
        UserChallengeStats stats = challengeService.getUserStats(userId);
        return ResponseEntity.ok(stats);
    }

    /**
     * Extract user ID from JWT token
     */
    private Long extractUserId(String authHeader) {
        if (authHeader == null || !authHeader.startsWith("Bearer ")) {
            throw new IllegalArgumentException("Invalid authorization header");
        }
        String token = authHeader.substring(7);
        return jwtUtil.extractUserId(token);
    }
}
