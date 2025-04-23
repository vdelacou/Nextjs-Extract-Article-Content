# Next.js Extract Article Content

A Next.js-based API service that extracts content from web pages using Playwright and article-extractor. This service provides a clean interface to scrape article content, titles, descriptions, and images from any given URL.

## Features

- 🔒 Secure API key authentication
- 📄 Extracts article content, title, and description
- 🖼️ Image URL extraction
- 🧹 Sanitized HTML output
- ⚡ Serverless-friendly with Chromium binary support
- 🔄 Automatic content extraction using article-extractor

## Getting Started

### Prerequisites

- Node.js 16.x or later
- npm, yarn, pnpm, or bun

### Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/nextjs-extract-article-content.git
cd nextjs-extract-article-content
```

2. Install dependencies:
```bash
npm install
# or
yarn install
# or
pnpm install
# or
bun install
```

3. Create a `.env.local` file in the root directory and add your API key:
```
SCRAPE_API_KEY=your-secret-key-here
```

4. Start the development server:
```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

The API will be available at `http://localhost:3000/api/scrape`

## API Usage

### Endpoint

```
GET /api/scrape
```

### Parameters

- `url` (required): The URL of the webpage to scrape
- `key` (required): Your API key (can also be sent as `x-api-key` header)

### Example Request

```bash
curl -X GET "http://localhost:3000/api/scrape?url=https://example.com/article" \
     -H "x-api-key: your-secret-key-here"
```

### Response Format

```json
{
  "title": "Article Title",
  "description": "Article description or summary",
  "content": "Extracted and sanitized article content",
  "images": [
    "https://example.com/image1.jpg",
    "https://example.com/image2.png"
  ]
}
```

### Error Responses

- `401 Unauthorized`: Invalid or missing API key
- `400 Bad Request`: Missing URL parameter
- `500 Internal Server Error`: Scraping failed

## Environment Variables

- `SCRAPE_API_KEY`: Your secret API key for authentication

## Technologies Used

- [Next.js](https://nextjs.org/) - React framework
- [Playwright](https://playwright.dev/) - Browser automation
- [@sparticuz/chromium](https://github.com/Sparticuz/chromium) - Serverless Chromium
- [article-extractor](https://github.com/extractus/article-extractor) - Content extraction
- [sanitize-html](https://github.com/apostrophecms/sanitize-html) - HTML sanitization

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
