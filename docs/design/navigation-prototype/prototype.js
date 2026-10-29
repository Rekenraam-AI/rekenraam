'use strict';

// Design fixtures only. This artifact has no API client or financial calculator.
const sections = {
  Ledger: [['Transactions', '/app/transactions'], ['Accounts', '/app/accounts'], ['Categories', '/app/categories'], ['Payees', '/app/payees'], ['Import', '/app/import'], ['Reconcile', '/app/reconcile']],
  Investments: [['Overview', '/app/investments'], ['Instruments', '/app/investments/instruments'], ['Returns', '/app/investments/returns'], ['Income', '/app/investments/income'], ['Gains', '/app/investments/gains'], ['Prices', '/app/pricing']],
  Planning: [['Budgets', '/app/budgets'], ['Recurring', '/app/recurring'], ['Forecast', '/app/forecast']],
  // Gains belongs to Investments; Reports lists it in its menu only, so a tab row never holds another section's page.
  Reports: [['Spending', '/app/reports/spending'], ['Cashflow', '/app/reports/cashflow'], ['Net worth', '/app/reports/net-worth'], ['Gains', '/app/investments/gains'], ['Export', '/app/reports/export']],
  Settings: [['Settings', '/app/settings'], ['Security', '/app/settings/security'], ['Backups and data', '/app/settings/data'], ['Language', '/app/settings/language'], ['Appearance', '/app/settings/appearance'], ['Currencies', '/app/settings/currencies'], ['Trash', '/app/settings/trash']]
};
const settingsMenu = [['Settings', '/app/settings'], ['Security', '/app/settings/security'], ['Backups and data', '/app/settings/data'], ['Language and appearance', '/app/settings/language'], ['Trash', '/app/settings/trash']];
const accounts = [
  { id: '1', name: 'Checking', detail: 'Asset · ING · EUR', balance: '€4,210.55', pinned: true },
  { id: '2', name: 'Savings', detail: 'Asset · ASN · EUR', balance: '€31,500.00', pinned: true },
  { id: '3', name: 'Brokerage', detail: 'Asset · DEGIRO · EUR', balance: '€12,000.00', pinned: true },
  { id: '4', name: 'VWRL holding', detail: 'DEGIRO › VWRL · holding account', balance: '€8,400.00', holding: true },
  { id: '5', name: 'Old checking', detail: 'Asset · ING · closed · EUR', balance: '€0.00', closed: true }
];
const categories = [{ id: '10', name: 'Groceries', detail: 'Everyday › Groceries' }, { id: '11', name: 'Energy', detail: 'Housing › Energy' }];
const instruments = [{ id: '20', name: 'VWRL', detail: 'Vanguard FTSE All-World · held at DEGIRO' }];
const payees = [{ id: '30', name: 'Albert Heijn', count: 214, category: 'Groceries' }, { id: '31', name: 'Jumbo', count: 61, category: 'Groceries' }];
const registry = [{ name: 'Home', path: '/app', section: 'Home' }];
for (const [section, pages] of Object.entries(sections)) {
  for (const [name, path] of pages) {
    if (!registry.some(page => page.path === path)) registry.push({ name, path, section });
  }
}
const $ = selector => document.querySelector(selector);
const esc = value => String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const href = path => `#${path}`;
const link = (label, path, cls = '') => `<a class="${cls}" href="${esc(href(path))}">${label}</a>`;
const paths = {
  plus: '<path d="M12 5v14M5 12h14"/>',
  transfer: '<path d="M4 8h15l-4-4M20 16H5l4 4"/>',
  upload: '<path d="M12 15V4m-5 5 5-5 5 5M5 20h14"/>',
  list: '<path d="M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01"/>',
  chart: '<path d="M4 4v16h16M9 16v-5m5 5V7m5 9v-3"/>',
  bank: '<path d="M3 21h18M5 10h14M12 3 4 8h16zM6 10v8m4-8v8m4-8v8m4-8v8"/>',
  scale: '<path d="M12 3v18M7 21h10M4 7h16M7 7l-3 7a3 3 0 0 0 6 0zm10 0-3 7a3 3 0 0 0 6 0z"/>',
  trend: '<path d="m22 7-8.5 8.5-5-5L2 17M16 7h6v6"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  eye: '<path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
  eyeOff: '<path d="m3 3 18 18M10.6 5.1A10 10 0 0 1 12 5c6.4 0 10 7 10 7a17 17 0 0 1-3.2 4.2M6.6 6.6C3.7 8.4 2 12 2 12s3.6 7 10 7a9.7 9.7 0 0 0 5.4-1.6M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
  gear: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.8 1.2V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-2.9-1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.7 1.7 0 0 0 3.1 14H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.2-2.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.7 1.7 0 0 0 10 3.1V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0 1.2 2.9h.1a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>',
  sliders: '<path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6"/>',
  more: '<circle cx="5" cy="12" r="1.2"/><circle cx="12" cy="12" r="1.2"/><circle cx="19" cy="12" r="1.2"/>',
  chevron: '<path d="m6 9 6 6 6-6"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>'
};
const icon = (name, cls = '') => `<svg class="ic ${cls}" viewBox="0 0 24 24" aria-hidden="true" focusable="false">${paths[name]}</svg>`;
let hidden = false;
let theme = 'system';
try { hidden = localStorage.getItem('navigation-prototype-hidden') === 'true'; theme = localStorage.getItem('navigation-prototype-theme') || 'system'; } catch { /* File previews may deny storage. */ }
let scenario = 'everyday';
let route;
let searchScope = '';
let homeScope = '';
let searchResults = [];
let selected = 0;
let returnFocus;
let entryDirty = false;
let reconciliationPreview = false;
let reconciliationFinished = false;
let homeScroll = 0;
let reviewExpanded = false;
let noticeTimer;
// Home layout lives in memory here; production stores it per owner and book on the server (#438).
const defaultLayout = () => ({ headline: 'networth', home: ['spending', 'budgets', 'accounts', 'recurring', 'networth'], shortcuts: ['add', 'transfer', 'import', 'transactions', 'spending', 'checking'], exploreOpen: true });
let layout = defaultLayout();
let undoLayout = null;
const money = value => `<span class="money">${hidden ? '••••••' : esc(value)}</span>`;
const accountPath = id => `/app/accounts/${id}/register`;
const currentAccount = () => accounts.find(account => account.id === (route.path.match(/^\/app\/accounts\/(\d+)\/register$/)?.[1] || route.params.get('account_id'))) || accounts[0];
const phone = () => matchMedia('(max-width: 600px)').matches;
function notify(message, undo = false) {
  clearTimeout(noticeTimer);
  $('#notice').innerHTML = `${esc(message)}${undo ? ' <button data-undo>Undo</button>' : ''}`;
  noticeTimer = setTimeout(() => { $('#notice').textContent = ''; }, undo ? 8000 : 6000);
}
function applyTheme() {
  document.documentElement.dataset.theme = theme === 'system' ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light') : theme;
}
applyTheme();
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', applyTheme);

