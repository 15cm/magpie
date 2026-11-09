// The provider editor's configured Smart backup policy, loaded with accounts.
// configuredAutoBackup is the server's window classification, before weekly
// promotion. Only a routing trace can say the live tier for a requested model.
window.acctBackupFill = function (p, a, list, ls, quota) {
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
