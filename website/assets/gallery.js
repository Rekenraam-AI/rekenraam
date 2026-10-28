(() => {
  const gallery = document.querySelector('[data-gallery]');
  if (!gallery) return;
  const slides = [...gallery.querySelectorAll('[data-gallery-slide]')];
  const controls = gallery.querySelector('[data-gallery-controls]');
  const selectors = [...gallery.querySelectorAll('[data-gallery-select]')];
  const announcement = gallery.querySelector('[data-gallery-announcement]');
  let current = 0;

  function show(index, announce = true) {
    current = (index + slides.length) % slides.length;
    slides.forEach((slide, position) => { slide.hidden = position !== current; });
    selectors.forEach((button, position) => { button.setAttribute('aria-pressed', String(position === current)); });
    if (announce) announcement.textContent = slides[current].dataset.caption;
  }

  gallery.querySelector('[data-gallery-previous]').addEventListener('click', () => show(current - 1));
  gallery.querySelector('[data-gallery-next]').addEventListener('click', () => show(current + 1));
  selectors.forEach((button, index) => button.addEventListener('click', () => show(index)));
  gallery.classList.add('gallery-enhanced');
  show(0, false);
  controls.hidden = false;
})();
