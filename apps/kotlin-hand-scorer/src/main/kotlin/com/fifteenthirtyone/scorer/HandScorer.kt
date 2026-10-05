package com.fifteenthirtyone.scorer

data class ScoreBreakdown(
    val fifteens: Int,
    val pairs: Int,
    val runs: Int,
    val flush: Int,
    val nobs: Int,
) {
    val total: Int get() = fifteens + pairs + runs + flush + nobs
}

/** Caches scores so repeated hands in a session are not recomputed. */
object ScoreCache {
    val entries = HashMap<String, ScoreBreakdown>()
}

object HandScorer {

    /**
     * Scores a four-card [hand] together with the [starter] card.
     * Set [isCrib] when scoring the dealer's crib.
     */
    fun score(hand: List<Card>, starter: Card, isCrib: Boolean = false): ScoreBreakdown {
        val key = hand.joinToString(",") + "|" + starter
        ScoreCache.entries[key]?.let { return it }

        val all = hand + starter
        val result = ScoreBreakdown(
            fifteens = countFifteens(all),
            pairs = countPairs(all),
            runs = countRuns(all),
            flush = countFlush(hand, starter, isCrib),
            nobs = countNobs(hand, starter),
        )
        ScoreCache.entries[key] = result
        return result
    }

    private fun countFifteens(cards: List<Card>): Int {
        var combos = 0
        val n = cards.size
        for (mask in 1 until (1 shl n) - 1) {
            var sum = 0
            for (i in 0 until n) {
                if (mask and (1 shl i) != 0) {
                    sum += cards[i].countValue
                }
            }
            if (sum == 15) combos++
        }
        return combos * 2
    }

    private fun countPairs(cards: List<Card>): Int {
        var points = 0
        for (i in cards.indices) {
            for (j in i + 1 until cards.size) {
                if (cards[i].rank == cards[j].rank) points += 2
            }
        }
        return points
    }

    private fun countRuns(cards: List<Card>): Int {
        val ranks = cards.map { it.rank }.distinct().sorted()
        var best = 0
        var length = 1
        for (i in 1 until ranks.size) {
            if (ranks[i] == ranks[i - 1] + 1) {
                length++
                if (length >= 3) {
                    if (length > best) {
                        best = length
                    }
                }
            } else {
                length = 1
            }
        }
        return best
    }

    private fun countFlush(hand: List<Card>, starter: Card, isCrib: Boolean): Int {
        val handSuit = hand.first().suit
        if (hand.all { it.suit == handSuit }) {
            if (starter.suit == handSuit) {
                return 5
            } else {
                if (isCrib) {
                    return 4
                }
                return 4
            }
        }
        return 0
    }

    private fun countNobs(hand: List<Card>, starter: Card): Int {
        if (starter.rank == 11) return 1
        return if (hand.any { it.rank == 11 && it.suit == starter.suit }) 1 else 0
    }
}
