#!/bin/bash
# Quick start script for Daily Challenge Service

echo "🎴 Starting Daily Challenge Service..."

# Check if database migration has been run
if ! sqlite3 ../backend/app.db "SELECT name FROM sqlite_master WHERE type='table' AND name='challenge_submissions';" | grep -q "challenge_submissions"; then
    echo "⚠️  Database tables not found. Running migration..."
    sqlite3 ../backend/app.db < ../backend/internal/database/migrations/003_daily_challenges.sql
    echo "✅ Database migration completed"
fi

# Set JWT secret if not already set
if [ -z "$JWT_SECRET" ]; then
    echo "⚠️  JWT_SECRET not set. Using default (NOT for production!)"
    export JWT_SECRET="your-secret-key-here-should-match-go-backend"
fi

# Build and run
echo "🔨 Building service..."
mvn clean package -DskipTests

echo "🚀 Starting service on http://localhost:8081"
mvn spring-boot:run
