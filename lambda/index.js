// index.js
// AWS Lambda Handler for Web Scraping
'use strict';

const scraper = require('./scraper');

exports.handler = async (event, context) => {
  // Return immediately once we resolve; don't wait for the event loop to drain
  context.callbackWaitsForEmptyEventLoop = false;

  // Basic CORS (add OPTIONS support if behind API Gateway HTTP API)
  const baseHeaders = {
    'Content-Type': 'application/json; charset=utf-8',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'Content-Type,X-Api-Key,x-api-key',
    'Access-Control-Allow-Methods': 'GET,OPTIONS'
  };

  // Preflight
  if (event.requestContext?.http?.method === 'OPTIONS' || event.httpMethod === 'OPTIONS') {
    return { statusCode: 204, headers: baseHeaders, body: '' };
  }

  console.log('Request received:', JSON.stringify(event, null, 2));

  try {
    // --- API key check
    const apiKey =
      event.headers?.['x-api-key'] ||
      event.headers?.['X-Api-Key'] ||
      event.queryStringParameters?.key;

    const validKey = process.env.SCRAPE_API_KEY;

    if (!validKey) {
      console.error('SCRAPE_API_KEY environment variable not set');
      return { statusCode: 500, headers: baseHeaders, body: JSON.stringify({ error: 'Server misconfiguration' }) };
    }

    if (!apiKey || apiKey !== validKey) {
      return { statusCode: 401, headers: baseHeaders, body: JSON.stringify({ error: 'Invalid or missing API key' }) };
    }

    // --- URL validation
    const url = event.queryStringParameters?.url;
    if (!url) {
      return { statusCode: 400, headers: baseHeaders, body: JSON.stringify({ error: 'Missing "url" query parameter' }) };
    }
    try { new URL(url); } catch {
      return { statusCode: 400, headers: baseHeaders, body: JSON.stringify({ error: 'Invalid URL format' }) };
    }

    console.info(`Starting scrape for: ${url}`);

    // --- Soft-timeout: always end BEFORE Lambda would
    // Use remaining time – 3s safety margin; cap to 70s to be safe.
    const remaining = typeof context.getRemainingTimeInMillis === 'function'
      ? context.getRemainingTimeInMillis()
      : 90000;
    const SOFT_TIMEOUT_MS = Math.max(1000, Math.min(70000, remaining - 3000));

    const watchdog = new Promise((_, reject) =>
      setTimeout(() => reject(new Error('Scrape timeout')), SOFT_TIMEOUT_MS)
    );

    const start = Date.now();

    // IMPORTANT: call the function you actually export: scrapeSmart
    const result = await Promise.race([
      scraper.scrapeSmart(url),
      watchdog
    ]);

    const duration = Date.now() - start;
    console.info(`✓ Scraped in ${duration}ms`);

    return {
      statusCode: 200,
      headers: baseHeaders,
      body: JSON.stringify({
        ...result,
        metadata: { url, scrapedAt: new Date().toISOString(), durationMs: duration }
      })
    };
  } catch (error) {
    console.error('Error processing request:', error);
    const statusCode = error.message === 'Scrape timeout' ? 504 : 500;
    return {
      statusCode,
      headers: baseHeaders,
      body: JSON.stringify({
        error: error.message === 'Scrape timeout' ? 'Scrape took too long' : 'Failed to scrape',
        details: String(error.message || error)
      })
    };
  }
};
