#!/bin/bash

# Test script for Google Cloud Run deployment
SERVICE_URL=${1:-"https://extract-html-scraper-xxxxx-uc.a.run.app"}
API_KEY=${2:-"test-key-123"}
TEST_URL=${3:-"https://example.com"}

echo "Testing Google Cloud Run service..."
echo "Service URL: $SERVICE_URL"
echo "API Key: $API_KEY"
echo "Test URL: $TEST_URL"
echo ""

# Test the service
curl -v "$SERVICE_URL?url=$TEST_URL&key=$API_KEY"
