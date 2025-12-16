#!/bin/sh

# Run database migrations
echo "Running database migrations..."
if [ -n "$DATABASE_URL" ]; then
    migrate -path /app/migrations -database "$DATABASE_URL" up
    if [ $? -eq 0 ]; then
        echo "Migrations completed successfully"
    else
        echo "Migration failed"
        exit 1
    fi
else
    echo "DATABASE_URL not set, skipping migrations"
fi

# Start the application
echo "Starting meet service..."
exec /app/meet-service