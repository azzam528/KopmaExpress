/**
 * KOPMA Express - New Order (/orders/new) Live Interactive Script
 * Handles real-time product search, cart operations, cash calculation, and atomic order creation.
 */

(function () {
  const $ = (s) => document.querySelector(s);
  const rp = (n) => formatCurrency(n);
  const BTN = 'w-7 h-7 rounded-lg bg-canvas-bg text-on-surface hover:bg-surface-container';
  let cart = [], total = null, pay = 'CASH', catalogOk = true, lastResults = [], timer = null;

  function el(tag, props = {}, ...kids) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(props)) {
      if (k === 'class') n.className = v;
      else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
      else n.setAttribute(k, v);
    }
    for (const kid of kids.flat()) n.append(kid instanceof Node ? kid : String(kid ?? ''));
    return n;
  }

  function alertBox(kind, title, body) {
    const cls = { ok: 'bg-status-success-bg text-status-success-text', err: 'bg-status-critical-bg text-status-critical-text', warn: 'bg-secondary-container/30 text-status-warning-text' }[kind];
    $('#alert').replaceChildren(el('div', { class: 'p-space-md rounded-xl mb-space-md ' + cls }, el('div', { class: 'font-title-md text-title-md' }, title), body || ''));
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  function detailsTable(d) {
    if (!Array.isArray(d) || !d.length || typeof d[0] !== 'object') return null;
    const cols = Object.keys(d[0]);
    return el('table', { class: 'mt-2 text-left text-label-md' },
      el('thead', {}, el('tr', {}, cols.map((c) => el('th', { class: 'pr-6 font-semibold' }, c)))),
      el('tbody', {}, d.map((r) => el('tr', {}, cols.map((c) => el('td', { class: 'pr-6' }, r[c]))))));
  }

  function calcChange() {
    const paid = parseInt(($('#cash').value || '').replace(/[^0-9]/g, '') || '0', 10);
    $('#change').textContent = total == null ? '-' : (paid >= total ? rp(paid - total) : 'Kurang ' + rp(total - paid));
  }

  function render() {
    const body = $('#cart');
    body.replaceChildren();
    cart.forEach((it, i) => body.append(el('tr', { class: 'border-b border-surface-container' },
      el('td', { class: 'py-space-sm pr-space-sm' },
        el('div', { class: 'font-title-md text-title-md text-on-surface' }, it.name || it.code),
        el('div', { class: 'text-label-sm font-label-sm text-outline' }, 'SKU: ' + it.code + (it.stock != null ? ' • Stok: ' + it.stock : ''))),
      el('td', { class: 'text-right' }, it.price != null ? rp(it.price) : '-'),
      el('td', {}, el('div', { class: 'flex items-center justify-center gap-1.5' },
        el('button', { type: 'button', class: BTN, onclick: () => chg(i, -1) }, '−'),
        el('span', { class: 'w-8 text-center font-title-md' }, it.qty),
        el('button', { type: 'button', class: BTN, onclick: () => chg(i, 1) }, '+'))),
      el('td', { class: 'text-right font-title-md text-primary' }, it.price != null ? rp(it.price * it.qty) : '-'),
      el('td', { class: 'text-center' }, el('button', { type: 'button', title: 'Hapus', class: 'p-1.5 rounded-lg text-outline hover:text-error hover:bg-status-critical-bg', onclick: () => { cart.splice(i, 1); render(); } }, '✕'))
    )));
    $('#cart-empty').classList.toggle('hidden', cart.length > 0);
    $('#cart-count').textContent = `${cart.length} jenis barang (${cart.reduce((a, c) => a + c.qty, 0)} unit)`;
    total = cart.length && cart.every((c) => c.price != null) ? cart.reduce((a, c) => a + c.price * c.qty, 0) : null;
    $('#total').textContent = cart.length === 0 ? rp(0) : (total == null ? 'Dihitung server' : rp(total));
    calcChange();
  }

  function chg(i, d) {
    const it = cart[i], q = it.qty + d;
    if (q < 1) return;
    if (it.stock != null && q > it.stock) { alertBox('warn', 'Stok terbatas', `Stok ${it.name || it.code} hanya ${it.stock}.`); return; }
    it.qty = q; render();
  }

  function addToCart(p) {
    const ex = cart.find((c) => c.code === p.kode_barang);
    if (ex) chg(cart.indexOf(ex), 1);
    else { cart.push({ code: p.kode_barang, name: p.nama_barang, price: p.harga, stock: p.stok, qty: 1 }); render(); }
  }

  // ---- pilih barang (butuh GET /products dari Orang 1; jika belum ada -> ketik kode manual)
  function unavailable() {
    catalogOk = false;
    $('#catalog-note').textContent = 'Daftar barang belum tersedia (GET /products belum aktif). Ketik kode barang lalu tekan Enter.';
    $('#suggest').classList.add('hidden');
  }

  async function search(q) {
    const ul = $('#suggest');
    try {
      const r = await apiClient.get('/products', { q, limit: 8 });
      catalogOk = true; $('#catalog-note').textContent = '';
      lastResults = r.data || [];
      ul.replaceChildren(...(lastResults.length ? lastResults.map((p) => {
        const out = p.is_active === false || p.stok < 1;
        return el('li', { class: 'px-space-md py-space-sm flex justify-between gap-3 ' + (out ? 'opacity-50' : 'cursor-pointer hover:bg-canvas-bg'),
          onclick: () => { if (out) return; addToCart(p); $('#q').value = ''; ul.classList.add('hidden'); } },
          el('div', {}, el('div', { class: 'font-title-md text-title-md' }, p.nama_barang), el('div', { class: 'text-label-sm font-label-sm text-outline' }, p.kode_barang + ' • Stok ' + p.stok)),
          el('div', { class: 'text-label-lg font-label-lg text-primary whitespace-nowrap' }, out ? 'Habis' : rp(p.harga)));
      }) : [el('li', { class: 'px-space-md py-space-sm text-outline' }, 'Barang tidak ditemukan.')]));
      ul.classList.remove('hidden');
    } catch (e) {
      if (e.status === 404 || !e.status) unavailable(); else alertBox('err', 'Gagal mencari barang', e.message);
    }
  }

  $('#q').addEventListener('input', (e) => { clearTimeout(timer); const v = e.target.value.trim(); timer = setTimeout(() => (catalogOk ? search(v) : null), 250); });
  $('#q').addEventListener('focus', (e) => { if (catalogOk) search(e.target.value.trim()); });
  $('#q').addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const v = e.target.value.trim();
    if (!v) return;
    if (!catalogOk) addToCart({ kode_barang: v, nama_barang: null, harga: null, stok: null });
    else { const p = lastResults.find((x) => x.is_active !== false && x.stok > 0); if (p) addToCart(p); }
    e.target.value = ''; $('#suggest').classList.add('hidden');
  });
  document.addEventListener('click', (e) => { if (!e.target.closest('#q, #suggest')) $('#suggest').classList.add('hidden'); });
  // ---- pilih anggota (butuh GET /members; jika belum ada -> ketik NIM manual)
  let memberOk = true, memberTimer = null, memberSeq = 0, memberResults = [], memberQuery = null;

  function setMemberInfo(m) {
    $('#nim-info').textContent = m ? `✓ ${m.nama}${m.unit ? ' • ' + m.unit : ''}` : '';
  }

  function pickMember(m) {
    $('#nim').value = m.nim;
    $('#nim-suggest').classList.add('hidden');
    setMemberInfo(m);
  }

  async function searchMember(q) {
    const ul = $('#nim-suggest'), seq = ++memberSeq;
    try {
      const r = await apiClient.get('/members', { q, limit: 8 });
      if (seq !== memberSeq) return; // respons usang, abaikan
      memberOk = true; $('#nim-note').textContent = '';
      memberResults = r.data || []; memberQuery = q;
      setMemberInfo(memberResults.find((m) => m.nim === q && m.is_active !== false) || null);
      ul.replaceChildren(...(memberResults.length ? memberResults.map((m) => {
        const off = m.is_active === false;
        return el('li', { class: 'px-space-md py-space-sm flex justify-between gap-3 ' + (off ? 'opacity-50' : 'cursor-pointer hover:bg-canvas-bg'),
          onclick: () => { if (!off) pickMember(m); } },
          el('div', {}, el('div', { class: 'font-title-md text-title-md' }, m.nama), el('div', { class: 'text-label-sm font-label-sm text-outline' }, m.nim + (m.unit ? ' • ' + m.unit : ''))),
          off ? el('div', { class: 'text-label-lg font-label-lg text-status-critical-text whitespace-nowrap' }, 'Tidak aktif') : '');
      }) : [el('li', { class: 'px-space-md py-space-sm text-outline' }, 'Anggota tidak ditemukan. NIM tetap bisa diketik penuh.')]));
      ul.classList.remove('hidden');
    } catch (e) {
      if (seq !== memberSeq) return;
      if (e.status === 404 || !e.status) { memberOk = false; $('#nim-note').textContent = 'Daftar anggota belum tersedia. Ketik NIM secara manual.'; ul.classList.add('hidden'); }
      else alertBox('err', 'Gagal mencari anggota', e.message);
    }
  }

  $('#nim').addEventListener('input', (e) => {
    setMemberInfo(null); // NIM diubah -> label pilihan sebelumnya tidak berlaku lagi
    clearTimeout(memberTimer);
    const v = e.target.value.trim();
    memberTimer = setTimeout(() => (memberOk ? searchMember(v) : null), 250);
  });
  $('#nim').addEventListener('focus', (e) => { if (memberOk) searchMember(e.target.value.trim()); });
  $('#nim').addEventListener('keydown', (e) => {
    if (e.key === 'Escape') { $('#nim-suggest').classList.add('hidden'); return; }
    if (e.key !== 'Enter') return;
    e.preventDefault();
    clearTimeout(memberTimer); memberSeq++; // batalkan pencarian yang masih tertunda
    const v = e.target.value.trim();
    // pilih dari hasil hanya bila hasil itu untuk teks yang sedang tampil (hindari hasil usang)
    if (memberOk && memberQuery === v) {
      const m = memberResults.find((x) => x.is_active !== false && x.nim === v) || memberResults.find((x) => x.is_active !== false);
      if (m) pickMember(m);
    }
    $('#nim-suggest').classList.add('hidden');
  });
  document.addEventListener('click', (e) => { if (!e.target.closest('#nim, #nim-suggest')) $('#nim-suggest').classList.add('hidden'); });
  document.querySelectorAll('input[name=pay]').forEach((r) => r.addEventListener('change', () => {
    pay = r.value; $('#cash-box').classList.toggle('hidden', pay !== 'CASH'); calcChange();
  }));
  $('#cash').addEventListener('input', calcChange)  ;
    $('#btn-pas').addEventListener('click', () => { if (total != null) { $('#cash').value = String(total); calcChange(); } });

  // ---- kirim pesanan
  $('#btn-submit').addEventListener('click', async () => {
    const nim = $('#nim').value.trim();
    if (!nim) return alertBox('err', 'NIM wajib diisi');
    if (!cart.length) return alertBox('err', 'Keranjang masih kosong');
        if (pay === 'CASH' && total != null) {
      const paid = parseInt(($('#cash').value || '').replace(/[^0-9]/g, '') || '0', 10);
      if (paid < total) return alertBox('err', 'Uang tunai belum cukup',
        paid ? `Kurang ${rp(total - paid)} dari total ${rp(total)}.` : `Isi uang tunai yang diterima (total ${rp(total)}) atau klik "Uang pas".`);
    }
    const btn = $('#btn-submit'); btn.disabled = true;
    try {
      const r = await apiClient.post('/orders', { nim, metode_pembayaran: pay, items: cart.map((c) => ({ kode_barang: c.code, qty: c.qty })) });
      const o = r.data;
      alertBox('ok', 'Pesanan berhasil', `${o.order_no} - ${o.nama_anggota} - total ${rp(o.total)}`);
      cart = []; $('#cash').value = ''; render(); recent();
    } catch (e) {
      alertBox(e.status === 409 ? 'warn' : 'err', e.message, detailsTable(e.details));
    } finally { btn.disabled = false; }
  });

  // ---- transaksi terakhir + batalkan
  async function recent() {
    const ul = $('#recent');
    try {
      const r = await apiClient.get('/orders', { limit: 5 });
      ul.replaceChildren(...(r.data.length ? r.data.map((o) => el('li', { class: 'py-space-sm flex items-center justify-between gap-3 border-b border-surface-container' },
        el('div', {}, el('div', { class: 'font-title-md text-title-md' }, o.nama_anggota), el('div', { class: 'text-label-sm font-label-sm text-outline' }, `${o.order_no} • ${rp(o.total)} • ${o.metode_pembayaran}`)),
        el('div', { class: 'flex items-center gap-2' },
          el('span', { class: 'px-2 py-0.5 rounded-full text-label-sm font-label-sm ' + (o.status === 'COMPLETED' ? 'bg-status-success-bg text-status-success-text' : 'bg-status-critical-bg text-status-critical-text') }, o.status),
          o.status === 'COMPLETED' ? el('button', { type: 'button', class: 'text-label-sm font-label-sm text-status-critical-text hover:underline', onclick: () => cancel(o.order_no) }, 'Batalkan') : ''))
      ) : [el('li', { class: 'text-outline py-space-sm' }, 'Belum ada transaksi.')]));
    } catch (e) { ul.replaceChildren(el('li', { class: 'text-outline' }, 'Gagal memuat: ' + e.message)); }
  }

  async function cancel(no) {
    if (!confirm('Batalkan pesanan ' + no + '? Stok akan dikembalikan.')) return;
    try { await apiClient.post('/orders/' + encodeURIComponent(no) + '/cancel'); alertBox('ok', 'Pesanan dibatalkan', no); }
    catch (e) { alertBox('err', e.message); }
    recent();
  }

  apiClient.get('/products', { limit: 1 }).catch((e) => { if (e.status === 404 || !e.status) unavailable(); });
    apiClient.get('/members', { limit: 1 }).catch((e) => {
    if (e.status === 404 || !e.status) { memberOk = false; $('#nim-note').textContent = 'Daftar anggota belum tersedia. Ketik NIM secara manual.'; }
  });
  render(); recent();
})();
