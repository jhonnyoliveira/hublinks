#!/usr/bin/env node
// Verificações automáticas em navegador (Chromium headless) do passo 9 do quickstart:
// layout a 375 px sem rolagem horizontal, os dois temas, ordem de foco por Tab com indicador
// visível, acessibilidade/contraste (axe-core, WCAG 2.x A/AA), recursos de terceiros, erros de
// console e capturas de tela. Não substitui o olhar de uma pessoa sobre o visual.
//
// Pré-requisito (uma vez): sh scripts/browser-check/setup.sh
// Uso:
//   node scripts/browser-check/check.mjs --base http://localhost:8080 \
//     --path /admin/login --path /privacidade --path /p/fone-x1 --path /zzzzzzz=404 \
//     [--login admin@example.com:senha] [--out var/browser-check] \
//     [--viewports 375x667,1280x800] [--schemes light,dark] [--max-tabs 40]
// "caminho=status" declara o status esperado (padrão 200). Caminhos /admin exigem --login
// (ou BROWSER_CHECK_LOGIN). Sai com 1 se houver falha; avisos não falham.
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';

const argv = process.argv.slice(2);
const opts = { path: [], base: 'http://localhost:8080', out: 'var/browser-check', viewports: '375x667,1280x800', schemes: 'light,dark', 'max-tabs': '40', login: process.env.BROWSER_CHECK_LOGIN || '' };
for (let i = 0; i < argv.length; i++) {
  const m = /^--([a-z-]+)$/.exec(argv[i]);
  if (!m || argv[i + 1] === undefined) { console.error(`argumento inválido: ${argv[i]} (veja o cabeçalho do arquivo)`); process.exit(2); }
  if (m[1] === 'path') opts.path.push(argv[++i]); else if (m[1] === 'paths') opts.path.push(...argv[++i].split(',')); else opts[m[1]] = argv[++i];
}
if (opts.path.length === 0) { console.error('informe ao menos um --path'); process.exit(2); }

const toolsDir = process.env.BROWSER_TOOLS_DIR || join(homedir(), '.cache', 'hublinks-browser');
process.env.PLAYWRIGHT_BROWSERS_PATH ??= join(toolsDir, 'browsers');
let chromium, axeSource;
try {
  const req = createRequire(join(toolsDir, 'noop.js'));
  ({ chromium } = req('playwright'));
  axeSource = req('axe-core').source;
} catch {
  console.error(`Playwright não encontrado em ${toolsDir}. Rode: sh scripts/browser-check/setup.sh`);
  process.exit(2);
}

const base = new URL(opts.base);
const viewports = opts.viewports.split(',').map((v) => { const [w, h] = v.split('x').map(Number); return { width: w, height: h }; });
const schemes = opts.schemes.split(',');
const maxTabs = Number(opts['max-tabs']);
const slug = (p) => p.replace(/[^a-z0-9]+/gi, '_').replace(/^_|_$/g, '') || 'raiz';
mkdirSync(opts.out, { recursive: true });

const browser = await chromium.launch();
let storageState;
if (opts.login) {
  const [email, ...rest] = opts.login.split(':');
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  try {
    await page.goto(new URL('/admin/login', base).href);
  } catch (e) {
    console.error(`não foi possível abrir ${base.origin}/admin/login (o app está no ar?): ${String(e.message).split('\n')[0]}`);
    process.exit(2);
  }
  await page.fill('input[name=email]', email);
  await page.fill('input[name=password]', rest.join(':'));
  try {
    await Promise.all([page.waitForURL((u) => !u.pathname.endsWith('/admin/login'), { timeout: 10000 }), page.click('button[type=submit], button:not([type])')]);
  } catch {
    console.error('o login não saiu da tela /admin/login: confira o e-mail e a senha de --login');
    process.exit(2);
  }
  storageState = await ctx.storageState();
  await ctx.close();
}

