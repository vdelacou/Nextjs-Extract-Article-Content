// pages/api/scrape.ts
import type { NextApiRequest, NextApiResponse } from 'next'
import chromiumBinary from '@sparticuz/chromium'
import sanitize from 'sanitize-html'
import { extractFromHtml } from '@extractus/article-extractor'
import { Browser, Page } from 'puppeteer-core'

/** --------------------------
 *  Config
 *  -------------------------- */
const CACHE_TTL = 1000 * 60 * 60 // 1 hour
const cache = new Map<string, { data: ScraperResponse; timestamp: number }>()
const MAX_RETRIES = 2 // Reduced retries for faster failure
const REQUEST_TIMEOUT = 8000 // 8 seconds to stay under Vercel's 10s limit

/** --------------------------
 *  Types
 *  -------------------------- */
type ScraperResponse = {
  title?: string
  description?: string
  content?: string
  images?: string[]
  error?: string
  details?: string
}

/** --------------------------
 *  API Handler
 *  -------------------------- */
export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<ScraperResponse>
) {
  const startTime = Date.now()
  
  try {
    // API key validation
    if (!validateApiKey(req)) {
      return res.status(401).json({ error: 'Invalid or missing API key' })
    }

    const url = getUrlParam(req)
    if (!url) {
      return res.status(400).json({ error: 'Missing "url" query parameter' })
    }

    // Clean expired cache entries on-demand (serverless-friendly)
    cleanExpiredCache()

    // Cache lookup
    const cached = cache.get(url)
    if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
      console.log(`Cache hit for ${url}`)
      return res.status(200).json(cached.data)
    }

    // Retry loop with shorter backoff
    let lastErr: unknown = null
    for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
      try {
        const result = await scrapeWebsite(url)
        cache.set(url, { data: result, timestamp: Date.now() })
        
        const duration = Date.now() - startTime
        console.log(`✓ Scraped ${url} in ${duration}ms`)
        
        return res.status(200).json(result)
      } catch (err) {
        lastErr = err
        const msg = err instanceof Error ? err.message : String(err)
        
        // Shorter backoff for serverless
        if (attempt < MAX_RETRIES) {
          const backoff = 300 * Math.pow(2, attempt - 1)
          console.warn(`Attempt ${attempt}/${MAX_RETRIES} failed (${backoff}ms backoff): ${msg}`)
          await new Promise((r) => setTimeout(r, backoff))
        }
      }
    }

    const duration = Date.now() - startTime
    const msg = lastErr instanceof Error ? lastErr.message : String(lastErr)
    console.error(`✗ Failed to scrape ${url} after ${duration}ms`)
    return res.status(500).json({ error: 'Failed to scrape', details: msg })
  } catch (err) {
    const duration = Date.now() - startTime
    const msg = err instanceof Error ? err.message : String(err)
    console.error(`Unexpected handler error after ${duration}ms:`, err)
    return res.status(500).json({ error: 'Internal error', details: msg })
  }
}

/** Clean expired cache entries (serverless-friendly) */
function cleanExpiredCache() {
  const now = Date.now()
  for (const [key, entry] of cache.entries()) {
    if (now - entry.timestamp > CACHE_TTL) {
      cache.delete(key)
    }
  }
}

/** --------------------------
 *  Validation & Parameters
 *  -------------------------- */
function validateApiKey(req: NextApiRequest): boolean {
  const apiKey = (req.headers['x-api-key'] as string) || (req.query.key as string | undefined)
  const validKey = process.env.SCRAPE_API_KEY
  if (!validKey) {
    console.error('Missing SCRAPE_API_KEY env var')
    throw new Error('Server misconfiguration')
  }
  return apiKey === validKey
}

function getUrlParam(req: NextApiRequest): string {
  const urlParam = req.query.url
  return typeof urlParam === 'string' ? urlParam : ''
}

/** --------------------------
 *  Browser Launch (Serverless-Ready)
 *  -------------------------- */
