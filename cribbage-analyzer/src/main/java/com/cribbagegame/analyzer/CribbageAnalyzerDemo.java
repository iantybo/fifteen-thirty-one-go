package com.cribbagegame.analyzer;

import java.util.*;

/**
 * Interactive demo of the Cribbage Hand Analyzer.
 * Shows scoring, statistics, and fun facts about random cribbage hands.
 */
public class CribbageAnalyzerDemo {

    private static final Scanner scanner = new Scanner(System.in);
    private static final CribbageHandAnalyzer analyzer = new CribbageHandAnalyzer();

    public static void main(String[] args) {
        printWelcome();

        while (true) {
            printMenu();
            String choice = scanner.nextLine().trim();

            switch (choice) {
                case "1":
                    analyzeRandomHand();
                    break;
                case "2":
                    analyzeCustomHand();
                    break;
                case "3":
                    showPerfectHand();
                    break;
                case "4":
                    simulateMultipleHands();
                    break;
                case "5":
                    showScoringGuide();
                    break;
                case "6":
                case "q":
                case "quit":
                case "exit":
                    System.out.println("\n👋 Thanks for using the Cribbage Analyzer! Good luck at the board!");
                    return;
                default:
                    System.out.println("❌ Invalid choice. Please try again.\n");
            }
        }
    }

    private static void printWelcome() {
        System.out.println("╔════════════════════════════════════════════════╗");
        System.out.println("║   🎴  CRIBBAGE HAND ANALYZER  🎴               ║");
        System.out.println("║   Analyze hands, see stats, and have fun!     ║");
        System.out.println("╚════════════════════════════════════════════════╝");
        System.out.println();
    }

    private static void printMenu() {
        System.out.println("Choose an option:");
        System.out.println("  1. 🎲 Analyze a random hand");
        System.out.println("  2. ✍️  Analyze a custom hand");
        System.out.println("  3. 🏆 Show the perfect 29-point hand");
        System.out.println("  4. 📊 Simulate multiple hands");
        System.out.println("  5. 📖 Scoring guide");
        System.out.println("  6. 🚪 Quit");
        System.out.print("\nYour choice: ");
    }

    private static void analyzeRandomHand() {
        System.out.println("\n" + "=".repeat(50));
        System.out.println("🎲 RANDOM HAND ANALYSIS");
        System.out.println("=".repeat(50));

        Deck deck = new Deck();
        deck.shuffle();

        List<Card> hand = deck.drawMultiple(4);
        Card starter = deck.draw();

        analyzeAndDisplayHand(hand, starter, false);

        System.out.println("\n" + "=".repeat(50) + "\n");
        waitForEnter();
    }

    private static void analyzeCustomHand() {
        System.out.println("\n" + "=".repeat(50));
        System.out.println("✍️  CUSTOM HAND ANALYSIS");
        System.out.println("=".repeat(50));
        System.out.println("Enter cards in format: rank+suit (e.g., 5H, JC, QD, AS)");
        System.out.println("Ranks: A,2,3,4,5,6,7,8,9,10,J,Q,K");
        System.out.println("Suits: C(♣), D(♦), H(♥), S(♠)");
        System.out.println();

        try {
            List<Card> hand = new ArrayList<>();

            System.out.println("Enter 4 cards for your hand:");
            for (int i = 1; i <= 4; i++) {
                System.out.print("  Card " + i + ": ");
                String input = scanner.nextLine().trim().toUpperCase();
                hand.add(parseCard(input));
            }

            System.out.print("Enter starter card: ");
            String starterInput = scanner.nextLine().trim().toUpperCase();
            Card starter = parseCard(starterInput);

            System.out.println();
            analyzeAndDisplayHand(hand, starter, false);

        } catch (IllegalArgumentException e) {
            System.out.println("❌ Error: " + e.getMessage());
        }

        System.out.println("\n" + "=".repeat(50) + "\n");
        waitForEnter();
    }

