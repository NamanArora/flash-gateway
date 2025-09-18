#!/bin/sh
set -e

echo "🚀 Starting Flash Gateway Dashboard..."

# Wait for postgres to be ready
echo "⏳ Waiting for PostgreSQL..."
while ! nc -z ${PGHOST:-postgres} ${PGPORT:-5432}; do
  echo "   PostgreSQL not ready, waiting..."
  sleep 2
done
echo "✅ PostgreSQL is ready!"

# Install dependencies if node_modules is missing (for mounted volumes)
if [ ! -d "node_modules" ]; then
  echo "📦 Installing dependencies..."
  npm ci
fi

# Start based on environment
if [ "$NODE_ENV" = "production" ]; then
  echo "🏭 Starting in production mode..."

  # Install serve if not available
  if ! command -v serve >/dev/null 2>&1; then
    npm install -g serve
  fi

  # Start backend server in background
  echo "   Starting API server on port ${API_PORT:-4000}..."
  node server/index.js &

  # Serve static frontend files
  echo "   Starting frontend server on port 5173..."
  exec serve -s dist -l 5173 -d
else
  echo "🛠️  Starting in development mode..."

  # Start backend server in background
  echo "   Starting API server on port ${API_PORT:-4000}..."
  node server/index.js &

  # Start Vite dev server with hot reload
  echo "   Starting Vite dev server on port 5173..."
  exec npm run dev -- --host 0.0.0.0
fi