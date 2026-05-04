const tokenMeta = document.querySelector('meta[name="lol-autobuild-api-token"]');
const token = tokenMeta ? tokenMeta.content : "";
const localeStorageKey = "lol-autobuild.locale";
const fallbackLocale = "en";
const localeFiles = {
  "en": "/i18n/en.json",
  "pt-BR": "/i18n/pt-BR.json"
};
const translations = Object.create(null);
const translationLoads = Object.create(null);
const knownMessageKeys = {
  "LCU is off.": "lcu.off",
  "LCU is off": "lcu.off",
  "League Client is not open.": "lcu.lockfile_not_found",
  "Champ select is not ready.": "lcu.champ_select_unavailable",
  "Select a champion first.": "lcu.champion_not_selected",
  "Coachless login is missing.": "coachless.login_missing",
  "Another sync is already running": "sync.already_running",
  "Rune page limit reached. Delete a rune page in League Client or keep an AutoBuild page available for reuse.": "sync.rune_page_limit_reached",
  "Watcher pre-start failed.": "watch.pre_start_failed",
  "Watcher start failed.": "watch.start_failed",
  "Invalid UI token.": "ui.invalid_token",
  "Settings are invalid.": "ui.invalid_settings",
  "Method is not allowed.": "ui.method_not_allowed",
  "UI file is missing.": "ui.file_missing"
};
let currentLocale = resolveInitialLocale();
const ids = {
  main: document.querySelector("main"),
  localeSelect: document.getElementById("localeSelect"),
  lcuStatus: document.getElementById("lcuStatus"),
  watcherStatus: document.getElementById("watcherStatus"),
  modeStatus: document.getElementById("modeStatus"),
  messageBox: document.getElementById("messageBox"),
  logList: document.getElementById("logList"),
  form: document.getElementById("settingsForm"),
  lcuEnabled: document.getElementById("lcuEnabled"),
  patch: document.getElementById("patch"),
  patchRangeSlider: document.getElementById("patchRangeSlider"),
  patchRangeValue: document.getElementById("patchRangeValue"),
  patchRangeTicks: document.getElementById("patchRangeTicks"),
  leagueTierPresetSlider: document.getElementById("leagueTierPresetSlider"),
  leagueTierPresetValue: document.getElementById("leagueTierPresetValue"),
  leagueTierPresetTicks: document.getElementById("leagueTierPresetTicks"),
  applyItems: document.getElementById("applyItems"),
  applyRunes: document.getElementById("applyRunes"),
  applySpells: document.getElementById("applySpells"),
  spellsOptionsButton: document.getElementById("spellsOptionsButton"),
  spellsSuboptions: document.getElementById("spellsSuboptions"),
  keepFlash: document.getElementById("keepFlash"),
  dryRun: document.getElementById("dryRun"),
  syncButton: document.getElementById("syncButton"),
  watcherButton: document.getElementById("watcherButton"),
  autoSaveStatus: document.getElementById("autoSaveStatus"),
  watcherConfigWarning: document.getElementById("watcherConfigWarning"),
  updateButton: document.getElementById("updateButton"),
  updateBanner: document.getElementById("updateBanner"),
  updateText: document.getElementById("updateText"),
  updateLink: document.getElementById("updateLink"),
  updatedValue: document.getElementById("updatedValue"),
  championValue: document.getElementById("championValue"),
  positionValue: document.getElementById("positionValue"),
  queueValue: document.getElementById("queueValue"),
  itemsValue: document.getElementById("itemsValue"),
  runesValue: document.getElementById("runesValue"),
  spellsValue: document.getElementById("spellsValue")
};
let watcherRunning = false;

const logHistory = [{ key: "log.no_events", logKey: "empty" }];

let updateFeedbackTimer = 0;
let updateFeedbackKey = "";
let updateChecking = false;
let currentUpdateStatus = "idle";

let currentState = null;

let currentMessage = { key: "message.no_sync_session" };
let currentMessageIsError = false;
let currentAutoSaveStatus = { key: "settings.autosave_saved", tone: "good" };

