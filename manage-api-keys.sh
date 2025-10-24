#!/bin/bash

# WARNING: Do not commit API keys or sensitive data to version control
PROJECT_ID=${GOOGLE_CLOUD_PROJECT}
if [ -z "$PROJECT_ID" ]; then
    echo "❌ GOOGLE_CLOUD_PROJECT environment variable is required"
    echo "Please set it with: export GOOGLE_CLOUD_PROJECT=your-project-id"
    exit 1
fi
API_NAME="extract-html-scraper-api"

case "$1" in
  create)
    echo "Creating new API key..."
    gcloud services api-keys create \
        --display-name="Scraper API Key" \
        --api-target=service=extract-html-scraper-api \
        --project=$PROJECT_ID
    ;;
  list)
    gcloud services api-keys list --project=$PROJECT_ID
    ;;
  *)
    echo "Usage: $0 {create|list}"
    exit 1
    ;;
esac
