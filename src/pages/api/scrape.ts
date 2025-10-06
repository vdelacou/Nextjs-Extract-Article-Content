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
const MAX_RETRIES = 3

// Periodic cache cleanup
setInterval(() => {
  const now = Date.now()
  for (const [key, entry] of cache.entries()) {
    if (now - entry.timestamp > CACHE_TTL) cache.delete(key)
  }
}, 1000 * 60 * 5)

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
  try {
    // API key validation
    if (!validateApiKey(req)) {
      return res.status(401).json({ error: 'Invalid or missing API key' })
    }

    const url = getUrlParam(req)
    if (!url) {
      return res.status(400).json({ error: 'Missing "url" query parameter' })
    }

    // Cache lookup
    const cached = cache.get(url)
    if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
      return res.status(200).json(cached.data)
    }

    // Retry loop with exponential backoff
    let lastErr: unknown = null
    for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
      try {
        const result = await scrapeWebsite(url)
        cache.set(url, { data: result, timestamp: Date.now() })
        return res.status(200).json(result)
      } catch (err) {
        lastErr = err
        const msg = err instanceof Error ? err.message : String(err)
        
        // Exponential backoff
        if (attempt < MAX_RETRIES) {
          const backoff = 500 * Math.pow(2, attempt - 1)
          console.warn(`Scrape attempt ${attempt} failed. Retrying after ${backoff}ms — reason: ${msg}`)
          await new Promise((r) => setTimeout(r, backoff))
        }
      }
    }

    const msg = lastErr instanceof Error ? lastErr.message : String(lastErr)
    return res.status(500).json({ error: 'Failed to scrape', details: msg })
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    console.error('Unexpected handler error:', err)
    return res.status(500).json({ error: 'Internal error', details: msg })
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
      args: chromiumBinary.args,
      headless: chromiumBinary.headless,
      defaultViewport: { width: 1200, height: 900 },
      timeout: 60000
    })
  } else {
    // Local: Use regular puppeteer with bundled Chromium
    const puppeteer = await import('puppeteer')
    
    return await puppeteer.default.launch({
      headless: true,
      args: ['--no-sandbox', '--disable-setuid-sandbox'],
      defaultViewport: { width: 1200, height: 900 },
      timeout: 60000
    })
  }
}

/** --------------------------
 *  Main Scraping Function
 *  -------------------------- */
async function scrapeWebsite(url: string): Promise<ScraperResponse> {
  const browser = await launchBrowser()

  try {
    const page: Page = await browser.newPage()

    // Set realistic headers
    const userAgent = process.env.SCRAPE_USER_AGENT || 
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36'
    
    await page.setExtraHTTPHeaders({
      'user-agent': userAgent,
      'accept-language': 'en-US,en;q=0.9'
    })

    page.setDefaultNavigationTimeout(60000)
    page.setDefaultTimeout(60000)

    // Navigate to page
    await page.goto(url, { waitUntil: 'networkidle2', timeout: 60000 })
      .catch(async () => {
        // Fallback to domcontentloaded if networkidle2 fails
        return await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 60000 })
      })

    // Wait for content to load
    await sleep(1500)

    // Scroll to trigger lazy-loaded content
    await autoScroll(page)
    await sleep(800)

    // Get HTML and sanitize it
    const rawHtml = await page.content()
    
    // Extract content from sanitized HTML
    const article = await extractFromHtml(rawHtml, url)
    
    // Extract images
    const images = await extractImageUrls(page, url)

    await browser.close()

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
      const distance = 200
      const timer = setInterval(() => {
        window.scrollBy(0, distance)
        total += distance
        if (total > document.body.scrollHeight - window.innerHeight) {
          clearInterval(timer)
          resolve()
        }
      }, 200)
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

  // Measure image sizes
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
        setTimeout(() => finish(0, 0), 2500)
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