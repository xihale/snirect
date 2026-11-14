'use strict';

/* Popover polyfill: only fetched on browsers without the Popover API */
if (!HTMLElement.prototype.hasOwnProperty('popover')) {
  const base = 'https://cdn.jsdelivr.net/npm/@oddbird/popover-polyfill/dist/popover';
  const css = Object.assign(document.createElement('link'), { rel: 'stylesheet', href: base + '.css' });
  const js = Object.assign(document.createElement('script'), { src: base + '.min.js', defer: true });
  document.head.append(css, js);
}

/* Navbar: inline links when they fit, popover menu when they don't */
const nav = document.querySelector('.site-nav');
const bar = nav.firstElementChild;
const menu = document.getElementById('menu');
let lastWidth = -1;

function layoutNav() {
  const width = bar.clientWidth;
  if (width === lastWidth) return;
  lastWidth = width;
  try { menu.hidePopover(); } catch { /* not a popover or already closed */ }
  menu.removeAttribute('popover');
  nav.classList.add('is-wide');
  if (bar.scrollWidth > bar.clientWidth) {
    nav.classList.remove('is-wide');
    menu.setAttribute('popover', 'auto');
  }
}
new ResizeObserver(layoutNav).observe(bar);
layoutNav();
menu.addEventListener('click', (e) => {
  if (e.target.closest('a')) { try { menu.hidePopover(); } catch { /* inline mode */ } }
});

/* Scroll spy */
const spyLinks = new Map([...menu.querySelectorAll('a')].map((a) => [a.hash.slice(1), a]));
const spy = new IntersectionObserver((entries) => {
  for (const { target, isIntersecting } of entries) {
    const link = spyLinks.get(target.id);
    if (isIntersecting) {
      spyLinks.forEach((a) => a.removeAttribute('aria-current'));
      link.setAttribute('aria-current', 'true');
    } else if (link.hasAttribute('aria-current')) {
      link.removeAttribute('aria-current');
    }
  }
}, { rootMargin: '-40% 0px -55% 0px' });
spyLinks.forEach((_, id) => spy.observe(document.getElementById(id)));

/* Tabs (hidden="until-found" panels; native focusgroup with keyboard fallback) */
const tablist = document.querySelector('[role="tablist"]');
const tabs = [...tablist.querySelectorAll('[role="tab"]')];
const panelOf = (tab) => document.getElementById('p-' + tab.id.slice(2));
const hasFocusGroup = Object.hasOwn(HTMLElement.prototype, 'focusGroup');

function selectTab(tab) {
  for (const t of tabs) {
    const active = t === tab;
    t.setAttribute('aria-selected', String(active));
    t.toggleAttribute('focusgroupstart', active);
    if (!hasFocusGroup) t.tabIndex = active ? 0 : -1;
    if (active) panelOf(t).removeAttribute('hidden');
    else panelOf(t).setAttribute('hidden', 'until-found');
  }
  tab.scrollIntoView({ block: 'nearest', inline: 'nearest', container: 'nearest' });
}

tablist.addEventListener('click', (e) => {
  const tab = e.target.closest('[role="tab"]');
  if (tab) selectTab(tab);
});
tablist.addEventListener('focusin', (e) => {
  const tab = e.target.closest('[role="tab"]');
  if (tab && tab.getAttribute('aria-selected') !== 'true') selectTab(tab);
});
for (const tab of tabs) {
  panelOf(tab).addEventListener('beforematch', () => selectTab(tab));
}
if (!hasFocusGroup) {
  tabs.forEach((t) => { t.tabIndex = t.getAttribute('aria-selected') === 'true' ? 0 : -1; });
  tablist.addEventListener('keydown', (e) => {
    const i = tabs.indexOf(document.activeElement);
    if (i < 0) return;
    const next = {
      ArrowRight: (i + 1) % tabs.length,
      ArrowLeft: (i - 1 + tabs.length) % tabs.length,
      Home: 0,
      End: tabs.length - 1,
    }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    tabs[next].focus();
  });
}

/* Copy buttons: <button command="--copy" commandfor="code-id"> */
const timers = new WeakMap();
const lastMsg = new WeakMap();

function showTip(button, message) {
  const tip = button.parentElement.querySelector('.tip');
  clearTimeout(timers.get(tip));
  const text = lastMsg.get(tip) === message ? message + '\u00A0' : message;
  lastMsg.set(tip, text);
  tip.textContent = text;
  tip.classList.add('show');
  timers.set(tip, setTimeout(() => { tip.classList.remove('show'); tip.textContent = ''; }, 1600));
}

async function copyFrom(target, button) {
  const text = target.dataset.copy ?? target.textContent;
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = Object.assign(document.createElement('textarea'), { value: text });
    ta.style.cssText = 'position:fixed;opacity:0';
    document.body.append(ta);
    ta.select();
    const ok = document.execCommand('copy');
    ta.remove();
    if (!ok) return showTip(button, '复制失败');
  }
  showTip(button, '已复制');
}

document.addEventListener('command', (e) => {
  if (e.command === '--copy') copyFrom(e.target, e.source);
}, true);

if (!('CommandEvent' in window)) {
  document.addEventListener('click', (e) => {
    const button = e.target.closest('button[command="--copy"]');
    const target = button && document.getElementById(button.getAttribute('commandfor'));
    if (target) copyFrom(target, button);
  });
}
