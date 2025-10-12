# ⚡ Quick Start - Pure Lambda

Deploy a web scraper to AWS Lambda in 5 minutes!

## What You're Deploying

A simple, efficient Lambda function that:
- Scrapes any webpage
- Extracts article content
- Returns clean JSON
- No Docker, no Next.js overhead

## Prerequisites (One-time Setup)

```bash
# 1. Install AWS CLI
brew install awscli

# 2. Install SAM CLI  
brew install aws-sam-cli

# 3. Configure AWS
aws configure
# Enter your:
#   - Access Key ID
#   - Secret Access Key
#   - Region (e.g., us-east-1)
#   - Output format (json)
```

## Deploy Now! 🚀

```bash
# 1. Set API key (replace with your actual key)
export SCRAPE_API_KEY="my-super-secret-key-123"

# 2. Deploy (first time takes 3-5 minutes)
./deploy.sh
```

**Note:** The first deployment will:
- ✅ Create an S3 bucket for your Lambda code
- ✅ Create the Lambda function
- ✅ Create API Gateway endpoint
- ✅ Return your API URL

That's it! You'll get an API URL when done.

## Test It

```bash
# Copy the URL from deployment output, then:
curl "https://YOUR-API-URL/scrape?url=https://example.com" \
  -H "x-api-key: your-secret-key-here"
```

## What Happens During Deployment?

1. ✅ Installs dependencies in `lambda/` folder
2. ✅ Builds SAM application
3. ✅ Creates Lambda function (3GB RAM, 5min timeout)
4. ✅ Creates API Gateway endpoint
5. ✅ Returns your API URL

## Expected Response

```json
{
  "title": "Article Title",
  "description": "Article description",
  "content": "Full article content...",
  "images": ["url1.jpg", "url2.jpg"]
}
```

## View Logs

```bash
sam logs --stack-name extract-html-scraper --tail
```

## Update After Changes

```bash
./deploy.sh
```

## Delete Everything

```bash
sam delete --stack-name extract-html-scraper --no-prompts
```

## Cost

**Free for most users!**
- AWS Free Tier: 1M requests/month forever
- Typical cost: $0-7/month
- First year is basically free

## Project Structure

```
lambda/
  ├── index.js         # Handler (API Gateway → Lambda)
  ├── scraper.js       # Scraping logic (Puppeteer)
  └── package.json     # Dependencies

template.yaml          # AWS SAM template
deploy.sh              # One-command deploy
```

## Common Issues

### "AWS credentials not configured"
```bash
aws configure
```

### "SAM CLI not installed"
```bash
brew install aws-sam-cli
```

### Timeout errors?
Edit `template.yaml`:
```yaml
Timeout: 600  # Increase to 10 minutes
```

### Out of memory?
Edit `template.yaml`:
```yaml
MemorySize: 5120  # Increase to 5GB
```

## What's Next?

- 📖 Read [README.md](README.md) for full docs
- 🔧 Customize settings in `template.yaml`
- 📊 View metrics in AWS CloudWatch Console
- 🔐 Set up custom domains, auth, etc.

## Need Help?

Check the full guide: [README.md](README.md)

---

**You're 5 minutes away from a production-ready web scraper! 🎉**

