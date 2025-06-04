# Makefile
# ===========================================
# Makefile content (save as separate file):

.PHONY: build migrate-dev migrate-prod migrate-plugins migrate-packs help

# Build the migration tool
build:
	go build -o migrate migrate.go

# Migrate to development database
migrate-dev: build
	./scripts/migrate.sh \
		--db-name=kraken \
		--db-user=kraken \
		--db-password=$(DEV_PASSWORD)

# Migrate to production database (with dry-run first)
migrate-prod-dry: build
	./scripts/migrate.sh \
		--db-host=$(PROD_HOST) \
		--db-name=kraken \
		--db-user=kraken \
		--db-password=$(PROD_PASSWORD) \
		--dry-run

migrate-prod: build
	@echo "Are you sure you want to migrate to production? [y/N]"
	@read -r REPLY; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		./scripts/migrate.sh \
			--db-host=$(PROD_HOST) \
			--db-name=kraken \
			--db-user=kraken \
			--db-password=$(PROD_PASSWORD) \
	else \
		echo "Migration cancelled."; \
	fi

help:
	@echo "Available commands:"
	@echo "  build            - Build the migration tool"
	@echo "  migrate-dev      - Migrate to development database"
	@echo "  migrate-prod-dry - Dry run migration to production"
	@echo "  migrate-prod     - Migrate to production (with confirmation)"
	@echo ""
	@echo "Environment variables:"
	@echo "  DEV_PASSWORD      - Database password"
	@echo "  PROD_PASSWORD      - Database password"
	@echo "  PROD_HOST          - Database host (default: localhost)"
	@echo ""
	@echo "Examples:"
	@echo "  make migrate-dev"
	@echo "  make migrate-prod PROD_PASSWORD=secretpassword"
	@echo "  DB_NAME=mydb DB_USER=user make migrate-dev"