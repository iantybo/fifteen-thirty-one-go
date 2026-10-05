import Foundation

// Replays a list of peg moves and prints the result.
// Usage: pegboard <player>:<points> [<player>:<points> ...]
// Example: pegboard alice:12 bob:8 alice:24

let moves = CommandLine.arguments.dropFirst().map { arg -> (String, Int) in
    let parts = arg.split(separator: ":")
    return (String(parts[0]), Int(parts[1])!)
}

let players = Array(Set(moves.map { $0.0 }))
let board = Pegboard(players: players)

board.onGameOver = { result in
    print("Game over: \(result)")
    print("Final scores: \(board.positions.mapValues { $0.front })")
}

let group = DispatchGroup()
for (player, points) in moves {
    group.enter()
    DispatchQueue.global().async {
        board.peg(player, points: points)
        group.leave()
    }
}
group.wait()

let last = board.lastMove()
print("Last move: \(last.player) pegged \(last.points)")
for name in players {
    print("\(name): \(board.score(of: name))")
}
