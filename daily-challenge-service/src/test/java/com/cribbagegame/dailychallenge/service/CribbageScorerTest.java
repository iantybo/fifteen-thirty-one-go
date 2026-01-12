package com.cribbagegame.dailychallenge.service;

import com.cribbagegame.dailychallenge.model.Card;
import org.junit.jupiter.api.Test;

import java.util.Arrays;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class CribbageScorerTest {

    private final CribbageScorer scorer = new CribbageScorer();

    @Test
    void testPerfectHand() {
        // 5H 5C 5S JD with 5D starter = 29 points
        List<Card> hand = Arrays.asList(
                new Card(Card.Rank.FIVE, Card.Suit.HEARTS),
                new Card(Card.Rank.FIVE, Card.Suit.CLUBS),
                new Card(Card.Rank.FIVE, Card.Suit.SPADES),
                new Card(Card.Rank.JACK, Card.Suit.DIAMONDS)
        );
        Card starter = new Card(Card.Rank.FIVE, Card.Suit.DIAMONDS);

        int score = scorer.scoreHand(hand, starter, false);
        assertEquals(29, score, "Perfect 29 hand should score 29 points");
    }

    @Test
    void testFifteens() {
        List<Card> hand = Arrays.asList(
                new Card(Card.Rank.TEN, Card.Suit.HEARTS),
                new Card(Card.Rank.FIVE, Card.Suit.CLUBS),
                new Card(Card.Rank.KING, Card.Suit.SPADES),
                new Card(Card.Rank.ACE, Card.Suit.DIAMONDS)
        );
        Card starter = new Card(Card.Rank.FOUR, Card.Suit.HEARTS);

        Map<String, Integer> breakdown = scorer.getScoreBreakdown(hand, starter, false);
        assertTrue(breakdown.get("fifteens") >= 2, "Should have at least one fifteen");
    }

    @Test
    void testPairs() {
        List<Card> hand = Arrays.asList(
                new Card(Card.Rank.KING, Card.Suit.HEARTS),
                new Card(Card.Rank.KING, Card.Suit.CLUBS),
                new Card(Card.Rank.QUEEN, Card.Suit.SPADES),
                new Card(Card.Rank.QUEEN, Card.Suit.DIAMONDS)
        );
        Card starter = new Card(Card.Rank.ACE, Card.Suit.HEARTS);

        Map<String, Integer> breakdown = scorer.getScoreBreakdown(hand, starter, false);
        assertEquals(4, breakdown.get("pairs"), "Two pairs should score 4 points");
    }

    @Test
    void testRun() {
        List<Card> hand = Arrays.asList(
                new Card(Card.Rank.THREE, Card.Suit.HEARTS),
                new Card(Card.Rank.FOUR, Card.Suit.CLUBS),
                new Card(Card.Rank.FIVE, Card.Suit.SPADES),
                new Card(Card.Rank.SIX, Card.Suit.DIAMONDS)
        );
        Card starter = new Card(Card.Rank.SEVEN, Card.Suit.HEARTS);

        Map<String, Integer> breakdown = scorer.getScoreBreakdown(hand, starter, false);
        assertEquals(5, breakdown.get("runs"), "Five card run should score 5 points");
    }

    @Test
    void testFlush() {
        List<Card> hand = Arrays.asList(
                new Card(Card.Rank.ACE, Card.Suit.HEARTS),
                new Card(Card.Rank.THREE, Card.Suit.HEARTS),
                new Card(Card.Rank.SEVEN, Card.Suit.HEARTS),
                new Card(Card.Rank.NINE, Card.Suit.HEARTS)
        );
        Card starter = new Card(Card.Rank.KING, Card.Suit.CLUBS);

        Map<String, Integer> breakdown = scorer.getScoreBreakdown(hand, starter, false);
        assertEquals(4, breakdown.get("flush"), "Four card flush should score 4 points");
    }

    @Test
    void testNobs() {
        List<Card> hand = Arrays.asList(
                new Card(Card.Rank.JACK, Card.Suit.HEARTS),
                new Card(Card.Rank.THREE, Card.Suit.CLUBS),
                new Card(Card.Rank.SEVEN, Card.Suit.SPADES),
                new Card(Card.Rank.NINE, Card.Suit.DIAMONDS)
        );
        Card starter = new Card(Card.Rank.KING, Card.Suit.HEARTS);

        Map<String, Integer> breakdown = scorer.getScoreBreakdown(hand, starter, false);
        assertEquals(1, breakdown.get("nobs"), "Jack matching starter suit should score 1 point");
    }
}
