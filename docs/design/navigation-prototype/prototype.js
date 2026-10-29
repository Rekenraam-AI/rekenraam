'use strict';

// Design fixtures only. This artifact has no API client or financial calculator.
const sections = {
  Ledger: [['Transactions', '/app/transactions'], ['Accounts', '/app/accounts'], ['Categories', '/app/categories'], ['Payees', '/app/payees'], ['Import', '/app/import'], ['Reconcile', '/app/reconcile']],
  Investments: [['Overview', '/app/investments'], ['Instruments', '/app/investments/instruments'], ['Returns', '/app/investments/returns'], ['Income', '/app/investments/income'], ['Gains', '/app/investments/gains'], ['Prices', '/app/pricing']],
  Planning: [['Budgets', '/app/budgets'], ['Recurring', '/app/recurring'], ['Forecast', '/app/forecast']],
  Reports: [['Spending', '/app/reports/spending'], ['Cashflow', '/app/reports/cashflow'], ['Net worth', '/app/reports/net-worth'], ['Gains', '/app/investments/gains'], ['Export', '/app/reports/export']],
  Settings: [['Settings', '/app/settings'], ['Security', '/app/settings/security'], ['Backups and data', '/app/settings/data'], ['Language', '/app/settings/language'], ['Appearance', '/app/settings/appearance'], ['Currencies', '/app/settings/currencies'], ['Trash', '/app/settings/trash']]
};
const accounts = [
  { id: '1', name: 'Checking', detail: 'Asset · ING · EUR', balance: '€4,210.55' },
  { id: '2', name: 'Savings', detail: 'Asset · ASN · EUR', balance: '€31,500.00' },
  { id: '3', name: 'Brokerage', detail: 'Asset · DEGIRO · EUR', balance: '€12,000.00' },
  { id: '4', name: 'VWRL holding', detail: 'DEGIRO › VWRL · holding account', balance: '€8,400.00', holding: true },
  { id: '5', name: 'Old checking', detail: 'Asset · ING · closed · EUR', balance: '€0.00', closed: true }
];
const categories = [{ id: '10', name: 'Groceries', detail: 'Everyday › Groceries' }, { id: '11', name: 'Energy', detail: 'Housing › Energy' }];
const instruments = [{ id: '20', name: 'VWRL', detail: 'Vanguard FTSE All-World · held at DEGIRO' }];
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
let hidden = false;
let theme = 'system';
try { hidden = localStorage.getItem('navigation-prototype-hidden') === 'true'; theme = localStorage.getItem('navigation-prototype-theme') || 'system'; } catch { /* File previews may deny storage. */ }
let scenario = 'everyday';
let route;
let searchScope = '';
let searchResults = [];
let selected = 0;
let returnFocus;
let entryDirty = false;
let reconciliationPreview = false;
let reconciliationFinished = false;
let homeScroll = 0;
let reviewExpanded = false;
let exploreExpanded = true;
let noticeTimer;
const money = value => `<span class="money">${hidden ? '••••••' : esc(value)}</span>`;
const accountPath = id => `/app/accounts/${id}/register`;
const currentAccount = () => accounts.find(account => account.id === (route.path.match(/^\/app\/accounts\/(\d+)\/register$/)?.[1] || route.params.get('account_id'))) || accounts[0];
function notify(message) {
  clearTimeout(noticeTimer);
  $('#notice').textContent = message;
  noticeTimer = setTimeout(() => { $('#notice').textContent = ''; }, 6000);
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
  return path === '/app/import' ? '1 batch' : path === '/app/recurring' ? '2 drafts' : '';
}
function pageLinks(pages) {
  return pages.map(([name, path]) => {
    const isParent = ['/app/accounts', '/app/categories', '/app/investments/instruments'].includes(path) && route.path.startsWith(`${path}/`);
    return `<a href="${href(path)}" ${route.path === path ? 'aria-current="page"' : isParent ? 'aria-current="true"' : ''}>${name}<small class="muted">${attentionHint(path)}</small></a>`;
  }).join('');
}
function pins() {
  return scenario === 'empty' ? '<p class="muted">No pinned accounts yet.</p>' : accounts.slice(0, 3).map(account => link(`${account.name} ${money(account.balance)}`, accountPath(account.id))).join('');
}
function renderShell(info) {
  const menus = Object.entries(sections).filter(([name]) => name !== 'Settings').map(([name, pages]) => `
    <div class="nav-item"><button class="nav-trigger" data-disclosure="nav-${name}" aria-expanded="false" aria-controls="nav-${name}" ${info.section === name ? 'aria-current="true"' : ''}>${name} ▾</button>
    <div id="nav-${name}" class="disclosure ${name === 'Ledger' ? 'ledger' : ''}" hidden><div>${pageLinks(pages)}</div>${name === 'Ledger' ? `<div><p class="menu-caption">Pinned accounts</p>${pins()}${link('All accounts', '/app/accounts')}</div>` : ''}</div></div>`).join('');
  $('#shell').innerHTML = `<div class="shell-inner">
    ${link('Rekenraam', '/app', 'brand')}
    <nav class="desktop-nav" aria-label="Sections"><a class="home-link" href="#/app" ${route.path === '/app' ? 'aria-current="page"' : ''}>Home</a>${menus}</nav>
    <div class="shell-actions">
      ${route.path !== '/app' ? '<button data-search class="compact-search" aria-label="Search pages, accounts or commands"><span aria-hidden="true">⌕</span><span class="search-copy">Search…</span></button>' : ''}
      <button id="hide-amounts" aria-label="Hide amounts" aria-pressed="${hidden}" title="Hide amounts">${hidden ? '◉' : '◎'}</button>
      <div class="nav-item"><button class="nav-trigger" data-disclosure="settings-menu" aria-label="Settings and appearance" aria-expanded="false" aria-controls="settings-menu">⚙</button>
        <div id="settings-menu" class="disclosure settings" hidden>${pageLinks(sections.Settings)}
          <label>Theme <select id="theme"><option value="light">Light</option><option value="dark">Dark</option><option value="system">System</option></select></label>
          <button data-sign-out>Sign out (sample)</button>
        </div></div>
      <button class="mobile-toggle" data-disclosure="mobile-pages" aria-label="All pages" aria-expanded="false" aria-controls="mobile-pages">☰</button>
    </div></div>
    <nav id="mobile-pages" class="mobile-pages" aria-label="All pages" hidden>${link('Home', '/app')}${Object.entries(sections).map(([name, pages]) => `<details><summary>${name}</summary>${pageLinks(pages)}${name === 'Ledger' ? `<p class="menu-caption">Pinned accounts</p>${pins()}` : ''}</details>`).join('')}</nav>
    ${sections[info.section] ? `<nav class="section-tabs" aria-label="${info.section} pages">${pageLinks(sections[info.section])}</nav>` : ''}`;
  $('#theme').value = theme;
}
function heading(info, scope = '', add = false) {
  const breadcrumb = info.object ? `<nav class="breadcrumbs" aria-label="Breadcrumb">${link(info.object, info.object === 'Accounts' ? '/app/accounts' : info.object === 'Categories' ? '/app/categories' : '/app/investments/instruments')} › ${esc(info.name)}</nav>` : '';
  return `${breadcrumb}<div class="page-heading"><div><h1>${info.object ? `<button class="object-trigger" data-scope="${info.object}">${esc(info.name)} ▾</button>` : esc(info.name)}</h1>${scope ? `<p class="scope">${scope}</p>` : ''}</div>${add ? '<button class="primary" data-add>Add</button>' : ''}</div>`;
}
function reviewItems() {
  if (scenario === 'failed') return '<li>Review status could not be checked. <button data-retry>Retry check</button></li>';
  if (scenario === 'empty') return '<li>No actionable items in the checked sample sources.</li>';
  return `<li>${link('2 recurring drafts awaiting review', '/app/recurring?tab=due')}</li><li>${link('1 import batch to review · 3 rows need a category', '/app/import?batch_id=october')}</li><li>${link('Note: 3 dividends unassigned', '/app/investments/income')}</li>${scenario === 'missing' ? `<li>${link('VWRL has no price for the valuation date', '/app/pricing?base_commodity_id=20')}</li>` : ''}`;
}
function renderHome() {
  const headline = scenario === 'missing' ? `Net worth unavailable · ${link('Add a VWRL price', '/app/pricing?base_commodity_id=20')}` : scenario === 'empty' ? 'Your book is ready for its first account' : `Net worth <strong>${money('€56,110.55')}</strong> · ${money('+€2,140.00')} since 1 Oct`;
  return `<section class="hero"><h1>${headline}</h1>
    <details class="briefing" id="home-review" ${reviewExpanded ? 'open' : ''}><summary>${scenario === 'failed' ? 'Review status unavailable' : scenario === 'empty' ? 'All clear, nothing to review' : '2 drafts and 1 import to review'}</summary><ul>${reviewItems()}</ul></details>
    <p class="scope">${scenario === 'empty' ? 'No accounts or transactions yet' : '5 sample accounts in EUR · valued 29 Oct · transactions through 28 Oct'}${scenario === 'missing' ? ' · 1 required price missing' : ''}</p>
    <button class="search-launcher" data-search>⌕ Search pages, accounts or commands <kbd>/</kbd></button>
    <div class="shortcuts">
      <button class="shortcut first" data-add><span aria-hidden="true">＋</span>Add transaction</button>
      <button class="shortcut" data-transfer><span aria-hidden="true">⇄</span>Transfer</button>
      ${link('<span aria-hidden="true">↥</span>Import file', '/app/import', 'shortcut')}
      ${link('<span aria-hidden="true">≡</span>Transactions', '/app/transactions', 'shortcut extra')}
      ${link('<span aria-hidden="true">▥</span>Spending', '/app/reports/spending', 'shortcut extra')}
      ${link('<span aria-hidden="true">▤</span>Checking', accountPath('1'), 'shortcut extra')}
      <button class="shortcut shortcut-more" data-shortcuts><span aria-hidden="true">···</span>More</button>
    </div></section>
    ${scenario === 'empty' ? `<section class="home-section card"><h2>Start with an account</h2><p>Add an account before entering or importing transactions.</p>${link('Set up accounts', '/app/accounts', 'button primary')}</section>` : `
    <section class="home-section"><h2>My Home</h2><div class="grid">
      <article class="card"><h3>Spending this month</h3><p class="fact">${money('€3,720.40')}</p><p class="scope">1–29 Oct · expense categories · EUR</p>${link('Open spending report →', '/app/reports/spending?from=2026-10-01&to=2026-10-29')}</article>
      <article class="card"><h3>Budgets</h3><p class="fact">${money('€1,579.60')} left</p><p class="scope">October · on-budget accounts · EUR</p>${link('Open budgets →', '/app/budgets?month=2026-10')}</article>
      <article class="card"><h3>Accounts</h3><ul class="rows">${accounts.slice(0, 2).map(a => `<li>${link(a.name, accountPath(a.id))}${money(a.balance)}</li>`).join('')}</ul>${link('All accounts →', '/app/accounts')}</article>
      <article class="card"><h3>Recurring</h3><p class="fact">2 drafts</p><p class="scope">Awaiting review; nothing posted by this prototype.</p>${link('Review drafts →', '/app/recurring?tab=due')}</article>
      <article class="card wide"><h3>Net worth trend</h3><p class="fact">${scenario === 'missing' ? 'Unavailable' : money('+€2,140.00')}</p><p class="scope">1–29 Oct · same sample account set in EUR · absolute change</p>${link('Open net worth report →', '/app/reports/net-worth')}</article>
    </div></section>`}
    <details class="home-section" id="explore" ${exploreExpanded ? 'open' : ''}><summary><strong>More to explore</strong></summary><div class="grid">
      ${[['Wealth & investments', 'Returns', '/app/investments/returns'], ['Planning', 'Forecast', '/app/forecast'], ['Ledger & data', 'Reconciliation', '/app/reconcile']].map(([group, name, path]) => `<article class="card explore"><p class="scope">${group}</p><h3>${name}</h3><p>Open the full workflow. Expanded charts and customization remain later slices.</p>${link(`Open ${name.toLowerCase()} →`, path)}</article>`).join('')}
    </div></details>`;
}
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
    <div class="actions">${link('Reconcile this account', `/app/reconcile?account_id=${account.id}`, 'button')}${account.holding ? link('Open VWRL instrument', '/app/investments/instruments/20', 'button') : ''}${link('All accounts', '/app/accounts')}</div>`;
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
    <nav class="actions" aria-label="Import views">${link('Files', '/app/import', 'button')}${link('Dividends', '/app/import?view=dividends', 'button')}</nav>
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
function renderOther(info) {
  if (info.name === 'Accounts') return renderIndex(info, accounts, accountPath);
  if (info.name === 'Categories') return renderIndex(info, categories, id => `/app/categories/${id}`);
  if (route.path === '/app/investments/instruments') return renderIndex(info, instruments, id => `/app/investments/instruments/${id}`);
  if (info.object === 'Categories') return `${heading(info, 'October · expense category · posted actuals', true)}<section class="card"><h2>Category activity</h2><p>${link('View matching transactions', `/app/transactions?category_id=${route.path.split('/').at(-1)}`)}</p>${link('Open spending report', '/app/reports/spending')}</section>`;
  if (info.object === 'Instruments') return `${heading(info, 'Held at DEGIRO · EUR')}<section class="card"><h2>Holding and split history</h2>${link('Open VWRL holding register', accountPath('4'), 'button')}<p class="scope">No sample splits. Whole-book split history remains reachable from Overview.</p></section>`;
  if (info.name === 'Transactions') return `${heading(info, `Sample entries${route.params.has('payee_id') ? ' · payee: Albert Heijn' : ''}${route.params.has('category_id') ? ' · selected category' : ''}`, true)}${filters()}<section class="card"><h2>Transactions</h2>${link('Open Checking register', accountPath('1'), 'button')}<p class="scope">Global search and report links retain their URL filters.</p></section>`;
  if (info.name === 'Payees') return `${heading(info, 'Payee management belongs to #443')}<section class="card"><h2>Albert Heijn</h2><p>214 transactions · last used 27 Oct</p>${link('Transactions with Albert Heijn', '/app/transactions?payee_id=30', 'button')}</section>`;
  if (info.name === 'Recurring') return `${heading(info, 'Drafts and templates are separate views')}<nav class="actions" aria-label="Recurring views">${link('Due', '/app/recurring?tab=due', 'button')}${link('Templates', '/app/recurring?tab=templates', 'button')}</nav><section class="card"><h2>${route.params.get('tab') === 'templates' ? 'Templates' : '2 drafts awaiting review'}</h2><p>Sample review destination; posting drafts is outside this navigation prototype.</p></section>`;
  if (info.name === 'Net worth') return `${heading(info, '29 Oct 2026 · same 5 sample accounts in EUR')}<section class="card"><h2>Valuation</h2><p class="fact">${scenario === 'missing' ? 'Unavailable · VWRL price missing' : money('€56,110.55')}</p>${link('Inspect accounts', '/app/accounts')}</section>`;
  if (info.name === 'Export') return `${heading(info)}<section class="card"><h2>Ledger CSV and QIF</h2><p>Navigation destination only. No sample files are generated.</p>${link('Backup and restore settings', '/app/settings/data')}</section>`;
  if (info.section === 'Settings') return `${heading(info)}<section class="card"><h2>${esc(info.name)}</h2><p>Settings destination only; live security, backup and preference changes remain in the application.</p>${info.name === 'Backups and data' ? link('Ledger CSV and QIF export →', '/app/reports/export') : info.name === 'Currencies' ? '<p>EUR · USD · FX-rate history stays in Currencies.</p>' : '<p>Use the header settings menu to try Light, Dark or System.</p>'}</section>`;
  return `${heading(info)}<section class="card"><h2>${esc(info.name)} destination</h2><p>This screen establishes navigation and page placement. Its detailed workflow is outside this prototype.</p>${info.section === 'Investments' ? `<div class="actions">${link('Instruments', '/app/investments/instruments')}${link('Split history', '/app/investments?view=splits')}${link('Review suggestions', '/app/investments?view=suggestions')}</div>${route.params.has('view') ? `<p role="status">Selected view: ${esc(route.params.get('view'))}</p>` : ''}` : ''}${link('Back to Home', '/app', 'button')}</section>`;
}
function render(navigated = false) {
  const old = route;
  if (old?.path === '/app') homeScroll = scrollY;
  route = readRoute();
  const info = pageInfo();
  if (old?.path === '/app/reconcile' && old.canonical !== route.canonical) { reconciliationPreview = false; reconciliationFinished = false; }
  renderShell(info);
  $('#main').className = info.object === 'Accounts' || ['Transactions', 'Import'].includes(info.name) ? 'fluid' : '';
  $('#main').innerHTML = route.path === '/app' ? renderHome() : info.object === 'Accounts' ? renderRegister(info) : route.path === '/app/reports/spending' ? renderSpending(info) : info.name === 'Import' ? renderImport(info) : info.name === 'Reconcile' ? renderReconcile(info) : renderOther(info);
  document.title = `${info.name} · Rekenraam design prototype`;
  if (navigated) { $('#main').focus({ preventScroll: true }); requestAnimationFrame(() => scrollTo(0, route.path === '/app' ? homeScroll : 0)); }
}
render();
window.addEventListener('hashchange', () => render(true));
document.addEventListener('toggle', event => {
  if (event.target.id === 'home-review') reviewExpanded = event.target.open;
  if (event.target.id === 'explore') exploreExpanded = event.target.open;
}, true);

function closeDisclosures() {
  for (const trigger of document.querySelectorAll('[data-disclosure]')) {
    trigger.setAttribute('aria-expanded', 'false');
    document.getElementById(trigger.dataset.disclosure).hidden = true;
  }
}
function scopeDestination(kind, id) {
  const path = kind === 'Accounts' ? (route.path === '/app/reconcile' ? '/app/reconcile' : accountPath(id)) : kind === 'Categories' ? `/app/categories/${id}` : `/app/investments/instruments/${id}`;
  const params = searchScope ? new URLSearchParams(route.params) : new URLSearchParams();
  params.delete('dropped');
  if (kind === 'Accounts' && searchScope) {
    if (params.has('holding_id')) { params.delete('holding_id'); params.set('dropped', 'holding selection'); }
    if (route.path === '/app/reconcile') params.set('account_id', id);
  }
  return path + (params.size ? `?${params}` : '');
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
  const input = $('#search-input');
  const query = input.value.toLocaleLowerCase().trim();
  $('#scope-chip').hidden = !searchScope;
  $('#scope-chip').textContent = `${searchScope} × · remove scope`;
  $('#closed-control').hidden = searchScope !== 'Accounts';
  $('#scope-options').innerHTML = searchScope ? '' : ['Accounts', 'Categories', 'Instruments'].map(scope => `<button data-search-scope="${scope}">${scope}</button>`).join('');
  const matches = item => `${item.name} ${item.detail || ''}`.toLocaleLowerCase().includes(query);
  const sources = searchScope === 'Accounts' ? accounts : searchScope === 'Categories' ? categories : searchScope === 'Instruments' ? instruments : [];
  if (searchScope) {
    searchResults = sources.filter(item => matches(item) && (!item.closed || $('#show-closed').checked) && (!item.holding || query)).map(item => ({ label: item.name, detail: `${item.detail}${item.balance ? ` · ${hidden ? '••••••' : item.balance}` : ''}`, path: scopeDestination(searchScope, item.id) }));
  } else {
    searchResults = [
      { label: 'Add transaction', detail: 'Command · opens the editor', action: 'add' },
      ...registry.map(page => ({ label: page.name, detail: `Page · ${page.section}`, path: page.path })),
      ...accounts.filter(a => !a.closed && (!a.holding || query)).map(a => ({ label: a.name, detail: `${a.detail} · ${hidden ? '••••••' : a.balance}`, path: accountPath(a.id) })),
      ...categories.map(c => ({ label: c.name, detail: `Category · ${c.detail}`, path: `/app/categories/${c.id}` })),
      ...instruments.map(i => ({ label: i.name, detail: `Instrument · ${i.detail}`, path: `/app/investments/instruments/${i.id}` }))
    ].filter(item => `${item.label} ${item.detail}`.toLocaleLowerCase().includes(query)).slice(0, query ? 12 : 6);
    if (query) searchResults.push({ label: `Search all transactions for “${input.value}”`, detail: 'Payees, descriptions, references and notes', path: `/app/transactions?q=${encodeURIComponent(input.value)}` });
  }
  renderSearchResults();
}
function renderSearchResults() {
  selected = 0;
  $('#search-results').innerHTML = searchResults.length ? searchResults.map((item, index) => `<div class="result" id="result-${index}" role="option" aria-selected="${index === 0}" data-result="${index}">${esc(item.label)}<small>${esc(item.detail)}</small></div>`).join('') : '<div role="option" aria-disabled="true">No matching objects</div>';
  updateSelection();
}
function updateSelection() {
  $('#search-input').removeAttribute('aria-activedescendant');
  for (const item of document.querySelectorAll('[data-result]')) item.setAttribute('aria-selected', String(Number(item.dataset.result) === selected));
  if (searchResults.length) $('#search-input').setAttribute('aria-activedescendant', `result-${selected}`);
}
function chooseResult(index) {
  const result = searchResults[index];
  if (!result) return;
  $('#search-dialog').close();
  if (result.action === 'add') openEntry(); else location.hash = href(result.path);
}
$('#search-input').addEventListener('input', updateSearch);
$('#search-input').addEventListener('keydown', event => {
  if (['ArrowDown', 'ArrowUp'].includes(event.key) && searchResults.length) {
    event.preventDefault();
    selected = (selected + (event.key === 'ArrowDown' ? 1 : -1) + searchResults.length) % searchResults.length;
    updateSelection();
    document.getElementById(`result-${selected}`).scrollIntoView({ block: 'nearest' });
  }
  if (event.key === 'Enter') { event.preventDefault(); chooseResult(selected); }
});
$('#show-closed').addEventListener('change', updateSearch);
$('#search-dialog').addEventListener('close', () => { if (returnFocus?.isConnected) returnFocus.focus(); });
function openEntry() {
  returnFocus = document.activeElement;
  closeDisclosures();
  $('#entry-account').innerHTML = accounts.filter(a => !a.closed && !a.holding).map(a => `<option value="${a.id}">${a.name} · ${a.detail}</option>`).join('');
  $('#entry-form').reset();
  $('#entry-account').value = currentAccount().holding ? '3' : currentAccount().id;
  if (pageInfo().object === 'Categories') $('#entry-form').elements.category.value = pageInfo().name === 'Groceries' ? 'Groceries' : 'Housing';
  entryDirty = false;
  $('#discard-warning').hidden = true;
  $('#keep-entry').hidden = true;
  $('#discard-entry').hidden = true;
  $('#entry-dialog').showModal();
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
document.addEventListener('click', event => {
  const target = event.target.closest('button, a, [data-result]');
  if (!target) { if (!event.target.closest('.disclosure, #mobile-pages')) closeDisclosures(); return; }
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
  } else if (target.hasAttribute('data-retry')) { scenario = 'everyday'; $('#scenario').value = scenario; render(); notify('Sample review check completed.'); }
  else if (target.hasAttribute('data-preview-reconcile')) { reconciliationPreview = true; render(); $('[data-finish-reconcile]').focus(); }
  else if (target.hasAttribute('data-finish-reconcile')) { reconciliationFinished = true; render(); $('#main').focus(); }
  else if (target.hasAttribute('data-review-import')) notify($('#import-category').value ? 'Sample changes reviewed. The batch remains uncommitted.' : 'Choose a category for the sample row first.');
  else if (target.hasAttribute('data-shortcuts')) {
    openSearch();
    searchResults = [
      { label: 'Transactions', detail: 'Shortcut', path: '/app/transactions' },
      { label: 'Spending', detail: 'Shortcut', path: '/app/reports/spending' },
      { label: 'Checking', detail: 'Shortcut', path: accountPath('1') }
    ];
    renderSearchResults();
  }
  else if (target.hasAttribute('data-transfer')) notify('Transfer editor is outside this navigation prototype. Use Add to test the shared entry interaction.');
  else if (target.hasAttribute('data-sign-out')) { closeDisclosures(); notify('Sample session only. There is no authenticated connection to sign out.'); }
  else if (target.tagName === 'A') closeDisclosures();
});
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
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && !document.querySelector('dialog[open]')) {
    const trigger = document.querySelector('[data-disclosure][aria-expanded="true"]');
    closeDisclosures(); trigger?.focus(); return;
  }
  if (document.querySelector('dialog[open]') || event.target.closest('input, select, textarea, [contenteditable="true"]')) return;
  if (((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') || (!event.ctrlKey && !event.metaKey && !event.altKey && event.key === '/')) { event.preventDefault(); openSearch(); }
  else if (!event.ctrlKey && !event.metaKey && !event.altKey && event.key.toLowerCase() === 'n') { event.preventDefault(); openEntry(); }
  else if (event.key === '?') $('#help-dialog').showModal();
});
