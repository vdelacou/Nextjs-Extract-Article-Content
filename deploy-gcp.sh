# Google Cloud Run deployment script
#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Google Cloud Run Deployment${NC}"
echo "================================="

# Configuration
PROJECT_ID=${GOOGLE_CLOUD_PROJECT:-"panda-social-473507"}
SERVICE_NAME="extract-html-scraper"
REGION=${GOOGLE_CLOUD_REGION:-"us-central1"}
IMAGE_NAME="gcr.io/$PROJECT_ID/$SERVICE_NAME"

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ gcloud CLI not found. Please install it first:${NC}"
    echo "https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Check if user is authenticated
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" | grep -q .; then
    echo -e "${RED}❌ Not authenticated with gcloud. Please run:${NC}"
    echo "gcloud auth login"
    exit 1
fi

# Set project
echo -e "${BLUE}Setting project to: $PROJECT_ID${NC}"
gcloud config set project $PROJECT_ID

# Enable required APIs
echo -e "${BLUE}Enabling required APIs...${NC}"
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com

# Build and push Docker image
echo -e "${BLUE}Building Docker image...${NC}"
gcloud builds submit --config cloudbuild.yaml --substitutions=_PROJECT_ID=$PROJECT_ID .

# Deploy to Cloud Run
echo -e "${BLUE}Deploying to Cloud Run...${NC}"
gcloud run deploy $SERVICE_NAME \
    --image $IMAGE_NAME \
    --platform managed \
    --region $REGION \
    --allow-unauthenticated \
    --memory 2Gi \
    --cpu 2 \
    --timeout 300 \
    --concurrency 10 \
    --max-instances 100 \
    --set-env-vars="SCRAPE_API_KEY=${SCRAPE_API_KEY:-test-key-123}"

# Get service URL
SERVICE_URL=$(gcloud run services describe $SERVICE_NAME --region=$REGION --format="value(status.url)")

# Create service account for API Gateway
echo -e "${BLUE}Creating service account for API Gateway...${NC}"
gcloud iam service-accounts create cr-gw-invoker \
    --display-name="Cloud Run Gateway Invoker" \
    --project=$PROJECT_ID || echo "Service account already exists"

# Grant service account permission to invoke Cloud Run
echo -e "${BLUE}Granting Cloud Run invoker role to service account...${NC}"
gcloud run services add-iam-policy-binding $SERVICE_NAME \
    --region=$REGION \
    --member="serviceAccount:cr-gw-invoker@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/run.invoker"

# Enable API Gateway API
echo -e "${BLUE}Enabling API Gateway API...${NC}"
gcloud services enable apigateway.googleapis.com
gcloud services enable servicemanagement.googleapis.com
gcloud services enable servicecontrol.googleapis.com

# Create API Gateway config
echo -e "${BLUE}Creating API Gateway configuration...${NC}"
gcloud api-gateway api-configs create scraper-config-v1 \
    --api=extract-html-scraper-api \
    --openapi-spec=api-gateway-config.yaml \
    --backend-auth-service-account=cr-gw-invoker@$PROJECT_ID.iam.gserviceaccount.com \
    --project=$PROJECT_ID || echo "Config already exists, creating new version..."

# Create or update API
echo -e "${BLUE}Creating API...${NC}"
gcloud api-gateway apis create extract-html-scraper-api \
    --project=$PROJECT_ID || echo "API already exists"

# Deploy gateway
echo -e "${BLUE}Deploying API Gateway...${NC}"
gcloud api-gateway gateways create extract-html-scraper-gateway \
    --api=extract-html-scraper-api \
    --api-config=scraper-config-v1 \
    --location=$REGION \
    --project=$PROJECT_ID || echo "Gateway exists, updating..."

# Get gateway URL
GATEWAY_URL=$(gcloud api-gateway gateways describe extract-html-scraper-gateway \
    --location=$REGION \
    --project=$PROJECT_ID \
    --format="value(defaultHostname)")

echo -e "${GREEN}✅ API Gateway deployed!${NC}"
echo -e "${GREEN}Gateway URL: https://$GATEWAY_URL${NC}"
echo -e "${BLUE}Test the service:${NC}"
echo "curl \"https://$GATEWAY_URL?url=https://example.com&key=YOUR_API_KEY\""