const autoSaveDebounceMillis = 600;
let autoSaveTimer = 0;
let autoSaveDirty = false;
let autoSaveInFlight = null;
let actionInFlight = false;

let formsLoaded = false;

function localeFrom(value) {
  const normalized = String(value || "").replace("_", "-").toLowerCase();
  if (normalized === "pt" || normalized.startsWith("pt-")) {
    return "pt-BR";
  }
  if (normalized === "en" || normalized.startsWith("en-")) {
    return "en";
  }
  return "";
}

function storedLocale() {
  try {
    return localStorage.getItem(localeStorageKey) || "";
  } catch {
    return "";
  }
}

function resolveInitialLocale() {
  const stored = localeFrom(storedLocale());
  if (stored) {
    return stored;
  }

  const browserLocales = Array.isArray(navigator.languages) && navigator.languages.length > 0
    ? navigator.languages
    : [navigator.language];

  for (const browserLocale of browserLocales) {
    const matched = localeFrom(browserLocale);
    if (matched) {
      return matched;
    }
  }

  return "en";
}

function saveLocale(locale) {
  try {
    localStorage.setItem(localeStorageKey, locale);
  } catch {
  }
}

async function loadLocale(locale) {
  if (translations[locale]) {
    return translations[locale];
  }

  const url = localeFiles[locale];
  if (!url) {
    throw new Error(`Unknown locale: ${locale}`);
  }

  if (!translationLoads[locale]) {
    const jsonRequest = {
      headers: { Accept: "application/json" }
    };
    translationLoads[locale] = fetch(url, jsonRequest).then(async response => {
      if (!response.ok) {
        throw new Error(`Could not load ${locale} translations.`);
      }

      const catalog = await response.json();
      translations[locale] = catalog;
      return catalog;
    }).catch(error => {
      delete translationLoads[locale];
      throw error;
    });
  }

  return translationLoads[locale];
}

async function ensureLocale(locale) {
  const nextLocale = localeFrom(locale) || fallbackLocale;
  await loadLocale(fallbackLocale);
  if (nextLocale === fallbackLocale) {
    return fallbackLocale;
  }

  try {
    await loadLocale(nextLocale);
    return nextLocale;
  } catch (error) {
    console.warn(error);
    return fallbackLocale;
  }
}

function hasTranslation(key) {
  const catalog = translations[currentLocale] || {};
  const fallbackCatalog = translations[fallbackLocale] || {};
  return Boolean(key && (catalog[key] || fallbackCatalog[key]));
}

function t(key, params = {}, fallback = "") {
  const catalog = translations[currentLocale] || {};
  const fallbackCatalog = translations[fallbackLocale] || {};
  const template = catalog[key] || fallbackCatalog[key];
  if (!template) {
    return fallback || key;
  }

  return template.replace(/\{(\w+)\}/g, (_match, name) => {
    const value = params[name];
    return value === undefined || value === null ? "" : String(value);
  });
}

function textForDescriptor(descriptor) {
  if (!descriptor) {
    return "";
  }

  if (typeof descriptor === "string") {
    if (hasTranslation(descriptor)) {
      return t(descriptor);
    }
    const key = knownMessageKeys[descriptor] || "";
    return key ? t(key) : descriptor;
  }

  const fallback = descriptor.fallback || "";
  const key = descriptor.key || knownMessageKeys[fallback] || "";
  if (hasTranslation(key)) {
    return t(key, {}, fallback);
  }

  return fallback;
}

function applyStaticTranslations() {
  document.documentElement.lang = currentLocale;
  document.title = t("app.title");

  document.querySelectorAll("[data-i18n]").forEach(element => {
    element.textContent = t(element.dataset.i18n);
  });

  document.querySelectorAll("[data-i18n-aria-label]").forEach(element => {
    element.setAttribute("aria-label", t(element.dataset.i18nAriaLabel));
  });

  ids.localeSelect.value = currentLocale;
}