function readRoute() {
  const url = new URL(location.hash.slice(1) || '/app', 'https://prototype.invalid');
  // Same destinations as the proposed route migration; keep unrelated filters.
  if (url.pathname === '/app/reports') {
    const view = url.searchParams.get('view') || 'spending';
    url.pathname = `/app/reports/${['spending', 'cashflow', 'net-worth'].includes(view) ? view : 'spending'}`;
    url.searchParams.delete('view');
  }
  if (url.pathname === '/app/investments' && url.searchParams.has('tab')) {
    const tab = url.searchParams.get('tab');
    if (['instruments', 'gains'].includes(tab)) url.pathname += `/${tab}`;
    if (tab === 'splits' || tab === 'suggestions') url.searchParams.set('view', tab);
    url.searchParams.delete('tab');
  }
  if (url.pathname === '/app/import/dividends') { url.pathname = '/app/import'; url.searchParams.set('view', 'dividends'); }
  const canonical = url.pathname + url.search;
  if (location.hash !== href(canonical)) history.replaceState(null, '', href(canonical));
  return { path: url.pathname, params: url.searchParams, canonical };
}
function pageInfo() {
  const exact = registry.find(page => page.path === route.path);
  if (exact) return exact;
  if (/^\/app\/accounts\/\d+\/register$/.test(route.path)) return { name: currentAccount().name, section: 'Ledger', object: 'Accounts' };
  if (/^\/app\/categories\/\d+$/.test(route.path)) return { name: categories.find(c => c.id === route.path.split('/').at(-1))?.name || 'Category', section: 'Ledger', object: 'Categories' };
  if (/^\/app\/investments\/instruments\/\d+$/.test(route.path)) return { name: 'VWRL', section: 'Investments', object: 'Instruments' };
  return { name: 'Page not in this prototype', section: 'Home' };
}
function attentionHint(path) {
  if (scenario === 'failed' && ['/app/import', '/app/recurring'].includes(path)) return 'Check unavailable';
  if (scenario === 'empty') return '';
  return path === '/app/import' ? '1 batch' : path === '/app/recurring' ? '2 drafts' : path === '/app/investments/income' ? '3 to assign' : '';
}
function pageLinks(pages, section = '') {
  return pages.map(([name, path]) => {
    const isParent = ['/app/accounts', '/app/categories', '/app/investments/instruments'].includes(path) && route.path.startsWith(`${path}/`);
    const home = registry.find(page => page.path === path)?.section;
    const hint = section && home !== section ? `in ${home}` : attentionHint(path);
    return `<a href="${href(path)}" ${route.path === path ? 'aria-current="page"' : isParent ? 'aria-current="true"' : ''}>${name}<small class="muted">${hint}</small></a>`;
  }).join('');
}
function pins() {
  return scenario === 'empty' ? '<p class="muted">No pinned accounts yet.</p>' : accounts.filter(a => a.pinned).map(account => link(`${account.name} ${money(account.balance)}`, accountPath(account.id))).join('');
}
function renderShell(info) {
  const menus = Object.entries(sections).filter(([name]) => name !== 'Settings').map(([name, pages]) => `
    <div class="nav-item"><button class="nav-trigger" data-disclosure="nav-${name}" aria-expanded="false" aria-controls="nav-${name}" ${info.section === name ? 'aria-current="true"' : ''}>${name} ▾</button>
    <div id="nav-${name}" class="disclosure ${name === 'Ledger' ? 'ledger' : ''}" hidden><div>${pageLinks(pages, name)}</div>${name === 'Ledger' ? `<div><p class="menu-caption">Pinned accounts</p>${pins()}${link('All accounts', '/app/accounts')}</div>` : ''}</div></div>`).join('');
  const isHome = route.path === '/app';
  // Only the section's own pages form its tab row; Settings keeps its side list instead.
  const tabs = info.section !== 'Settings' && sections[info.section] ? sections[info.section].filter(([, path]) => registry.find(page => page.path === path)?.section === info.section) : null;
  $('#shell').innerHTML = `<div class="shell-inner">
    ${link('<span class="brand-mark" aria-hidden="true"><span></span><span></span><span></span><span></span></span><span class="brand-name">Rekenraam</span>', '/app', 'brand')}
    <nav class="desktop-nav" aria-label="Sections"><a class="home-link" href="#/app" ${isHome ? 'aria-current="page"' : ''}>Home</a>${menus}</nav>
    <div class="shell-actions">
      ${isHome ? '' : `<button data-search class="compact-search" aria-label="Search pages, accounts or commands">${icon('search')}<span class="search-copy">Search…</span><kbd class="search-copy">/</kbd></button>`}
      <button id="hide-amounts" class="icon-button" aria-label="Hide amounts" aria-pressed="${hidden}" title="Hide amounts">${icon(hidden ? 'eyeOff' : 'eye')}</button>
      ${isHome ? `<button class="icon-button" data-customize aria-label="Customize Home" title="Customize Home">${icon('sliders')}</button>` : ''}
      <div class="nav-item"><button class="nav-trigger icon-button" data-disclosure="settings-menu" aria-label="Settings and appearance" aria-expanded="false" aria-controls="settings-menu">${icon('gear')}</button>
        <div id="settings-menu" class="disclosure settings" hidden>${pageLinks(settingsMenu)}
          <label>Theme <select id="theme"><option value="light">Light</option><option value="dark">Dark</option><option value="system">System</option></select></label>
          <button data-sign-out>Sign out (sample)</button>
        </div></div>
      <button class="mobile-toggle icon-button" data-disclosure="mobile-pages" aria-label="All pages" aria-expanded="false" aria-controls="mobile-pages">${icon('menu')}</button>
    </div></div>
    <nav id="mobile-pages" class="mobile-pages" aria-label="All pages" hidden>${link('Home', '/app')}${Object.entries(sections).filter(([name]) => name !== 'Settings').map(([name, pages]) => `<details><summary>${name}</summary>${pageLinks(pages, name)}${name === 'Ledger' ? `<p class="menu-caption">Pinned accounts</p>${pins()}` : ''}</details>`).join('')}<details><summary>Settings</summary>${pageLinks(settingsMenu)}</details></nav>
    ${tabs ? `<nav class="section-tabs" aria-label="${info.section} pages">${pageLinks(tabs)}</nav>` : ''}`;
  $('#theme').value = theme;
}
function heading(info, scope = '', add = false) {
  const breadcrumb = info.object ? `<nav class="breadcrumbs" aria-label="Breadcrumb">${link(info.object, info.object === 'Accounts' ? '/app/accounts' : info.object === 'Categories' ? '/app/categories' : '/app/investments/instruments')} › ${esc(info.name)}</nav>` : '';
  return `${breadcrumb}<div class="page-heading"><div><h1>${info.object ? `<button class="object-trigger" data-scope="${info.object}">${esc(info.name)} ▾</button>` : esc(info.name)}</h1>${scope ? `<p class="scope">${scope}</p>` : ''}</div>${add ? '<button class="primary" data-add>Add</button>' : ''}</div>`;
}

// ---- Home -----------------------------------------------------------------

