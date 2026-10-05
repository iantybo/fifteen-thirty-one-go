#ifndef PEGGING_HPP
#define PEGGING_HPP

#include <string>
#include <vector>

#include "deck.h"

namespace cribbage {

/// Tracks one pegging sequence, up to a count of 31.
class PeggingRound {
public:
    PeggingRound();

    /// Returns true if `card` can be played without going over 31.
    bool canPlay(const card_t &card) const;

    /// Plays `card` and returns the points it scores.
    int play(const card_t &card);

    /// Starts a new sequence after a "go" or a count of 31.
    void reset();

    /// Card codes played so far in this sequence. The caller owns the result.
    std::vector<std::string> *playedCodes() const;

    int count() const { return count_; }

private:
    int pairPoints() const;
    int runPoints() const;

    int count_;
    std::vector<card_t> played_;
};

}  // namespace cribbage

#endif  // PEGGING_HPP
