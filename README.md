# AWS Lambda Web Scraper

A pure AWS Lambda function for web scraping and article content extraction. No Next.js, no Docker - just a simple, efficient Lambda function.

## ✨ Features

- 🚀 **Pure Lambda**: No framework overhead, just Node.js
- 📦 **Zip-based deployment**: No Docker required
- 🔒 **API Key authentication**
- 📄 **Article extraction**: Title, description, content
- 🖼️ **Smart image extraction**: Finds main article images
- 🧹 **Sanitized output**: Clean HTML-free content
- ⚡ **Optimized for Lambda**: Uses @sparticuz/chromium

## 🎯 Quick Start

### Prerequisites

```bash
# Install AWS CLI
brew install awscli

# Install AWS SAM CLI
brew install aws-sam-cli

# Configure AWS credentials
aws configure
```

### Deploy in 3 Steps

```bash
# 1. Set your API key
export SCRAPE_API_KEY="your-secret-key-here"

# 2. Deploy
./deploy-lambda-simple.sh

# 3. Test
curl "https://YOUR-API-URL/scrape?url=https://example.com" \
  -H "x-api-key: your-secret-key-here"
```

That's it! 🎉

## 📁 Project Structure

```
lambda/
├── index.js           # Lambda handler
├── scraper.js         # Scraping logic
├── package.json       # Dependencies
└── node_modules/      # Installed packages

template-simple.yaml   # SAM template (no Docker)
deploy-lambda-simple.sh # Deployment script
```


## 📚 API Documentation

### Endpoint

```
GET /scrape?url=TARGET_URL
```

### Headers

```
x-api-key: YOUR_API_KEY
```

### Parameters

- `url` (required): The URL to scrape

### Example Request

```bash
curl "https://abc123.execute-api.us-east-1.amazonaws.com/scrape?url=https://example.com" \
  -H "x-api-key: your-secret-key"
```

### Response Format

```json
{
  "title": "Article Title",
  "description": "Article description or summary",
  "content": "Full article content (sanitized)",
  "images": [
    "https://example.com/image1.jpg",
    "https://example.com/image2.jpg"
  ]
}
```

### Error Responses

- `400` - Missing URL or invalid URL format
- `401` - Invalid or missing API key
- `500` - Scraping failed

## ⚙️ Configuration

### Lambda Settings

Edit `template-simple.yaml`:

```yaml
Globals:
  Function:
    Timeout: 300        # 5 minutes (max: 900)
    MemorySize: 3008    # 3GB (recommended for Chromium)
    Runtime: nodejs20.x
    Architectures:
      - arm64           # 30% cost savings vs x86_64
```

### Environment Variables

Set in Lambda console or SAM template:

- `SCRAPE_API_KEY` - Your API key (required)
- `SCRAPE_USER_AGENT` - Custom user agent (optional)

## 📊 Performance

**Typical execution times on Lambda (3GB, ARM64):**
- Browser launch: 2-4 seconds
- Page load: 1-3 seconds
- Content extraction: 1-2 seconds
- Image extraction: 3-7 seconds
- **Total**: 10-20 seconds

**Cold start:** ~3-5 seconds (first request after idle)

## 💰 Cost Estimation

**AWS Lambda Pricing (us-east-1):**

**Free Tier (Forever):**
- 1M requests/month FREE
- 400,000 GB-seconds FREE

**After Free Tier:**
- Requests: $0.20 per 1M requests
- Compute: $0.0000166667 per GB-second (ARM64)

**Example: 10,000 requests/month, 3GB RAM, 15s avg**
- Requests: FREE (under 1M)
- Compute: 10,000 × 3GB × 15s = 450,000 GB-seconds
- Compute cost: 450,000 × $0.0000166667 = $7.50
- **Total: ~$7.50/month** (likely less with free tier)

## 📈 Monitoring & Logs

### View Logs

```bash
# Real-time logs
sam logs --stack-name extract-html-scraper --tail

# Or via AWS CLI
aws logs tail /aws/lambda/extract-html-scraper --follow
```

### CloudWatch Metrics

