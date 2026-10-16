// Shared by the demo scenes. demos.spec.ts drives these through page.evaluate.
(function () {
  const keys = document.createElement('div');
  keys.id = 'keys';
  document.body.appendChild(keys);

  const pointer = document.createElement('div');
  pointer.id = 'pointer';
  pointer.innerHTML = '<span class="ring"></span><svg width="24" height="24" viewBox="0 0 24 24">' +
    '<path d="M3 2l15 9-6.8 1.6L7.6 19z" fill="#F4F4F5" stroke="#18181B" stroke-width="1.5" stroke-linejoin="round"/></svg>';
  document.body.appendChild(pointer);

  let hideTimer = 0;
  // Callers pass viewport pixels; the scene is zoomed (stage.css).
  const Z = parseFloat(getComputedStyle(document.documentElement).zoom) || 1;

  window.stage = {
    /** Shows key caps, e.g. (['Ctrl', 'G'], 'Fix'). 'G G' style taps: pass ['Ctrl', 'G', 'G']. */
    keys(caps, hint, ms) {
      clearTimeout(hideTimer);
      keys.innerHTML = '';
      caps.forEach((c, i) => {
        if (i > 0) {
          const sep = document.createElement('span');
          sep.className = 'plus';
          // A repeated key is a second tap, not another modifier.
          sep.textContent = c === caps[i - 1] ? '' : '+';
          keys.appendChild(sep);
        }
        const cap = document.createElement('span');
        cap.className = 'cap';
        cap.textContent = c;
        keys.appendChild(cap);
      });
      if (hint) {
        const h = document.createElement('span');
        h.className = 'hint';
        h.textContent = hint;
        keys.appendChild(h);
      }
      keys.classList.add('show');
      hideTimer = setTimeout(() => keys.classList.remove('show'), ms || 1300);
    },
    /** Animates the last cap as pressed (for a second tap). */
    tapLast() {
      const caps = keys.querySelectorAll('.cap');
      const last = caps[caps.length - 1];
      if (!last) return;
      last.classList.remove('tap');
      void last.offsetWidth;
      last.classList.add('tap');
    },
    pointer(x, y) {
      pointer.style.transform = `translate(${x / Z - 3}px, ${y / Z - 2}px)`;
    },
    click() {
      pointer.classList.remove('click');
      void pointer.offsetWidth;
      pointer.classList.add('click');
    },
    busy(on) {
      document.querySelector('.tray-keylint')?.classList.toggle('is-busy', on);
    },
  };
})();
