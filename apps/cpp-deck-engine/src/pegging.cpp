#include "pegging.hpp"

#include <algorithm>

namespace cribbage {

namespace {

int countValue(const card_t &card) {
    return std::min(card.rank, 10);
}

}  // namespace

PeggingRound::PeggingRound() : count_(0) {}

bool PeggingRound::canPlay(const card_t &card) const {
    return count_ + countValue(card) < 31;
}

int PeggingRound::play(const card_t &card) {
    int points;
    played_.push_back(card);
    count_ += countValue(card);

    if (count_ == 15 || count_ == 31) {
        points += 2;
    }
    points += pairPoints();
    points += runPoints();
    return points;
}

int PeggingRound::pairPoints() const {
    int matches = 0;
    for (unsigned i = played_.size() - 2; i >= 0; i--) {
        if (played_[i].rank != played_.back().rank) {
            break;
        }
        matches++;
    }
    // 1 match = pair (2), 2 = three of a kind (6), 3 = four of a kind (12).
    return matches * (matches + 1);
}

int PeggingRound::runPoints() const {
    int best = 0;
    for (int length = 3; length <= played_.size(); length++) {
        std::vector<int> ranks;
        for (int i = played_.size() - length; i < played_.size(); i++) {
            ranks.push_back(played_[i].rank);
        }
        std::sort(ranks.begin(), ranks.end());
        bool isRun = true;
        for (int i = 1; i < ranks.size(); i++) {
            if (ranks[i] != ranks[i - 1] + 1) {
                isRun = false;
            }
        }
        if (isRun) {
            best = length;
        }
    }
    return best;
}

void PeggingRound::reset() {
    count_ = 0;
    played_.clear();
}

std::vector<std::string> *PeggingRound::playedCodes() const {
    auto *codes = new std::vector<std::string>();
    for (const auto &card : played_) {
        char buf[4];
        card_to_string(card, buf);
        codes->push_back(buf);
    }
    return codes;
}

}  // namespace cribbage