const headlines = {
  networth: { label: 'Net worth', value: '€56,110.55', change: '+€2,140.00', scope: 'All 4 open accounts in EUR · VWRL priced 29 Oct · transactions through 28 Oct' },
  checking: { label: 'Checking', value: '€4,210.55', change: '−€1,320.10', scope: 'Checking · ING · EUR · transactions through 28 Oct' },
  liquid: { label: 'Liquid accounts', value: '€35,710.55', change: '+€2,140.00', scope: 'Liquid accounts: Checking, Savings · EUR · transactions through 28 Oct' }
};
function spark(values, label) {
  const lo = Math.min(...values), hi = Math.max(...values);
  const pts = values.map((v, i) => `${Math.round(i * 600 / (values.length - 1))} ${Math.round(135 - (v - lo) / (hi - lo || 1) * 115)}`);
  const line = `M${pts.join(' L')}`;
  return `<svg class="spark" viewBox="0 0 600 150" preserveAspectRatio="none" role="img" aria-label="${esc(label)}"><path class="spark-area" d="${line} L600 150 L0 150 Z"/><path class="spark-line" d="${line}"/></svg>`;
}
function widgetDefs() {
  const missing = scenario === 'missing';
  const empty = scenario === 'empty';
  const bars = rows => `<div class="w-bars">${rows.map(([name, width, value, path]) => `<div class="bar-row">${link(name, path)}<span class="bar" aria-hidden="true"><span style="width:${width}%"></span></span>${money(value)}</div>`).join('')}</div>`;
  const list = rows => `<ul class="rows">${rows.map(([name, sub, right]) => `<li><span>${name}<br><small class="muted">${sub}</small></span><span>${right}</span></li>`).join('')}</ul>`;
  const defs = [
    { id: 'spending', cat: 'Everyday', title: 'Spending this month', open: ['Open spending report', '/app/reports/spending?from=2026-10-01&to=2026-10-29'], fact: money('€3,720.40'), sub: `of ${money('€5,300.00')} budgeted · 1–29 Oct · expense categories · EUR`,
      body: bars([['Housing', 100, '€1,450.00', '/app/categories/11'], ['Groceries', 42, '€612.30', '/app/categories/10'], ['Transport', 20, '€284.10', '/app/transactions?category_id=12'], ['Other', 95, '€1,373.00', '/app/reports/spending']]) },
    { id: 'budgets', cat: 'Everyday', title: 'Budgets · October', open: ['Open budgets', '/app/budgets?month=2026-10'], fact: `${money('€1,579.60')} left`, sub: 'On pace: 70% spent, 87% of the month gone',
      body: `<ul class="rows">${[['Groceries', '€612.30', '€700.00', ''], ['Dining out', '€231.00', '€200.00', hidden ? 'over' : '€31 over'], ['Transport', '€284.10', '€350.00', '']].map(([name, used, of, note]) => `<li><span>${name}${note ? ` <span class="warning">· ${note}</span>` : ''}</span><span>${money(used)} of ${money(of)}</span></li>`).join('')}</ul>` },
    { id: 'cashflow', cat: 'Everyday', title: 'Cashflow · October', open: ['Open cashflow report', '/app/reports/cashflow'], fact: money('+€2,140.00'), sub: 'Liquid accounts · posted actuals · EUR',
      body: list([['Money in', 'salary, refunds', money('€8,960.00')], ['Money out', 'spending, transfers out', money('€6,820.00')]]) },
    // Shows the owner's pins, the same list as the Ledger menu, rather than a second "largest balances" ranking.
    { id: 'accounts', cat: 'Everyday', title: 'Accounts', open: ['All accounts', '/app/accounts'], fact: '4 open accounts', sub: 'Pinned accounts · balances at 29 Oct',
      body: `<ul class="rows">${accounts.filter(a => a.pinned).map(a => `<li>${link(a.name, accountPath(a.id))}<span>${money(a.balance)}</span></li>`).join('')}</ul><p class="scope">+ 1 more open account · 1 closed</p>` },
    { id: 'networth', cat: 'Wealth & investments', title: 'Net worth · 12 months', wide: true, open: ['Open net worth report', '/app/reports/net-worth'], fact: missing ? 'Unavailable' : money('+€7,210.55'), sub: missing ? 'VWRL has no price for 29 Oct; the total is not shown partially' : 'Nov 2025 – Oct 2026 · month-end values · EUR',
      body: missing ? '' : spark([48900, 49600, 49100, 50400, 51300, 50900, 52200, 53100, 52800, 53970, 54400, 56110], 'Net worth rose from about 48.9 to 56.1 thousand euro over 12 months') },
    { id: 'holdings', cat: 'Wealth & investments', title: 'Holdings', open: ['Open investments', '/app/investments'], fact: '1 position', sub: missing ? 'Known value only · 1 position unpriced' : 'Largest by value · 29 Oct',
      body: list([[link('VWRL', '/app/investments/instruments/20'), 'Vanguard FTSE All-World · DEGIRO', missing ? 'No price' : money('€8,400.00')]]) },
    { id: 'returns', cat: 'Wealth & investments', title: 'Returns · year to date', open: ['Open returns', '/app/investments/returns'], fact: missing ? 'Unavailable' : '+7.4%', sub: 'Time-weighted · 1 Jan – 29 Oct · all investment accounts', body: '' },
    { id: 'allocation', cat: 'Wealth & investments', title: 'Allocation', open: ['Open returns', '/app/investments/returns'], fact: missing ? 'Unavailable' : 'Equity 41%', sub: missing ? '1 position unpriced' : 'Investment accounts · market value 29 Oct',
      body: missing ? '' : '<div class="stack" role="img" aria-label="Equity 41 percent, cash 59 percent"><span style="width:41%"></span><span style="width:59%"></span></div><p class="scope">Equity 41% · Cash 59%</p>' },
    { id: 'income', cat: 'Wealth & investments', title: 'Dividend income', badge: ['Review', 'warn'], open: ['Open income', '/app/investments/income'], fact: '3 to assign', sub: `${money('€412.80')} confirmed this year`,
      body: list([['VWRL', '25 Sep · unassigned cash', money('€96.40')]]) },
    { id: 'recurring', cat: 'Planning', title: 'Recurring', badge: ['2 drafts', 'warn'], open: ['Review drafts', '/app/recurring?tab=due'], fact: 'Next: Rent on 1 Nov', sub: '9 schedules active · 2 drafts awaiting review',
      body: list([['Rent', '1 Nov · Checking', money('−€1,450.00')], ['Energy', '3 Nov · Checking', money('−€142.00')], ['Salary', '25 Nov · Checking', money('+€3,200.00')]]) },
    { id: 'forecast', cat: 'Planning', title: 'Forecast · liquid cash', badge: ['Estimate', 'info'], open: ['Open forecast', '/app/forecast'], fact: `${money('€33,900')} on 31 Dec`, sub: 'Includes recurring schedules and learned spending',
      body: spark([35710, 34900, 35400, 34600, 34100, 35200, 34800, 34300, 33900], 'Projected liquid cash eases to about 33.9 thousand euro by 31 December') },
    { id: 'imports', cat: 'Ledger & data', title: 'Imports', badge: ['Review', 'warn'], open: ['Open import', '/app/import'], fact: '1 batch in review', sub: 'Last import 2 days ago',
      body: list([[link('ING CSV · October', '/app/import?batch_id=october'), '42 rows · 3 need a category', '']]) },
    { id: 'reconcile', cat: 'Ledger & data', title: 'Reconciliation', open: ['Open reconcile', '/app/reconcile'], fact: '2 of 3 accounts', sub: 'reconciled in the last 60 days',
      body: list([['Checking', 'last reconciled', '12 Sep'], ['Savings', 'last reconciled', '30 Sep']]) },
    { id: 'prices', cat: 'Ledger & data', title: 'Prices', badge: missing ? ['Missing', 'warn'] : ['Up to date', 'info'], open: ['Open prices', '/app/pricing'], fact: missing ? 'VWRL has no price' : '1 price from 29 Oct', sub: 'FX rates are in Settings › Currencies',
      body: missing ? `<p>${link('Add a VWRL price', '/app/pricing?base_commodity_id=20')}</p>` : '' },
    { id: 'backups', cat: 'Ledger & data', title: 'Backups', badge: ['Set up', 'setup'], open: ['Open backups and data', '/app/settings/data'], fact: 'Not set up', sub: 'Protect your books with automatic backups',
      body: `<p>${link('Set up backups', '/app/settings/data', 'button')}</p>` }
  ];
  if (empty) for (const d of defs) if (d.id !== 'backups') Object.assign(d, { fact: 'No data yet', sub: 'Appears once you add an account and transactions', body: '', badge: null });
  return defs;
}
const shortcutPool = {
  add: { label: 'Add transaction', icon: 'plus', action: 'add' },
  transfer: { label: 'Transfer', icon: 'transfer', action: 'transfer' },
  import: { label: 'Import file', icon: 'upload', path: '/app/import' },
  transactions: { label: 'Transactions', icon: 'list', path: '/app/transactions' },
  spending: { label: 'Spending by category', icon: 'chart', path: '/app/reports/spending' },
  checking: { label: 'Checking', icon: 'bank', path: accountPath('1') },
  reconcile: { label: 'Reconcile Checking', icon: 'scale', path: '/app/reconcile?account_id=1' },
  savings: { label: 'Savings', icon: 'bank', path: accountPath('2') },
  returns: { label: 'Returns year to date', icon: 'trend', path: '/app/investments/returns' }
};
function reviewItems() {
  if (scenario === 'failed') return '<li><span class="dot warn"></span><span class="t">Review status could not be checked.</span><button data-retry>Retry check</button></li>';
  return [
    ['warn', link('2 recurring drafts awaiting review', '/app/recurring?tab=due')],
    ['warn', link('1 import batch to review · 3 rows need a category', '/app/import?batch_id=october')],
    ['info', link('Note: 3 dividends not yet assigned', '/app/investments/income')],
    ...(scenario === 'missing' ? [['warn', link('VWRL has no price for the valuation date', '/app/pricing?base_commodity_id=20')]] : [])
  ].map(([tone, text]) => `<li><span class="dot ${tone}"></span><span class="t">${text}</span></li>`).join('');
}
function shortcutTile(id, index) {
  const s = shortcutPool[id];
  const inner = `<span class="tile-ic ${id === 'add' ? 'primary' : ''}">${icon(s.icon)}</span><span class="tile-l">${esc(s.label)}</span>`;
  const main = s.path ? link(inner, s.path, 'tile-a') : `<button class="tile-a" ${s.action === 'add' ? 'data-add' : 'data-transfer'}>${inner}</button>`;
  const last = index === layout.shortcuts.length - 1;
  return `<li class="tile ${index >= 3 ? 'extra' : ''}">${main}
    <button class="tile-more" data-disclosure="sc-${id}" aria-expanded="false" aria-controls="sc-${id}" aria-label="Options for shortcut ${esc(s.label)}">${icon('more')}</button>
    <div id="sc-${id}" class="disclosure tile-menu" hidden>
      <button data-sc="left" data-id="${id}" ${index === 0 ? 'disabled' : ''}>Move left</button>
      <button data-sc="right" data-id="${id}" ${last ? 'disabled' : ''}>Move right</button>
      <button data-sc="remove" data-id="${id}" ${id === 'add' ? 'disabled' : ''}>${id === 'add' ? 'Always available' : 'Remove shortcut'}</button>
    </div></li>`;
}
function renderShortcuts() {
  const left = Object.keys(shortcutPool).filter(id => !layout.shortcuts.includes(id));
  const extra = layout.shortcuts.slice(3);
  return `<ul class="shortcuts" aria-label="Shortcuts">${layout.shortcuts.map(shortcutTile).join('')}
    <li class="tile ${left.length ? '' : 'none-left'} add-tile"><button class="tile-a" data-disclosure="sc-new" aria-expanded="false" aria-controls="sc-new"><span class="tile-ic add">${icon('plus')}</span><span class="tile-l">Add shortcut</span></button>
      <div id="sc-new" class="disclosure tile-menu" hidden><p class="menu-caption">Actions, pages and saved views</p>${left.map(id => `<button data-sc-add="${id}">${esc(shortcutPool[id].label)}</button>`).join('') || '<p class="muted">Every shortcut is on Home.</p>'}</div></li>
    <li class="tile more-tile"><button class="tile-a" data-disclosure="sc-more" aria-expanded="false" aria-controls="sc-more"><span class="tile-ic">${icon('more')}</span><span class="tile-l">More</span></button>
      <div id="sc-more" class="disclosure tile-menu" hidden>${extra.map(id => shortcutPool[id].path ? link(esc(shortcutPool[id].label), shortcutPool[id].path) : `<button ${shortcutPool[id].action === 'add' ? 'data-add' : 'data-transfer'}>${esc(shortcutPool[id].label)}</button>`).join('')}<button data-customize>Customize shortcuts…</button></div></li>
  </ul>`;
}
function widgetCard(d, zone) {
  const onHome = zone === 'home';
  const i = layout.home.indexOf(d.id);
  const menu = onHome ? `<button data-w="up" data-id="${d.id}" ${i === 0 ? 'disabled' : ''}>Move up</button><button data-w="down" data-id="${d.id}" ${i === layout.home.length - 1 ? 'disabled' : ''}>Move down</button><button data-w="settings" data-id="${d.id}">Settings…</button><button data-w="remove" data-id="${d.id}">Remove from My Home</button>`
    : `<button data-w="add" data-id="${d.id}">Add to My Home</button><button data-w="top" data-id="${d.id}">Add to top of My Home</button>`;
  return `<article class="card widget ${d.wide && onHome ? 'wide' : ''} ${onHome ? '' : 'explore'}" id="${zone}-${d.id}" aria-labelledby="${zone}-${d.id}-title">
    <div class="w-head"><h3 id="${zone}-${d.id}-title">${esc(d.title)}</h3>${d.badge ? `<span class="badge ${d.badge[1]}">${esc(d.badge[0])}</span>` : ''}
      <button class="w-more" data-disclosure="${zone}-${d.id}-menu" aria-expanded="false" aria-controls="${zone}-${d.id}-menu" aria-label="Options for ${esc(d.title)}">${icon('more')}</button>
      <div id="${zone}-${d.id}-menu" class="disclosure w-menu" hidden>${menu}${link(`${esc(d.open[0])}`, d.open[1])}</div></div>
    <p class="fact">${d.fact}</p><p class="scope">${d.sub}</p>${d.body}
    <div class="w-foot">${onHome ? '' : `<button class="add-home" data-w="add" data-id="${d.id}" aria-label="Add ${esc(d.title)} to My Home">${icon('plus')}Add to My Home</button>`}${link(`${esc(d.open[0])} →`, d.open[1], 'w-open')}</div>
  </article>`;
}
function renderHome() {
  const empty = scenario === 'empty';
  const h = headlines[layout.headline];
  const missing = scenario === 'missing' && layout.headline === 'networth';
  const line1 = empty ? 'Your book is ready for its first account'
    : missing ? `Net worth <span class="muted">not available</span> · VWRL has no price · ${link('Add a price', '/app/pricing?base_commodity_id=20')}`
    : `${h.label} <strong>${money(h.value)}</strong> <span class="muted">·</span> ${money(h.change)} <span class="muted">since 1 Oct</span>`;
  const switcher = empty ? '' : `<div class="nav-item"><button class="bl-switch" data-disclosure="headline-menu" aria-expanded="false" aria-controls="headline-menu" aria-label="Change headline">${icon('chevron')}</button>
    <div id="headline-menu" class="disclosure center" hidden>${Object.entries(headlines).map(([id, v]) => `<button data-headline="${id}" aria-pressed="${layout.headline === id}">${v.label}<small class="muted">${id === 'networth' ? 'default' : id === 'checking' ? 'one account' : 'account set'}</small></button>`).join('')}<button data-customize>Customize Home…</button></div></div>`;
  const line2 = empty ? '<p class="bl"><span class="ok">All clear, nothing to review</span></p>'
    : `<p class="bl"><button id="home-review" class="bl-review" aria-expanded="${reviewExpanded}" aria-controls="home-review-list"><span class="dot warn"></span>${scenario === 'failed' ? '<span class="warning">Review status unavailable</span>' : '<span><strong>2 drafts</strong> and <strong>1 import</strong> to review</span>'}${icon('chevron', reviewExpanded ? 'flip' : '')}</button></p>`;
  const scope = empty ? 'No accounts or transactions yet' : missing ? 'All 4 open accounts in EUR · 1 required price missing · transactions through 28 Oct' : h.scope;
  const defs = widgetDefs();
  const byId = Object.fromEntries(defs.map(d => [d.id, d]));
  const rest = defs.filter(d => !layout.home.includes(d.id));
  const groups = ['Everyday', 'Wealth & investments', 'Planning', 'Ledger & data'].map(cat => [cat, rest.filter(d => d.cat === cat)]);
  return `<section class="hero" aria-label="Briefing">
      <div class="brief"><div class="bl headline"><h1>${line1}</h1>${switcher}</div>${line2}<p class="scope">${scope}</p>
      ${empty ? '' : `<ul id="home-review-list" class="slist" ${reviewExpanded ? '' : 'hidden'}>${reviewItems()}</ul>`}</div>
      <div class="cmd" role="search">
        <div class="cmd-box">${icon('search')}${homeScope ? `<button id="home-scope-chip" class="chip" aria-label="Remove ${homeScope} scope">${homeScope} ×</button>` : ''}
          <label class="sr-only" for="home-cmd">Search pages, accounts, payees and commands</label>
          <input id="home-cmd" type="search" role="combobox" aria-expanded="false" aria-controls="home-cmd-list" aria-autocomplete="list" autocomplete="off" placeholder="Search or type a payee">
          <kbd>/</kbd></div>
        <div id="home-cmd-panel" class="cmd-drop" hidden><div id="home-cmd-chips" class="chips"></div><div id="home-cmd-list" role="listbox" aria-label="Search results"></div></div>
      </div>
      ${renderShortcuts()}
    </section>
    <section class="home-section" aria-labelledby="my-home-title"><div class="section-head"><h2 id="my-home-title">My Home</h2><span class="muted">${layout.home.length} widgets</span></div>
      ${empty ? `<div class="card onboarding"><h3>Start with an account</h3><p>Add an account before entering or importing transactions. The widgets below fill in as data arrives.</p>${link('Set up accounts', '/app/accounts', 'button primary')}</div>` : ''}
      ${layout.home.length ? `<div class="grid">${layout.home.map(id => widgetCard(byId[id], 'home')).join('')}</div>` : '<p class="muted">My Home is empty. Add widgets from More to explore below.</p>'}
    </section>
    <section class="home-section" id="explore" aria-labelledby="explore-title"><div class="section-head"><h2 id="explore-title">More to explore</h2><span class="muted">${rest.length} more widgets, shown as they would look on your Home</span>
      <button id="explore-toggle" aria-expanded="${layout.exploreOpen}" aria-controls="explore-body">${layout.exploreOpen ? 'Collapse' : 'Show all'}</button></div>
      <div id="explore-body" ${layout.exploreOpen ? '' : 'hidden'}>${groups.map(([cat, items]) => `<h3 class="cat">${cat} <span class="muted">${items.length}</span></h3>${items.length ? `<div class="grid">${items.map(d => widgetCard(d, 'explore')).join('')}</div>` : '<p class="muted">Everything in this group is already on your Home.</p>'}`).join('')}</div>
    </section>`;
}
function changeLayout(patch, message, undoable = true) {
  undoLayout = undoable ? structuredClone(layout) : null;
  layout = { ...layout, ...patch };
  render();
  if (message) notify(message, undoable);
  if ($('#customize-dialog').open) renderCustomize();
}
const swap = (list, i, j) => { const copy = list.slice(); [copy[i], copy[j]] = [copy[j], copy[i]]; return copy; };
function renderCustomize() {
  const defs = Object.fromEntries(widgetDefs().map(d => [d.id, d]));
  const row = (list, id, label, removable) => {
    const i = layout[list].indexOf(id);
    return `<li><span>${esc(label)}</span><span class="actions-inline"><button data-cz="up" data-list="${list}" data-id="${id}" aria-label="Move ${esc(label)} ${list === 'home' ? 'up' : 'earlier'}" ${i === 0 ? 'disabled' : ''}>↑</button><button data-cz="down" data-list="${list}" data-id="${id}" aria-label="Move ${esc(label)} ${list === 'home' ? 'down' : 'later'}" ${i === layout[list].length - 1 ? 'disabled' : ''}>↓</button><button data-cz="remove" data-list="${list}" data-id="${id}" aria-label="Remove ${esc(label)}" ${removable ? '' : 'disabled'}>×</button></span></li>`;
  };
  $('#customize-body').innerHTML = `
    <fieldset><legend>Headline</legend>${Object.entries(headlines).map(([id, v]) => `<label class="radio"><input type="radio" name="cz-headline" value="${id}" ${layout.headline === id ? 'checked' : ''}> ${v.label}</label>`).join('')}</fieldset>
    <h3>Shortcuts</h3><p class="scope">Add transaction can be moved but not removed.</p><ul class="cz-list">${layout.shortcuts.map(id => row('shortcuts', id, shortcutPool[id].label, id !== 'add')).join('')}</ul>
    <h3>My Home order</h3><p class="scope">Top to bottom. Anything removed goes back to More to explore.</p><ul class="cz-list">${layout.home.map(id => row('home', id, defs[id].title, true)).join('')}</ul>
    <label class="radio"><input type="checkbox" id="cz-explore" ${layout.exploreOpen ? 'checked' : ''}> Show More to explore expanded</label>
    <div class="actions"><button data-cz="reset">Reset Home</button><button class="primary" data-close="customize-dialog">Done</button></div>`;
}

