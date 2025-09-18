#!/bin/sh

# run-migrations.sh
# Flash Gateway database migration runner
# Runs schema.sql if migrations haven't been applied yet

set -e  # Exit on any error

# Configuration
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-gateway}"
DB_USER="${DB_USER:-gateway}"
DB_PASSWORD="${DB_PASSWORD:-gateway_pass}"

# Build connection string
if [ -n "$DATABASE_URL" ]; then
    # Use DATABASE_URL if provided
    PSQL_CMD="psql $DATABASE_URL"
else
    # Use individual parameters
    export PGPASSWORD="$DB_PASSWORD"
    PSQL_CMD="psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"
fi

echo "🔄 Checking database connection..."

# Test database connectivity with timeout
timeout 30 sh -c "until $PSQL_CMD -c 'SELECT 1;' > /dev/null 2>&1; do
    echo '⏳ Waiting for database...'
    sleep 2
done"

if [ $? -ne 0 ]; then
    echo "❌ Failed to connect to database after 30 seconds"
    exit 1
fi

echo "✅ Database connection established"

# Check if migrations have already been run
echo "🔍 Checking migration status..."

# Check if request_logs table exists (primary indicator)
TABLE_EXISTS=$($PSQL_CMD -t -c "SELECT EXISTS (
    SELECT FROM information_schema.tables
    WHERE table_schema = 'public'
    AND table_name = 'request_logs'
);" 2>/dev/null | tr -d ' ')

if [ "$TABLE_EXISTS" = "t" ]; then
    echo "✅ Database schema already exists, skipping migration"
    exit 0
fi

echo "📦 Running database migrations..."

# Run the combined schema migration
if ! $PSQL_CMD -f /root/migrations/schema.sql; then
    echo "❌ Migration failed"
    exit 1
fi

echo "✅ Database migration completed successfully"

# Verify migration by checking if tables were created
VERIFY_TABLES=$($PSQL_CMD -t -c "SELECT count(*) FROM information_schema.tables
    WHERE table_schema = 'public'
    AND table_name IN ('request_logs', 'guardrail_metrics');" 2>/dev/null | tr -d ' ')

if [ "$VERIFY_TABLES" = "2" ]; then
    echo "✅ Migration verification passed - all tables created"
else
    echo "⚠️  Warning: Expected 2 tables, found $VERIFY_TABLES"
fi

echo "🎉 Migration process completed"