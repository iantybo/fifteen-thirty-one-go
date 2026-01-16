package com.cribbagegame.analyzer;

import java.util.ArrayList;
import java.util.List;

/**
 * Represents the detailed scoring breakdown of a cribbage hand.
 */
public class HandScore {
    private final List<ScoreComponent> components;
    private final int totalScore;

    public HandScore() {
        this.components = new ArrayList<>();
        this.totalScore = 0;
    }

    public HandScore(List<ScoreComponent> components) {
        this.components = new ArrayList<>(components);
        this.totalScore = components.stream().mapToInt(ScoreComponent::getPoints).sum();
    }

    public List<ScoreComponent> getComponents() {
        return new ArrayList<>(components);
    }

    public int getTotalScore() {
        return totalScore;
    }

    public int getFifteensCount() {
        return (int) components.stream()
                .filter(c -> c.getType() == ScoreType.FIFTEEN)
                .count();
    }

    public int getPairsCount() {
        return (int) components.stream()
                .filter(c -> c.getType() == ScoreType.PAIR)
                .count();
    }

    public int getRunsScore() {
        return components.stream()
                .filter(c -> c.getType() == ScoreType.RUN)
                .mapToInt(ScoreComponent::getPoints)
                .sum();
    }

    public int getFlushScore() {
        return components.stream()
                .filter(c -> c.getType() == ScoreType.FLUSH)
                .mapToInt(ScoreComponent::getPoints)
                .sum();
    }

    public int getNobsScore() {
        return components.stream()
                .filter(c -> c.getType() == ScoreType.NOBS)
                .mapToInt(ScoreComponent::getPoints)
                .sum();
    }

    @Override
    public String toString() {
        if (components.isEmpty()) {
            return "Total: 0 points";
        }

        StringBuilder sb = new StringBuilder();
        sb.append("Score Breakdown:\n");
        for (ScoreComponent component : components) {
            sb.append("  ").append(component).append("\n");
        }
        sb.append("Total: ").append(totalScore).append(" points");
        return sb.toString();
    }

    public enum ScoreType {
        FIFTEEN("Fifteen"),
        PAIR("Pair"),
        RUN("Run"),
        FLUSH("Flush"),
        NOBS("Nobs");

        private final String displayName;

        ScoreType(String displayName) {
            this.displayName = displayName;
        }

        public String getDisplayName() {
            return displayName;
        }
    }

    public static class ScoreComponent {
        private final ScoreType type;
        private final int points;
        private final String description;

        public ScoreComponent(ScoreType type, int points, String description) {
            this.type = type;
            this.points = points;
            this.description = description;
        }

        public ScoreType getType() {
            return type;
        }

        public int getPoints() {
            return points;
        }

        public String getDescription() {
            return description;
        }

        @Override
        public String toString() {
            return String.format("%s: %d points (%s)", type.getDisplayName(), points, description);
        }
    }
}