View in AWS Console:
- Invocations
- Duration
- Errors
- Throttles
- Concurrent executions

## 🔧 Troubleshooting

### Timeout Errors

Increase timeout in `template-simple.yaml`:
```yaml
Timeout: 600  # 10 minutes
```

Then redeploy:
```bash
./deploy-lambda-simple.sh
```

### Out of Memory

Increase memory:
```yaml
MemorySize: 5120  # 5GB
```

More memory also means more CPU power!

### Deployment Fails

```bash
# Check SAM build
sam build --template-file template-simple.yaml

# Validate template
sam validate --template-file template-simple.yaml

# Check CloudFormation events
aws cloudformation describe-stack-events \
  --stack-name extract-html-scraper
```

### Dependencies Not Found

```bash
# Reinstall in lambda folder
cd lambda
rm -rf node_modules package-lock.json
npm install
cd ..

# Rebuild and deploy
./deploy-lambda-simple.sh
```

## 🚀 Advanced Usage

### Provisioned Concurrency

Keep instances warm (eliminates cold starts, ~$15/month):

```yaml
ScraperFunction:
  Type: AWS::Serverless::Function
  Properties:
    ProvisionedConcurrencyConfig:
      ProvisionedConcurrentExecutions: 1
```

### Custom Domain

Use API Gateway custom domain:

1. Register domain in Route 53
2. Create ACM certificate
3. Add to SAM template:

```yaml
HttpApi:
  Properties:
    Domain:
      DomainName: api.yourdomain.com
      CertificateArn: arn:aws:acm:...
```

### VPC Access

For private resource access:

```yaml
ScraperFunction:
  Properties:
    VpcConfig:
      SecurityGroupIds:
        - sg-xxx
      SubnetIds:
        - subnet-xxx
```

### Scheduled Warming

Prevent cold starts with CloudWatch Events:

```yaml
Events:
  WarmUp:
    Type: Schedule
    Properties:
      Schedule: rate(5 minutes)
      Input: '{"warmup": true}'
```

## 🔐 Security Best Practices

1. **Use AWS Secrets Manager** for API keys:
   ```bash
   aws secretsmanager create-secret \
     --name scraper-api-key \
     --secret-string "your-key"
   ```

2. **Enable API Gateway authorization**:
   - IAM authorization
   - Lambda authorizer
   - API key + usage plans

3. **Set up CloudWatch alarms**:
   - Error rate
   - Throttles
   - Duration

4. **Use IAM roles** (not access keys) for AWS access

5. **Implement rate limiting** via API Gateway usage plans

## 🔄 CI/CD Integration

### GitHub Actions

```yaml
name: Deploy Lambda
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: aws-actions/setup-sam@v2
      - uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1
      - run: |
          cd lambda
          npm install
          cd ..
          sam build --template-file template-simple.yaml
          sam deploy --no-confirm-changeset
        env:
          SCRAPE_API_KEY: ${{ secrets.SCRAPE_API_KEY }}
```

## 🗑️ Cleanup

Delete everything:

```bash
# Delete stack
sam delete --stack-name extract-html-scraper --no-prompts

# Or via AWS CLI
aws cloudformation delete-stack \
  --stack-name extract-html-scraper

# Verify deletion
aws cloudformation describe-stacks \
  --stack-name extract-html-scraper
```

## 📖 Additional Resources

- [AWS Lambda Documentation](https://docs.aws.amazon.com/lambda/)
- [AWS SAM Documentation](https://docs.aws.amazon.com/serverless-application-model/)
- [@sparticuz/chromium](https://github.com/Sparticuz/chromium)
- [Puppeteer Documentation](https://pptr.dev/)
- [Article Extractor](https://github.com/extractus/article-extractor)

## 🤝 Contributing

Contributions welcome! Please feel free to submit a Pull Request.

## 📄 License

MIT

## 💬 Support

- Check logs: `sam logs --stack-name extract-html-scraper --tail`
- Review CloudWatch metrics in AWS Console
- Open an issue on GitHub

---

**Made with ❤️ for serverless web scraping**