    private static void showPerfectHand() {
        System.out.println("\n" + "=".repeat(50));
        System.out.println("🏆 THE PERFECT 29-POINT HAND");
        System.out.println("=".repeat(50));

        // The perfect hand: J-5-5-5 with starter being the 5 of Jack's suit
        List<Card> hand = Arrays.asList(
                new Card(Card.Rank.JACK, Card.Suit.HEARTS),
                new Card(Card.Rank.FIVE, Card.Suit.CLUBS),
                new Card(Card.Rank.FIVE, Card.Suit.DIAMONDS),
                new Card(Card.Rank.FIVE, Card.Suit.SPADES)
        );
        Card starter = new Card(Card.Rank.FIVE, Card.Suit.HEARTS);

        analyzeAndDisplayHand(hand, starter, false);

        System.out.println("\n💎 This legendary hand has:");
        System.out.println("   • 16 points from EIGHT different fifteen combinations");
        System.out.println("   • 12 points from four-of-a-kind fives (6 pairs × 2)");
        System.out.println("   • 1 point for nobs (Jack of starter suit)");
        System.out.println("   • Probability: 1 in 216,580 hands!");

        System.out.println("\n" + "=".repeat(50) + "\n");
        waitForEnter();
    }

    private static void simulateMultipleHands() {
        System.out.println("\n" + "=".repeat(50));
        System.out.println("📊 HAND SIMULATION");
        System.out.println("=".repeat(50));

        System.out.print("How many hands to simulate? (10-1000): ");
        int count;
        try {
            count = Integer.parseInt(scanner.nextLine().trim());
            if (count < 10 || count > 1000) {
                System.out.println("❌ Please enter a number between 10 and 1000.");
                return;
            }
        } catch (NumberFormatException e) {
            System.out.println("❌ Invalid number.");
            return;
        }

        System.out.println("\n🎲 Simulating " + count + " hands...\n");

        Map<Integer, Integer> scoreDistribution = new TreeMap<>();
        int totalScore = 0;
        int maxScore = 0;
        int minScore = Integer.MAX_VALUE;
        int zeroCount = 0;

        for (int i = 0; i < count; i++) {
            Deck deck = new Deck();
            deck.shuffle();
            List<Card> hand = deck.drawMultiple(4);
            Card starter = deck.draw();

            HandScore score = analyzer.scoreHand(hand, starter, false);
            int points = score.getTotalScore();

            scoreDistribution.merge(points, 1, Integer::sum);
            totalScore += points;
            maxScore = Math.max(maxScore, points);
            minScore = Math.min(minScore, points);
            if (points == 0) zeroCount++;
        }

        double avgScore = (double) totalScore / count;

        System.out.println("📈 RESULTS:");
        System.out.println("   Average score: " + String.format("%.2f", avgScore) + " points");
        System.out.println("   Highest score: " + maxScore + " points");
        System.out.println("   Lowest score:  " + minScore + " points");
        System.out.println("   Zero-point hands: " + zeroCount + " (" +
                          String.format("%.1f%%", 100.0 * zeroCount / count) + ")");

        System.out.println("\n📊 Score Distribution:");
        for (Map.Entry<Integer, Integer> entry : scoreDistribution.entrySet()) {
            int score = entry.getKey();
            int frequency = entry.getValue();
            double percentage = 100.0 * frequency / count;

            String bar = "█".repeat(Math.max(1, (int) (percentage * 2)));
            System.out.printf("   %2d pts: %s %3d hands (%.1f%%)\n",
                            score, bar, frequency, percentage);
        }

        System.out.println("\n" + "=".repeat(50) + "\n");
        waitForEnter();
    }

    private static void showScoringGuide() {
        System.out.println("\n" + "=".repeat(50));
        System.out.println("📖 CRIBBAGE SCORING GUIDE");
        System.out.println("=".repeat(50));

        System.out.println("\n🎯 FIFTEENS (2 points each)");
        System.out.println("   Any combination of cards that sum to 15");
        System.out.println("   Example: 7+8, 5+10, 5+5+5");

        System.out.println("\n👥 PAIRS (2 points each pair)");
        System.out.println("   Two cards of same rank = 2 pts");
        System.out.println("   Three of a kind = 6 pts (3 pairs)");
        System.out.println("   Four of a kind = 12 pts (6 pairs)");

        System.out.println("\n🏃 RUNS (1 point per card)");
        System.out.println("   Three or more cards in sequence");
        System.out.println("   Example: 3-4-5 = 3 pts, A-2-3-4 = 4 pts");
        System.out.println("   Double runs count twice (e.g., 3-3-4-5)");

        System.out.println("\n♠️ FLUSH");
        System.out.println("   4 cards in hand same suit = 4 pts");
        System.out.println("   5 cards including starter = 5 pts");
        System.out.println("   (Crib requires all 5 to match)");

        System.out.println("\n🎴 NOBS (1 point)");
        System.out.println("   Jack in hand matching starter card's suit");

        System.out.println("\n💡 TIPS:");
        System.out.println("   • Fives are the most valuable cards");
        System.out.println("   • Average hand scores about 8 points");
        System.out.println("   • Perfect hand (29 pts) needs J-5-5-5");
        System.out.println("   • Scores of 25, 26, 27, 28 are impossible");

        System.out.println("\n" + "=".repeat(50) + "\n");
        waitForEnter();
    }

