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

type ImageExtractOpts = {
  minWidth?: number;
  minHeight?: number;
  minArea?: number;
  limit?: number; // optional cap on returned images
};

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
    const msg = error instanceof Error ? error.message : String(error);
    const isAntiBot = /anti-bot|cloudflare|challenge/i.test(msg);
    return res.status(isAntiBot ? 403 : 500).json({
      error: isAntiBot ? 'Page is protected by anti-bot measures' : 'Failed to scrape',
      details: msg
    });
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
    const response = await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
    await page.waitForTimeout(1500);

    const status = response?.status();
    const headers = response?.headers() ?? {};
    // Get full page HTML for extraction
    const html = await page.content();

    if (looksLikeCloudflare(html, headers, status)) {
      throw new Error('Target is protected by anti-bot measures (e.g., Cloudflare). Use an approved API/RSS/whitelisted access.');
    }

    // Extract structured content
    const extractResult = await extractFromHtml(html, url)
    if (!extractResult?.content) {
      throw new Error('Failed to extract content')
    }

    const rawHtml = extractResult.content

    // Extract image URLs from content
    const imageUrls = await extractImageUrls(rawHtml, page, url, {
      minWidth: 640,
      minHeight: 480,
      minArea: 640 * 480,
      limit: 6,
    })

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
  const isServerless = !!process.env.AWS_EXECUTION_ENV || !!process.env.VERCEL;
  if (isServerless) {
    const executablePath = await chromiumBinary.executablePath();
    return chromium.launch({
      executablePath,
      args: chromiumBinary.args,     // important on AWS Lambda / serverless
      headless: true,
      timeout: 60000,
    });
  }
  // local dev
  return chromium.launch({
    headless: true,
    timeout: 60000,
  });
}

/**
 * Extracts image URLs from HTML content
 */