// ---- Other pages ----------------------------------------------------------

function filters() {
  return `<form id="filters" class="filters"><label>Month<input name="month" type="month" value="${esc(route.params.get('month') || '2026-03')}"></label><label>Find in entries<input name="q" value="${esc(route.params.get('q') || '')}"></label><button>Apply filters</button></form>`;
}
function registerRows(account) {
  if (account.id !== '1') return `<tr><td>01 Mar</td><td>${account.id === '2' ? 'Opening savings' : 'Sample balance'}</td><td>Opening balance</td><td class="numeric">${money(account.balance)}</td></tr>`;
  return `<tr><td>27 Mar</td><td>${link('Albert Heijn', '/app/transactions?payee_id=30')}</td><td>${link('Groceries', '/app/categories/10')}</td><td class="numeric">${money('−€42.15')}</td></tr><tr><td>25 Mar</td><td>Salary</td><td>Income</td><td class="numeric">${money('+€3,200.00')}</td></tr>`;
}
function renderRegister(info) {
  const account = currentAccount();
  return `${heading(info, `${esc(account.detail)} · ${money(account.balance)} at 29 Oct · filters below apply to entries`, true)}${filters()}
    ${route.params.has('dropped') ? `<p role="status" class="scope">Removed incompatible filter: ${esc(route.params.get('dropped'))}.</p>` : ''}
    <section class="card"><h2>Entries</h2><div class="table-region" role="region" aria-label="Account entries" tabindex="0"><table><thead><tr><th>Date</th><th>Payee</th><th>Category</th><th class="numeric">Amount</th></tr></thead><tbody>${registerRows(account)}</tbody></table></div>
    <p class="scope">Illustrative rows; the prototype does not recalculate account balances.</p></section>
    <div class="actions">${link('Reconcile this account', `/app/reconcile?account_id=${account.id}`, 'button')}${account.holding ? link('Open VWRL instrument', '/app/investments/instruments/20', 'button') : ''}</div>`;
}
function renderSpending(info) {
  const rows = [['Housing', '€1,450.00', '100%', '/app/categories/11'], ['Groceries', '€612.30', '42%', '/app/categories/10'], ['Transport', '€284.10', '20%', '/app/transactions?category_id=12']];
  return `${heading(info, '1–29 Oct 2026 · all expense categories · EUR · posted actuals')}
    <div class="split-layout"><section class="card"><h2>Spending by category</h2><p class="fact">${money('€3,720.40')}</p>${rows.map(([name, value, width, path]) => `<div class="bar-row">${link(name, path)}<span class="bar" aria-hidden="true"><span style="width:${width}"></span></span>${money(value)}</div>`).join('')}<p class="scope">Top three categories shown. Other categories total ${money('€1,373.00')}.</p></section>
    <aside class="card"><h2>Explore this report</h2><p>Category links open their detail page. Your browser Back button returns here.</p>${link('Transactions in this period', '/app/transactions?from=2026-10-01&to=2026-10-29', 'button')}</aside></div>`;
}
function renderImport(info) {
  const dividends = route.params.get('view') === 'dividends';
  const batch = route.params.has('batch_id');
  return `${heading(info, 'Review source rows before committing. No real upload or commit occurs here.')}
    <nav class="segmented" aria-label="Import views">${link('Files', '/app/import', dividends ? '' : 'on')}${link('Dividends', '/app/import?view=dividends', dividends ? 'on' : '')}</nav>
    <section class="card"><h2>${dividends ? 'Dividend review' : batch ? 'ING CSV · October' : 'File imports'}</h2>
      ${dividends ? `<p>3 sample payments need optional attribution.</p>${link('Open income coverage', '/app/investments/income', 'button')}` : batch ? `<p><span class="badge">In review</span> 42 source rows · 3 need a category</p><p>Known cash remains visible while incomplete rows stay in review.</p><label>Category for sample row <select id="import-category"><option value="">Choose category…</option><option>Groceries</option><option>Housing</option></select></label><div class="actions"><button data-review-import>Review sample changes</button>${link('Back to batches', '/app/import')}</div>` : `<ul class="rows"><li><span>ING CSV · October<br><small class="muted">42 rows · 3 need a category</small></span>${link('Continue review', '/app/import?batch_id=october', 'button')}</li></ul>`}
    </section>`;
}
function renderReconcile(info) {
  const account = currentAccount();
  return `${heading(info, `${esc(account.detail)} · sample statement at 29 Oct`)}
    <section class="card"><h2>Statement for <button class="object-trigger" data-scope="Accounts">${esc(account.name)} ▾</button></h2>
    ${reconciliationFinished ? '<p role="status">Sample reconciliation finished. No checkpoint was written.</p>' : `<p>Statement balance ${money(account.balance)} · difference ${money('€0.00')}</p><p>Review the selected entries and exact statement before finishing.</p>${reconciliationPreview ? `<div><h3>Review checkpoint</h3><p>Account: ${esc(account.name)} · 29 Oct 2026 · difference ${money('€0.00')}</p><button class="primary" data-finish-reconcile>Finish sample reconciliation</button></div>` : '<button class="primary" data-preview-reconcile>Preview reconciliation</button>'}`}
    <div class="actions">${link('Open account register', accountPath(account.id))}</div></section>`;
}
function renderIndex(info, objects, makePath) {
  return `${heading(info, 'Sample index · detail pages also support scoped search', info.name === 'Categories')}
    <section class="card"><h2>${esc(info.name)}</h2><ul class="rows">${objects.map(object => `<li><span>${link(esc(object.name), makePath(object.id))}<br><small class="muted">${esc(object.detail)}</small></span>${object.balance ? money(object.balance) : ''}</li>`).join('')}</ul></section>`;
}
function renderSettings(info) {
  return `${heading(info)}<div class="settings-layout"><nav class="side-list" aria-label="Settings pages">${pageLinks(sections.Settings)}</nav>
    <section class="card"><h2>${esc(info.name)}</h2><p>Settings destination only; live security, backup and preference changes remain in the application.</p>${info.name === 'Backups and data' ? link('Ledger CSV and QIF export →', '/app/reports/export') : info.name === 'Currencies' ? '<p>EUR · USD · FX-rate history stays in Currencies.</p>' : '<p>Use the header settings menu to try Light, Dark or System.</p>'}</section></div>`;
}
function renderOther(info) {
  if (info.name === 'Accounts') return renderIndex(info, accounts, accountPath);
  if (info.name === 'Categories') return renderIndex(info, categories, id => `/app/categories/${id}`);
  if (route.path === '/app/investments/instruments') return renderIndex(info, instruments, id => `/app/investments/instruments/${id}`);
  if (info.object === 'Categories') return `${heading(info, 'October · expense category · posted actuals', true)}<section class="card"><h2>Category activity</h2><p>${link('View matching transactions', `/app/transactions?category_id=${route.path.split('/').at(-1)}`)}</p>${link('Open spending report', '/app/reports/spending')}</section>`;
  if (info.object === 'Instruments') return `${heading(info, 'Held at DEGIRO · EUR')}<section class="card"><h2>Holding and split history</h2>${link('Open VWRL holding register', accountPath('4'), 'button')}<p class="scope">No sample splits. Whole-book split history remains reachable from Overview.</p></section>`;
  if (info.name === 'Transactions') return `${heading(info, `Sample entries${route.params.has('payee_id') ? ' · payee: Albert Heijn' : ''}${route.params.has('category_id') ? ' · selected category' : ''}${route.params.has('q') ? ` · matching “${esc(route.params.get('q'))}”` : ''}`, true)}${filters()}<section class="card"><h2>Transactions</h2>${link('Open Checking register', accountPath('1'), 'button')}<p class="scope">Global search and report links retain their URL filters.</p></section>`;
  if (info.name === 'Payees') return `${heading(info, 'Payee management belongs to #443')}<section class="card"><h2>Albert Heijn</h2><p>214 transactions · last used 27 Oct</p>${link('Transactions with Albert Heijn', '/app/transactions?payee_id=30', 'button')}</section>`;
  if (info.name === 'Recurring') return `${heading(info, 'Drafts and templates are separate views')}<nav class="segmented" aria-label="Recurring views">${link('Due', '/app/recurring?tab=due', route.params.get('tab') === 'templates' ? '' : 'on')}${link('Templates', '/app/recurring?tab=templates', route.params.get('tab') === 'templates' ? 'on' : '')}</nav><section class="card"><h2>${route.params.get('tab') === 'templates' ? 'Templates' : '2 drafts awaiting review'}</h2><p>Sample review destination; posting drafts is outside this navigation prototype.</p></section>`;
  if (info.name === 'Net worth') return `${heading(info, '29 Oct 2026 · same 4 open sample accounts in EUR')}<section class="card"><h2>Valuation</h2><p class="fact">${scenario === 'missing' ? 'Unavailable · VWRL price missing' : money('€56,110.55')}</p>${link('Inspect accounts', '/app/accounts')}</section>`;
  if (info.name === 'Export') return `${heading(info)}<section class="card"><h2>Ledger CSV and QIF</h2><p>Navigation destination only. No sample files are generated.</p>${link('Backup and restore settings', '/app/settings/data')}</section>`;
  if (info.section === 'Settings') return renderSettings(info);
  return `${heading(info)}<section class="card"><h2>${esc(info.name)} destination</h2><p>This screen establishes navigation and page placement. Its detailed workflow is outside this prototype.</p>${info.section === 'Investments' ? `<div class="actions">${link('Instruments', '/app/investments/instruments')}${link('Split history', '/app/investments?view=splits')}${link('Review suggestions', '/app/investments?view=suggestions')}</div>${route.params.has('view') ? `<p role="status">Selected view: ${esc(route.params.get('view'))}</p>` : ''}` : ''}${link('Back to Home', '/app', 'button')}</section>`;
}
function render(navigated = false) {
  const old = route;
  if (old?.path === '/app' && navigated) homeScroll = lastHomeScroll;
  route = readRoute();
  const info = pageInfo();
  if (old?.path === '/app/reconcile' && old.canonical !== route.canonical) { reconciliationPreview = false; reconciliationFinished = false; }
  renderShell(info);
  $('#main').className = info.object === 'Accounts' || ['Transactions', 'Import'].includes(info.name) ? 'fluid' : route.path === '/app' ? 'home' : '';
  $('#main').innerHTML = route.path === '/app' ? renderHome() : info.object === 'Accounts' ? renderRegister(info) : route.path === '/app/reports/spending' ? renderSpending(info) : info.name === 'Import' ? renderImport(info) : info.name === 'Reconcile' ? renderReconcile(info) : renderOther(info);
  document.title = `${info.name} · Rekenraam design prototype`;
  if (navigated) { $('#main').focus({ preventScroll: true }); requestAnimationFrame(() => scrollTo(0, route.path === '/app' ? homeScroll : 0)); }
}
// The hash changes before render runs, so remember Home's scroll as it happens.
let lastHomeScroll = 0;
addEventListener('scroll', () => { if (route?.path === '/app') lastHomeScroll = scrollY; }, { passive: true });
render();
window.addEventListener('hashchange', () => render(true));

