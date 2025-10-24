# Google Cloud Run - Quick Reference

## Essential Commands

### Authentication
```bash
gcloud auth login
gcloud auth application-default login
gcloud config set project YOUR_PROJECT_ID
```

### Enable APIs
```bash
gcloud services enable cloudbuild.googleapis.com run.googleapis.com containerregistry.googleapis.com
```

### Build & Deploy
```bash
# Option 1: Automated
SCRAPE_API_KEY="your-key" ./deploy-gcp.sh

# Option 2: Manual
gcloud builds submit --config cloudbuild.yaml .
gcloud run deploy extract-html-scraper \
    --image gcr.io/YOUR_PROJECT_ID/extract-html-scraper \
    --platform managed --region us-central1 \
    --allow-unauthenticated --memory 2Gi --cpu 2 \
    --timeout 300 --set-env-vars="SCRAPE_API_KEY=your-key"
```

### Get Service URL
```bash
gcloud run services describe extract-html-scraper --region=us-central1 --format="value(status.url)"
```

### Test Service
```bash
curl "YOUR_SERVICE_URL?url=https://example.com&key=your-api-key"
```

### View Logs
```bash
gcloud run services logs tail extract-html-scraper --region=us-central1
```

### Update Service
```bash
gcloud run deploy extract-html-scraper --image gcr.io/YOUR_PROJECT_ID/extract-html-scraper --region=us-central1
```

## Configuration Options

| Parameter | Value | Description |
|-----------|-------|-------------|
| `--memory` | 2Gi | Required for Chrome |
| `--cpu` | 2 | Faster scraping |
| `--timeout` | 300 | 5 min max |
| `--concurrency` | 10 | Requests per instance |
| `--max-instances` | 100 | Scale limit |

## Troubleshooting

### Common Issues
- **Auth error**: `gcloud auth login`
- **API not enabled**: Run enable commands above
- **Build fails**: Check `Dockerfile.gcp` syntax
- **Service won't start**: Check logs for errors

### Debug Commands
```bash
# Check service status
gcloud run services describe extract-html-scraper --region=us-central1

# View recent logs
gcloud run services logs read extract-html-scraper --region=us-central1 --limit=20

# Test locally
docker build -f Dockerfile.gcp -t test . && docker run -p 8080:8080 test
```

## Performance Expectations

| Metric | Expected |
|--------|----------|
| Cold Start | 1-2s |
| HTTP Scrape | 500ms-2s |
| Browser Scrape | 3-8s |
| Memory Usage | 1-2GB |