async function setLocale(locale, shouldPersist = true) {
  currentLocale = await ensureLocale(locale);
  if (shouldPersist) {
    saveLocale(currentLocale);
  }

  applyStaticTranslations();
  renderAutoSaveStatus();
  positionAdvancedSliderTicks();
  renderUpdateButton();
  if (formsLoaded) {
    renderModeStatus(readSettings());
    renderAdvancedSliderValues();
  } else if (currentState && currentState.settings) {
    renderModeStatus(currentState.settings);
  }
  if (currentState) {
    renderState(currentState);
  } else {
    setMessage(currentMessage, currentMessageIsError);
  }
  renderLogList();
}

async function api(path, options = {}) {
  const join = path.includes("?") ? "&" : "?";
  const url = `${path}${join}token=${encodeURIComponent(token)}`;
  const jsonRequest = {
    headers: { "Content-Type": "application/json" },
    ...options
  };
  const response = await fetch(url, jsonRequest);

  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(data.error || t("api.request_failed"));
    error.code = data.error_code || "";
    error.fallback = data.error || "";
    throw error;
  }

  return data;
}

const patchRangeValues = ["auto", "0", "1", "2", "3", "4"];
const leagueTierPresets = [
  { value: "gold_plus", key: "settings.rank_gold_plus", letter: "G", className: "rank-gold" },
  { value: "platinum_plus", key: "settings.rank_platinum_plus", letter: "P", className: "rank-platinum" },
  { value: "emerald_plus", key: "settings.rank_emerald_plus", letter: "E", className: "rank-emerald" },
  { value: "diamond_plus", key: "settings.rank_diamond_plus", letter: "D", className: "rank-diamond" },
  { value: "master_plus", key: "settings.rank_master_plus", letter: "M", className: "rank-master" }
];

function clampSliderIndex(rawValue, maxIndex, fallback = 0) {
  const value = Number(rawValue);
  if (!Number.isInteger(value)) {
    return fallback;
  }
  return Math.max(0, Math.min(maxIndex, value));
}

function patchRangeIndexFromValue(value) {
  const index = patchRangeValues.indexOf(value);
  return index >= 0 ? index : 0;
}

function leagueTierIndexFromPreset(value) {
  const index = leagueTierPresets.findIndex(preset => preset.value === value);
  return index >= 0 ? index : 2;
}

function patchRangeValueFromSettings(settings) {
  if (settings.patch_additions_mode === "manual") {
    return String(settings.patch_additions ?? 0);
  }
  return "auto";
}

function patchRangeSettingsFromValue(value) {
  if (value === "auto") {
    return {
      patch_additions_mode: "auto",
      patch_additions: 2
    };
  }

  return {
    patch_additions_mode: "manual",
    patch_additions: Number(value)
  };
}

function patchLabelFromInput() {
  return ids.patch.value.trim();
}

function numericPatchParts(label) {
  const match = label.match(/^(\d+)\.(\d+)$/);
  if (!match) {
    return null;
  }

  const major = Number(match[1]);
  const minor = Number(match[2]);
  if (!Number.isInteger(major) || !Number.isInteger(minor) || minor < 0) {
    return null;
  }

  return { major, minor };
}

function explicitPatchRangeLabel(patchLabel, additions) {
  const parts = numericPatchParts(patchLabel);
  if (!parts || !Number.isInteger(additions) || additions < 0 || additions > parts.minor) {
    return "";
  }

  if (additions === 0) {
    return patchLabel;
  }

  return `${patchLabel} - ${parts.major}.${parts.minor - additions}`;
}

function patchRangeLabel(value, patchLabel = "") {
  if (value === "auto") {
    return t("settings.patch_range_auto");
  }

  const additions = Number(value);
  const explicitLabel = patchLabel.trim();
  if (explicitLabel) {
    const rangeLabel = explicitPatchRangeLabel(explicitLabel, additions);
    if (rangeLabel) {
      return rangeLabel;
    }
  }

  if (value === "0") {
    return t("settings.patch_current");
  }
  return t(`settings.patch_previous_${value}`);
}

