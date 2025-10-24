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

// Match your @sparticuz/chromium major (package.json uses ^133)
const CHROME_MAJOR = 133;

const UA =
  process.env.SCRAPE_USER_AGENT ||
  `Mozilla/5.0 (Windows NT 10; Win64; x64) AppleWebKit(KHTML, like Gecko) Chrome/${CHROME_MAJOR}.0.6943.126 Safari/537.36`.replace('AppleWebKit(', 'AppleWebKit/537.36 (');

// --------- Helpers ----------
function sanitize(t) {
  return sanitizeHtml(t ?? '', { allowedTags: [], allowedAttributes: {} }).trim();
}

// Image extraction constants and utilities
const IMAGE_CONFIG = {
  MIN_SHORT_SIDE: 300,
  MIN_AREA: 140000,
  MIN_ASPECT: 0.5,
  MAX_ASPECT: 2.6,
  RATIO_WHITELIST: [1.333, 1.5, 1.6, 1.667, 1.777, 1.85, 2],
  RATIO_TOL: 0.09,
  AD_SIZES: new Set([
    '728x90','970x90','970x250','468x60','320x50','300x50',
    '300x250','336x280','300x600','160x600','120x600',
    '250x250','200x200','180x150','234x60','120x240','88x31'
  ]),
  BAD_HINT: /(sprite|icon|favicon|logo|avatar|emoji|placeholder|pixel|tracker|ads?|adserver|promo|beacon)/i
};

