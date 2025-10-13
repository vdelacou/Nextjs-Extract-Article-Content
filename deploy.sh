#!/bin/bash

# Simple AWS Lambda Deployment Script (No Docker)
# Deploys pure Lambda function with zip-based packaging

set -e

echo "🚀 AWS Lambda Simple Deployment"
echo "================================"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

STACK_NAME="extract-html-scraper"
REGION="${AWS_REGION:-us-east-1}"

# Check prerequisites
if ! command -v aws &> /dev/null; then
    echo -e "${RED}❌ AWS CLI not installed${NC}"
    echo "Install: brew install awscli"
    exit 1
fi

if ! command -v sam &> /dev/null; then
    echo -e "${RED}❌ AWS SAM CLI not installed${NC}"
    echo "Install: brew install aws-sam-cli"
    exit 1
fi

# Check AWS credentials
if ! aws sts get-caller-identity &> /dev/null; then
    echo -e "${RED}❌ AWS credentials not configured${NC}"
    echo "Run: aws configure"
    exit 1
fi

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
echo -e "${GREEN}✓ AWS Account: $ACCOUNT_ID${NC}"
echo -e "${GREEN}✓ Region: $REGION${NC}"

# Get API key
if [ -z "$SCRAPE_API_KEY" ]; then
    echo ""
    echo -e "${YELLOW}Enter your SCRAPE_API_KEY:${NC}"
    read -rs SCRAPE_API_KEY
    export SCRAPE_API_KEY
fi

# Install dependencies in lambda folder
echo ""
echo "Installing dependencies..."
cd lambda
npm init -y 2>/dev/null || true
npm install --save \
  @extractus/article-extractor@8.0.20 \
  @sparticuz/chromium@141.0.0 \
  puppeteer-core@24.24.0 \
  sanitize-html@2.17.0 \
  undici@7.16.0
cd ..
echo -e "${GREEN}✓ Dependencies installed${NC}"

# Build with SAM (no container needed)
echo ""
echo "Building SAM application..."
sam build --template-file template.yaml

# Deploy
echo ""
echo "Deploying to AWS Lambda..."
sam deploy \
  --template-file .aws-sam/build/template.yaml \
  --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides ScrapeApiKey="$SCRAPE_API_KEY" \
  --resolve-s3 \
  --no-confirm-changeset \
  --no-fail-on-empty-changeset

# Get API URL
API_URL=$(aws cloudformation describe-stacks \
  --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --query 'Stacks[0].Outputs[?OutputKey==`ApiUrl`].OutputValue' \
  --output text)

echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}✅ Deployment successful!${NC}"
echo -e "${GREEN}================================${NC}"
echo ""
echo "API Endpoint: ${API_URL}/scrape"
echo ""
echo "Test with:"
echo -e "${YELLOW}curl \"${API_URL}/scrape?url=https://example.com\" \\${NC}"
echo -e "${YELLOW}  -H \"x-api-key: YOUR_API_KEY\"${NC}"
echo ""
echo "View logs:"
echo -e "${YELLOW}sam logs --stack-name $STACK_NAME --tail${NC}"
echo ""

