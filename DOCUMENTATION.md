# Documentation Index

This project includes comprehensive documentation for deploying the Go web scraper to both AWS Lambda and Google Cloud Run.

## 📚 Complete Documentation

### Google Cloud Run (Recommended)
- **[GCP-DEPLOYMENT.md](./GCP-DEPLOYMENT.md)** - Complete deployment guide with step-by-step instructions
- **[GCP-QUICK-REFERENCE.md](./GCP-QUICK-REFERENCE.md)** - Quick command reference for common operations
- **[GCP-TROUBLESHOOTING.md](./GCP-TROUBLESHOOTING.md)** - Troubleshooting guide for common issues

### AWS Lambda
- **[QUICKSTART.md](./QUICKSTART.md)** - AWS Lambda quick start guide
- **[README.md](./README.md)** - Main project documentation with both platforms

## 🚀 Quick Start Links

### For Google Cloud Run Users
1. **Start Here**: [GCP-DEPLOYMENT.md](./GCP-DEPLOYMENT.md) - Complete guide
2. **Quick Commands**: [GCP-QUICK-REFERENCE.md](./GCP-QUICK-REFERENCE.md) - Command cheat sheet
3. **Problems?**: [GCP-TROUBLESHOOTING.md](./GCP-TROUBLESHOOTING.md) - Fix common issues

### For AWS Lambda Users
1. **Start Here**: [QUICKSTART.md](./QUICKSTART.md) - AWS deployment guide
2. **Main Docs**: [README.md](./README.md) - Project overview and AWS instructions

## 📋 What Each Document Contains

### GCP-DEPLOYMENT.md
- ✅ Prerequisites and setup
- ✅ Step-by-step deployment instructions
- ✅ Configuration options and performance tuning
- ✅ Cost optimization tips
- ✅ Security best practices
- ✅ Monitoring and logging
- ✅ Migration from AWS Lambda

### GCP-QUICK-REFERENCE.md
- ✅ Essential commands cheat sheet
- ✅ Common configuration parameters
- ✅ Troubleshooting quick fixes
- ✅ Performance expectations

### GCP-TROUBLESHOOTING.md
- ✅ Authentication problems and solutions
- ✅ Build failures and fixes
- ✅ Service deployment issues
- ✅ Performance optimization
- ✅ Debugging commands
- ✅ Cost optimization issues

### QUICKSTART.md
- ✅ AWS Lambda deployment guide
- ✅ SAM template configuration
- ✅ ECR setup and Docker builds
- ✅ Testing and validation

### README.md
- ✅ Project overview and features
- ✅ Performance comparison between platforms
- ✅ Quick start for both AWS and GCP
- ✅ Project structure and architecture
- ✅ API documentation

## 🎯 Platform Recommendations

### Choose Google Cloud Run if:
- ✅ You need longer timeouts (up to 60 minutes)
- ✅ You want better cold start performance (1-2s)
- ✅ You need more memory/CPU resources
- ✅ You prefer simpler deployment process
- ✅ You want better scaling for concurrent requests

### Choose AWS Lambda if:
- ✅ You're already using AWS ecosystem
- ✅ You need ultra-fast cold starts (100-300ms)
- ✅ You have existing AWS infrastructure
- ✅ You prefer AWS services integration

## 📞 Getting Help

### Google Cloud Run Issues
1. Check [GCP-TROUBLESHOOTING.md](./GCP-TROUBLESHOOTING.md)
2. Review [GCP-DEPLOYMENT.md](./GCP-DEPLOYMENT.md) configuration section
3. Use [GCP-QUICK-REFERENCE.md](./GCP-QUICK-REFERENCE.md) for command help

### AWS Lambda Issues
1. Check [QUICKSTART.md](./QUICKSTART.md) troubleshooting section
2. Review AWS CloudFormation logs
3. Check Docker build logs

### General Issues
1. Review [README.md](./README.md) for project overview
2. Check project structure and file organization
3. Verify all prerequisites are installed

## 🔄 Migration Between Platforms

### From AWS Lambda to Google Cloud Run
1. Follow [GCP-DEPLOYMENT.md](./GCP-DEPLOYMENT.md) migration section
2. Test thoroughly with [GCP-QUICK-REFERENCE.md](./GCP-QUICK-REFERENCE.md)
3. Update DNS/API Gateway to point to Cloud Run

### From Google Cloud Run to AWS Lambda
1. Follow [QUICKSTART.md](./QUICKSTART.md) AWS deployment
2. Test with existing test scripts
3. Update endpoints to point to Lambda

## 📊 Performance Expectations

| Platform | Cold Start | Timeout | Memory | CPU | Best For |
|----------|------------|---------|--------|-----|----------|
| **Google Cloud Run** | 1-2s | 60 min | 32Gi | 8 vCPUs | Long-running tasks |
| **AWS Lambda** | 100-300ms | 15 min | 10Gi | 6 vCPUs | Quick requests |

Both platforms provide significant performance improvements over the original Node.js implementation:
- **4-6x faster** HTTP scraping (500ms-2s vs 3-5s)
- **2-3x faster** browser scraping (3-8s vs 10-20s)
- **Better resource utilization** and cost efficiency
