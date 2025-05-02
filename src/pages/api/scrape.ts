import type { NextApiRequest, NextApiResponse } from 'next'
import { chromium, Page } from 'playwright-core'
import chromiumBinary from '@sparticuz/chromium'
import sanitize from 'sanitize-html'
import { extractFromHtml } from '@extractus/article-extractor'

// Cache implementation
interface CacheEntry {
  data: ScraperResponse
  timestamp: number
}

const CACHE_TTL = 1000 * 60 * 60 // 1 hour in milliseconds
const cache = new Map<string, CacheEntry>()

// Clean up expired cache entries periodically
setInterval(() => {
  const now = Date.now()
  for (const [key, entry] of cache.entries()) {
    if (now - entry.timestamp > CACHE_TTL) {
      cache.delete(key)
    }
  }
}, 1000 * 60 * 5) // Clean up every 5 minutes

type ScraperResponse = {
  title?: string
  description?: string
  content?: string
  images?: string[]
  error?: string
  details?: string
}

/**
 * API handler for web scraping functionality
 */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ScraperResponse>
) {
  // Validate API key
  if (!validateApiKey(req)) {
    return res.status(401).json({ error: 'Invalid or missing API key' })
  }

  // Validate URL parameter
  const url = getUrlParam(req)
  if (!url) {
    return res.status(400).json({ error: 'Missing "url" query parameter' })
  }

  // Check cache first
  const cachedResult = cache.get(url)
  if (cachedResult && Date.now() - cachedResult.timestamp < CACHE_TTL) {
    return res.status(200).json(cachedResult.data)
  }

  try {
    const result = await scrapeWebsite(url)
    
    // Store in cache
    cache.set(url, {
      data: result,
      timestamp: Date.now()
    })
    
    return res.status(200).json(result)
  } catch (error) {
    console.error('❌ Scrape error:', error)
    return res.status(500).json({ 
      error: 'Failed to scrape', 
      details: error instanceof Error ? error.message : String(error) 
    })
  }
}

/**
 * Validates if the API key in the request is valid
 */
function validateApiKey(req: NextApiRequest): boolean {
  const apiKey = req.headers['x-api-key'] || req.query.key
  const validKey = process.env.SCRAPE_API_KEY
  
  if (!validKey) {
    console.error('Missing SCRAPE_API_KEY env var')
    throw new Error('Server misconfiguration')
  }
  
  return apiKey === validKey
}

/**
 * Extracts and validates the URL parameter from the request
 */
function getUrlParam(req: NextApiRequest): string {
  const urlParam = req.query.url
  return typeof urlParam === 'string' ? urlParam : ''
}

/**
 * Scrapes a website and extracts relevant content
 */
async function scrapeWebsite(url: string) {
  // Launch browser
  const browser = await launchBrowser()
  
  try {
    const page = await browser.newPage()
    // Reduce timeouts to 30 seconds
    page.setDefaultNavigationTimeout(30000)
    page.setDefaultTimeout(30000)

    // Navigate to target URL with more lenient wait conditions
    await page.goto(url, { 
      waitUntil: 'domcontentloaded', // Changed from networkidle to domcontentloaded
      timeout: 30000 
    })

    // Add a small delay to allow for dynamic content
    await page.waitForTimeout(2000)

    // Get full page HTML for extraction
    const html = await page.content()

    // Extract structured content
    const extractResult = await extractFromHtml(html, url)
    if (!extractResult?.content) {
      throw new Error('Failed to extract content')
    }

    const rawHtml = extractResult.content
    
    // Extract image URLs from content
    const imageUrls = await extractImageUrls(rawHtml, page)
    
    // Sanitize the content
    const cleanContent = sanitize(rawHtml, {
      allowedTags: [],
      allowedAttributes: {},
    }).trim()

    return {
      title: extractResult.title,
      description: extractResult.description,
      content: cleanContent,
      images: imageUrls
    }
  } catch (error) {
    console.error('Scraping error:', error)
    throw new Error(`Failed to scrape ${url}: ${error instanceof Error ? error.message : String(error)}`)
  } finally {
    await browser.close()
  }
}

/**
 * Launches a serverless-friendly Chromium browser
 */
async function launchBrowser() {
  const execPath = await chromiumBinary.executablePath()
  return chromium.launch({
    args: chromiumBinary.args,
    executablePath: execPath,
    headless: true,
    timeout: 60000,
  })
}

/**
 * Extracts image URLs from HTML content
 */
async function extractImageUrls(rawHtml: string, page: Page): Promise<string[]> {
  const regex = /<img[^>]*src=['"]([^'"]+\.(?:jpe?g|png))['"][^>]*>/gi
  const found: string[] = []
  let match: RegExpExecArray | null
  
  while ((match = regex.exec(rawHtml))) {
    // Clean the URL by removing query parameters
    const cleanUrl = match[1].split('?')[0]
    found.push(cleanUrl)
  }
  
  // Remove duplicates
  const imageUrls = Array.from(new Set(found))

  // Fallback to og:image if no images found
  if (imageUrls.length === 0) {
    try {
      const ogImage = await page.$eval(
        'meta[property="og:image"]',
        (el: HTMLMetaElement) => el.content
      )
      
      // Modified regex to match image extensions before any query parameters
      if (ogImage && /\.(jpe?g|png)(?:\?|$)/i.test(ogImage)) {
        // Clean og:image URL as well
        const cleanOgImage = ogImage.split('?')[0]
        imageUrls.push(cleanOgImage)
      }
    } catch (error) {
      // No og:image available, continue without it
      console.debug('No og:image found:', error instanceof Error ? error.message : String(error))
      return [];
    }
  }
  
  return imageUrls
}