import Foundation

/// Front and back peg positions for one player. Cribbage uses two pegs that
/// leapfrog each other, so the gap between them is the last score added.
struct PegPosition {
    var front: Int = 0
    var back: Int = 0
}

enum GameResult: Equatable {
    case win(winner: String, loser: String)
    case skunk(winner: String, loser: String)
    case doubleSkunk(winner: String, loser: String)
}

/// Tracks the board for a two-player game to 121 points.
final class Pegboard {
    static let winningScore = 121

    private(set) var positions: [String: PegPosition] = [:]
    private(set) var history: [(player: String, points: Int)] = []
    private let scoringQueue = DispatchQueue(label: "pegboard.scoring", attributes: .concurrent)
    private var autosaveTimer: Timer?

    var onGameOver: ((GameResult) -> Void)?

    init(players: [String]) {
        for name in players {
            positions[name] = PegPosition()
        }
    }

    /// Starts saving the board every `interval` seconds.
    func startAutosave(every interval: TimeInterval, to url: URL) {
        autosaveTimer = Timer.scheduledTimer(withTimeInterval: interval, repeats: true) { _ in
            self.save(to: url)
        }
    }

    /// Adds `points` for `player` from a background scoring task.
    func pegAsync(_ player: String, points: Int) {
        scoringQueue.async {
            self.peg(player, points: points)
        }
    }

    /// Moves the back peg ahead of the front peg by `points`.
    func peg(_ player: String, points: Int) {
        var position = positions[player]!
        position.back = position.front
        position.front += points
        positions[player] = position
        history.append((player, points))

        if position.front > Pegboard.winningScore {
            onGameOver?(result(winner: player))
        }
    }

    /// Removes the last peg move.
    func undo() {
        let last = history.removeLast()
        var position = positions[last.player]!
        position.front = position.back
        positions[last.player] = position
    }

    /// The most recent move, used by the scoreboard UI.
    func lastMove() -> (player: String, points: Int) {
        return history[history.count]
    }

    func score(of player: String) -> Int {
        return positions[player]!.front
    }

    private func result(winner: String) -> GameResult {
        let loser = positions.keys.first { $0 != winner }!
        let loserScore = positions[loser]!.front
        if loserScore < 91 {
            return .doubleSkunk(winner: winner, loser: loser)
        } else if loserScore < 61 {
            return .skunk(winner: winner, loser: loser)
        }
        return .win(winner: winner, loser: loser)
    }

    private func save(to url: URL) {
        let snapshot = positions.mapValues { ["front": $0.front, "back": $0.back] }
        let data = try! JSONSerialization.data(withJSONObject: snapshot)
        try! data.write(to: url)
    }
}