const results = [];
for (const spec of opts.path) {
  const [path, expected = '200'] = spec.split('=');
  for (const vp of viewports) {
    for (const scheme of schemes) {
      const ctx = await browser.newContext({ viewport: vp, colorScheme: scheme, storageState });
      const page = await ctx.newPage();
      const r = { path, viewport: `${vp.width}x${vp.height}`, scheme, failures: [], warnings: [] };
      const external = new Set();
      page.on('request', (req) => { const u = new URL(req.url()); if (!['data:', 'blob:', 'about:'].includes(u.protocol) && u.origin !== base.origin) external.add(u.origin); });
      page.on('console', (m) => {
        if (m.type() !== 'error') return;
        // O navegador registra como erro toda resposta 4xx/5xx; se era o status esperado, não é falha.
        const http = /Failed to load resource: the server responded with a status of (\d+)/.exec(m.text());
        if (http && http[1] === expected) return;
        r.failures.push(`console.error: ${m.text().slice(0, 160)}`);
      });
      page.on('pageerror', (e) => r.failures.push(`erro de JavaScript: ${String(e).slice(0, 160)}`));
      let resp;
      try {
        resp = await page.goto(new URL(path, base).href, { waitUntil: 'load' });
      } catch (e) {
        r.status = 0;
        r.failures.push(`não foi possível abrir ${new URL(path, base).href} (o app está no ar em ${base.origin}?): ${String(e.message).split('\n')[0]}`);
        r.focusable = 0;
        results.push(r);
        await ctx.close();
        continue;
      }
      await page.waitForTimeout(300);
      r.status = resp ? resp.status() : 0;
      if (String(r.status) !== expected) r.failures.push(`status ${r.status}, esperado ${expected}`);
      if (external.size) r.failures.push(`recursos de terceiros: ${[...external].join(', ')}`);

      const dom = await page.evaluate(() => ({
        overflow: document.documentElement.scrollWidth - window.innerWidth,
        h1: document.querySelectorAll('h1').length,
        lang: document.documentElement.lang,
        theme: document.documentElement.className,
      }));
      if (dom.overflow > 0) r.failures.push(`rolagem horizontal de ${dom.overflow}px`);
      if (dom.h1 !== 1) r.warnings.push(`${dom.h1} elementos <h1> (esperado 1)`);
      if (!dom.lang) r.failures.push('<html> sem atributo lang');
      r.theme = dom.theme;

      await page.addScriptTag({ content: axeSource });
      const axe = await page.evaluate(() => axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] } }));
      for (const v of axe.violations) {
        const msg = `axe ${v.id} (${v.impact}, ${v.nodes.length} nó[s]): ${v.help}`;
        (v.impact === 'serious' || v.impact === 'critical' ? r.failures : r.warnings).push(msg);
      }

      const seen = new Set();
      const noIndicator = [];
      for (let i = 0; i < maxTabs; i++) {
        await page.keyboard.press('Tab');
        const f = await page.evaluate(() => {
          const el = document.activeElement;
          if (!el || el === document.body) return null;
          const cs = getComputedStyle(el);
          const label = (el.getAttribute('aria-label') || el.innerText || el.getAttribute('name') || el.getAttribute('href') || '').trim().slice(0, 40);
          const key = el.tagName + '|' + label + '|' + Array.from(el.parentElement ? el.parentElement.children : []).indexOf(el);
          const visible = (cs.outlineStyle !== 'none' && parseFloat(cs.outlineWidth) > 0) || cs.boxShadow !== 'none';
          return { key, tag: el.tagName.toLowerCase(), label, visible };
        });
        if (!f || seen.has(f.key)) break;
        seen.add(f.key);
        if (!f.visible) noIndicator.push(`${f.tag} "${f.label}"`);
      }
      r.focusable = seen.size;
      if (seen.size === 0) r.warnings.push('nenhum elemento focável por Tab');
      if (noIndicator.length) r.warnings.push(`sem indicador de foco visível: ${noIndicator.slice(0, 5).join('; ')}${noIndicator.length > 5 ? '…' : ''}`);

      r.screenshot = join(opts.out, `${slug(path)}_${scheme}_${r.viewport}.png`);
      await page.screenshot({ path: r.screenshot, fullPage: true });
      results.push(r);
      await ctx.close();
    }
  }
}
await browser.close();

writeFileSync(join(opts.out, 'report.json'), JSON.stringify(results, null, 2));
let failed = 0;
for (const r of results) {
  const flag = r.failures.length ? 'FALHA' : r.warnings.length ? 'aviso' : 'ok   ';
  console.log(`${flag} ${r.path} ${r.viewport} ${r.scheme} [status ${r.status}, ${r.focusable} focáveis]`);
  r.failures.forEach((m) => console.log(`      falha: ${m}`));
  r.warnings.forEach((m) => console.log(`      aviso: ${m}`));
  if (r.failures.length) failed++;
}
console.log(`\n${results.length} combinações, ${failed} com falha. Relatório e capturas em ${opts.out}/`);
process.exit(failed ? 1 : 0);
