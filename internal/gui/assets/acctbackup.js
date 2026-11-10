const backupTierDrafts = new Map();

// The provider editor's configured Smart backup policy, loaded with accounts.
// configuredAutoBackup is the server's window classification, before weekly
// promotion. Only a routing trace can say the live tier for a requested model.
window.acctBackupFill = function (p, a, list, ls, quota) {
  if (p.backupUsageTiersDefault && !list.querySelector(".backup-usage-tiers")) {
    list.append(backupUsageTiersEditor(p));
  }
  const title = t("Smart backup setting; Routing shows the current tier for each model, including promotion before a weekly reset");
  for (const row of list.children) {
    const user = row.dataset.accountId;
    if (!user || row.querySelector(".abackup")) continue;
    const mode = p.accountBackup?.[user.toLowerCase()] || "";
    const auto = p.backupNoFiveHour && quota?.[user]?.configuredAutoBackup;
    const set = !!mode || auto;
    const b = el("button", "sess-pick abackup" + (set ? " set" : ""), t(mode === "backup" ? "Backup" : mode === "primary" ? "Primary" : "Automatic"));
    b.type = "button";
    b.title = title;
    b.setAttribute("aria-haspopup", "menu");
    b.setAttribute("aria-expanded", "false");
    b.onclick = (e) => {
      e.stopPropagation();
      if (b.classList.contains("open")) return closeProtoMenu();
      openProtoMenu(b, [
        { v: "auto", name: "Automatic", note: "" },
        { v: "backup", name: "Backup", note: "" },
        { v: "primary", name: "Primary", note: "" },
      ], mode || "auto", (backup) => {
        if (backup === (mode || "auto")) return;
        accountAction("provider/accountbackup", { id: p.id, account: user, backup }, t("Saved"));
      }, "Smart backup setting", "sess-menu");
    };
    if (set) row.querySelector(".acc-line .grow").before(b);
    else {
      let tools = row.querySelector(".acc-tools");
      if (!tools) {
        tools = el("div", "acc-tools");
        const key = p.id + "@" + user.toLowerCase(), more = el("button", "acc-more");
        more.type = "button";
        more.append(svg(DOTS, 14, 2.4));
        more.title = t("Settings for {user}", { user });
        more.setAttribute("aria-label", more.title);
        const shown = (v) => {
          row.classList.toggle("tools-open", v);
          more.setAttribute("aria-expanded", v);
          v ? accountToolsOpen.add(key) : accountToolsOpen.delete(key);
        };
        shown(accountToolsOpen.has(key));
        more.onclick = () => shown(!row.classList.contains("tools-open"));
        row.querySelector(".acc-line .grow").after(more);
        row.append(tools);
      }
      tools.insertBefore(b, tools.querySelector(".text.quiet"));
    }
  }
  if ((ls.length > 1 || p.backupNoFiveHour || Object.keys(p.accountBackup || {}).length) && !list.querySelector(".backup-5h")) {
    const [label, cb] = tick(t("Use accounts without a 5-hour limit as backup"), !!p.backupNoFiveHour);
    label.title = title;
    cb.onchange = async () => {
      cb.disabled = true;
      const saved = await accountAction("provider/backupnofivehour", { id: p.id, backupNoFiveHour: cb.checked }, t("Saved"));
      if (saved) {
        if (loginUsage[a.agent]) loginUsage[a.agent].at = 0;
        loginUsageOf(a.agent);
      } else cb.checked = !!p.backupNoFiveHour;
      cb.disabled = false;
    };
    const box = el("div", "switch-only backup-5h", label);
    list.append(box);
  }
};

// Drafts survive provider redraws and failed saves; only success reloads policy.
function backupUsageTiersEditor(p) {
  const box = el("div", "acct-models backup-usage-tiers");
  const mode = p.backupUsageTiers == null ? "Default" : p.backupUsageTiers.length ? "Custom" : "Off";
  box.append(el("div", "", t("Weekly promotion tiers") + " · " + t(mode)));
  box.append(el("div", "hint", t("Applies to configured backups. Passing allows primary competition; every applicable weekly window must qualify. Each row requires both limits; any row may match. Days are elapsed 24-hour periods.")));
  let draft = backupTierDrafts.get(p.id);
  if (!draft) {
    draft = (p.backupUsageTiers ?? p.backupUsageTiersDefault).map((r) => ({ used: String(r.maxUsed), days: String(r.maxResetDays) }));
  }
  const rows = el("div"), error = el("div", "hint err");
  const draw = () => {
    rows.replaceChildren();
    for (const row of draft) {
      const line = el("div", "acm-foot");
      for (const [key, label, min, max] of [["used", "Weekly usage used, at most (%)", "0", "100"], ["days", "Reset within (days)", "0", "7"]]) {
        const input = el("input", "");
        input.type = "number"; input.step = "any"; input.min = min; input.max = max;
        input.value = row[key]; input.setAttribute("aria-label", t(label));
        input.oninput = () => { row[key] = input.value; backupTierDrafts.set(p.id, draft); };
        line.append(el("label", "", t(label), input));
      }
      const remove = el("button", "text quiet", t("Remove"));
      remove.type = "button";
      remove.onclick = () => { draft.splice(draft.indexOf(row), 1); backupTierDrafts.set(p.id, draft); draw(); };
      line.append(remove); rows.append(line);
    }
  };
  draw();
  const foot = el("div", "acm-foot");
  let saving = false;
  const post = async (tiers) => {
    if (saving) return;
    saving = true; error.textContent = "";
    const saved = await accountAction("provider/backupusagetiers", { id: p.id, backupUsageTiers: tiers }, t("Saved"), false, (message) => { error.textContent = message; return message; });
    saving = false;
    if (saved) { backupTierDrafts.delete(p.id); renderProviders(); }
  };
  const button = (text, run) => { const b = el("button", "text", t(text)); b.type = "button"; b.onclick = run; foot.append(b); };
  button("Add tier", () => { draft.push({ used: "", days: "" }); backupTierDrafts.set(p.id, draft); draw(); });
  button("Save", () => {
    if (draft.some((r) => !r.used.trim() || !r.days.trim() || !Number.isFinite(Number(r.used)) || !Number.isFinite(Number(r.days)))) {
      error.textContent = t("Enter a number for both limits in every tier."); return;
    }
    post(draft.map((r) => ({ maxUsed: Number(r.used), maxResetDays: Number(r.days) })));
  });
  button("Disable promotion", () => post([]));
  button("Use defaults", () => post(null));
  box.append(rows, foot, error);
  return box;
}
