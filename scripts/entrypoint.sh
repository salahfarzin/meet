#!/bin/sh

# Config (DATABASE_URL included) is delivered as a mounted .env file, not
# as container env vars - load it so the check below can actually see it.
# Assigned line-by-line rather than sourced: values like
# DATABASE_URL=...tcp(host:port)... contain shell metacharacters that break
# `. /app/.env`.
if [ -f /app/.env ]; then
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            ''|'#'*) continue ;;
        esac
        export "${line%%=*}=${line#*=}"
    done < /app/.env
fi

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