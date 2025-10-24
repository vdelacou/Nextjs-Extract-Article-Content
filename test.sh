#!/bin/bash

# Test script for Go Lambda scraper
# Tests the scraper with various URLs and validates performance

set -e

echo "🧪 Go Lambda Scraper Test Suite"
echo "================================"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
API_URL="${API_URL:-}"
API_KEY="${API_KEY:-}"
TEST_URLS=(
    "https://example.com"
    "https://httpbin.org/html"
    "https://jsonplaceholder.typicode.com/posts/1"
)

# Check if API URL is provided
if [ -z "$API_URL" ]; then
    echo -e "${YELLOW}Enter API URL (e.g., https://abc123.execute-api.us-east-1.amazonaws.com):${NC}"
    read API_URL
fi

if [ -z "$API_KEY" ]; then
    echo -e "${YELLOW}Enter API Key:${NC}"
    read -rs API_KEY
fi

echo ""
echo -e "${BLUE}Testing API Endpoint: $API_URL${NC}"
echo -e "${BLUE}Using API Key: ${API_KEY:0:8}...${NC}"
echo ""

# Test function
test_scrape() {
    local url="$1"
    local test_name="$2"
    
    echo -e "${BLUE}Testing: $test_name${NC}"
    echo -e "${BLUE}URL: $url${NC}"
    
    start_time=$(date +%s%3N)
    
    response=$(curl -s -w "\n%{http_code}" \
        "$API_URL/scrape?url=$url" \
        -H "x-api-key: $API_KEY" \
        -H "Content-Type: application/json")
    
    end_time=$(date +%s%3N)
    duration=$((end_time - start_time))
    
    # Split response and status code
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    echo -e "${BLUE}Status: $http_code${NC}"
    echo -e "${BLUE}Duration: ${duration}ms${NC}"
    
    if [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✓ Success${NC}"
        
        # Extract and display key fields
        title=$(echo "$body" | jq -r '.title // "N/A"' 2>/dev/null || echo "N/A")
        description=$(echo "$body" | jq -r '.description // "N/A"' 2>/dev/null || echo "N/A")
        image_count=$(echo "$body" | jq -r '.images | length' 2>/dev/null || echo "0")
        duration_ms=$(echo "$body" | jq -r '.metadata.durationMs // "N/A"' 2>/dev/null || echo "N/A")
        
        echo -e "${GREEN}  Title: $title${NC}"
        echo -e "${GREEN}  Description: ${description:0:100}...${NC}"
        echo -e "${GREEN}  Images: $image_count${NC}"
        echo -e "${GREEN}  Server Duration: ${duration_ms}ms${NC}"
        
    elif [ "$http_code" = "451" ]; then
        echo -e "${YELLOW}⚠ Blocked by Cloudflare${NC}"
        provider=$(echo "$body" | jq -r '.provider // "unknown"' 2>/dev/null || echo "unknown")
        domain=$(echo "$body" | jq -r '.domain // "unknown"' 2>/dev/null || echo "unknown")
        echo -e "${YELLOW}  Provider: $provider${NC}"
        echo -e "${YELLOW}  Domain: $domain${NC}"
        
    else
        echo -e "${RED}✗ Failed${NC}"
        echo -e "${RED}  Response: $body${NC}"
    fi
    
    echo ""
}

# Test CORS preflight
echo -e "${BLUE}Testing CORS preflight...${NC}"
cors_response=$(curl -s -o /dev/null -w "%{http_code}" \
    -X OPTIONS \
    "$API_URL/scrape" \
    -H "Origin: https://example.com" \
    -H "Access-Control-Request-Method: GET" \
    -H "Access-Control-Request-Headers: x-api-key")

if [ "$cors_response" = "204" ]; then
    echo -e "${GREEN}✓ CORS preflight successful${NC}"
else
    echo -e "${RED}✗ CORS preflight failed (status: $cors_response)${NC}"
fi
echo ""

# Test invalid API key
echo -e "${BLUE}Testing invalid API key...${NC}"
invalid_response=$(curl -s -w "\n%{http_code}" \
    "$API_URL/scrape?url=https://example.com" \
    -H "x-api-key: invalid-key")

invalid_code=$(echo "$invalid_response" | tail -n1)
if [ "$invalid_code" = "401" ]; then
    echo -e "${GREEN}✓ Invalid API key properly rejected${NC}"
else
    echo -e "${RED}✗ Invalid API key not rejected (status: $invalid_code)${NC}"
fi
echo ""

# Test missing URL parameter
echo -e "${BLUE}Testing missing URL parameter...${NC}"
missing_url_response=$(curl -s -w "\n%{http_code}" \
    "$API_URL/scrape" \
    -H "x-api-key: $API_KEY")

missing_url_code=$(echo "$missing_url_response" | tail -n1)
if [ "$missing_url_code" = "400" ]; then
    echo -e "${GREEN}✓ Missing URL parameter properly rejected${NC}"
else
    echo -e "${RED}✗ Missing URL parameter not rejected (status: $missing_url_code)${NC}"
fi
echo ""

# Test actual scraping
echo -e "${BLUE}Testing actual scraping...${NC}"
for url in "${TEST_URLS[@]}"; do
    test_scrape "$url" "Basic scraping test"
done

# Performance test
echo -e "${BLUE}Performance test (multiple requests)...${NC}"
total_duration=0
success_count=0
test_count=5

for i in $(seq 1 $test_count); do
    echo -e "${BLUE}Request $i/$test_count${NC}"
    
    start_time=$(date +%s%3N)
    response=$(curl -s -w "\n%{http_code}" \
        "$API_URL/scrape?url=https://example.com" \
        -H "x-api-key: $API_KEY")
    end_time=$(date +%s%3N)
    
    duration=$((end_time - start_time))
    total_duration=$((total_duration + duration))
    
    http_code=$(echo "$response" | tail -n1)
    if [ "$http_code" = "200" ]; then
        success_count=$((success_count + 1))
    fi
    
    echo -e "${BLUE}  Duration: ${duration}ms${NC}"
done

avg_duration=$((total_duration / test_count))
echo ""
echo -e "${GREEN}Performance Summary:${NC}"
echo -e "${GREEN}  Success Rate: $success_count/$test_count ($(( success_count * 100 / test_count ))%)${NC}"
echo -e "${GREEN}  Average Duration: ${avg_duration}ms${NC}"
echo -e "${GREEN}  Total Duration: ${total_duration}ms${NC}"

# Test with a potentially blocked site
echo ""
echo -e "${BLUE}Testing with potentially blocked site...${NC}"
test_scrape "https://httpbin.org/status/403" "403 Error Test"

echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}✅ Test suite completed!${NC}"
echo -e "${GREEN}================================${NC}"
