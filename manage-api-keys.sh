#!/bin/bash

PROJECT_ID="panda-social-473507"
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