function closeDisclosures() {
  for (const trigger of document.querySelectorAll('[data-disclosure]')) {
    trigger.setAttribute('aria-expanded', 'false');
    const panel = document.getElementById(trigger.dataset.disclosure);
    if (panel) panel.hidden = true;
  }
  closeHomeList();
}

// ---- Search: one engine for Home's bar, the header field and object switchers ----

function scopeDestination(kind, id, scoped) {
  const path = kind === 'Accounts' ? (route.path === '/app/reconcile' ? '/app/reconcile' : accountPath(id)) : kind === 'Categories' ? `/app/categories/${id}` : `/app/investments/instruments/${id}`;
  const params = scoped ? new URLSearchParams(route.params) : new URLSearchParams();
  params.delete('dropped');
  if (kind === 'Accounts' && scoped) {
    if (params.has('holding_id')) { params.delete('holding_id'); params.set('dropped', 'holding selection'); }
    if (route.path === '/app/reconcile') params.set('account_id', id);
  }
  return path + (params.size ? `?${params}` : '');
}
function buildResults(raw, scope, showClosed = false) {
  const query = raw.toLocaleLowerCase().trim();
  const matches = text => text.toLocaleLowerCase().includes(query);
  const amount = a => hidden ? '••••••' : a.balance;
  if (scope) {
    // Scoped from an object page keeps that page's context; scoped from Home opens the default page.
    const keep = route.path !== '/app';
    const sources = scope === 'Accounts' ? accounts : scope === 'Categories' ? categories : instruments;
    const items = sources.filter(item => matches(`${item.name} ${item.detail}`) && (!item.closed || showClosed) && (!item.holding || query));
    const row = item => ({ label: item.name, detail: `${item.detail}${item.balance ? ` · ${amount(item)}` : ''}`, path: scopeDestination(scope, item.id, keep) });
    if (scope !== 'Accounts' || query) return items.map(row);
    return [...items.filter(a => a.pinned).map(a => ({ ...row(a), group: 'Pinned' })), ...items.filter(a => !a.pinned).map(a => ({ ...row(a), group: 'Other accounts' }))];
  }
  if (!query) {
    return [
      { label: 'Add transaction', detail: 'Command · opens the editor', action: 'add', group: 'Suggested' },
      ...['/app/transactions', '/app/reports/spending', '/app/investments/returns'].map(path => registry.find(p => p.path === path)).map(p => ({ label: p.name, detail: `Page · ${p.section}`, path: p.path, group: 'Suggested' })),
      { label: 'Checking', detail: `Pinned account · ${amount(accounts[0])}`, path: accountPath('1'), group: 'Suggested' }
    ];
  }
  const out = [];
  const payee = payees.find(p => matches(p.name));
  if (payee) {
    const group = `Payee · ${payee.name}`;
    out.push({ label: `Add expense at ${payee.name}`, detail: `Opens the editor prefilled · ${payee.category} (last used)`, action: 'add', payee, group });
    out.push({ label: `Transactions with ${payee.name}`, detail: `${payee.count} transactions · filtered by payee`, path: `/app/transactions?payee_id=${payee.id}`, group });
  }
  const group = 'Pages, accounts and categories';
  const commands = [
    { label: 'Add transaction', detail: 'Command', action: 'add' },
    { label: 'Transfer between accounts', detail: 'Command', action: 'transfer' },
    { label: 'Import a file', detail: 'Command', path: '/app/import' },
    { label: 'Reconcile an account', detail: 'Command', path: '/app/reconcile' }
  ];
  out.push(...[
    ...commands,
    ...registry.map(page => ({ label: page.name, detail: `Page · ${page.section}`, path: page.path })),
    ...accounts.filter(a => !a.closed).map(a => ({ label: a.name, detail: `Account · ${a.detail} · ${amount(a)}`, path: accountPath(a.id) })),
    ...categories.map(c => ({ label: c.name, detail: `Category · ${c.detail}`, path: `/app/categories/${c.id}` })),
    ...instruments.map(i => ({ label: i.name, detail: `Instrument · ${i.detail}`, path: `/app/investments/instruments/${i.id}` }))
  ].filter(item => matches(`${item.label} ${item.detail}`)).slice(0, 8).map(item => ({ ...item, group })));
  out.push({ label: `Search all transactions for “${raw.trim()}”`, detail: 'Payees, descriptions, references and notes', path: `/app/transactions?q=${encodeURIComponent(raw.trim())}`, group: 'Everything' });
  return out;
}
function paintResults(list, input, items, prefix, empty = 'No matching objects') {
  searchResults = items;
  selected = 0;
  let html = '';
  let current;
  items.forEach((item, index) => {
    if (item.group && item.group !== current) {
      if (current !== undefined) html += '</div>';
      current = item.group;
      html += `<div role="group" aria-labelledby="${prefix}-g${index}"><div class="dh" id="${prefix}-g${index}" role="presentation">${esc(item.group)}</div>`;
    }
    html += `<div class="result" id="${prefix}-${index}" role="option" aria-selected="${index === 0}" data-result="${index}">${esc(item.label)}<small>${esc(item.detail)}</small></div>`;
  });
  if (current !== undefined) html += '</div>';
  list.innerHTML = items.length ? html : `<div role="option" aria-disabled="true" aria-selected="false" class="result">${empty}</div>`;
  updateSelection(input, prefix);
}
function updateSelection(input, prefix) {
  input.removeAttribute('aria-activedescendant');
  for (const item of document.querySelectorAll(`[id^="${prefix}-"][data-result]`)) item.setAttribute('aria-selected', String(Number(item.dataset.result) === selected));
  if (searchResults.length) input.setAttribute('aria-activedescendant', `${prefix}-${selected}`);
}
function chooseResult(index) {
  const result = searchResults[index];
  if (!result) return;
  if ($('#search-dialog').open) $('#search-dialog').close();
  closeHomeList();
  if (result.action === 'add') openEntry(result.payee);
  else if (result.action === 'transfer') notify('Transfer editor is outside this navigation prototype. Use Add to test the shared entry interaction.');
  else location.hash = href(result.path);
}
function searchKeys(event, input, prefix, onEscape) {
  if (['ArrowDown', 'ArrowUp'].includes(event.key) && searchResults.length) {
    event.preventDefault();
    selected = (selected + (event.key === 'ArrowDown' ? 1 : -1) + searchResults.length) % searchResults.length;
    updateSelection(input, prefix);
    document.getElementById(`${prefix}-${selected}`).scrollIntoView({ block: 'nearest' });
  } else if (event.key === 'Enter') { event.preventDefault(); chooseResult(selected); }
  else if (event.key === 'Backspace' && !input.value && onEscape.scope) { onEscape.scope(); }
  else if (event.key === 'Escape' && onEscape.close) { event.preventDefault(); onEscape.close(); }
}
function openSearch(scope = '') {
  returnFocus = document.activeElement;
  closeDisclosures();
  searchScope = scope;
  $('#search-input').value = '';
  $('#show-closed').checked = false;
  updateSearch();
  $('#search-dialog').showModal();
  $('#search-input').focus();
}
function updateSearch() {
  $('#scope-chip').hidden = !searchScope;
  $('#scope-chip').textContent = `${searchScope} ×`;
  $('#scope-chip').setAttribute('aria-label', `Remove ${searchScope} scope`);
  $('#closed-control').hidden = searchScope !== 'Accounts';
  $('#scope-options').innerHTML = searchScope ? '' : ['Accounts', 'Categories', 'Instruments'].map(scope => `<button class="chip" data-search-scope="${scope}">${scope}</button>`).join('');
  paintResults($('#search-results'), $('#search-input'), buildResults($('#search-input').value, searchScope, $('#show-closed').checked), 'result');
}
$('#search-input').addEventListener('input', updateSearch);
$('#search-input').addEventListener('keydown', event => searchKeys(event, $('#search-input'), 'result', { scope: () => { searchScope = ''; updateSearch(); } }));
$('#show-closed').addEventListener('change', updateSearch);
$('#search-dialog').addEventListener('close', () => { if (returnFocus?.isConnected) returnFocus.focus(); });