function parseDimsFromTag(tag) {
  const num = (s) => s ? Number.parseInt(String(s).replaceAll(/[^\d]/g, ''), 10) : 0;
  const wAttr = /(?:^|\s)width=["']?(\d+)[^"'>]*/i.exec(tag)?.[1];
  const hAttr = /(?:^|\s)height=["']?(\d+)[^"'>]*/i.exec(tag)?.[1];
  const style = /style=["']([^"']+)["']/i.exec(tag)?.[1] || '';
  const wStyle = /(?:^|;|\s)width\s*:\s*(\d+(?:\.\d+)?)px\b/i.exec(style)?.[1];
  const hStyle = /(?:^|;|\s)height\s*:\s*(\d+(?:\.\d+)?)px\b/i.exec(style)?.[1];
  return { width: num(wAttr || wStyle), height: num(hAttr || hStyle) };
}

function parseDimsFromUrl(url) {
  const m1 = /(?:^|[^\d])(\d{3,4})x(\d{3,4})(?:[^\d]|$)/i.exec(url);
  const m2w = /[?&](?:w|width)=(\d{3,4})\b/i.exec(url);
  const m2h = /[?&](?:h|height)=(\d{3,4})\b/i.exec(url);
  if (m1) return { width: Number.parseInt(m1[1], 10), height: Number.parseInt(m1[2], 10) };
  if (m2w && m2h) return { width: Number.parseInt(m2w[1], 10), height: Number.parseInt(m2h[1], 10) };
  return { width: 0, height: 0 };
}

function pickFromSrcset(srcset) {
  if (!srcset) return null;
  const items = srcset.split(',').map(s => s.trim()).map(s => {
    const m = /(\S+)\s+(\d+)w/.exec(s);
    return m ? { url: m[1], w: Number.parseInt(m[2], 10) } : null;
  }).filter(Boolean);
  if (!items.length) return null;
  items.sort((a,b) => Math.abs(a.w - 1000) - Math.abs(b.w - 1000) || b.w - a.w);
  return items[0].url;
}

function inArticleScope(fullHtml, tagIndex) {
  const before = fullHtml.slice(0, tagIndex);
  const openIdx = Math.max(before.search(/<article[\s>]/i), before.search(/<main[\s>]/i));
  if (openIdx < 0) return false;
  const afterOpen = fullHtml.slice(openIdx, tagIndex);
  return !/<\/(article|main)>/i.test(afterOpen);
}

function goodAspect(w, h) {
  if (!w || !h) return false;
  const a = w / h;
  if (a >= IMAGE_CONFIG.MIN_ASPECT && a <= IMAGE_CONFIG.MAX_ASPECT) return true;
  return IMAGE_CONFIG.RATIO_WHITELIST.some(r => Math.abs(a - r) <= IMAGE_CONFIG.RATIO_TOL);
}

function isAdSize(w, h) {
  return (w && h) && IMAGE_CONFIG.AD_SIZES.has(`${w}x${h}`);
}

function extractOgImage(html, baseUrl) {
  const toAbs = (u) => { try { return new URL(u, baseUrl).toString(); } catch { return null; } };
  const og = /<meta[^>]*property=["']og:image(?::secure_url)?["'][^>]*content=["']([^"']+)["']/i.exec(html)?.[1];
  
  if (!og) return null;
  
  const url = toAbs(og);
  if (!url || !/\.(jpe?g|png|gif|webp|avif)(?:$|[?#])/i.test(url)) return null;

  const ogW = /<meta[^>]*property=["']og:image:width["'][^>]*content=["']([^"']+)["']/i.exec(html)?.[1];
  const ogH = /<meta[^>]*property=["']og:image:height["'][^>]*content=["']([^"']+)["']/i.exec(html)?.[1];
  let width = Number.parseInt(ogW || '0', 10), height = Number.parseInt(ogH || '0', 10);
  
  if (!width || !height) {
    const fromUrl = parseDimsFromUrl(url);
    width = width || fromUrl.width;
    height = height || fromUrl.height;
  }
  
  return { url, width, height, inArticle: true, source: 'og' };
}

function extractImgTags(html, baseUrl) {
  const toAbs = (u) => { try { return new URL(u, baseUrl).toString(); } catch { return null; } };
  const candidates = [];
  const imgRe = /<img\b[^>]*>/gi;
  let m;
  
  while ((m = imgRe.exec(html))) {
    const tag = m[0];
    const idx = m.index;

    let raw =
      /(?:\s|^)(?:src|data-src|data-original|data-lazy-src)=["']([^"']+)["']/i.exec(tag)?.[1] ||
      pickFromSrcset(/srcset=["']([^"']+)["']/i.exec(tag)?.[1]);
    if (!raw) continue;

    const abs = toAbs(raw);
    if (!abs) continue;
    if (!/\.(jpe?g|png|gif|webp|avif)(?:$|[?#])/i.test(abs)) continue;

    let { width, height } = parseDimsFromTag(tag);
    if (!width || !height) {
      const fromUrl = parseDimsFromUrl(abs);
      width = width || fromUrl.width;
      height = height || fromUrl.height;
    }

    const inArticle = inArticleScope(html, idx);
    const badHint = IMAGE_CONFIG.BAD_HINT.test(tag) || IMAGE_CONFIG.BAD_HINT.test(abs);

    candidates.push({ url: abs, width, height, inArticle, badHint, source: 'img' });
  }
  
  return candidates;
}

function filterAndScoreCandidates(candidates) {
  return candidates.filter(c => {
    if (c.width && c.height) {
      const shortSide = Math.min(c.width, c.height);
      const area = c.width * c.height;

      if (shortSide < IMAGE_CONFIG.MIN_SHORT_SIDE) return false;
      if (area < IMAGE_CONFIG.MIN_AREA) return false;
      if (!goodAspect(c.width, c.height)) return false;
      if (isAdSize(c.width, c.height)) return false;

      if (c.badHint && !(shortSide >= 400 && area >= 300000)) return false;
    } else if (c.badHint) {
      return false;
    }
    return true;
  }).map(c => {
    const area = (c.width && c.height) ? (c.width * c.height) : 0;
    const aspect = (c.width && c.height) ? (c.width / c.height) : 0;
    const ratioBonus = IMAGE_CONFIG.RATIO_WHITELIST.some(r => Math.abs(aspect - r) <= IMAGE_CONFIG.RATIO_TOL) ? 1 : 0;
    const articleBoost = c.inArticle ? 2 : 0;
    const ogBoost = (c.source === 'og') ? 1 : 0;
    const score = articleBoost + ogBoost + ratioBonus + Math.log10(Math.max(1, area));
    return { ...c, score, area };
  });
}

function extractImagesFromHtml(html, baseUrl) {
  const candidates = [];
  
  // Extract og:image first
  const ogImage = extractOgImage(html, baseUrl);
  if (ogImage) candidates.push(ogImage);
  
  // Extract img tags
  candidates.push(...extractImgTags(html, baseUrl));
  
  // Filter and score candidates
  const filtered = filterAndScoreCandidates(candidates);
  filtered.sort((a, b) => b.score - a.score || b.area - a.area);

  // Return top 3 unique URLs
  const seen = new Set();
  const out = [];
  for (const c of filtered) {
    if (!seen.has(c.url)) { 
      seen.add(c.url); 
      out.push(c.url); 
    }
    if (out.length === 3) break;
  }
  return out;
}


function looksLikeCfBlock(html, title = '') {
  const t = (title || '').toLowerCase();
  const h = (html || '').toLowerCase();
  return (
    t.includes('attention required') ||
    h.includes('cloudflare ray id') ||
    h.includes('what can i do to resolve this?') ||
    h.includes('why have i been blocked?') ||
    h.includes('performance & security by cloudflare')
  );
}

function altUrls(url) {
  const u = new URL(url);
  const out = new Set();

  // AMP prefix (/amp/path)
  if (!u.pathname.startsWith('/amp/')) out.add(new URL(`/amp${u.pathname}`, u.origin).toString());
  // AMP suffix (/path/amp)
  if (!u.pathname.endsWith('/amp')) out.add(new URL(`${u.pathname.replace(/\/$/, '')}/amp`, u.origin).toString());
  // Query AMP
  const q = new URL(url);
  q.searchParams.set('outputType', 'amp');
  out.add(q.toString());
  // m. subdomain
  if (!u.hostname.startsWith('m.')) {
    const m = new URL(url);
    m.hostname = `m.${u.hostname}`;
    out.add(m.toString());
  }
  return Array.from(out);
}
// ----------------------------

async function fetchHtml(url, timeoutMs = 15000, sizeLimitBytes = 6_000_000, retryCount = 0) {
  const { statusCode, headers, body } = await request(url, {
    method: 'GET',
    maxRedirections: 5,
    headers: {
      'user-agent': UA,
      'accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
      'accept-language': 'en-US,en;q=0.9',
      'cache-control': 'no-cache',
      'upgrade-insecure-requests': '1',
      'referer': 'https://www.google.com/'
    },
    headersTimeout: timeoutMs,
    bodyTimeout: timeoutMs,
  });

  // Handle 5xx server errors with retry logic
  if (statusCode >= 500) {
    const maxRetries = 2;
    if (retryCount < maxRetries) {
      const delay = Math.min(1000 * Math.pow(2, retryCount), 5000); // Exponential backoff, max 5s
      console.warn(`Server error ${statusCode} for ${url}, retrying in ${delay}ms (attempt ${retryCount + 1}/${maxRetries + 1})`);
      await new Promise(resolve => setTimeout(resolve, delay));
      return fetchHtml(url, timeoutMs, sizeLimitBytes, retryCount + 1);
    }
    throw new Error(`HTTP ${statusCode} (after ${maxRetries} retries)`);
  }

  if (statusCode >= 400) throw new Error(`HTTP ${statusCode}`);
  const ct = (headers['content-type'] || '').toLowerCase();
  if (!ct.includes('text/html')) throw new Error(`Non-HTML content-type: ${ct}`);

  const chunks = [];
  let total = 0;
  for await (const chunk of body) {
    total += chunk.length;
    if (total > sizeLimitBytes) throw new Error('HTML too large');
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString('utf8');
}

async function fetchWithAlts(url, timeoutMs) {
  // Primary
  try {
    const html = await fetchHtml(url, timeoutMs);
    if (looksLikeCfBlock(html, '')) throw new Error('CF_BLOCK_PRIMARY');
    return { html, finalUrl: url };
  } catch (e) {
    // Include 5xx errors in the fallback logic - they should try alternates
    if (!/HTTP (403|406|451|5\d{2})|Non-HTML|CF_BLOCK_PRIMARY/i.test(String(e.message))) throw e;
  }

  // Alternates
  for (const candidate of altUrls(url)) {
    try {
      const html = await fetchHtml(candidate, timeoutMs);
      if (looksLikeCfBlock(html, '')) continue; // still blocked
      return { html, finalUrl: candidate };
    } catch { /* try the next alt */ }
  }
  throw new Error('CF_BLOCKED_FETCH');
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
      ],
      defaultViewport: { width: 1366, height: 900, deviceScaleFactor: 1 },
      ignoreHTTPSErrors: true,
      timeout: 30000,
    });

    const page = await browser.newPage();

    // Realistic environment
    await page.emulateTimezone('America/Los_Angeles');
    await page.setUserAgent(UA, {
      brands: [
        { brand: 'Chromium', version: `${CHROME_MAJOR}` },
        { brand: 'Not)A;Brand', version: '99' }
      ],
      mobile: false,
      platform: 'Windows',
      architecture: 'x86'
    });
    await page.setExtraHTTPHeaders({
      'accept-language': 'en-US,en;q=0.9',
      'sec-ch-ua': `"Chromium";v="${CHROME_MAJOR}", "Not)A;Brand";v="99"`,
      'sec-ch-ua-mobile': '?0',
      'sec-ch-ua-platform': '"Windows"',
      'upgrade-insecure-requests': '1',
      'accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8',
      'referer': 'https://www.google.com/'
    });
    // Hide webdriver flag
    await page.evaluateOnNewDocument(() => {
      Object.defineProperty(navigator, 'webdriver', { get: () => false });
    });

    await page.setJavaScriptEnabled(true);

    // IMPORTANT: set up interception before navigating, but never block main "document"
    await page.setRequestInterception(true);
    page.on('request', (req) => {
      const type = req.resourceType();
      if (type === 'document') return req.continue();
      if (type === 'image' || type === 'media' || type === 'font' || type === 'stylesheet') return req.abort();
      const u = req.url();
      if (/\b(doubleclick|googlesyndication|google-analytics|facebook\.com\/tr|taboola|outbrain|scorecardresearch|chartbeat|amazon-adsystem)\b/i.test(u)) {
        return req.abort();
      }
      return req.continue();
    });

    // Safe content getter (avoids title() and guards context races)
    const safeContent = async () => {
      try { return await page.content(); } catch { return ''; }
    };

    let resp = await page.goto(url, { waitUntil: 'networkidle2', timeout: navBudgetMs });
    if (!resp) throw new Error('No response from goto');

    let html = await safeContent();

    const isBlockedNow = () =>
      resp.status() === 403 || looksLikeCfBlock(html /* no title needed */);

    // If blocked, try AMP/mobile quickly inside the same session
    if (isBlockedNow()) {
      for (const candidate of altUrls(url)) {
        try {
          resp = await page.goto(candidate, { waitUntil: 'domcontentloaded', timeout: navBudgetMs });
          html = await safeContent();
          if (resp && resp.status() < 400 && !looksLikeCfBlock(html)) break;
        } catch { /* try next alt */ }
      }
    }

    // Still blocked? surface a structured error
    if (looksLikeCfBlock(html) || resp?.status?.() === 403) {
      const finalUrl = page.url();
      const err = new Error('CF_BLOCKED_BROWSER');
      err.data = { finalUrl };
      throw err;
    }

    return await extractArticle(html, page.url());
  } finally {
    try { if (browser) await browser.close(); } catch {}
  }
}


async function scrapeSmart(url) {
  // Phase 1: fetch-first with alternates (AMP/mobile)
  try {
    const { html, finalUrl } = await fetchWithAlts(url, 18000);
    return await extractArticle(html, finalUrl);
  } catch (e) {
    console.warn('[fetch phase] fallback to browser:', e.message);
  }

  // Phase 2: short-budget browser fallback
  try {
    return await scrapeWithPuppeteer(url, 40000);
  } catch (e) {
    if (/CF_BLOCKED_BROWSER|CF_BLOCKED_FETCH/i.test(String(e.message))) {
      const domain = new URL(url).hostname;
      return {
        title: undefined,
        description: undefined,
        content: undefined,
        images: [],
        blocked: true,
        provider: 'cloudflare',
        domain
      };
    }
    throw e;
  }
}

module.exports = { scrapeSmart };
