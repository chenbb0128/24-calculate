/* Read-only production endpoint probe. It never sends credentials or game data. */
const http = require('http');
const https = require('https');

const baseURL = String(process.argv[2] || 'https://calc-api.pdurl.cn').replace(/\/$/, '');

function request(url, options = {}) {
  return new Promise((resolve, reject) => {
    const target = new URL(url);
    const transport = target.protocol === 'https:' ? https : http;
    const req = transport.request(target, {
      method: options.method || 'GET',
      headers: options.headers || {},
      timeout: 15000,
    }, (res) => {
      const chunks = [];
      res.on('data', (chunk) => chunks.push(chunk));
      res.on('end', () => resolve({
        status: res.statusCode || 0,
        headers: res.headers,
        body: Buffer.concat(chunks).toString('utf8'),
      }));
    });
    req.on('timeout', () => req.destroy(new Error('request timeout')));
    req.on('error', reject);
    if (options.body) req.write(options.body);
    req.end();
  });
}

function bodyJSON(response) {
  try { return JSON.parse(response.body); } catch (error) { return null; }
}

function check(condition, message) {
  if (!condition) throw new Error(message);
}

async function main() {
  const health = await request(`${baseURL}/health`);
  const healthJSON = bodyJSON(health);
  check(health.status === 200 && healthJSON && healthJSON.code === 0 && healthJSON.data.status === 'ok', 'health probe failed');

  const ready = await request(`${baseURL}/ready`);
  const readyJSON = bodyJSON(ready);
  check(ready.status === 200 && readyJSON && readyJSON.code === 0 && readyJSON.data.status === 'ready', 'ready probe failed');

  const httpHealth = await request(`${baseURL.replace(/^https:/i, 'http:')}/health`);
  check([301, 302, 307, 308].includes(httpHealth.status), 'HTTP endpoint does not redirect');
  check(/^https:\/\//i.test(String(httpHealth.headers.location || '')), 'HTTP redirect target is not HTTPS');

  const devLogin = await request(`${baseURL}/api/v1/auth/dev-login`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: '{}',
  });
  check(devLogin.status === 404, 'dev-login is exposed in production');

  const legacySettlementRoutes = [
    '/api/v1/player/levels/1/complete',
    '/api/v1/player/daily/complete',
    '/api/v1/player/leaderboards/overall/submit',
  ];
  for (const route of legacySettlementRoutes) {
    const response = await request(`${baseURL}${route}`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: '{}',
    });
    check(response.status === 404, `legacy client settlement route is exposed: ${route} (HTTP ${response.status})`);
  }

  const swagger = await request(`${baseURL}/swagger/index.html`);
  check(swagger.status === 404, 'Swagger is exposed in production');

  console.log(`PRODUCTION_PROBE_OK base=${baseURL}`);
}

main().catch((error) => {
  console.error(`PRODUCTION_PROBE_FAILED ${error.message}`);
  process.exitCode = 1;
});
