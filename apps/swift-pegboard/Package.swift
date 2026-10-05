// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "Pegboard",
    platforms: [.macOS(.v13), .iOS(.v16)],
    products: [
        .executable(name: "pegboard", targets: ["Pegboard"]),
    ],
    targets: [
        .executableTarget(name: "Pegboard", path: "Sources/Pegboard"),
    ]
)
