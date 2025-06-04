#!/bin/bash

set -e

# Default values
DB_HOST=${DB_HOST:-"localhost"}
DB_PORT=${DB_PORT:-"3306"}
DB_USER=${DB_USER:-"root"}
DB_PASSWORD=${DB_PASSWORD:-""}
DB_NAME=""
DRY_RUN=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --db-host)
            DB_HOST="$2"
            shift 2
            ;;
        --db-port)
            DB_PORT="$2"
            shift 2
            ;;
        --db-user)
            DB_USER="$2"
            shift 2
            ;;
        --db-password)
            DB_PASSWORD="$2"
            shift 2
            ;;
        --db-name)
            DB_NAME="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --db-host HOST         Database host (default: localhost)"
            echo "  --db-port PORT         Database port (default: 3306)"
            echo "  --db-user USER         Database user (default: root)"
            echo "  --db-password PASS     Database password"
            echo "  --db-name NAME         Database name (required)"
            echo "  --dry-run              Run without making changes"
            echo "  --help                 Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ -z "$DB_NAME" ]]; then
    echo "Error: --db-name is required"
    exit 1
fi

# Build the migration tool if it doesn't exist
if [[ ! -f "./migrate" ]]; then
    echo "Building migration tool..."
    go build -o migrate migrate.go
fi

# Run the migration
echo "Running migration on database: $DB_NAME"
if [[ "$DRY_RUN" == "true" ]]; then
    echo "DRY RUN MODE"
fi

./migrate \
    -db-host="$DB_HOST" \
    -db-port="$DB_PORT" \
    -db-user="$DB_USER" \
    -db-password="$DB_PASSWORD" \
    -db-name="$DB_NAME" \
    ${DRY_RUN:+-dry-run}

echo "Migration completed successfully!"