export async function extractImageUrls(
  _rawHtml: string,
  page: Page,
  baseUrl: string,
  opts: ImageExtractOpts = {}
): Promise<string[]> {
  const {
    minWidth = 320,
    minHeight = 180,
    minArea = 100_000, // 320x312 or 400x250-ish
    limit,
  } = opts;

  // 1) Gather candidate URLs from <img> (prefer the largest srcset entry)
  const candidates = await page.$$eval("img", (imgs) => {
    const pickFromSrcset = (el: HTMLImageElement): string | null => {
      const srcset = el.getAttribute("srcset") || "";
      if (!srcset) return null;
      // srcset format: "url 300w, url2 600w" or "url 1x, url2 2x"
      const parts = srcset
        .split(",")
        .map((p) => p.trim())
        .map((p) => {
          const [u, d] = p.split(/\s+/);
          return { url: u, desc: d || "" };
        })
        .filter((x) => !!x.url);
      if (parts.length === 0) return null;

      // Prefer largest width (w) otherwise highest density (x)
      const withW = parts
        .map((p) => {
          const regex = /(\d+)w/i;
          const m = regex.exec(p.desc);
          return m ? { ...p, w: parseInt(m[1], 10) } : null;
        })
        .filter(Boolean) as Array<{ url: string; desc: string; w: number }>;

      if (withW.length) {
        withW.sort((a, b) => b.w - a.w);
        return withW[0].url;
      }

      const withX = parts
        .map((p) => {
          const regex = /([\d.]+)x/i;
          const m = regex.exec(p.desc);
          return m ? { ...p, x: parseFloat(m[1]) } : null;
        })
        .filter(Boolean) as Array<{ url: string; desc: string; x: number }>;

      if (withX.length) {
        withX.sort((a, b) => b.x - a.x);
        return withX[0].url;
      }

      // If descriptors are missing, just take the last one (often largest)
      return parts[parts.length - 1].url;
    };

    return imgs
      .map((img) => {
        // common lazy attributes
        const lazySrc =
          img.getAttribute("data-src") ||
          img.getAttribute("data-lazy-src") ||
          img.getAttribute("data-original") ||
          img.getAttribute("data-srcset") || // rare, but seen
          null;

        const best =
          pickFromSrcset(img) ||
          lazySrc ||
          img.getAttribute("src") ||
          "";

        const wAttr =
          parseInt(img.getAttribute("width") || "", 10) || undefined;
        const hAttr =
          parseInt(img.getAttribute("height") || "", 10) || undefined;

        const cls = (img.getAttribute("class") || "").toLowerCase();
        const alt = (img.getAttribute("alt") || "").toLowerCase();
        const id = (img.getAttribute("id") || "").toLowerCase();

        return { url: best, wAttr, hAttr, cls, alt, id };
      })
      .filter((x) => !!x.url);
  });

  // 2) Normalize → absolute URLs, dedupe, drop obvious junk/extensions
  const junkRe = /(sprite|icon|favicon|logo|avatar|emoji|placeholder|spacer|1x1|pixel)/i;
  const allowedExt = /\.(jpe?g|png|gif|webp)(?:[?#].*)?$/i;

  const normalized = Array.from(
    new Map(
      candidates
        .map((c) => {
          // resolve relative to base
          let abs = "";
          try {
            abs = new URL(c.url, baseUrl).toString();
          } catch {
            return null;
          }
          if (!allowedExt.test(abs)) return null;
          if (junkRe.test(abs) || junkRe.test(c.cls) || junkRe.test(c.alt) || junkRe.test(c.id)) {
            return null;
          }
          return [abs, { ...c, url: abs }] as const;
        })
        .filter(Boolean) as Array<readonly [string, typeof candidates[number]]>
    ).values()
  );

  // 3) Measure natural size in-page (loads each image to read naturalWidth/Height)
  //    We also time out quickly to avoid hanging on broken images.
  type Sized = { url: string; w: number; h: number; area: number };
  const sized: Sized[] = await page.evaluate(async (items: { url: string; wAttr?: number; hAttr?: number }[]) => {
    const loadOne = (u: string): Promise<{ url: string; w: number; h: number }> =>
      new Promise((resolve) => {
        const img = new Image();
        let done = false;
        const finish = (w = 0, h = 0) => {
          if (!done) {
            done = true;
            resolve({ url: u, w, h });
          }
        };
        img.onload = () => finish(img.naturalWidth || 0, img.naturalHeight || 0);
        img.onerror = () => finish(0, 0);
        // hard timeout in case onload/onerror never fire
        setTimeout(() => finish(0, 0), 2500);
        img.src = u;
      });

    const results = await Promise.all(items.map((i) => loadOne(i.url)));
    // Fold in width/height attributes if natural sizes are 0 but attrs exist
    return results.map((r, idx) => {
      const hintW = items[idx].wAttr || 0;
      const hintH = items[idx].hAttr || 0;
      const w = r.w || hintW || 0;
      const h = r.h || hintH || 0;
      return { url: r.url, w, h, area: w * h };
    });
  }, normalized);

  // 4) If nothing measured (lazy images, blocked, etc.), try og:image(+size)
  let finalList = sized.filter(
    (s) => s.w >= minWidth && s.h >= minHeight && s.area >= minArea
  );

  if (finalList.length === 0) {
    try {
      // Read OG tags safely (no $eval typing issues)
      const og = await page.evaluate(() => {
        const q = <T extends Element = HTMLMetaElement>(sel: string) =>
          document.querySelector<T>(sel);
  
        const url =
          q<HTMLMetaElement>('meta[property="og:image:secure_url"]')?.content ||
          q<HTMLMetaElement>('meta[property="og:image"]')?.content ||
          '';
  
        const wStr = q<HTMLMetaElement>('meta[property="og:image:width"]')?.content || '';
        const hStr = q<HTMLMetaElement>('meta[property="og:image:height"]')?.content || '';
        const w = Number.isFinite(parseInt(wStr, 10)) ? parseInt(wStr, 10) : 0;
        const h = Number.isFinite(parseInt(hStr, 10)) ? parseInt(hStr, 10) : 0;
  
        return { url, w, h };
      });
  
      if (og.url) {
        const abs = new URL(og.url, baseUrl).toString();
  
        // Keep your existing guards for file types / obvious junk;
        // but **no size filter** anymore.
        if (allowedExt.test(abs) && !junkRe.test(abs)) {
          finalList = [{ url: abs, w: og.w, h: og.h, area: (og.w || 0) * (og.h || 0) }];
        }
      }
    } catch {
      // ignore
    }
  }
  

  // 5) Sort by area (largest first) and return URLs
  finalList.sort((a, b) => b.area - a.area);
  const urls = finalList.map((x) => x.url);
  return typeof limit === "number" ? urls.slice(0, limit) : urls;
}

/**
 * Checks if the HTML looks like a Cloudflare challenge
 */
function looksLikeCloudflare(html: string, headers: Record<string, string | string[] | undefined>, status?: number) {
  const h = Object.fromEntries(Object.entries(headers || {}).map(([k, v]) => [k.toLowerCase(), Array.isArray(v) ? v.join(',') : v || '']));
  const server = h['server'] || '';
  const cfRay = h['cf-ray'] || '';
  const titleMatch = /<title>\s*(Just a moment|Attention Required|Please Wait)\s*<\/title>/i.test(html);
  const cfMarkers = /(cloudflare|cf-browser-verification|turnstile|challenge-platform)/i.test(html);

  // Many CF challenges respond 403/409/503 and include these markers
  if (titleMatch || cfMarkers || server.toLowerCase().includes('cloudflare') || cfRay) {
    if (status && [403, 409, 503].includes(status)) return true;
    // sometimes status is 200 but it’s a challenge HTML
    if (titleMatch || cfMarkers) return true;
  }
  return false;
}