// Home's bar is the same search inline, with its results under it. Phones open it full screen.
function openHomeList() {
  const input = $('#home-cmd');
  if (!input) return;
  input.setAttribute('aria-expanded', 'true');
  $('#home-cmd-panel').hidden = false;
  paintResults($('#home-cmd-list'), input, buildResults(input.value, homeScope), 'hres', 'No matches');
  $('#home-cmd-chips').innerHTML = !input.value && !homeScope ? `<span class="muted">Narrow to</span> ${['Accounts', 'Categories', 'Instruments'].map(scope => `<button class="chip" data-home-scope="${scope}">${scope}</button>`).join('')}` : '';
}
function closeHomeList() {
  const input = $('#home-cmd');
  if (!input || $('#home-cmd-panel').hidden) return;
  input.setAttribute('aria-expanded', 'false');
  input.removeAttribute('aria-activedescendant');
  $('#home-cmd-panel').hidden = true;
}
function setHomeScope(scope) {
  const value = $('#home-cmd')?.value || '';
  homeScope = scope;
  render();
  const input = $('#home-cmd');
  input.value = value;
  input.focus();
  openHomeList();
}
document.addEventListener('focusin', event => {
  if (event.target.id !== 'home-cmd') return;
  if (phone()) { event.target.blur(); openSearch(homeScope); return; }
  openHomeList();
});
document.addEventListener('input', event => { if (event.target.id === 'home-cmd') openHomeList(); });
document.addEventListener('keydown', event => {
  if (event.target.id !== 'home-cmd') return;
  if ($('#home-cmd-panel').hidden && ['ArrowDown', 'Enter'].includes(event.key)) openHomeList();
  searchKeys(event, event.target, 'hres', { scope: homeScope ? () => setHomeScope('') : null, close: () => { if (!$('#home-cmd-panel').hidden) closeHomeList(); else event.target.blur(); } });
});
document.addEventListener('mousedown', event => { if (event.target.closest('#home-cmd-panel')) event.preventDefault(); });
document.addEventListener('focusout', event => {
  if (event.target.id === 'home-cmd' && !event.relatedTarget?.closest?.('.cmd')) closeHomeList();
});

