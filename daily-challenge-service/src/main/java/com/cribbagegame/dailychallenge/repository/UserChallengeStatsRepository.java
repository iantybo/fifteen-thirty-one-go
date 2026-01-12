package com.cribbagegame.dailychallenge.repository;

import com.cribbagegame.dailychallenge.entity.UserChallengeStats;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface UserChallengeStatsRepository extends JpaRepository<UserChallengeStats, Long> {

    Optional<UserChallengeStats> findByUserId(Long userId);

    List<UserChallengeStats> findTop10ByOrderByTotalPointsDesc();

    List<UserChallengeStats> findTop10ByOrderByLongestStreakDesc();

    @Query("SELECT s FROM UserChallengeStats s ORDER BY s.totalChallengesCorrect DESC, s.totalPoints DESC")
    List<UserChallengeStats> findTopByAccuracy();
}
