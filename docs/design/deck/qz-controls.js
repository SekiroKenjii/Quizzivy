/* Quizzivy controls: shadcn-style popovers for native <select> and a <qz-datetime> picker. */
(function () {
  if (window.__qzControls) return;
  window.__qzControls = true;

  const css = `
.qzc-pop{position:fixed;z-index:10000;background:var(--card,#fff);color:var(--fg,#1b2123);border:1px solid var(--border,#e4e7e8);border-radius:10px;box-shadow:0 12px 32px -8px rgba(20,28,30,.22),0 2px 6px rgba(20,28,30,.08);padding:4px;font-family:inherit;font-size:13.5px;line-height:1.4;outline:0;animation:qzcIn .12s ease-out;box-sizing:border-box}
.qzc-pop *{box-sizing:border-box}
@keyframes qzcIn{from{opacity:0;transform:translateY(-3px)}to{opacity:1;transform:none}}
@media (prefers-reduced-motion:reduce){.qzc-pop{animation:none}}
.qzc-list{max-height:288px;overflow-y:auto;display:flex;flex-direction:column;gap:1px;overscroll-behavior:contain}
.qzc-opt{display:flex;align-items:center;min-height:32px;padding:6px 10px 6px 30px;border-radius:6px;cursor:pointer;position:relative;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;user-select:none}
.qzc-opt[aria-selected="true"]{font-weight:500}
.qzc-opt[aria-selected="true"]::before{content:"";position:absolute;left:11px;top:50%;width:9px;height:5px;border-left:2px solid currentColor;border-bottom:2px solid currentColor;transform:translateY(-75%) rotate(-45deg)}
.qzc-opt.qzc-act{background:var(--hover,#eceff0)}
.qzc-opt[aria-disabled="true"]{opacity:.45;cursor:default}
.qzc-grp{padding:8px 10px 4px;font-size:11.5px;font-weight:600;color:var(--muted-fg,#646c6e)}
.qzc-sep{height:1px;background:var(--border,#e4e7e8);margin:4px -4px}
.qzc-cal{padding:8px;width:268px}
.qzc-head{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:2px 2px 8px}
.qzc-title{font-size:13.5px;font-weight:600}
.qzc-nav{width:30px;height:30px;display:grid;place-items:center;border:1px solid var(--border,#e4e7e8);border-radius:7px;background:var(--card,#fff);color:inherit;cursor:pointer;padding:0;font:inherit}
.qzc-nav:hover{background:var(--muted,#f2f4f4)}
.qzc-grid{display:grid;grid-template-columns:repeat(7,1fr);gap:2px}
.qzc-wd{height:26px;display:grid;place-items:center;font-size:11.5px;font-weight:500;color:var(--muted-fg,#646c6e)}
.qzc-day{height:34px;border:0;border-radius:7px;background:transparent;color:inherit;font:inherit;font-size:13px;cursor:pointer;font-variant-numeric:tabular-nums;padding:0}
.qzc-day:hover{background:var(--hover,#eceff0)}
.qzc-day.qzc-out{color:var(--muted-fg,#646c6e);opacity:.55}
.qzc-day.qzc-today{box-shadow:inset 0 0 0 1px var(--ring,#9aa3a5)}
.qzc-day.qzc-sel{background:var(--primary,#252c2e);color:var(--primary-fg,#fafbfb);font-weight:600}
.qzc-day:disabled{opacity:.3;cursor:default;background:transparent}
.qzc-day:focus-visible,.qzc-nav:focus-visible,.qzc-btn:focus-visible,.qzc-step:focus-visible{outline:2px solid var(--ring,#9aa3a5);outline-offset:1px}
.qzc-time{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-top:8px;padding-top:10px;border-top:1px solid var(--border,#e4e7e8)}
.qzc-tlabel{font-size:12.5px;font-weight:500;color:var(--muted-fg,#646c6e)}
.qzc-spin{display:flex;align-items:center;gap:4px}
.qzc-step{width:26px;height:30px;border:1px solid var(--border,#e4e7e8);border-radius:6px;background:var(--card,#fff);color:inherit;cursor:pointer;font:inherit;font-size:14px;line-height:1;padding:0}
.qzc-step:hover{background:var(--muted,#f2f4f4)}
.qzc-num{min-width:30px;text-align:center;font-weight:600;font-variant-numeric:tabular-nums}
.qzc-foot{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-top:10px}
.qzc-btn{height:30px;padding:0 12px;border-radius:7px;border:1px solid var(--border,#e4e7e8);background:var(--card,#fff);color:inherit;font:inherit;font-size:12.5px;font-weight:500;cursor:pointer}
.qzc-btn:hover{background:var(--muted,#f2f4f4)}
.qzc-btn.qzc-primary{background:var(--primary,#252c2e);color:var(--primary-fg,#fafbfb);border-color:var(--primary,#252c2e)}
.qzc-btn.qzc-primary:hover{opacity:.9}
.qzc-btn.qzc-ghost{border-color:transparent;background:transparent;color:var(--muted-fg,#646c6e)}
select:not([multiple]):not([size]){cursor:pointer}
input[type=date],input[type=time],input[type=datetime-local]{cursor:pointer;padding-right:34px!important;background-repeat:no-repeat!important;background-position:right 10px center!important;background-size:15px 15px!important;background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%238a9294' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Crect x='3' y='4' width='18' height='18' rx='2'/%3E%3Cpath d='M16 2v4M8 2v4M3 10h18'/%3E%3C/svg%3E")!important}
input[type=time]{background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%238a9294' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='10'/%3E%3Cpath d='M12 6v6l4 2'/%3E%3C/svg%3E")!important}
input[type=date]::-webkit-calendar-picker-indicator,input[type=time]::-webkit-calendar-picker-indicator,input[type=datetime-local]::-webkit-calendar-picker-indicator{display:none}
`;
  const style = document.createElement('style');
  style.textContent = css;
  (document.head || document.documentElement).appendChild(style);

  let cur = null; // { el, pop, onKey, close }
  const closeAll = () => { if (cur) { const c = cur; cur = null; c.close(); } };

  function place(pop, anchor, minW) {
    const r = anchor.getBoundingClientRect();
    pop.style.minWidth = minW === 0 ? '' : Math.max(minW || 0, r.width) + 'px';
    pop.style.left = '0px'; pop.style.top = '0px';
    const pw = pop.offsetWidth, ph = pop.offsetHeight;
    const vw = window.innerWidth, vh = window.innerHeight;
    let left = Math.min(Math.max(8, r.left), vw - pw - 8);
    let top = r.bottom + 6;
    if (top + ph > vh - 8 && r.top - ph - 6 > 8) top = r.top - ph - 6;
    top = Math.max(8, Math.min(top, vh - ph - 8));
    pop.style.left = left + 'px'; pop.style.top = top + 'px';
  }

  function mount(pop, anchor, minW, onKey) {
    closeAll();
    document.body.appendChild(pop);
    place(pop, anchor, minW);
    const onDown = e => { if (!pop.contains(e.target) && !anchor.contains(e.target)) closeAll(); };
    const onScroll = e => { if (pop.contains(e.target)) return; closeAll(); };
    const onResize = () => closeAll();
    document.addEventListener('mousedown', onDown, true);
    document.addEventListener('keydown', onKey, true);
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('resize', onResize);
    window.addEventListener('blur', onResize);
    cur = {
      el: anchor, pop,
      close: () => {
        document.removeEventListener('mousedown', onDown, true);
        document.removeEventListener('keydown', onKey, true);
        window.removeEventListener('scroll', onScroll, true);
        window.removeEventListener('resize', onResize);
        window.removeEventListener('blur', onResize);
        pop.remove();
      },
    };
  }

  /* ---------- select ---------- */
  const selSetter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value').set;
  const coarse = () => window.matchMedia && window.matchMedia('(pointer: coarse)').matches;
  const skip = s => !s || s.multiple || s.size > 1 || s.disabled || s.hasAttribute('data-native') || coarse();

  function openSelect(sel) {
    const pop = document.createElement('div');
    pop.className = 'qzc-pop';
    const list = document.createElement('div');
    list.className = 'qzc-list';
    list.setAttribute('role', 'listbox');
    const label = sel.getAttribute('aria-label') || (sel.labels && sel.labels[0] && sel.labels[0].textContent.trim()) || '';
    if (label) list.setAttribute('aria-label', label);
    pop.appendChild(list);
    const items = [];
    const addOpt = (o) => {
      const d = document.createElement('div');
      d.className = 'qzc-opt';
      d.setAttribute('role', 'option');
      d.textContent = o.textContent;
      d.title = o.textContent;
      d.setAttribute('aria-selected', o.selected ? 'true' : 'false');
      if (o.disabled) d.setAttribute('aria-disabled', 'true');
      const idx = items.length;
      d.addEventListener('mousemove', () => setAct(idx));
      d.addEventListener('mousedown', e => e.preventDefault());
      d.addEventListener('click', () => choose(idx));
      items.push({ o, d });
      list.appendChild(d);
    };
    [...sel.children].forEach(ch => {
      if (ch.tagName === 'OPTGROUP') {
        if (items.length) { const s = document.createElement('div'); s.className = 'qzc-sep'; list.appendChild(s); }
        const g = document.createElement('div'); g.className = 'qzc-grp'; g.textContent = ch.label; list.appendChild(g);
        [...ch.children].forEach(addOpt);
      } else if (ch.tagName === 'OPTION') addOpt(ch);
    });
    let act = Math.max(0, items.findIndex(i => i.o.selected));
    function setAct(i) {
      if (items[act]) items[act].d.classList.remove('qzc-act');
      act = i;
      const it = items[act];
      if (!it) return;
      it.d.classList.add('qzc-act');
      const top = it.d.offsetTop, bottom = top + it.d.offsetHeight;
      if (top < list.scrollTop) list.scrollTop = top - 4;
      else if (bottom > list.scrollTop + list.clientHeight) list.scrollTop = bottom - list.clientHeight + 4;
    }
    function move(dir) {
      let i = act;
      for (let n = 0; n < items.length; n++) { i = (i + dir + items.length) % items.length; if (!items[i].o.disabled) break; }
      setAct(i);
    }
    function choose(i) {
      const it = items[i];
      if (!it || it.o.disabled) return;
      closeAll();
      if (sel.value !== it.o.value) {
        selSetter.call(sel, it.o.value);
        sel.dispatchEvent(new Event('input', { bubbles: true }));
        sel.dispatchEvent(new Event('change', { bubbles: true }));
      }
      sel.focus({ preventScroll: true });
    }
    let typed = '', typedT = 0;
    const onKey = e => {
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeAll(); sel.focus({ preventScroll: true }); return; }
      if (e.key === 'Tab') { closeAll(); return; }
      if (e.key === 'ArrowDown') { e.preventDefault(); move(1); return; }
      if (e.key === 'ArrowUp') { e.preventDefault(); move(-1); return; }
      if (e.key === 'Home') { e.preventDefault(); setAct(0); return; }
      if (e.key === 'End') { e.preventDefault(); setAct(items.length - 1); return; }
      if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); choose(act); return; }
      if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
        clearTimeout(typedT); typed += e.key.toLowerCase(); typedT = setTimeout(() => { typed = ''; }, 600);
        const i = items.findIndex(it => !it.o.disabled && it.o.textContent.trim().toLowerCase().startsWith(typed));
        if (i >= 0) setAct(i);
      }
    };
    mount(pop, sel, 160, onKey);
    setAct(act);
    sel.setAttribute('aria-expanded', 'true');
    const prevClose = cur.close;
    cur.close = () => { prevClose(); sel.removeAttribute('aria-expanded'); };
  }

  document.addEventListener('mousedown', e => {
    if (e.button !== 0) return;
    const sel = e.target.closest && e.target.closest('select');
    if (skip(sel)) return;
    e.preventDefault();
    sel.focus({ preventScroll: true });
    if (cur && cur.el === sel) { closeAll(); return; }
    openSelect(sel);
  }, true);
  document.addEventListener('keydown', e => {
    const sel = e.target;
    if (!(sel instanceof HTMLSelectElement) || skip(sel)) return;
    if (cur && cur.el === sel) return;
    if (['Enter', ' ', 'ArrowDown', 'ArrowUp'].includes(e.key) && !e.altKey) { e.preventDefault(); openSelect(sel); }
  }, true);

  /* ---------- date / time ---------- */
  const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
  const MON3 = MONTHS.map(m => m.slice(0, 3));
  const DOW3 = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  const pad = n => String(n).padStart(2, '0');
  const parse = v => {
    if (!v) return null;
    const m = /^(\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2}))?$/.exec(v);
    if (m) return new Date(+m[1], +m[2] - 1, +m[3], +(m[4] || 0), +(m[5] || 0));
    const t = /^(\d{2}):(\d{2})$/.exec(v);
    if (t) { const d = new Date(); d.setHours(+t[1], +t[2], 0, 0); return d; }
    return null;
  };
  const ser = (d, mode) => mode === 'time' ? pad(d.getHours()) + ':' + pad(d.getMinutes())
    : d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + (mode === 'datetime' ? 'T' + pad(d.getHours()) + ':' + pad(d.getMinutes()) : '');
  const label = (d, mode) => !d ? '' : mode === 'time' ? pad(d.getHours()) + ':' + pad(d.getMinutes())
    : DOW3[d.getDay()] + ' ' + d.getDate() + ' ' + MON3[d.getMonth()] + (mode === 'datetime' ? ', ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) : ' ' + d.getFullYear());
  const sameDay = (a, b) => a && b && a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();

  class QzDatetime extends HTMLElement {
    static get observedAttributes() { return ['value', 'mode', 'placeholder']; }
    connectedCallback() { if (!this.style.display) this.style.display = 'block'; this.render(); }
    attributeChangedCallback() { if (this.isConnected) this.render(); }
    get mode() { const m = this.getAttribute('mode'); return m === 'date' || m === 'time' ? m : 'datetime'; }
    get value() { return this.getAttribute('value') || ''; }
    set value(v) { this.setAttribute('value', v); }
    render() {
      const d = parse(this.value);
      if (!this._btn) {
        const b = document.createElement('button');
        b.type = 'button';
        b.setAttribute('aria-haspopup', 'dialog');
        b.style.cssText = 'width:100%;display:flex;align-items:center;gap:8px;height:38px;padding:0 10px;border:1px solid var(--border);border-radius:8px;background:var(--bg);color:var(--fg);font:inherit;font-weight:400;font-size:13.5px;cursor:pointer;text-align:left';
        b.addEventListener('mouseenter', () => { b.style.borderColor = 'var(--ring)'; });
        b.addEventListener('mouseleave', () => { b.style.borderColor = 'var(--border)'; });
        b.addEventListener('click', () => { if (cur && cur.el === this) closeAll(); else this.open(); });
        b.addEventListener('keydown', e => { if (e.key === 'ArrowDown') { e.preventDefault(); this.open(); } });
        const ic = document.createElement('i');
        ic.style.cssText = 'font-size:15px;color:var(--muted-fg);flex:none;line-height:1;display:inline-flex';
        const tx = document.createElement('span');
        tx.style.cssText = 'flex:1;min-width:0;white-space:nowrap;overflow:hidden;text-overflow:ellipsis';
        const ch = document.createElement('i');
        ch.className = 'icon-chevron-down';
        ch.style.cssText = 'font-size:14px;color:var(--muted-fg);flex:none;line-height:1;display:inline-flex';
        b.append(ic, tx, ch);
        this.appendChild(b);
        this._btn = b; this._ic = ic; this._tx = tx;
      }
      this._ic.className = this.mode === 'time' ? 'icon-clock' : 'icon-calendar';
      this._tx.textContent = d ? label(d, this.mode) : (this.getAttribute('placeholder') || 'Pick a date');
      this._tx.style.color = d ? 'var(--fg)' : 'var(--muted-fg)';
      const aria = (this.getAttribute('aria-label') || '') + (d ? ' ' + label(d, this.mode) : '');
      if (aria.trim()) this._btn.setAttribute('aria-label', aria.trim());
    }
    commit(d) {
      const v = ser(d, this.mode);
      this.setAttribute('value', v);
      this.dispatchEvent(new CustomEvent('change', { bubbles: true, detail: { value: v } }));
    }
    open() { openPicker(this, this.mode, this.value, d => this.commit(d), () => this._btn.focus()); }
  }
  function openPicker(anchor, mode, value, commit, refocus) {
      const now = new Date();
      let sel = parse(value) || (() => { const x = new Date(); x.setMinutes(0, 0, 0); return x; })();
      let view = new Date(sel.getFullYear(), sel.getMonth(), 1);
      const pop = document.createElement('div');
      pop.className = 'qzc-pop';
      pop.setAttribute('role', 'dialog');
      pop.setAttribute('aria-label', mode === 'time' ? 'Choose a time' : 'Choose a date');
      const root = document.createElement('div');
      root.className = 'qzc-cal';
      pop.appendChild(root);
      let focusDay = null;
      const draw = () => {
        root.innerHTML = '';
        if (mode !== 'time') {
          const head = document.createElement('div'); head.className = 'qzc-head';
          const prev = document.createElement('button'); prev.className = 'qzc-nav'; prev.type = 'button'; prev.setAttribute('aria-label', 'Previous month'); prev.innerHTML = '<i class="icon-chevron-left" style="font-size:15px;line-height:1;display:inline-flex"></i>';
          const next = document.createElement('button'); next.className = 'qzc-nav'; next.type = 'button'; next.setAttribute('aria-label', 'Next month'); next.innerHTML = '<i class="icon-chevron-right" style="font-size:15px;line-height:1;display:inline-flex"></i>';
          const title = document.createElement('span'); title.className = 'qzc-title'; title.setAttribute('aria-live', 'polite'); title.textContent = MONTHS[view.getMonth()] + ' ' + view.getFullYear();
          prev.onclick = () => { view = new Date(view.getFullYear(), view.getMonth() - 1, 1); draw(); };
          next.onclick = () => { view = new Date(view.getFullYear(), view.getMonth() + 1, 1); draw(); };
          head.append(prev, title, next);
          root.appendChild(head);
          const grid = document.createElement('div'); grid.className = 'qzc-grid'; grid.setAttribute('role', 'grid');
          ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su'].forEach(w => { const c = document.createElement('span'); c.className = 'qzc-wd'; c.textContent = w; grid.appendChild(c); });
          const start = new Date(view); const off = (start.getDay() + 6) % 7; start.setDate(1 - off);
          for (let k = 0; k < 42; k++) {
            const day = new Date(start.getFullYear(), start.getMonth(), start.getDate() + k);
            const b = document.createElement('button'); b.type = 'button'; b.className = 'qzc-day';
            if (day.getMonth() !== view.getMonth()) b.classList.add('qzc-out');
            if (sameDay(day, now)) b.classList.add('qzc-today');
            const isSel = sameDay(day, sel);
            if (isSel) b.classList.add('qzc-sel');
            b.textContent = day.getDate();
            b.setAttribute('aria-label', label(day, 'date'));
            b.setAttribute('aria-pressed', isSel ? 'true' : 'false');
            b.tabIndex = isSel ? 0 : -1;
            b.dataset.t = day.getTime();
            b.onclick = () => {
              sel = new Date(day.getFullYear(), day.getMonth(), day.getDate(), sel.getHours(), sel.getMinutes());
              if (mode === 'date') { commit(sel); closeAll(); refocus(); }
              else { view = new Date(sel.getFullYear(), sel.getMonth(), 1); focusDay = sel.getTime(); draw(); }
            };
            grid.appendChild(b);
          }
          root.appendChild(grid);
        }
        if (mode !== 'date') {
          const row = document.createElement('div'); row.className = 'qzc-time';
          if (mode === 'time') row.style.cssText = 'margin-top:0;padding-top:0;border-top:0';
          const lab = document.createElement('span'); lab.className = 'qzc-tlabel'; lab.textContent = 'Time';
          const spin = document.createElement('span'); spin.className = 'qzc-spin';
          const unit = (get, step, max, name) => {
            const minus = document.createElement('button'); minus.type = 'button'; minus.className = 'qzc-step'; minus.textContent = '−'; minus.setAttribute('aria-label', 'Earlier ' + name);
            const num = document.createElement('span'); num.className = 'qzc-num'; num.textContent = pad(get());
            const plus = document.createElement('button'); plus.type = 'button'; plus.className = 'qzc-step'; plus.textContent = '+'; plus.setAttribute('aria-label', 'Later ' + name);
            const bump = dir => { const h = sel.getHours(), m = sel.getMinutes(); if (name === 'hour') sel = new Date(sel.getFullYear(), sel.getMonth(), sel.getDate(), (h + dir + 24) % 24, m); else { const nm = (Math.round(m / step) * step + dir * step + 60) % 60; sel = new Date(sel.getFullYear(), sel.getMonth(), sel.getDate(), h, nm); } num.textContent = pad(get()); };
            minus.onclick = () => bump(-1); plus.onclick = () => bump(1);
            return [minus, num, plus];
          };
          const colon = document.createElement('span'); colon.textContent = ':'; colon.style.cssText = 'font-weight:600;padding:0 2px';
          spin.append(...unit(() => sel.getHours(), 1, 23, 'hour'), colon, ...unit(() => sel.getMinutes(), 5, 55, 'minute'));
          row.append(lab, spin);
          root.appendChild(row);
          const foot = document.createElement('div'); foot.className = 'qzc-foot';
          const today = document.createElement('button'); today.type = 'button'; today.className = 'qzc-btn qzc-ghost'; today.textContent = mode === 'time' ? 'Now' : 'Today';
          today.onclick = () => { const n = new Date(); sel = mode === 'time' ? new Date(sel.getFullYear(), sel.getMonth(), sel.getDate(), n.getHours(), Math.round(n.getMinutes() / 5) * 5 % 60) : new Date(n.getFullYear(), n.getMonth(), n.getDate(), sel.getHours(), sel.getMinutes()); view = new Date(sel.getFullYear(), sel.getMonth(), 1); draw(); };
          const done = document.createElement('button'); done.type = 'button'; done.className = 'qzc-btn qzc-primary'; done.textContent = 'Done';
          done.onclick = () => { commit(sel); closeAll(); refocus(); };
          foot.append(today, done);
          root.appendChild(foot);
        }
        const f = root.querySelector(focusDay != null ? '.qzc-day[data-t="' + focusDay + '"]' : '.qzc-day.qzc-sel') || root.querySelector('.qzc-day:not(.qzc-out)') || root.querySelector('button');
        if (f) setTimeout(() => f.focus({ preventScroll: true }), 0);
        focusDay = null;
      };
      const onKey = e => {
        if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeAll(); refocus(); return; }
        const t = document.activeElement;
        if (!t || !t.classList || !t.classList.contains('qzc-day')) return;
        const delta = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }[e.key];
        if (delta) {
          e.preventDefault();
          const d0 = new Date(+t.dataset.t); const d1 = new Date(d0.getFullYear(), d0.getMonth(), d0.getDate() + delta);
          if (d1.getMonth() !== view.getMonth()) view = new Date(d1.getFullYear(), d1.getMonth(), 1);
          focusDay = d1.getTime(); draw();
        }
      };
      mount(pop, anchor, 0, onKey);
      draw();
      place(pop, anchor, 0);
  }

  const inSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
  const dateMode = el => el instanceof HTMLInputElement ? ({ date: 'date', time: 'time', 'datetime-local': 'datetime' })[el.type] : null;
  const skipIn = el => !dateMode(el) || el.disabled || el.readOnly || el.hasAttribute('data-native') || coarse();
  function openInput(el) {
    const mode = dateMode(el);
    openPicker(el, mode, el.value, d => { const v = ser(d, mode); if (el.value !== v) { inSetter.call(el, v); el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); } }, () => el.focus({ preventScroll: true }));
  }
  document.addEventListener('mousedown', e => { if (e.button !== 0 || skipIn(e.target)) return; e.preventDefault(); e.target.focus({ preventScroll: true }); if (cur && cur.el === e.target) { closeAll(); return; } openInput(e.target); }, true);
  document.addEventListener('click', e => { if (!skipIn(e.target)) e.preventDefault(); }, true);
  document.addEventListener('keydown', e => { const el = e.target; if (skipIn(el) || (cur && cur.el === el)) return; if (e.key === 'Enter' || e.key === ' ' || (e.key === 'ArrowDown' && e.altKey)) { e.preventDefault(); openInput(el); } }, true);
  if (!customElements.get('qz-datetime')) customElements.define('qz-datetime', QzDatetime);
})();
