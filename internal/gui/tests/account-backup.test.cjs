// Isolated provider editor and Routing fixtures, served with the real assets.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");
const assets = path.resolve(__dirname, "../assets");
const words = {
  en: ["Automatic", "Backup", "Primary", "Use accounts without a 5-hour limit as backup", "kept as backup", "is kept as a backup while primary accounts have quota to spare."],
  zh: ["自动", "备用", "主用", "将没有 5 小时限额的账号用作备用", "保留为备用", "主用账号还有可用额度时"],
  "zh-TW": ["自動", "備用", "主要", "將沒有 5 小時限額的帳號用作備用", "保留為備用", "主要帳號還有可用額度時"],
  ja: ["自動", "バックアップ", "優先", "5時間制限のないアカウントをバックアップにする", "バックアップとして待機", "優先アカウントに利用枠の余裕がある間"],
  de: ["Automatisch", "Reserve", "Primär", "Konten ohne 5-Stunden-Limit als Reserve verwenden", "als Reserve zurückgehalten", "solange primäre Konten noch Kontingent übrig haben."],
};

function fixture(lang, posts, configure = () => {}) {
  const p = {
    id: "codex", name: "Codex", icon: "codex-color", models: [{ id: "gpt-6", on: true }],
    agents: [], fallback: [], headers: {}, keyList: [], proxy: "", backupNoFiveHour: true,
    account: { agent: "codex", agentName: "Codex", user: "standard@test.invalid", plan: "PLUS", logins: [
      { user: "standard@test.invalid", active: true, on: true },
      { user: "Premium@test.invalid", on: true },
      { user: "unknown@test.invalid", on: true },
    ] },
  };
  configure(p);
  const providers = { providers: [p], presets: [], excluded: [], gateway: { running: true, window: true } };
  const now = new Date();
  const due = (days) => new Date(+now + days * 864e5).toISOString();
  const standard = { id: "codex", shared: true, agent: "codex", provider: "codex", name: "Codex", who: "standard@test.invalid", kind: "account", model: "gpt-6", known: true, used: 25, soon: [due(5)], routing: "" };
  const premium = { ...standard, id: "codex@premium@test.invalid", who: "Premium@test.invalid", used: 0, soon: [due(3)], backup: true };
  const makeRoute = (id, order) => ({ id, seq: id, time: now.toISOString(), agent: "codex", model: "gpt-6", provider: "codex", order,
    tries: [{ id: order[0].id, model: "gpt-6", start: now.toISOString(), done: true, status: 200, ms: 400 }], done: true, status: 200, ms: 400 });
  const routes = [makeRoute(3, [standard, premium]), makeRoute(2, [{ ...premium, backup: false, used: 70 }, standard]), makeRoute(1, [premium, { ...standard, used: 92, rateCap: true }])];
  const day = [now.getFullYear(), now.getMonth() + 1, now.getDate()].map((n) => String(n).padStart(2, "0")).join("-");
  let delayNextUsage = false;
  const serve = async (route) => {
    const u = new URL(route.request().url()), json = (data) => route.fulfill({ json: data });
    if (u.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs=${JSON.stringify({ lang, theme: "light", web: true })};` });
    if (u.pathname === "/api/state") return json({ agents: [], profiles: [], settings: { lang, theme: "light" } });
    if (u.pathname === "/api/providers") return json(providers);
    if (u.pathname === "/api/groups") return json({ groups: [] });
    if (u.pathname === "/api/plugins") return json({ plugins: [] });
    if (u.pathname === "/api/login/usage") {
      // Force the refresh/navigation overlap without relying on runner load.
      if (delayNextUsage) {
        delayNextUsage = false;
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
      return json({
        "standard@test.invalid": { windows: [{ name: "5 hours", used: 25 }, { name: "7 days", used: 20 }] },
        "Premium@test.invalid": { ...(p.backupNoFiveHour && { configuredAutoBackup: true }), windows: [{ name: "7 days", used: 0 }] },
      });
    }
    if (u.pathname.startsWith("/api/provider/") && route.request().method() === "POST") {
      const body = route.request().postDataJSON();
      posts.push({ action: u.pathname.split("/").at(-1), ...body });
      if (u.pathname.endsWith("accountbackup")) {
        p.accountBackup ||= {};
        if (body.backup === "auto") delete p.accountBackup[body.account.toLowerCase()];
        else p.accountBackup[body.account.toLowerCase()] = body.backup;
      }
      if (u.pathname.endsWith("backupnofivehour")) p.backupNoFiveHour = body.backupNoFiveHour;
      return json(providers);
    }
    if (u.pathname === "/api/gateway/trace") {
      if (u.searchParams.get("wait")) await new Promise((resolve) => setTimeout(resolve, 1000));
      return json({ mine: true, now: now.toISOString(), seq: 3, totals: { requests: 1, rerouted: 0, errors: 0 }, routes: [routes[0]] });
    }
    if (u.pathname === "/api/gateway/history") return json({ cut: false, days: [{ day, requests: routes.length }], routes: u.searchParams.get("day") ? routes : [] });
    if (u.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, u.pathname === "/" ? "index.html" : u.pathname);
    return route.fulfill({ body: await fs.readFile(file), contentType: { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)] });
  };
  return { serve, delayUsageRefresh: () => { delayNextUsage = true; } };
}

async function changeBackupDetection(page, change) {
  // The checkbox redraw precedes accountAction's state read and quota refresh.
  // Finish both the intercepted response and its consumer before reloading or
  // closing the document; WebKit can report a cancelled fetch as access control.
  const refreshed = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/login/usage");
  await change();
  assert.equal(await (await refreshed).finished(), null, "quota refresh was interrupted");
  await page.waitForFunction(() => Object.values(loginUsage).every((u) => !u.loading));
}

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium"])) {
  for (const width of [1100, 420]) {
    test(`${engine} ${width}: backup setting creates missing account tools`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" });
      const page = await context.newPage();
      page.setDefaultTimeout(5000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [], posts = [], user = "standard@test.invalid";
      page.on("pageerror", (e) => errors.push(e.message));
      t.after(async () => { await context.close(); await browser.close(); });
      await page.route("**/*", fixture("en", posts, (p) => {
        // Claude's active, lapsed sign-in has no Remove; every other pill is set.
        p.id = p.account.agent = "claude";
        p.name = p.account.agentName = "Claude Code";
        p.account.logins = [{ user, active: true, on: true, lapsed: "Sign in again" }];
        p.accountCaps = { [user]: 80 };
        p.accountConcurrency = { [user]: 2 };
      }).serve);
      await page.goto("http://magpie.test/?view=providers");
      await page.locator(".row.provider", { hasText: "Claude Code" }).first().click();
      const row = page.locator(`.editor .acc[data-account-id="${user}"]`);
      const more = row.getByRole("button", { name: `Settings for ${user}`, exact: true });
      const pill = row.locator(".abackup");
      await pill.waitFor({ state: "attached" });
      assert.equal(await row.locator(".acc-line .acap.set").count(), 1);
      assert.equal(await row.locator(".acc-line .alane.set").count(), 1);
      assert.equal(await row.getByRole("button", { name: "Remove", exact: true }).count(), 0);
      assert.equal(await row.locator(".acc-tools > button").count(), 1);
      assert.equal(await more.getAttribute("aria-expanded"), "false");
      await more.focus();
      await page.keyboard.press("Enter");
      assert.equal(await more.getAttribute("aria-expanded"), "true");
      assert(await pill.isVisible(), "backup setting is inaccessible without existing tools");
      for (const [backup, name] of [["backup", "Backup"], ["auto", "Automatic"]]) {
        await pill.click();
        await page.locator(".proto-menu").getByText(name, { exact: true }).click();
        await page.waitForFunction((name) => document.querySelector(".editor .abackup")?.textContent === name, name);
        assert.deepEqual(posts.at(-1), { action: "accountbackup", id: "claude", account: user, backup });
        if (backup === "auto") assert.equal(await more.getAttribute("aria-expanded"), "true", "open tools state lost after saving");
        else assert(await row.locator(".acc-line .abackup.set").isVisible());
      }
      assert(await row.locator(".acc-tools .abackup").isVisible());
      await more.click();
      // This last automatic setting disappears on save; use the keyboard so
      // Playwright needn't reread a checkbox already removed by the redraw.
      await page.locator(".backup-5h input").focus();
      await changeBackupDetection(page, () => page.keyboard.press("Space"));
      await page.waitForFunction(() => !document.querySelector(".backup-5h"));
      assert.equal(await more.getAttribute("aria-expanded"), "false", "closed tools state lost after saving");
      assert.deepEqual(errors, []);
    });
  }
  for (const lang of Object.keys(words)) {
    for (const width of [1100, 420]) {
      test(`${engine} ${lang} ${width}: backup settings and trace explanations`, async (t) => {
        const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
        const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" });
        const page = await context.newPage();
        page.setDefaultTimeout(5000);
        page.setDefaultNavigationTimeout(60000);
        const errors = [], posts = [], failedUsage = [];
        page.on("pageerror", (e) => errors.push(e.message));
        page.on("requestfailed", (r) => {
          if (new URL(r.url()).pathname === "/api/login/usage") failedUsage.push(r.failure().errorText);
        });
        t.after(async () => { await context.close(); await browser.close(); });
        const f = fixture(lang, posts);
        await page.route("**/*", f.serve);
        const screenshot = async (view) => {
          if (!process.env.ARTIFACT_DIR) return;
          await page.waitForTimeout(400);
          await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
          await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${width}-backup-${view}.png`), fullPage: true });
        };
        const w = words[lang];
        await page.goto("http://magpie.test/?view=providers");
        await page.locator(".row.provider", { hasText: "Codex" }).first().click();
        const row = (user) => page.locator(`.editor .acc[data-account-id="${user}"]`);
        const pill = (user) => row(user).locator(".abackup");
        const premium = "Premium@test.invalid", standard = "standard@test.invalid", unknown = "unknown@test.invalid";
        await page.locator(".backup-5h").waitFor();
        await page.waitForFunction(() => document.querySelector('.acc[data-account-id="Premium@test.invalid"] .acc-line .abackup.set'));
        assert.equal(await pill(premium).textContent(), w[0]);
        assert.match(await pill(premium).getAttribute("title"), /.+/);
        assert.equal(await row(standard).locator(".acc-tools .abackup").count(), 1);
        assert.equal(await row(unknown).locator(".acc-tools .abackup").count(), 1);
        for (const user of [standard, unknown]) {
          assert(await row(user).locator(".acc-tools").evaluate((tools) => tools.lastElementChild?.matches(".text.quiet")), "Remove must remain the last account action");
        }
        assert.equal(await page.locator(".backup-5h").innerText(), w[3]);
        await screenshot("editor");
        const choose = async (user, value, text) => {
          if (!await pill(user).isVisible()) await row(user).locator(".acc-more").click();
          await pill(user).scrollIntoViewIfNeeded();
          const before = await page.evaluate(() => [window.scrollY, document.querySelector("#modal .dialog")?.scrollTop]);
          await pill(user).click();
          assert.deepEqual(await page.evaluate(() => [window.scrollY, document.querySelector("#modal .dialog")?.scrollTop]), before, "opening menu scrolled");
          const n = posts.length;
          await page.locator(".proto-menu").getByText(text, { exact: true }).click();
          await page.waitForFunction((n) => document.querySelectorAll(".proto-menu").length === 0, n);
          assert.equal(posts.length, n + 1);
          assert.deepEqual(posts.at(-1), { action: "accountbackup", id: "codex", account: user, backup: value });
          await page.waitForFunction(([user, text]) => [...document.querySelectorAll(".acc")].find((r) => r.dataset.accountId === user)?.querySelector(".abackup")?.textContent === text, [user, value === "auto" ? w[0] : text]);
        };
        await choose(standard, "backup", w[1]);
        assert.equal(await row(standard).locator(".acc-line .abackup.set").count(), 1);
        await choose(premium, "primary", w[2]);
        await choose(premium, "auto", w[0]);
        await choose(unknown, "backup", w[1]);
        const check = page.locator(".backup-5h input");
        await changeBackupDetection(page, () => check.uncheck());
        await page.waitForFunction(() => !document.querySelector(".backup-5h input")?.checked);
        assert.deepEqual(posts.at(-1), { action: "backupnofivehour", id: "codex", backupNoFiveHour: false });
        if (width === 1100 && ["en", "zh"].includes(lang)) f.delayUsageRefresh();
        await changeBackupDetection(page, () => check.check());
        await page.waitForFunction(() => document.querySelector(".backup-5h input")?.checked);
        assert.deepEqual(posts.at(-1), { action: "backupnofivehour", id: "codex", backupNoFiveHour: true });
        await screenshot("configured");
        await page.goto("http://magpie.test/?view=routing");
        await page.getByText(new RegExp(w[4])).first().waitFor();
        assert((await page.locator("body").innerText()).includes(w[4]), "live backup row has no status");
        await screenshot("live-routing");
        await page.locator(".rt-days .rt-day").nth(1).click();
        await page.locator(".rt-req").nth(2).waitFor();
        await page.locator(".rt-req").nth(0).click();
        await page.getByText(new RegExp(w[5].replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))).first().waitFor();
        await screenshot("routing");
        await page.locator(".rt-req").nth(1).click();
        await page.waitForTimeout(200);
        assert(!(await page.locator(".rt-steps").innerText()).includes(w[5]), "promoted account is still explained as backup");
        await page.locator(".rt-req").nth(2).click();
        await page.waitForTimeout(200);
        assert(!(await page.locator(".rt-steps").innerText()).includes(w[5]), "leading backup is explained as held back");
        assert.deepEqual(errors, []);
        assert.deepEqual(failedUsage, [], "quota refresh must finish before document navigation");
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "horizontal overflow");
      });
    }
  }
}
