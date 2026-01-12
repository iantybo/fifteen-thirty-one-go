package com.cribbagegame.dailychallenge.repository;

import com.cribbagegame.dailychallenge.entity.ChallengeSubmission;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface ChallengeSubmissionRepository extends JpaRepository<ChallengeSubmission, Long> {

    Optional<ChallengeSubmission> findByUserIdAndChallengeId(Long userId, String challengeId);

    List<ChallengeSubmission> findByUserIdOrderBySubmittedAtDesc(Long userId);

    List<ChallengeSubmission> findByChallengeIdOrderByPointsEarnedDescTimeTakenSecondsAsc(String challengeId);

    @Query("SELECT s FROM ChallengeSubmission s WHERE s.challengeId = ?1 AND s.correct = true " +
           "ORDER BY s.timeTakenSeconds ASC, s.submittedAt ASC")
    List<ChallengeSubmission> findTopSolversByChallengeId(String challengeId);
}
