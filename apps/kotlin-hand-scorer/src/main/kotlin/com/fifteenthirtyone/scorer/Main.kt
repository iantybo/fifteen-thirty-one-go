package com.fifteenthirtyone.scorer

/**
 * Scores a cribbage hand from the command line.
 *
 * Usage: hand-scorer [--crib] <card> <card> <card> <card> <starter>
 * Example: hand-scorer 5H 5D JS 10C 5S
 */
fun main(args: Array<String>) {
    val isCrib = args.contains("--crib")
    val codes = args.filter { it != "--crib" }

    val cards = codes.map { Card.parse(it) }
    val hand = cards.subList(0, 4)
    val starter = cards[4]

    val score = HandScorer.score(hand, starter, isCrib)
    println("Hand:     ${hand.joinToString(" ")}  Starter: $starter")
    println("Fifteens: ${score.fifteens}")
    println("Pairs:    ${score.pairs}")
    println("Runs:     ${score.runs}")
    println("Flush:    ${score.flush}")
    println("Nobs:     ${score.nobs}")
    println("Total:    ${score.total}")
}