async function launchBrowser(): Promise<Browser> {
  const isServerless = !!process.env.AWS_LAMBDA_FUNCTION_NAME || !!process.env.VERCEL

  if (isServerless) {
    // Serverless: Use puppeteer-core with @sparticuz/chromium
    const puppeteerCore = await import('puppeteer-core')
    const executablePath = await chromiumBinary.executablePath()
    
    return await puppeteerCore.default.launch({
      executablePath,
      args: [
        ...chromiumBinary.args,
        '--disable-dev-shm-usage', // Prevents memory issues
        '--disable-gpu',
        '--single-process' // Faster startup for serverless
      ],
      headless: chromiumBinary.headless,
      defaultViewport: { width: 1200, height: 900 },
      timeout: 15000 // Reduced from 60s
    })
  } else {
    // Local: Use regular puppeteer with bundled Chromium
    const puppeteer = await import('puppeteer')
    
    return await puppeteer.default.launch({
      headless: true,
      args: ['--no-sandbox', '--disable-setuid-sandbox'],
      defaultViewport: { width: 1200, height: 900 },
      timeout: 15000 // Reduced from 60s
    })
  }
}

/** --------------------------
 *  Main Scraping Function
 *  -------------------------- */
async function scrapeWebsite(url: string): Promise<ScraperResponse> {
  const stepTimes: Record<string, number> = {}
  const logStep = (name: string, start: number) => {
    stepTimes[name] = Date.now() - start
  }

  let startStep = Date.now()
  const browser = await launchBrowser()
  logStep('browser_launch', startStep)

  try {
    startStep = Date.now()
    const page: Page = await browser.newPage()

    // Set realistic headers
    const userAgent = process.env.SCRAPE_USER_AGENT || 
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36'
    
    await page.setExtraHTTPHeaders({
      'user-agent': userAgent,
      'accept-language': 'en-US,en;q=0.9'
    })

    // Aggressive timeouts for serverless
    page.setDefaultNavigationTimeout(REQUEST_TIMEOUT)
    page.setDefaultTimeout(REQUEST_TIMEOUT)

    // Use domcontentloaded for faster page loads (networkidle2 is too slow)
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: REQUEST_TIMEOUT })
    logStep('page_load', startStep)

    // Minimal wait for dynamic content (reduced from 1500ms)
    startStep = Date.now()
    await sleep(500)

    // Quick scroll to trigger lazy-loaded content
    await autoScroll(page)
    await sleep(300) // Reduced from 800ms
    logStep('scroll_and_wait', startStep)

    // Get HTML and extract content
    startStep = Date.now()
    const rawHtml = await page.content()
    const article = await extractFromHtml(rawHtml, url)
    logStep('extract_article', startStep)
    
    // Extract images with timeout
    startStep = Date.now()
    const images = await extractImageUrls(page, url)
    logStep('extract_images', startStep)

    await browser.close()

    console.log(`Performance breakdown: ${JSON.stringify(stepTimes)}`)

    return {
      title: article?.title ? sanitize(article.title, { allowedTags: [], allowedAttributes: {} }).trim() : undefined,
      description: article?.description ? sanitize(article.description, { allowedTags: [], allowedAttributes: {} }).trim() : undefined,
      content: article?.content ? sanitize(article.content,{allowedTags: [], allowedAttributes: {}}).trim() : undefined,
      images
    }
  } catch (err) {
    try { await browser.close() } catch {}
    throw err
  }
}

/** --------------------------
 *  Helper Functions
 *  -------------------------- */
function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

async function autoScroll(page: Page) {
  await page.evaluate(async () => {
    await new Promise<void>((resolve) => {
      let total = 0
      const distance = 300 // Increased distance per scroll
      const timer = setInterval(() => {
        window.scrollBy(0, distance)
        total += distance
        if (total > document.body.scrollHeight - window.innerHeight) {
          clearInterval(timer)
          resolve()
        }
      }, 100) // Faster interval (was 200ms)
    })
  })
}

/** --------------------------
 *  Image Extraction
 *  -------------------------- */
