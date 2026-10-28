import { createServer } from 'node:http';
import { readFile, stat, watch } from 'node:fs/promises';
import { join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { build } from './build.mjs';

const development = process.argv.includes('--dev');
const port = Number(process.env.WEBSITE_PORT ?? '1889');
const website = fileURLToPath(new URL('.', import.meta.url));
const options = { basePath: process.env.SITE_BASE_PATH, origin: process.env.SITE_ORIGIN };
const result = development ? await build(options) : { outputDirectory: join(website, 'build'), basePath: (options.basePath ?? '').replace(/\/$/, '') };
const root = resolve(result.outputDirectory);
await stat(join(root, 'index.html')).catch(() => { throw new Error('Build the website first: pnpm website:build'); });
const contentTypes = { '.html': 'text/html; charset=utf-8', '.css': 'text/css; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.svg': 'image/svg+xml', '.png': 'image/png', '.xml': 'application/xml; charset=utf-8', '.txt': 'text/plain; charset=utf-8' };
const controller = new AbortController();
let rebuilding = Promise.resolve();

const server = createServer(async (request, response) => {
  try {
    if (!['GET', 'HEAD'].includes(request.method)) {
      response.writeHead(405, { Allow: 'GET, HEAD' }).end();
      return;
    }
    await rebuilding;
    let pathname;
    try { pathname = decodeURIComponent(new URL(request.url, 'http://localhost').pathname); }
    catch { response.writeHead(400).end(); return; }
    if (pathname.includes('\0') || pathname.includes('\\')) { response.writeHead(400).end(); return; }
    const inBase = !result.basePath || pathname.startsWith(`${result.basePath}/`);
    let file = inBase ? resolve(root, `.${pathname.slice(result.basePath.length)}`) : '';
    if (file && file !== root && !file.startsWith(root + sep)) { response.writeHead(403).end(); return; }
    let status = 200;
    if (file && (await stat(file).catch(() => null))?.isDirectory()) {
      if (!pathname.endsWith('/')) { response.writeHead(308, { Location: `${pathname}/` }).end(); return; }
      file = join(file, 'index.html');
    }
    if (!file || !(await stat(file).catch(() => null))?.isFile()) { file = join(root, '404.html'); status = 404; }
    const suffix = file.slice(file.lastIndexOf('.'));
    const body = await readFile(file);
    response.writeHead(status, { 'Content-Type': contentTypes[suffix] ?? 'application/octet-stream', 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' });
    response.end(request.method === 'HEAD' ? undefined : body);
  } catch (error) {
    console.error(error.message);
    response.writeHead(500).end();
  }
});

server.listen(port, '127.0.0.1', () => console.log(`Local: http://127.0.0.1:${port}${result.basePath}/`));

if (development) {
  const rebuild = () => {
    rebuilding = rebuilding.then(() => build(options)).then(() => console.log('Website rebuilt')).catch(error => console.error(error.message));
  };
  for (const relative of ['locales', 'assets']) {
    (async () => {
      for await (const event of watch(join(website, relative), { signal: controller.signal })) {
        if (event.filename) rebuild();
      }
    })().catch(error => { if (error.name !== 'AbortError') console.error(error.message); });
  }
  (async () => {
    for await (const event of watch(join(website, 'build.mjs'), { signal: controller.signal })) {
      if (event) console.log('Build template changed; restart pnpm website:dev to reload it.');
    }
  })().catch(error => { if (error.name !== 'AbortError') console.error(error.message); });
}

for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => { controller.abort(); server.close(); });
