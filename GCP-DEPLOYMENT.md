# Google Cloud Run Deployment Guide

## Overview

This guide covers deploying the Go-based HTML scraper to Google Cloud Run, providing superior performance compared to AWS Lambda with faster cold starts, longer timeouts, and better resource allocation.

## Prerequisites

### 1. Google Cloud CLI Installation
```bash
# macOS (using Homebrew)
brew install google-cloud-sdk

# Or download from: https://cloud.google.com/sdk/docs/install
```

### 2. Authentication
```bash
# Login to Google Cloud
gcloud auth login

# Set up application default credentials
gcloud auth application-default login

# Verify authentication
gcloud auth list
```

### 3. Project Setup
```bash
# Set your project (replace with your project ID)
gcloud config set project YOUR_PROJECT_ID

# Verify project
gcloud config get-value project
```

## Quick Start Deployment

### Option 1: Automated Deployment Script
```bash
# Clone and navigate to project
cd /path/to/extract-html

# Set your API key
export SCRAPE_API_KEY="your-secure-api-key"

# Run deployment script
./deploy-gcp.sh
```

### Option 2: Manual Step-by-Step Deployment

#### Step 1: Enable Required APIs
```bash
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com
```

#### Step 2: Build and Push Docker Image
```bash
# Build using Cloud Build
gcloud builds submit --config cloudbuild.yaml .

# Or build locally and push
docker build -f Dockerfile.gcp -t gcr.io/YOUR_PROJECT_ID/extract-html-scraper .
gcloud auth configure-docker
docker push gcr.io/YOUR_PROJECT_ID/extract-html-scraper
```

#### Step 3: Deploy to Cloud Run
```bash
gcloud run deploy extract-html-scraper \
    --image gcr.io/YOUR_PROJECT_ID/extract-html-scraper \
    --platform managed \
    --region us-central1 \
    --allow-unauthenticated \
    --memory 2Gi \
    --cpu 2 \
    --timeout 300 \
    --concurrency 10 \
    --max-instances 100 \
    --set-env-vars="SCRAPE_API_KEY=your-secure-api-key"
```

#### Step 4: Get Service URL
```bash
gcloud run services describe extract-html-scraper \
    --region=us-central1 \
    --format="value(status.url)"
```

## Configuration Options

### Resource Allocation

| Setting | Recommended | Description |
|---------|-------------|-------------|
| **Memory** | 2Gi | Required for Chrome/Chromium |
| **CPU** | 2 | Faster scraping performance |
| **Timeout** | 300s | 5 minutes max per request |
| **Concurrency** | 10 | Requests per instance |
| **Max Instances** | 100 | Adjust based on usage |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `SCRAPE_API_KEY` | Yes | API key for authentication |
| `PORT` | No | Port (set automatically by Cloud Run) |

### Regional Deployment

**Recommended Regions:**
- `us-central1` (Iowa) - Lowest latency for US
- `us-east1` (South Carolina) - Good for East Coast
- `europe-west1` (Belgium) - For European users
- `asia-southeast1` (Singapore) - For Asian users

## Performance Expectations

### Google Cloud Run vs AWS Lambda

| Metric | Google Cloud Run | AWS Lambda | Improvement |
|--------|------------------|------------|-------------|
| **Cold Start** | 1-2 seconds | 3-5 seconds | **2-3x faster** |
| **Timeout** | 60 minutes | 15 minutes | **4x longer** |
| **Memory** | Up to 32Gi | 10Gi max | **3x more** |
| **CPU** | Up to 8 vCPUs | 6 vCPUs max | **33% more** |
| **Concurrency** | Better scaling | Limited | **Better** |

### Expected Scraping Performance

| Scraping Type | Expected Time | Notes |
|---------------|---------------|-------|
| **HTTP Scraping** | 500ms - 2s | Fast, lightweight |
| **Browser Scraping** | 3-8s | Chrome automation |
| **Cold Start** | 1-2s | First request |
| **Memory Usage** | 1-2GB | With Chrome |

## Testing Your Deployment

### 1. Basic Health Check
```bash
# Replace YOUR_SERVICE_URL with your actual service URL
curl "YOUR_SERVICE_URL?url=https://example.com&key=your-api-key"
```

### 2. Test Script
```bash
# Use the provided test script
./test-gcp.sh "YOUR_SERVICE_URL" "your-api-key" "https://example.com"
```

### 3. Expected Response
```json
{
  "title": "Example Domain",
  "description": "This domain is for use in documentation examples...",
  "content": "Example Domain\n\nThis domain is for use in documentation examples...",
  "images": null,
  "metadata": {
    "url": "https://example.com",
    "scrapedAt": "2025-10-24T16:17:33.861308+08:00",
    "durationMs": 801
  }
}
```

## Monitoring and Logs

### View Logs
```bash
# Real-time logs
gcloud run services logs tail extract-html-scraper --region=us-central1

# Recent logs
gcloud run services logs read extract-html-scraper --region=us-central1 --limit=50
```

### Monitor Performance
```bash
# Service status
gcloud run services describe extract-html-scraper --region=us-central1

# Metrics in Cloud Console
# Visit: https://console.cloud.google.com/run
```

## Cost Optimization