// ---- Entry ----------------------------------------------------------------

function openEntry(payee) {
  returnFocus = document.activeElement;
  closeDisclosures();
  $('#entry-account').innerHTML = accounts.filter(a => !a.closed && !a.holding).map(a => `<option value="${a.id}">${a.name} · ${a.detail}</option>`).join('');
  $('#entry-form').reset();
  $('#entry-account').value = currentAccount().holding ? '3' : currentAccount().id;
  if (pageInfo().object === 'Categories') $('#entry-form').elements.category.value = pageInfo().name === 'Groceries' ? 'Groceries' : 'Housing';
  if (payee) { $('#entry-form').elements.payee.value = payee.name; $('#entry-form').elements.category.value = payee.category; }
  entryDirty = false;
  $('#discard-warning').hidden = true;
  $('#keep-entry').hidden = true;
  $('#discard-entry').hidden = true;
  $('#entry-dialog').showModal();
  if (payee) $('#entry-form').elements.amount.focus();
}
function cancelEntry() {
  if (entryDirty) { $('#discard-warning').hidden = false; $('#keep-entry').hidden = false; $('#discard-entry').hidden = false; $('#keep-entry').focus(); }
  else $('#entry-dialog').close();
}
$('#entry-form').addEventListener('input', () => { entryDirty = true; });
$('#entry-dialog').addEventListener('cancel', event => { event.preventDefault(); cancelEntry(); });
$('#entry-dialog').addEventListener('close', () => { if (returnFocus?.isConnected) returnFocus.focus(); });
$('#entry-form').addEventListener('submit', event => {
  event.preventDefault();
  entryDirty = false;
  $('#entry-dialog').close();
  notify('Sample entry confirmed. No financial data was written.');
});

// ---- Home layout actions ----------------------------------------------------

