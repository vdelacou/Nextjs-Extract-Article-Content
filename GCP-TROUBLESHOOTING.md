# Google Cloud Run Troubleshooting Guide

## Common Issues and Solutions

### 1. Authentication Problems

#### Issue: "Reauthentication failed"
```bash
ERROR: (gcloud.auth) There was a problem refreshing your current auth tokens
```

**Solution:**
```bash
# Re-authenticate
gcloud auth login
gcloud auth application-default login

# Verify authentication
gcloud auth list
```

#### Issue: "Project not found"
```bash
ERROR: (gcloud.config.set) Project [PROJECT_ID] not found
```

**Solution:**
```bash
# List available projects
gcloud projects list

# Set correct project
gcloud config set project YOUR_ACTUAL_PROJECT_ID
```

### 2. API Not Enabled Errors

#### Issue: "API [api] not enabled"
```bash
ERROR: (gcloud.services.enable) API [cloudbuild.googleapis.com] not enabled
```

**Solution:**
```bash
# Enable all required APIs
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com

# Verify APIs are enabled
gcloud services list --enabled
```

### 3. Build Failures

#### Issue: Docker build fails
```bash
ERROR: failed to build: Dockerfile not found
```

**Solution:**
```bash
# Check if Dockerfile.gcp exists
ls -la Dockerfile.gcp

# Build locally to test
docker build -f Dockerfile.gcp -t test-image .

# Check Dockerfile syntax
docker build --no-cache -f Dockerfile.gcp -t test-image .
```

#### Issue: Go build fails
```bash
ERROR: go: module not found
```

**Solution:**
```bash
# Check go.mod exists
ls -la go.mod

# Download dependencies
go mod download

# Test build locally
go build -o test-binary ./cmd/cloudrun
```

#### Issue: Chrome installation fails
```bash
ERROR: Could not find xdg-icon-resource
```

**Solution:**
This is a warning, not an error. Chrome will still work. If you want to fix it:
```dockerfile
# Add to Dockerfile.gcp before Chrome installation
RUN apk add --no-cache xdg-utils
```

### 4. Service Deployment Issues

#### Issue: Service won't start
```bash
ERROR: Container failed to start
```

**Solution:**
```bash
# Check logs for specific error
gcloud run services logs read extract-html-scraper --region=us-central1

# Common causes:
# 1. Missing environment variables
# 2. Port not exposed (should be 8080)
# 3. Binary not executable
```

#### Issue: Service returns 500 errors
```bash
HTTP 500 Internal Server Error
```

**Solution:**
```bash
# Check logs for Go panic or error
gcloud run services logs read extract-html-scraper --region=us-central1 --limit=50

# Common causes:
# 1. Missing SCRAPE_API_KEY environment variable
# 2. Chrome binary not found
# 3. Memory too low (increase to 2Gi)
```

#### Issue: Service times out
```bash
HTTP 504 Gateway Timeout
```

**Solution:**
```bash
# Increase timeout
gcloud run deploy extract-html-scraper \
    --timeout 600 \
    --region us-central1

# Check if scraping is taking too long
gcloud run services logs read extract-html-scraper --region=us-central1 | grep "duration"
```

### 5. Performance Issues

#### Issue: Slow cold starts
```bash
# Cold start taking >5 seconds
```

**Solution:**
```bash
# Increase memory allocation
gcloud run deploy extract-html-scraper \
    --memory 2Gi \
    --cpu 2 \
    --region us-central1

# Consider keeping warm instances
gcloud run deploy extract-html-scraper \
    --min-instances 1 \
    --region us-central1
```

#### Issue: High memory usage
```bash
# Service running out of memory
```

**Solution:**
```bash
# Increase memory
gcloud run deploy extract-html-scraper \
    --memory 4Gi \
    --region us-central1

# Or optimize Chrome settings in code
# Add to Dockerfile.gcp:
ENV CHROME_FLAGS="--no-sandbox --disable-dev-shm-usage --disable-gpu"
```

### 6. Network Issues

#### Issue: Cannot reach external URLs
```bash
ERROR: failed to fetch URL
```

**Solution:**
```bash
# Check if service has internet access
gcloud run services logs read extract-html-scraper --region=us-central1 | grep "network"

# Cloud Run has internet access by default
# If using VPC, ensure proper configuration
```

#### Issue: CORS errors
```bash
ERROR: CORS policy blocked request
```

**Solution:**
```bash
# CORS is handled in the Go code
# Check if headers are set correctly in cmd/cloudrun/main.go
# Ensure Access-Control-Allow-Origin is set to "*"
```

### 7. Chrome/Browser Issues

#### Issue: Chrome crashes
```bash
ERROR: Chrome process exited
```

**Solution:**
```bash
# Increase memory allocation
gcloud run deploy extract-html-scraper \
    --memory 2Gi \
    --region us-central1

# Add Chrome flags for stability
ENV CHROME_FLAGS="--no-sandbox --disable-dev-shm-usage --disable-gpu --disable-web-security"
```

#### Issue: Chrome not found
```bash
ERROR: Chrome binary not found
```

**Solution:**
```bash
# Check if Chrome is installed in container
docker run --rm gcr.io/YOUR_PROJECT_ID/extract-html-scraper which chromium-browser

# Verify environment variables
gcloud run services describe extract-html-scraper --region=us-central1 --format="value(spec.template.spec.template.spec.containers[0].env)"
```

### 8. Debugging Commands

#### Get Service Information
```bash
# Service details
gcloud run services describe extract-html-scraper --region=us-central1

# Service URL
gcloud run services describe extract-html-scraper --region=us-central1 --format="value(status.url)"

# Environment variables
gcloud run services describe extract-html-scraper --region=us-central1 --format="value(spec.template.spec.template.spec.containers[0].env)"
```

#### View Logs
```bash
# Real-time logs
gcloud run services logs tail extract-html-scraper --region=us-central1

# Recent logs
gcloud run services logs read extract-html-scraper --region=us-central1 --limit=50

# Logs with timestamps
gcloud run services logs read extract-html-scraper --region=us-central1 --format="table(timestamp,severity,text)"
```

#### Test Locally
```bash
# Build and test locally
docker build -f Dockerfile.gcp -t local-test .
docker run -p 8080:8080 -e SCRAPE_API_KEY="test-key" local-test

# Test the service
curl "http://localhost:8080?url=https://example.com&key=test-key"
```

### 9. Cost Optimization Issues

#### Issue: High costs
```bash
# Unexpected high billing
```

**Solution:**
```bash
# Check resource allocation
gcloud run services describe extract-html-scraper --region=us-central1

# Optimize settings
gcloud run deploy extract-html-scraper \
    --memory 1Gi \
    --cpu 1 \
    --concurrency 20 \
    --max-instances 50 \
    --region us-central1

# Monitor usage
gcloud billing accounts list
```

### 10. Getting Help

#### Useful Commands
```bash
# Get help for any gcloud command
gcloud run deploy --help

# Check gcloud version
gcloud version

# Get diagnostic information
gcloud info

# Check quotas
gcloud compute project-info describe --project=YOUR_PROJECT_ID
```

#### Support Resources
- [Google Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Cloud Run Troubleshooting](https://cloud.google.com/run/docs/troubleshooting)
- [Stack Overflow - google-cloud-run](https://stackoverflow.com/questions/tagged/google-cloud-run)
- [Google Cloud Community](https://cloud.google.com/community)

#### Contact Support
```bash
# Generate support case information
gcloud support cases create --display-name="Cloud Run Issue" --description="Describe your issue"

# Or use Cloud Console: https://console.cloud.google.com/support
```