    private static void analyzeAndDisplayHand(List<Card> hand, Card starter, boolean isCrib) {
        // Display the hand
        System.out.println("\n🎴 Your Hand:");
        System.out.print("   ");
        for (Card card : hand) {
            System.out.print(card + "  ");
        }
        System.out.println("\n\n⭐ Starter: " + starter);

        // Score the hand
        HandScore score = analyzer.scoreHand(hand, starter, isCrib);

        System.out.println("\n" + "─".repeat(50));
        System.out.println("📊 SCORE BREAKDOWN:");
        System.out.println("─".repeat(50));

        if (score.getComponents().isEmpty()) {
            System.out.println("   No scoring combinations found.");
        } else {
            for (HandScore.ScoreComponent component : score.getComponents()) {
                System.out.println("   " + component);
            }
        }

        System.out.println("─".repeat(50));
        System.out.println("🎯 TOTAL SCORE: " + score.getTotalScore() + " points");
        System.out.println("─".repeat(50));

        // Get statistics
        HandStatistics.Statistics stats = HandStatistics.analyze(hand, starter, score);

        System.out.println("\n" + stats.getHandQuality());
        System.out.println("\n📈 " + stats.getProbabilityInfo());

        List<String> patterns = stats.getInterestingPatterns();
        if (!patterns.isEmpty()) {
            System.out.println("\n✨ Interesting patterns:");
            for (String pattern : patterns) {
                System.out.println("   " + pattern);
            }
        }

        System.out.println("\n💡 Fun fact: " + stats.getFunFact());
        System.out.println("\n🤔 What if? " + stats.getWhatIfSuggestion());
    }

    private static Card parseCard(String input) {
        if (input.length() < 2) {
            throw new IllegalArgumentException("Invalid card format: " + input);
        }

        // Extract rank and suit
        String rankStr = input.substring(0, input.length() - 1);
        char suitChar = input.charAt(input.length() - 1);

        Card.Rank rank = parseRank(rankStr);
        Card.Suit suit = parseSuit(suitChar);

        return new Card(rank, suit);
    }

    private static Card.Rank parseRank(String rankStr) {
        switch (rankStr) {
            case "A": return Card.Rank.ACE;
            case "2": return Card.Rank.TWO;
            case "3": return Card.Rank.THREE;
            case "4": return Card.Rank.FOUR;
            case "5": return Card.Rank.FIVE;
            case "6": return Card.Rank.SIX;
            case "7": return Card.Rank.SEVEN;
            case "8": return Card.Rank.EIGHT;
            case "9": return Card.Rank.NINE;
            case "10": return Card.Rank.TEN;
            case "J": return Card.Rank.JACK;
            case "Q": return Card.Rank.QUEEN;
            case "K": return Card.Rank.KING;
            default: throw new IllegalArgumentException("Invalid rank: " + rankStr);
        }
    }

    private static Card.Suit parseSuit(char suitChar) {
        switch (suitChar) {
            case 'C': return Card.Suit.CLUBS;
            case 'D': return Card.Suit.DIAMONDS;
            case 'H': return Card.Suit.HEARTS;
            case 'S': return Card.Suit.SPADES;
            default: throw new IllegalArgumentException("Invalid suit: " + suitChar);
        }
    }

    private static void waitForEnter() {
        System.out.print("Press Enter to continue...");
        scanner.nextLine();
    }
}