async function extractImageUrls(page: Page, baseUrl: string): Promise<string[]> {
  const minWidth = 600
  const minHeight = 600
  const minArea = 100_000

  // First, try to get og:image as it's usually the main article image
  const ogImage = await page.evaluate(() => {
    const meta = document.querySelector<HTMLMetaElement>(
      'meta[property="og:image:secure_url"], meta[property="og:image"]'
    )
    return meta?.content || ''
  })

  const junkRe = /(sprite|icon|favicon|logo|avatar|emoji|placeholder|spacer|1x1|pixel)/i
  const allowedExt = /\.(jpe?g|png|gif|webp)(?:[?#].*)?$/i

  // If og:image exists and is valid, prioritize it
  if (ogImage) {
    try {
      const abs = new URL(ogImage, baseUrl).toString()
      if (allowedExt.test(abs) && !junkRe.test(abs)) {
        return [abs]
      }
    } catch {}
  }

  // Collect image candidates with context priority
  const candidates = await page.$$eval('img', (imgs) => {
    return imgs.map((img: HTMLImageElement, index: number) => {
      const src = img.getAttribute('data-src') ||
                  img.getAttribute('data-lazy-src') ||
                  img.getAttribute('src') || ''
      const wAttr = parseInt(img.getAttribute('width') || '', 10) || undefined
      const hAttr = parseInt(img.getAttribute('height') || '', 10) || undefined
      
      // Check if image is in main content area (article, main, or has high priority classes)
      const isInMainContent = img.closest('article, main, .article, .post-content, .entry-content') !== null
      const isInSidebar = img.closest('aside, .sidebar, .widget, .featured') !== null
      
      return { 
        url: src, 
        wAttr, 
        hAttr, 
        domIndex: index,
        isInMainContent,
        isInSidebar
      }
    }).filter((x: { url: string }) => !!x.url)
  })

  // Normalize URLs
  const normalized = candidates
    .map((c: { url: string; wAttr?: number; hAttr?: number; domIndex: number; isInMainContent: boolean; isInSidebar: boolean }) => {
      try {
        const abs = new URL(c.url, baseUrl).toString()
        if (!allowedExt.test(abs) || junkRe.test(abs)) return null
        return { ...c, url: abs }
      } catch {
        return null
      }
    })
    .filter(Boolean) as typeof candidates

  // Measure image sizes (with aggressive timeout for serverless)
  const sized = await page.evaluate(async (items: typeof candidates) => {
    const loadOne = (u: string): Promise<{ url: string; w: number; h: number }> =>
      new Promise((resolve) => {
        const img = new Image()
        let done = false
        const finish = (w = 0, h = 0) => {
          if (!done) {
            done = true
            resolve({ url: u, w, h })
          }
        }
        img.onload = () => finish(img.naturalWidth || 0, img.naturalHeight || 0)
        img.onerror = () => finish(0, 0)
        setTimeout(() => finish(0, 0), 1000) // Reduced from 2500ms
        img.src = u
      })

    const results = await Promise.all(items.map((i: { url: string; wAttr?: number; hAttr?: number; domIndex: number; isInMainContent: boolean; isInSidebar: boolean }) => loadOne(i.url)))
    return results.map((r: { url: string; w: number; h: number }, idx: number) => {
      const w = r.w || items[idx].wAttr || 0
      const h = r.h || items[idx].hAttr || 0
      return { 
        url: r.url, 
        w, 
        h, 
        area: w * h,
        domIndex: items[idx].domIndex,
        isInMainContent: items[idx].isInMainContent,
        isInSidebar: items[idx].isInSidebar
      }
    })
  }, normalized)

  // Filter by size
  const finalList = sized.filter((s: { w: number; h: number; area: number }) => 
    s.w >= minWidth && s.h >= minHeight && s.area >= minArea
  )

  // Sort with priority: main content first, then by area, then by DOM order
  finalList.sort((a: { isInMainContent: boolean; isInSidebar: boolean; area: number; domIndex: number }, b: { isInMainContent: boolean; isInSidebar: boolean; area: number; domIndex: number }) => {
    // Prioritize main content over sidebar
    if (a.isInMainContent && !b.isInMainContent) return -1
    if (!a.isInMainContent && b.isInMainContent) return 1
    
    // Deprioritize sidebar images
    if (a.isInSidebar && !b.isInSidebar) return 1
    if (!a.isInSidebar && b.isInSidebar) return -1
    
    // Then by area (larger first)
    if (Math.abs(b.area - a.area) > 10000) return b.area - a.area
    
    // Finally by DOM order (earlier first)
    return a.domIndex - b.domIndex
  })

  return finalList.map((x: { url: string }) => x.url)
}