function focusAfter(selector, fallback) { requestAnimationFrame(() => (document.querySelector(selector) || document.querySelector(fallback))?.focus()); }
function widgetAction(action, id) {
  const defs = Object.fromEntries(widgetDefs().map(d => [d.id, d]));
  const i = layout.home.indexOf(id);
  const title = defs[id].title;
  if (action === 'up' || action === 'down') {
    changeLayout({ home: swap(layout.home, i, action === 'up' ? i - 1 : i + 1) }, `${title} moved ${action}`, false);
    focusAfter(`[aria-controls="home-${id}-menu"]`);
  } else if (action === 'remove') {
    const next = layout.home[i + 1] || layout.home[i - 1];
    changeLayout({ home: layout.home.filter(x => x !== id) }, `${title} moved to More to explore`);
    focusAfter(next ? `[aria-controls="home-${next}-menu"]` : '#my-home-title', '#main');
  } else if (action === 'add' || action === 'top') {
    changeLayout({ home: action === 'top' ? [id, ...layout.home] : [...layout.home, id] }, `${title} added to ${action === 'top' ? 'the top of ' : ''}My Home`);
    focusAfter(`[aria-controls="home-${id}-menu"]`);
  } else if (action === 'settings') {
    notify('Widget settings (period, accounts) are part of personalization, #438.');
  }
}
function shortcutAction(action, id) {
  const i = layout.shortcuts.indexOf(id);
  const label = shortcutPool[id].label;
  if (action === 'remove') { changeLayout({ shortcuts: layout.shortcuts.filter(x => x !== id) }, `Shortcut ${label} removed`); focusAfter('[aria-controls="sc-new"]'); }
  else { changeLayout({ shortcuts: swap(layout.shortcuts, i, action === 'left' ? i - 1 : i + 1) }, `${label} moved ${action}`, false); focusAfter(`[aria-controls="sc-${id}"]`); }
}
function customizeAction(target) {
  const { cz: action, list, id } = target.dataset;
  if (action === 'reset') { changeLayout(defaultLayout(), 'Home reset to defaults'); focusAfter('[data-cz="reset"]'); return; }
  const i = layout[list].indexOf(id);
  if (action === 'remove') changeLayout({ [list]: layout[list].filter(x => x !== id) }, `${list === 'home' ? 'Widget moved to More to explore' : 'Shortcut removed'}`);
  else changeLayout({ [list]: swap(layout[list], i, action === 'up' ? i - 1 : i + 1) }, '', false);
  focusAfter(`[data-cz="${action}"][data-id="${id}"]:not([disabled])`, '#customize-body h3');
}

document.addEventListener('click', event => {
  const target = event.target.closest('button, a, [data-result]');
  if (!target) { if (!event.target.closest('.disclosure, #mobile-pages, .cmd')) closeDisclosures(); return; }
  if (target.classList.contains('skip')) { event.preventDefault(); $('#main').focus(); return; }
  if (target.dataset.disclosure) {
    const wasOpen = target.getAttribute('aria-expanded') === 'true';
    closeDisclosures();
    target.setAttribute('aria-expanded', String(!wasOpen));
    document.getElementById(target.dataset.disclosure).hidden = wasOpen;
  } else if (target.hasAttribute('data-search')) openSearch();
  else if (target.dataset.scope) openSearch(target.dataset.scope);
  else if (target.dataset.searchScope) { searchScope = target.dataset.searchScope; $('#search-input').value = ''; updateSearch(); $('#search-input').focus(); }
  else if (target.id === 'scope-chip') { searchScope = ''; updateSearch(); $('#search-input').focus(); }
  else if (target.dataset.homeScope) setHomeScope(target.dataset.homeScope);
  else if (target.id === 'home-scope-chip') setHomeScope('');
  else if (target.hasAttribute('data-result')) chooseResult(Number(target.dataset.result));
  else if (target.hasAttribute('data-add')) openEntry();
  else if (target.hasAttribute('data-cancel-entry')) cancelEntry();
  else if (target.id === 'keep-entry') { $('#discard-warning').hidden = true; $('#keep-entry').hidden = true; $('#discard-entry').hidden = true; $('#entry-form').elements.payee.focus(); }
  else if (target.id === 'discard-entry') { entryDirty = false; $('#entry-dialog').close(); }
  else if (target.dataset.close) document.getElementById(target.dataset.close).close();
  else if (target.id === 'hide-amounts') {
    hidden = !hidden;
    try { localStorage.setItem('navigation-prototype-hidden', String(hidden)); } catch { /* Optional browser persistence. */ }
    render(); $('#hide-amounts').focus();
  } else if (target.id === 'home-review') { reviewExpanded = !reviewExpanded; render(); $('#home-review').focus(); }
  else if (target.id === 'explore-toggle') { layout.exploreOpen = !layout.exploreOpen; render(); $('#explore-toggle').focus(); }
  else if (target.dataset.headline) { changeLayout({ headline: target.dataset.headline }, '', false); focusAfter('.bl-switch'); }
  else if (target.dataset.w) widgetAction(target.dataset.w, target.dataset.id);
  else if (target.dataset.sc) shortcutAction(target.dataset.sc, target.dataset.id);
  else if (target.dataset.scAdd) { changeLayout({ shortcuts: [...layout.shortcuts, target.dataset.scAdd] }, `Shortcut ${shortcutPool[target.dataset.scAdd].label} added`); focusAfter(`[aria-controls="sc-${target.dataset.scAdd}"]`); }
  else if (target.dataset.cz) customizeAction(target);
  else if (target.hasAttribute('data-customize')) { returnFocus = document.activeElement; closeDisclosures(); renderCustomize(); $('#customize-dialog').showModal(); }
  else if (target.hasAttribute('data-undo')) { if (undoLayout) { layout = undoLayout; undoLayout = null; render(); if ($('#customize-dialog').open) renderCustomize(); } $('#notice').textContent = 'Change undone'; }
  else if (target.hasAttribute('data-retry')) { scenario = 'everyday'; $('#scenario').value = scenario; render(); notify('Sample review check completed.'); }
  else if (target.hasAttribute('data-preview-reconcile')) { reconciliationPreview = true; render(); $('[data-finish-reconcile]').focus(); }
  else if (target.hasAttribute('data-finish-reconcile')) { reconciliationFinished = true; render(); $('#main').focus(); }
  else if (target.hasAttribute('data-review-import')) notify($('#import-category').value ? 'Sample changes reviewed. The batch remains uncommitted.' : 'Choose a category for the sample row first.');
  else if (target.hasAttribute('data-transfer')) notify('Transfer editor is outside this navigation prototype. Use Add to test the shared entry interaction.');
  else if (target.hasAttribute('data-sign-out')) { closeDisclosures(); notify('Sample session only. There is no authenticated connection to sign out.'); }
  else if (target.tagName === 'A') closeDisclosures();
});
$('#customize-dialog').addEventListener('close', () => { if (returnFocus?.isConnected) returnFocus.focus(); });
document.addEventListener('submit', event => {
  if (event.target.id !== 'filters') return;
  event.preventDefault();
  const params = new URLSearchParams(route.params);
  for (const [key, value] of new FormData(event.target)) { if (value) params.set(key, value); else params.delete(key); }
  location.hash = href(`${route.path}?${params}`);
});
document.addEventListener('change', event => {
  if (event.target.id === 'theme') { theme = event.target.value; applyTheme(); try { localStorage.setItem('navigation-prototype-theme', theme); } catch { /* Optional browser persistence. */ } }
  if (event.target.id === 'scenario') { scenario = event.target.value; reconciliationPreview = false; reconciliationFinished = false; render(); }
  if (event.target.name === 'cz-headline') changeLayout({ headline: event.target.value }, '', false);
  if (event.target.id === 'cz-explore') { layout.exploreOpen = event.target.checked; render(); }
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && !document.querySelector('dialog[open]')) {
    const trigger = document.querySelector('[data-disclosure][aria-expanded="true"]');
    closeDisclosures(); trigger?.focus(); return;
  }
  if (document.querySelector('dialog[open]') || event.target.closest('input, select, textarea, [contenteditable="true"]')) return;
  const focusSearch = () => { if ($('#home-cmd') && !phone()) { event.preventDefault(); $('#home-cmd').focus(); } else { event.preventDefault(); openSearch(); } };
  if (((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') || (!event.ctrlKey && !event.metaKey && !event.altKey && event.key === '/')) focusSearch();
  else if (!event.ctrlKey && !event.metaKey && !event.altKey && event.key.toLowerCase() === 'n') { event.preventDefault(); openEntry(); }
  else if (event.key === '?') $('#help-dialog').showModal();
});
