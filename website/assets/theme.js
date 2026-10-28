(() => {
  const root = document.documentElement;
  const button = document.querySelector('[data-theme-toggle]');
  if (!button) return;
  const preference = window.matchMedia('(prefers-color-scheme: dark)');
  const savedTheme = (() => {
    try { return localStorage.getItem('rekenraam-website-theme'); } catch { return null; }
  })();
  let selected = savedTheme === 'light' || savedTheme === 'dark' ? savedTheme : null;

  function render() {
    const active = selected ?? (preference.matches ? 'dark' : 'light');
    if (selected) root.dataset.theme = selected;
    else delete root.dataset.theme;
    button.textContent = active === 'dark' ? button.dataset.labelLight : button.dataset.labelDark;
  }

  button.addEventListener('click', () => {
    const active = selected ?? (preference.matches ? 'dark' : 'light');
    selected = active === 'dark' ? 'light' : 'dark';
    try { localStorage.setItem('rekenraam-website-theme', selected); } catch { /* Theme remains usable without storage. */ }
    render();
  });
  preference.addEventListener('change', render);
  render();
  button.hidden = false;
})();