### Pricing Model
- **Pay per request**: Only charged when processing requests
- **CPU allocation**: Charged only during request processing
- **Memory**: Charged only during request processing
- **No idle costs**: Unlike always-on servers

### Cost-Saving Tips

1. **Optimize Memory**: Start with 1Gi, increase if needed
2. **Adjust Concurrency**: Higher concurrency = fewer instances
3. **Regional Deployment**: Choose closest region to reduce latency
4. **Timeout Settings**: Set appropriate timeouts to avoid over-billing

### Estimated Costs (us-central1)
- **Free Tier**: 2 million requests/month
- **After Free Tier**: ~$0.40 per million requests
- **Memory**: ~$0.0000024 per GB-second
- **CPU**: ~$0.000024 per vCPU-second

## Troubleshooting

### Common Issues

#### 1. Authentication Errors
```bash
# Re-authenticate
gcloud auth login
gcloud auth application-default login

# Check current account
gcloud auth list
```

#### 2. API Not Enabled
```bash
# Enable required APIs
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com
```

#### 3. Build Failures
```bash
# Check Dockerfile syntax
docker build -f Dockerfile.gcp -t test-image .

# Check Cloud Build logs
gcloud builds list --limit=5
gcloud builds log BUILD_ID
```

#### 4. Service Not Starting
```bash
# Check logs for errors
gcloud run services logs read extract-html-scraper --region=us-central1

# Verify environment variables
gcloud run services describe extract-html-scraper --region=us-central1
```

#### 5. Chrome/Chromium Issues
```bash
# Check if Chrome is installed in container
gcloud run services logs read extract-html-scraper --region=us-central1 | grep -i chrome

# Verify Chrome environment variables
gcloud run services describe extract-html-scraper --region=us-central1 --format="value(spec.template.spec.template.spec.containers[0].env)"
```

### Debug Mode

#### Enable Debug Logging
```bash
# Deploy with debug environment variable
gcloud run deploy extract-html-scraper \
    --image gcr.io/YOUR_PROJECT_ID/extract-html-scraper \
    --set-env-vars="SCRAPE_API_KEY=your-key,DEBUG=true" \
    --region us-central1
```

#### Local Testing
```bash
# Test locally before deploying
docker build -f Dockerfile.gcp -t local-scraper .
docker run -p 8080:8080 -e SCRAPE_API_KEY="test-key" local-scraper

# Test the local service
curl "http://localhost:8080?url=https://example.com&key=test-key"
```

## Security Best Practices

### 1. API Key Management
```bash
# Use Google Secret Manager for production
gcloud secrets create scrape-api-key --data-file=api-key.txt

# Reference secret in deployment
gcloud run deploy extract-html-scraper \
    --set-secrets="SCRAPE_API_KEY=scrape-api-key:latest"
```

### 2. Network Security
```bash
# Deploy with VPC connector for private networks
gcloud run deploy extract-html-scraper \
    --vpc-connector=projects/YOUR_PROJECT/locations/us-central1/connectors/YOUR_CONNECTOR
```

### 3. IAM Permissions
```bash
# Create service account with minimal permissions
gcloud iam service-accounts create scraper-service \
    --display-name="Scraper Service Account"

# Grant only necessary permissions
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="serviceAccount:scraper-service@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/run.invoker"
```

## Scaling and Performance Tuning

### Auto-scaling Configuration
```bash
# Configure auto-scaling
gcloud run deploy extract-html-scraper \
    --min-instances=0 \
    --max-instances=100 \
    --concurrency=10 \
    --cpu-throttling
```

### Performance Optimization
1. **Memory Tuning**: Start with 1Gi, increase if Chrome crashes
2. **CPU Allocation**: Use 2 CPUs for faster scraping
3. **Concurrency**: Higher values = fewer instances needed
4. **Timeout**: Set based on expected scraping time

## Migration from AWS Lambda

### Comparison Table

| Feature | AWS Lambda | Google Cloud Run | Winner |
|---------|------------|-----------------|--------|
| Cold Start | 3-5s | 1-2s | **Cloud Run** |
| Timeout | 15 min | 60 min | **Cloud Run** |
| Memory | 10Gi max | 32Gi max | **Cloud Run** |
| CPU | 6 vCPUs max | 8 vCPUs max | **Cloud Run** |
| Cost | Pay per request | Pay per request | **Tie** |
| Ecosystem | AWS services | GCP services | **Depends** |

### Migration Steps
1. **Deploy to Cloud Run** (this guide)
2. **Test thoroughly** with your use cases
3. **Update DNS/API Gateway** to point to Cloud Run
4. **Monitor performance** and costs
5. **Decommission Lambda** once stable

## Support and Resources

### Documentation
- [Google Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Cloud Build Documentation](https://cloud.google.com/build/docs)
- [Container Registry Documentation](https://cloud.google.com/container-registry/docs)

### Community
- [Google Cloud Community](https://cloud.google.com/community)
- [Stack Overflow](https://stackoverflow.com/questions/tagged/google-cloud-run)

### Getting Help
```bash
# Check service status
gcloud run services describe extract-html-scraper --region=us-central1

# View recent errors
gcloud run services logs read extract-html-scraper --region=us-central1 --limit=20

# Get help with gcloud commands
gcloud run deploy --help
```