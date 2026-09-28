const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const os = require('node:os');
const { chromium } = require('playwright');
const root = path.resolve(__dirname, '..');

function htmlFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const file = path.join(directory, entry.name);
    return entry.isDirectory() ? htmlFiles(file) : (file.endsWith('.html') ? [file] : []);
  });
}

test('login template loads local assets and exposes a working bilingual selector', async () => {
  const edge = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
  const browser = await chromium.launch({ headless: true, ...(fs.existsSync(edge) ? { executablePath: edge } : {}) });
  try {
    const page = await browser.newPage({ locale: 'zh-CN', viewport: { width: 1100, height: 800 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.route('http://i18n.test/**', async route => {
      const pathname = new URL(route.request().url()).pathname;
      if (pathname === '/') {
        const selector = fs.readFileSync(path.join(root, 'views/common/language.html'), 'utf8');
        const html = fs.readFileSync(path.join(root, 'views/login.html'), 'utf8')
          .replace('{{template "common/language.html" .}}', selector)
          .replace(/{{if \.error}}[\s\S]*?{{end}}/g, '')
          .replace(/{{[\s\S]*?}}/g, '');
        await route.fulfill({ contentType: 'text/html; charset=utf-8', body: html });
      } else if (pathname.startsWith('/static/')) {
        const file = path.join(root, pathname);
        if (fs.existsSync(file)) await route.fulfill({ path: file });
        else await route.fulfill({ status: 404, body: '' });
      } else await route.fulfill({ status: 404, body: '' });
    });
    await page.goto('http://i18n.test/');
    assert.equal(await page.title(), 'OpenVPN UI | 登录');
    assert.equal(await page.getByRole('button', { name: '登录', exact: true }).count(), 1);
    await page.getByPlaceholder('登录名', { exact: true }).fill('Alice');
    await page.getByRole('combobox', { name: '语言' }).selectOption('en');
    assert.equal(await page.title(), 'OpenVPN UI | Log in');
    assert.equal(await page.getByPlaceholder('Login', { exact: true }).inputValue(), 'Alice');
    await page.reload();
    assert.equal(await page.title(), 'OpenVPN UI | Log in');
    await page.getByRole('combobox', { name: 'Language' }).selectOption('zh-CN');
    const screenshot = path.join(os.tmpdir(), 'openvpn-ui-login-zh.png');
    await page.screenshot({ path: screenshot, fullPage: true });
    console.log('Login preview:', screenshot);
    assert.deepEqual(errors, []);
  } finally { await browser.close(); }
});

test('every marked template string has a translation and switching leaves all form values intact', async () => {
  const edge = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
  const browser = await chromium.launch({ headless: true, ...(fs.existsSync(edge) ? { executablePath: edge } : {}) });
  try {
    const page = await browser.newPage({ locale: 'en-US' });
    for (const file of htmlFiles(path.join(root, 'views'))) {
      // Template smoke test: remove Go actions, comments, and unrelated application JS.
      // No live VPN, authentication, or database behavior is simulated by this test.
      const source = fs.readFileSync(file, 'utf8');
      const html = source.replace(/<!--[\s\S]*?-->/g, '')
        .replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, '')
        .replace(/{{[\s\S]*?}}/g, '');
      await page.goto('about:blank');
      await page.setContent(html);
      await page.addScriptTag({ path: path.join(root, 'static/js/i18n-zh.js') });
      await page.addScriptTag({ path: path.join(root, 'static/js/i18n.js') });
      const inspect = () => Array.from(document.querySelectorAll('input,textarea,select')).filter(e => !e.matches('[data-language-selector]')).map(e => [e.name, e.value]);
      const before = await page.evaluate(inspect);
      const missing = await page.evaluate(() => {
        const missing = [];
        document.querySelectorAll('*').forEach(element => {
          ['data-i18n', 'data-i18n-title', 'data-i18n-placeholder', 'data-i18n-aria-label'].forEach(attribute => {
            if (element.hasAttribute(attribute) && !Object.prototype.hasOwnProperty.call(OpenVPNUIZh, element.getAttribute(attribute))) missing.push(element.getAttribute(attribute));
          });
        });
        return missing;
      });
      assert.deepEqual(missing, [], file);
      assert.equal(await page.locator('textarea [data-i18n], pre [data-i18n], code [data-i18n]').count(), 0, file);
      await page.evaluate(() => OpenVPNUILanguage.setLanguage('zh-CN'));
      assert.deepEqual(await page.evaluate(inspect), before, file);
      if (path.basename(file) === 'profile.html') {
        const update = page.locator('[type="submit"][title="更新用户"]');
        const label = await update.evaluate(element => element.tagName === 'INPUT' ? element.value : element.textContent);
        assert.equal(label.trim(), '更新');
      }
      const untranslated = await page.evaluate(() => Array.from(document.querySelectorAll('[data-i18n]')).filter(e => e.textContent !== OpenVPNUIZh[e.getAttribute('data-i18n')]).map(e => e.outerHTML));
      assert.deepEqual(untranslated, [], file);
      await page.evaluate(() => OpenVPNUILanguage.setLanguage('en'));
      assert.deepEqual(await page.evaluate(inspect), before, file);
    }
  } finally { await browser.close(); }
});

test('language switching preserves data and form values, survives navigation, and handles dynamic messages', async () => {
  const dictionary = fs.readFileSync(path.join(root, 'static/js/i18n-zh.js'), 'utf8');
  const runtime = fs.readFileSync(path.join(root, 'static/js/i18n.js'), 'utf8');
  const server = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'text/html; charset=utf-8');
    res.end(`<!doctype html><html><head><script>${dictionary}</script><script>${runtime}</script></head><body>
      <select data-language-selector aria-label="Language"><option value="en">English</option><option value="zh-CN">简体中文</option></select>
      <button id="save" data-i18n="Save Config">Save Config</button>
      <input id="name" value="Certificates" placeholder="Enter name" data-i18n-placeholder="Enter name">
      <select id="protocol"><option value="tcp" data-i18n="Full">Full</option></select>
      <p id="user">Certificates</p><pre id="log">Save Config</pre><textarea id="config">push "route 10.1.0.0 255.255.0.0"</textarea>
      <span id="unknown" data-i18n="Unknown upstream message">Unknown upstream message</span>
      <span id="message" data-i18n-message>Successfully logged in</span>
      <span id="theme" data-i18n="Light">Light</span>
      </body></html>`);
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  let browser;
  try {
    const edge = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
    browser = await chromium.launch({ headless: true, ...(fs.existsSync(edge) ? { executablePath: edge } : {}) });
    const context = await browser.newContext({ locale: 'zh-CN' });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const url = `http://127.0.0.1:${server.address().port}`;
    await page.goto(url);
    assert.equal(await page.locator('#save').textContent(), '保存配置');
    assert.equal(await page.locator('html').getAttribute('lang'), 'zh-CN');
    assert.equal(await page.locator('#message').textContent(), '登录成功');
    await page.locator('#name').fill('用户 Alice');
    await page.locator('[data-language-selector]').selectOption('en');
    assert.equal(await page.locator('#save').textContent(), 'Save Config');
    assert.equal(await page.locator('#name').inputValue(), '用户 Alice');
    assert.equal(await page.locator('#name').getAttribute('placeholder'), 'Enter name');
    await page.reload();
    assert.equal(await page.locator('#save').textContent(), 'Save Config');
    await page.locator('[data-language-selector]').selectOption('zh-CN');
    await page.evaluate(() => {
      OpenVPNUILanguage.setText(document.querySelector('#theme'), 'Dark');
      document.querySelector('#message').textContent = 'Success! Certificate for the name "Alice <admin>" has been created';
    });
    await page.waitForFunction(() => document.querySelector('#message').textContent.includes('已创建'));
    assert.equal(await page.locator('#theme').textContent(), '深色');
    assert.equal(await page.locator('#message').textContent(), '已创建证书“Alice <admin>”');
    assert.equal(await page.locator('#message').locator('*').count(), 0);
    assert.equal(await page.locator('#user').textContent(), 'Certificates');
    assert.equal(await page.locator('#log').textContent(), 'Save Config');
    assert.equal(await page.locator('#protocol').inputValue(), 'tcp');
    assert.equal(await page.locator('#config').inputValue(), 'push "route 10.1.0.0 255.255.0.0"');
    assert.equal(await page.locator('#unknown').textContent(), 'Unknown upstream message');
    await page.locator('[data-language-selector]').selectOption('en');
    assert.equal(await page.locator('#message').textContent(), 'Success! Certificate for the name "Alice <admin>" has been created');
    assert.equal(await page.locator('#theme').textContent(), 'Dark');
    assert.deepEqual(errors, []);
    await context.close();

    const blocked = await browser.newContext({ locale: 'fr-FR' });
    await blocked.addInitScript(() => {
      Object.defineProperty(window, 'localStorage', { get() { throw new Error('Storage blocked'); } });
    });
    const fallback = await blocked.newPage();
    await fallback.goto(url);
    assert.equal(await fallback.locator('#save').textContent(), 'Save Config');
    await fallback.locator('[data-language-selector]').selectOption('zh-CN');
    assert.equal(await fallback.locator('#save').textContent(), '保存配置');
    await blocked.close();
  } finally {
    if (browser) await browser.close();
    await new Promise(resolve => server.close(resolve));
  }
});
