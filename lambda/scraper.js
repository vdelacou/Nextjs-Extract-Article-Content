// scraper.js
const { request } = require('undici');
const sanitizeHtml = require('sanitize-html');
const chromium = require('@sparticuz/chromium');
const puppeteer = require('puppeteer-core');

let extractFromHtml;
async function getExtractor() {
  if (!extractFromHtml) {
    ({ extractFromHtml } = await import('@extractus/article-extractor'));
  }
  return extractFromHtml;
}

const UA =
  process.env.SCRAPE_USER_AGENT ||
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36';

function sanitize(t) {
  return sanitizeHtml(t ?? '', { allowedTags: [], allowedAttributes: {} }).trim();
}

function extractImagesFromHtml(html, baseUrl) {
  const urls = new Set();
  const og = html.match(/<meta[^>]+property=["']og:image(?::secure_url)?["'][^>]+content=["']([^"']+)["']/i);
  if (og?.[1]) {
    try { urls.add(new URL(og[1], baseUrl).toString()); } catch {}
  }
  const re = /<img[^>]+(?:src|data-src|data-lazy-src)=["']([^"']+)["'][^>]*>/gi;
  let m;
  while ((m = re.exec(html))) {
    try {
      const u = new URL(m[1], baseUrl).toString();
      if (/\.(jpe?g|png|gif|webp)(?:$|[?#])/.test(u) && !/(sprite|icon|favicon|logo|avatar|emoji|placeholder)/i.test(u)) {
        urls.add(u);
      }
    } catch {}
  }
  return Array.from(urls).slice(0, 3);
}

async function fetchHtml(url, timeoutMs = 15000, sizeLimitBytes = 6_000_000) {
  const { statusCode, headers, body } = await request(url, {
    method: 'GET',
    maxRedirections: 5,
    headers: {
      'user-agent': UA,
      'accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
      'accept-language': 'en-US,en;q=0.9',
      'cache-control': 'no-cache',
    },
    headersTimeout: timeoutMs,
    bodyTimeout: timeoutMs,
  });

  if (statusCode >= 400) throw new Error(`HTTP ${statusCode}`);
  const ct = (headers['content-type'] || '').toLowerCase();
  if (!ct.includes('text/html')) throw new Error(`Non-HTML content-type: ${ct}`);

  // Stream with size cap
  const chunks = [];
  let total = 0;
  for await (const chunk of body) {
    total += chunk.length;
    if (total > sizeLimitBytes) throw new Error('HTML too large');
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString('utf8');
}

async function extractArticle(html, baseUrl) {
  const extractor = await getExtractor();
  const article = await extractor(html, baseUrl);
  return {
    title: article?.title ? sanitize(article.title) : undefined,
    description: article?.description ? sanitize(article.description) : undefined,
    content: article?.content ? sanitize(article.content) : undefined,
    images: extractImagesFromHtml(html, baseUrl),
  };
}

async function scrapeWithPuppeteer(url, navBudgetMs = 20000) {
  let browser;
  try {
    browser = await puppeteer.launch({
      executablePath: await chromium.executablePath(),
      headless: chromium.headless,
      args: [
        ...chromium.args,
        '--no-sandbox',
        '--disable-dev-shm-usage',
        '--disable-gpu',
        // Avoid funky flags like disable-web-security / site-per-process
      ],
      defaultViewport: { width: 1200, height: 900 },
      ignoreHTTPSErrors: true,
      timeout: 30000,
    });

    const page = await browser.newPage();
    await page.setUserAgent(UA);
    await page.setExtraHTTPHeaders({ 'accept-language': 'en-US,en;q=0.9' });
    page.setDefaultNavigationTimeout(navBudgetMs);
    page.setDefaultTimeout(navBudgetMs);

    // Interception: NEVER touch the main document
    await page.setRequestInterception(true);
    page.on('request', (req) => {
      const type = req.resourceType();
      if (type === 'document') return req.continue();
      if (type === 'image' || type === 'media' || type === 'font') return req.abort();
      if (type === 'stylesheet') return req.abort();
      const u = req.url();
      if (/\b(doubleclick|googlesyndication|google-analytics|facebook\.com\/tr|taboola|outbrain|scorecardresearch|chartbeat|amazon-adsystem)\b/i.test(u)) {
        return req.abort();
      }
      return req.continue();
    });

    // Keep JS ON so CDN/interstitials settle
    await page.setJavaScriptEnabled(true);

    const resp = await page.goto(url, { waitUntil: 'networkidle2', timeout: navBudgetMs });
    if (!resp) throw new Error('No response from goto');
    const status = resp.status();
    if (status >= 400) throw new Error(`HTTP ${status}`);

    // If a site keeps churning, give it a tiny grace period but keep it bounded
    await page.waitForTimeout(500);

    const html = await page.content();
    const out = await extractArticle(html, resp.url());
    return out;
  } finally {
    try { if (browser) await browser.close(); } catch {}
  }
}

async function scrapeSmart(url) {
  // Phase 1: Fast path (HTML fetch) — 12–18s budget
  try {
    const html = await fetchHtml(url, 18000);
    return await extractArticle(html, url);
  } catch (e) {
    // Only fall back for “fetchable but blocked/JS needed” cases
    if (
      /Non-HTML|HTML too large|HTTP 4\d\d|ECONNRESET|ECONNREFUSED|ENOTFOUND|EAI_AGAIN|socket hang up/i.test(
        String(e.message)
      )
    ) {
      // These are hard failures—still try browser once
    }
    // proceed to puppeteer
  }

  // Phase 2: Puppeteer fallback — keep it short (e.g., 18–22s)
  return await scrapeWithPuppeteer(url, 20000);
}

module.exports = { scrapeSmart };
