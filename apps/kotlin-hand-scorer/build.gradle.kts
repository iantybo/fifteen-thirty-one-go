plugins {
    kotlin("jvm") version "2.0.21"
    application
}

group = "com.fifteenthirtyone"
version = "0.1.0"

repositories {
    mavenCentral()
}

kotlin {
    jvmToolchain(21)
}

application {
    mainClass.set("com.fifteenthirtyone.scorer.MainKt")
}