function selectedPatchRangeValue() {
  return patchRangeValues[clampSliderIndex(ids.patchRangeSlider.value, patchRangeValues.length - 1)];
}

function selectedLeagueTierPreset() {
  return leagueTierPresets[clampSliderIndex(ids.leagueTierPresetSlider.value, leagueTierPresets.length - 1, 2)].value;
}

function renderLeagueTierPresetValue(value) {
  const preset = leagueTierPresets.find(item => item.value === value) || leagueTierPresets[2];
  const badge = document.createElement("span");
  badge.className = `rank-badge ${preset.className}`;
  badge.setAttribute("aria-hidden", "true");
  badge.textContent = preset.letter;

  const label = document.createElement("span");
  label.textContent = t(preset.key);

  ids.leagueTierPresetValue.replaceChildren(badge, label);
}

function cssPixelValue(rawValue, fallback) {
  const parsed = Number.parseFloat(rawValue);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function sliderEndpointGutter(slider) {
  const control = slider.closest(".slider-control");
  if (!control) {
    return 16;
  }

  return cssPixelValue(getComputedStyle(control).getPropertyValue("--slider-endpoint-gutter"), 16);
}

function formatPixelOffset(value) {
  const rounded = Math.round(value * 100) / 100;
  return `${Object.is(rounded, -0) ? 0 : rounded}px`;
}

function positionSliderTicks(slider, ticks) {
  const min = Number(slider.min);
  const max = Number(slider.max);
  const step = Number(slider.step || 1);
  const labels = Array.from(ticks.querySelectorAll("span"));
  if (!Number.isFinite(min) || !Number.isFinite(max) || !Number.isFinite(step) || max <= min || step <= 0) {
    return;
  }

  labels.forEach((label, index) => {
    const value = min + (index * step);
    const ratio = Math.max(0, Math.min(1, (value - min) / (max - min)));
    const position = ratio * 100;
    const offset = (1 - (2 * ratio)) * sliderEndpointGutter(slider);

    label.style.setProperty("--slider-tick-position", `calc(${position}% + ${formatPixelOffset(offset)})`);
  });
}

function positionAdvancedSliderTicks() {
  positionSliderTicks(ids.patchRangeSlider, ids.patchRangeTicks);
  positionSliderTicks(ids.leagueTierPresetSlider, ids.leagueTierPresetTicks);
}

function renderAdvancedSliderValues() {
  const patchRange = selectedPatchRangeValue();
  const rankPreset = selectedLeagueTierPreset();
  ids.patchRangeValue.textContent = patchRangeLabel(patchRange, patchLabelFromInput());
  renderLeagueTierPresetValue(rankPreset);
}


function readSettings() {
  const patchRangeSettings = patchRangeSettingsFromValue(selectedPatchRangeValue());
  return {
    lcu_enabled: ids.lcuEnabled.checked,
    patch: ids.patch.value.trim(),
    ...patchRangeSettings,
    league_tier_preset: selectedLeagueTierPreset(),
    apply_items: ids.applyItems.checked,
    apply_runes: ids.applyRunes.checked,
    apply_spells: ids.applySpells.checked,
    keep_flash: ids.keepFlash.checked,
    dry_run: ids.dryRun.checked
  };
}

function setSpellsSuboptionsOpen(isOpen) {
  ids.spellsOptionsButton.setAttribute("aria-expanded", String(isOpen));
  ids.spellsSuboptions.hidden = !isOpen;
}

function syncSpellsSuboptions() {
  const spellsEnabled = ids.applySpells.checked;
  ids.spellsOptionsButton.disabled = !spellsEnabled;
  ids.spellsOptionsButton.setAttribute("aria-disabled", String(!spellsEnabled));
  ids.keepFlash.disabled = !spellsEnabled;
  ids.keepFlash.closest(".check").classList.toggle("is-disabled", !spellsEnabled);

  if (!spellsEnabled) {
    setSpellsSuboptionsOpen(false);
  }
}

function setBusy(isBusy) {
  ids.main.setAttribute("aria-busy", String(isBusy));
  ids.syncButton.disabled = isBusy;
  ids.watcherButton.disabled = isBusy;
}

function renderUpdateButton() {
  if (updateChecking || currentUpdateStatus === "checking") {
    ids.updateButton.disabled = true;
    ids.updateButton.textContent = t("action.checking");
    return;
  }

  ids.updateButton.disabled = false;
  ids.updateButton.textContent = updateFeedbackKey ? t(updateFeedbackKey) : t("action.check_updates");
}

function resetUpdateButton() {
  updateFeedbackKey = "";
  renderUpdateButton();
}

function setUpdateFeedback(key) {
  if (updateFeedbackTimer) {
    clearTimeout(updateFeedbackTimer);
  }

  updateFeedbackKey = key;
  renderUpdateButton();

  updateFeedbackTimer = setTimeout(() => {
    updateFeedbackTimer = 0;
    resetUpdateButton();
  }, 2400);
}

function renderUpdate(state) {
  const update = state.update || { status: "idle" };
  currentUpdateStatus = update.status || "idle";

  const hasUpdate = update.status === "available";
  ids.updateBanner.hidden = !hasUpdate;

  if (hasUpdate) {
    const version = update.latest_version || t("update.new_version_generic");
    const current = update.current_version ? t("update.current_version", { version: update.current_version }) : "";
    ids.updateText.textContent = t("update.download_version", { version, current });
    ids.updateLink.href = update.download_url || "https://github.com/controlado/lol-autobuild/releases/latest";
  }

  if (update.status === "checking") {
    if (updateFeedbackTimer) {
      clearTimeout(updateFeedbackTimer);
      updateFeedbackTimer = 0;
    }
    renderUpdateButton();
    return;
  }

  renderUpdateButton();
}

function setMessage(descriptor, isError = false) {
  currentMessage = descriptor || { key: "message.no_sync_session" };
  currentMessageIsError = isError;
  ids.messageBox.classList.toggle("error", isError);
  ids.messageBox.setAttribute("role", isError ? "alert" : "status");
  ids.messageBox.textContent = textForDescriptor(currentMessage) || t("message.no_sync_session");
}

function renderAutoSaveStatus() {
  ids.autoSaveStatus.className = `value${currentAutoSaveStatus.tone ? ` is-${currentAutoSaveStatus.tone}` : ""}`;
  ids.autoSaveStatus.textContent = t(currentAutoSaveStatus.key);
}

function setAutoSaveStatus(key, tone = "") {
  currentAutoSaveStatus = { key, tone };
  renderAutoSaveStatus();
}

function setValue(element, text, tone = "") {
  element.textContent = text;
  element.className = `value${tone ? ` is-${tone}` : ""}`;
}

function setCell(element, text, tone = "") {
  element.textContent = text;
  element.className = `cell-value${tone ? ` is-${tone}` : ""}`;
}

function appliedText(value) {
  return value ? t("state.applied") : t("state.not_applied");
}

function championText(sync) {
  if (!sync) {
    return t("state.no_sync_yet");
  }

  const name = String(sync.DetectedChampionName || "").trim();
  const id = Number(sync.DetectedChampionID || 0);
  if (name && id > 0) {
    return `${name} (#${id})`;
  }
  if (name) {
    return name;
  }
  return id > 0 ? String(id) : t("state.not_detected");
}

function renderModeStatus(settings) {
  if (!settings) {
    return;
  }

  setValue(
    ids.modeStatus,
    settings.dry_run ? t("mode.preview") : t("mode.live"),
    settings.dry_run ? "info" : "warn",
  );
}

function formatTime(value) {
  if (!value) {
    return t("state.no_sync_yet");
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat(currentLocale, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(date);
}

function renderLogList() {
  ids.logList.replaceChildren(...logHistory.map(item => {
    const li = document.createElement("li");
    const text = textForDescriptor(item);
    li.textContent = item.time ? `${formatTime(item.time)} - ${text}` : text;
    if (item.tone) {
      li.className = item.tone;
    }
    return li;
  }));
}

function latestLogKey() {
  const last = logHistory[logHistory.length - 1];
  return last ? last.logKey : "";
}

function appendLog(items, logKey) {
  if (logKey === latestLogKey()) {
    return false;
  }

  if (logHistory.length === 1 && logHistory[0].logKey === "empty") {
    logHistory.length = 0;
  }

  const time = new Date().toISOString();
  logHistory.push(...items.map(item => ({ ...item, logKey, time })));
  renderLogList();
  return true;
}

function ensureStateLog(items, logKey) {
  if (logHistory.some(item => item.logKey === logKey)) {
    return true;
  }

  return appendLog(items, logKey);
}

function renderLog(state, sync) {
  if (state.last_error) {
    appendLog([{ key: state.last_error_code, fallback: state.last_error, tone: "error" }], `error:${state.last_error_code || state.last_error}`);
    return;
  }

  let wrote = false;
  const notice = state.watcher && state.watcher.last_notice;
  if (notice && notice.kind) {
    const fallback = notice.error || notice.message || notice.kind;
    const tone = notice.error || notice.kind === "reconnecting" ? "warn" : "";
    const noticeKey = `watch_notice:${notice.at || ""}:${notice.kind}:${notice.error || notice.message || ""}`;
    wrote = ensureStateLog([{ key: `watch.notice.${notice.kind}`, fallback, tone }], noticeKey) || wrote;
  }

  if (sync && sync.Warnings && sync.Warnings.length > 0) {
    wrote = ensureStateLog(
      sync.Warnings.map(warning => ({ fallback: warning, tone: "warn" })),
      `warnings:${state.last_sync_at || ""}:${sync.Warnings.join("|")}`
    ) || wrote;
  }

  if (sync && !(sync.Warnings && sync.Warnings.length > 0)) {
    wrote = ensureStateLog([{ key: "log.sync_without_warnings" }], `sync:${state.last_sync_at || JSON.stringify(sync)}`) || wrote;
  }

  if (!wrote && state.watcher.running) {
    ensureStateLog([{ key: "log.watcher_waiting" }], "watcher:running");
  }

  if (!wrote && logHistory.length === 0) {
    ensureStateLog([{ key: "log.no_events" }], "empty");
  }
}

function renderForms(state) {
  const settings = state.settings;
  formsLoaded = true;
  ids.lcuEnabled.checked = settings.lcu_enabled;
  ids.patch.value = settings.patch || "";
  ids.patchRangeSlider.value = String(patchRangeIndexFromValue(patchRangeValueFromSettings(settings)));
  ids.leagueTierPresetSlider.value = String(leagueTierIndexFromPreset(settings.league_tier_preset || "emerald_plus"));
  ids.applyItems.checked = settings.apply_items;
  ids.applyRunes.checked = settings.apply_runes;
  ids.applySpells.checked = settings.apply_spells;
  ids.keepFlash.checked = settings.keep_flash;
  ids.dryRun.checked = settings.dry_run;
  syncSpellsSuboptions();
  renderAdvancedSliderValues();
  renderModeStatus(settings);
}

function renderState(state) {
  currentState = state;
  renderUpdate(state);

  watcherRunning = state.watcher.running;
  ids.watcherButton.textContent = watcherRunning ? t("action.stop_watcher") : t("action.start_watcher");
  ids.watcherButton.setAttribute("aria-pressed", String(watcherRunning));
  setValue(ids.watcherStatus, watcherRunning ? t("watcher.running") : t("watcher.stopped"), watcherRunning ? "good" : "");
  ids.watcherConfigWarning.hidden = !(state.watcher && state.watcher.config_stale);

  if (state.lcu.state === "connected") {
    setValue(ids.lcuStatus, t("lcu.connected"), "good");
  } else if (state.lcu.state === "off") {
    setValue(ids.lcuStatus, t("lcu.disabled"), "warn");
  } else {
    setValue(ids.lcuStatus, textForDescriptor(state.lcu.message || "lcu.not_connected"), "bad");
  }

  const sync = state.last_sync;
  setCell(ids.updatedValue, formatTime(state.last_sync_at));
  setCell(ids.championValue, championText(sync));
  setCell(ids.positionValue, sync ? sync.DetectedPosition || t("state.not_detected") : t("state.no_sync_yet"));
  setCell(ids.queueValue, sync ? String(sync.DetectedQueueID) : t("state.no_sync_yet"));
  setCell(ids.itemsValue, sync ? appliedText(sync.ItemSetApplied) : t("state.not_applied"), sync && sync.ItemSetApplied ? "good" : "");
  setCell(ids.runesValue, sync ? appliedText(sync.RunePageApplied) : t("state.not_applied"), sync && sync.RunePageApplied ? "good" : "");
  setCell(ids.spellsValue, sync ? appliedText(sync.SpellsApplied) : t("state.not_applied"), sync && sync.SpellsApplied ? "good" : "");

  if (state.last_error) {
    setMessage({ key: state.last_error_code, fallback: state.last_error }, true);
  } else if (sync && sync.Warnings && sync.Warnings.length > 0) {
    setMessage({ key: "message.sync_with_warnings" });
  } else if (sync) {
    setMessage({ key: "message.sync_finished" });
  } else if (state.watcher.running) {
    setMessage({ key: "message.watcher_waiting" });
  } else {
    setMessage({ key: "message.no_sync_session" });
  }

  renderLog(state, sync);
}

async function loadForms() {
  const state = await api("/api/state");
  renderForms(state);
}

async function loadState() {
  const state = await api("/api/state");
  renderState(state);
}

async function checkUpdates(isManual) {
  updateChecking = true;
  renderUpdateButton();

  try {
    const state = await api("/api/update/check", {
      method: "POST",
      body: "{}"
    });
    renderState(state);

    if (isManual) {
      const status = state.update && state.update.status;
      if (status === "current") {
        setUpdateFeedback("update.up_to_date");
      } else if (status === "unavailable") {
        setUpdateFeedback("update.cannot_check");
      } else if (status === "error") {
        setUpdateFeedback("update.check_failed");
      }
    }
  } catch (error) {
    currentUpdateStatus = "idle";
    if (isManual) {
      setUpdateFeedback("update.check_failed");
    } else {
      resetUpdateButton();
    }
  } finally {
    updateChecking = false;
    if (!updateFeedbackTimer) {
      resetUpdateButton();
    } else {
      renderUpdateButton();
    }
  }
}

function scheduleSave() {
  autoSaveDirty = true;
  setAutoSaveStatus("settings.autosave_saving");

  if (autoSaveTimer) {
    clearTimeout(autoSaveTimer);
  }

  autoSaveTimer = setTimeout(() => {
    autoSaveTimer = 0;
    saveSettingsNow().catch(() => { });
  }, autoSaveDebounceMillis);
}

async function saveSettingsNow() {
  if (autoSaveInFlight) {
    await autoSaveInFlight;
    if (autoSaveDirty) {
      return saveSettingsNow();
    }
    return;
  }

  if (!autoSaveDirty) {
    return;
  }

  const settings = readSettings();
  autoSaveDirty = false;
  setAutoSaveStatus("settings.autosave_saving");

  autoSaveInFlight = api("/api/config", {
    method: "POST",
    body: JSON.stringify(settings)
  }).then(state => {
    currentState = state;
    renderState(state);
    setAutoSaveStatus("settings.autosave_saved", "good");
  }).catch(error => {
    autoSaveDirty = true;
    setAutoSaveStatus("settings.autosave_failed", "bad");
    throw error;
  }).finally(() => {
    autoSaveInFlight = null;
  });

  await autoSaveInFlight;

  if (autoSaveDirty) {
    return saveSettingsNow();
  }
}

async function flushPendingSave() {
  if (autoSaveTimer) {
    clearTimeout(autoSaveTimer);
    autoSaveTimer = 0;
  }

  await saveSettingsNow();
}

async function runExclusiveAction(action) {
  if (actionInFlight) {
    return;
  }

  actionInFlight = true;
  setBusy(true);
  try {
    await action();
  } finally {
    actionInFlight = false;
    setBusy(false);
  }
}

async function postRequest(path, body, progressKey) {
  setMessage({ key: progressKey });
  appendLog([{ key: progressKey }], `action:${progressKey}`);
  try {
    const state = await api(path, {
      method: "POST",
      body: body ? JSON.stringify(body) : "{}"
    });
    if (path === "/api/config") {
      renderForms(state);
    }
    renderState(state);
  } catch (error) {
    setMessage({ key: error.code, fallback: error.fallback || error.message }, true);
    await loadState().catch(() => { });
  }
}

async function post(path, body, progressKey) {
  await runExclusiveAction(() => postRequest(path, body, progressKey));
}

async function postAfterSaving(path, body, progressKey) {
  await runExclusiveAction(async () => {
    try {
      await flushPendingSave();
    } catch (error) {
      setMessage({ key: error.code, fallback: error.fallback || error.message }, true);
      return;
    }

    await postRequest(path, body, progressKey);
  });
}

async function initialize() {
  currentLocale = await ensureLocale(currentLocale);
  applyStaticTranslations();
  positionAdvancedSliderTicks();
  loadForms().catch(error => setMessage({ key: error.code, fallback: error.fallback || error.message }, true));
  loadState().catch(error => setMessage({ key: error.code, fallback: error.fallback || error.message }, true));
  checkUpdates(false).catch(() => { });
  setInterval(() => loadState().catch(() => { }), 3000);
}

ids.localeSelect.addEventListener("change", () => {
  setLocale(ids.localeSelect.value).catch(error => {
    console.error(error);
    setMessage({ fallback: error.message }, true);
  });
});
ids.form.addEventListener("submit", event => {
  event.preventDefault();
  flushPendingSave().catch(error => {
    setMessage({ key: error.code, fallback: error.fallback || error.message }, true);
  });
});

ids.lcuEnabled.addEventListener("change", scheduleSave);
ids.applyItems.addEventListener("change", scheduleSave);
ids.applyRunes.addEventListener("change", scheduleSave);
ids.applySpells.addEventListener("change", () => {
  syncSpellsSuboptions();
  scheduleSave();
});
ids.keepFlash.addEventListener("change", scheduleSave);
ids.dryRun.addEventListener("change", () => {
  renderModeStatus(readSettings());
  scheduleSave();
});
ids.patch.addEventListener("input", () => {
  renderAdvancedSliderValues();
  scheduleSave();
});
ids.patchRangeSlider.addEventListener("input", () => {
  renderAdvancedSliderValues();
  scheduleSave();
});
ids.leagueTierPresetSlider.addEventListener("input", () => {
  renderAdvancedSliderValues();
  scheduleSave();
});
ids.spellsOptionsButton.addEventListener("click", () => {
  const isOpen = ids.spellsOptionsButton.getAttribute("aria-expanded") === "true";
  setSpellsSuboptionsOpen(!isOpen);
});
ids.syncButton.addEventListener("click", () => {
  postAfterSaving("/api/sync", null, "action.running_sync").catch(error => {
    setMessage({ fallback: error.message }, true);
  });
});
ids.updateButton.addEventListener("click", () => checkUpdates(true));
ids.watcherButton.addEventListener("click", () => {
  const path = watcherRunning ? "/api/watch/stop" : "/api/watch/start";
  const message = watcherRunning ? "action.stopping_watcher" : "action.starting_watcher";
  const action = watcherRunning ? post(path, null, message) : postAfterSaving(path, null, message);
  action.catch(error => {
    setMessage({ fallback: error.message }, true);
  });
});

initialize().catch(error => {
  console.error(error);
  setMessage({ fallback: error.message || "Could not load translations." }, true);
});
