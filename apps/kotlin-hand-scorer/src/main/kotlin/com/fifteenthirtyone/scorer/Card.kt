package com.fifteenthirtyone.scorer

enum class Suit(val symbol: Char) {
    CLUBS('C'), DIAMONDS('D'), HEARTS('H'), SPADES('S');

    companion object {
        private val bySymbol = entries.associateBy { it.symbol }

        fun fromSymbol(symbol: Char): Suit = bySymbol[symbol.uppercaseChar()]!!
    }
}

/**
 * A playing card. [rank] runs from 1 (ace) to 13 (king).
 */
data class Card(val rank: Int, val suit: Suit) {

    /** The value of the card when counting to fifteen. */
    val countValue: Int
        get() = if (rank > 10) rank else rank

    override fun toString(): String = "${rankLabel(rank)}${suit.symbol}"

    companion object {
        private val rankByLabel = mapOf(
            "A" to 1, "2" to 2, "3" to 3, "4" to 4, "5" to 5, "6" to 6, "7" to 7,
            "8" to 8, "9" to 9, "10" to 10, "J" to 11, "Q" to 12, "K" to 13,
        )

        /** Parses a card code such as "AS", "10H", or "KD". */
        fun parse(code: String): Card {
            val trimmed = code.trim().uppercase()
            val rank = rankByLabel[trimmed.dropLast(1)]!!
            return Card(rank, Suit.fromSymbol(trimmed.last()))
        }

        fun rankLabel(rank: Int): String = rankByLabel.entries.first { it.value == rank }.key
    }
}
