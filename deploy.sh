#!/bin/bash

# Go Lambda Container Deployment Script
# Deploys Go-based Lambda function with Docker container to ECR

set -e

echo "🚀 Go Lambda Container Deployment"
echo "================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

STACK_NAME="extract-html-scraper-go"
REGION="${AWS_REGION:-us-east-1}"
REPOSITORY_NAME="extract-html-scraper"

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

if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker not installed${NC}"
    echo "Install: https://docs.docker.com/get-docker/"
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

# Initialize Go module if needed
if [ ! -f "go.mod" ]; then
    echo ""
    echo -e "${BLUE}Initializing Go module...${NC}"
    go mod init extract-html-scraper
fi

# Download Go dependencies
echo ""
echo -e "${BLUE}Downloading Go dependencies...${NC}"
go mod tidy
echo -e "${GREEN}✓ Dependencies downloaded${NC}"

# Build Go binary locally first (for testing)
echo ""
echo -e "${BLUE}Building Go binary locally...${NC}"
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bootstrap ./cmd/lambda
echo -e "${GREEN}✓ Go binary built${NC}"

# Create ECR repository if it doesn't exist
echo ""
echo -e "${BLUE}Setting up ECR repository...${NC}"
aws ecr describe-repositories --repository-names $REPOSITORY_NAME --region $REGION &> /dev/null || {
    echo -e "${YELLOW}Creating ECR repository...${NC}"
    aws ecr create-repository --repository-name $REPOSITORY_NAME --region $REGION
    echo -e "${GREEN}✓ ECR repository created${NC}"
}

# Get ECR login token
echo -e "${BLUE}Logging into ECR...${NC}"
aws ecr get-login-password --region $REGION | docker login --username AWS --password-stdin $ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com
echo -e "${GREEN}✓ ECR login successful${NC}"

# Build Docker image for single platform (Lambda requirement)
echo ""
echo -e "${BLUE}Building Docker image...${NC}"
IMAGE_TAG="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$REPOSITORY_NAME:latest"
docker build --platform linux/amd64 -t $IMAGE_TAG .
echo -e "${GREEN}✓ Docker image built${NC}"

# Push image to ECR
echo ""
echo -e "${BLUE}Pushing image to ECR...${NC}"
docker push $IMAGE_TAG
echo -e "${GREEN}✓ Image pushed to ECR${NC}"

# Deploy with SAM
echo ""
echo -e "${BLUE}Deploying with SAM...${NC}"
sam deploy \
  --template-file template.yaml \
  --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides ScrapeApiKey="$SCRAPE_API_KEY" \
  --image-repository "$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$REPOSITORY_NAME" \
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
echo -e "${BLUE}Stack Name:${NC} $STACK_NAME"
echo -e "${BLUE}API Endpoint:${NC} ${API_URL}/scrape"
echo -e "${BLUE}ECR Repository:${NC} $IMAGE_TAG"
echo ""
echo -e "${YELLOW}Test with:${NC}"
echo -e "${YELLOW}curl \"${API_URL}/scrape?url=https://example.com\" \\${NC}"
echo -e "${YELLOW}  -H \"x-api-key: YOUR_API_KEY\"${NC}"
echo ""
echo -e "${YELLOW}View logs:${NC}"
echo -e "${YELLOW}sam logs --stack-name $STACK_NAME --tail${NC}"
echo ""
echo -e "${YELLOW}Update image:${NC}"
echo -e "${YELLOW}docker build -t $IMAGE_TAG . && docker push $IMAGE_TAG${NC}"
echo -e "${YELLOW}aws lambda update-function-code --function-name extract-html-scraper-go --image-uri $IMAGE_TAG${NC}"
echo ""

# Clean up local binary
rm -f bootstrap
echo -e "${GREEN}✓ Cleanup completed${NC}"