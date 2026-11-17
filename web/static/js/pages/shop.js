/**
 * KOPMA Express - /belanja (belanja online anggota).
 * Alur: cari barang -> keranjang -> pilih QRIS/Transfer -> konfirmasi (simulasi) -> POST /orders (channel ONLINE).
 */
(function () {
  const KEY = 'kopma_member';
  const PAGE_SIZE = 20;
  const $ = (s) => document.querySelector(s);
  const rp = (n) => formatCurrency(n);

  // ---- sesi sementara
  let member = null;
  try { member = JSON.parse(localStorage.getItem(KEY) || 'null'); } catch (_) { /* abaikan */ }
  if (!member || !member.nim) { location.replace('/masuk'); return; }

  $('#who-name').textContent = member.nama || member.nim;
  $('#who-nim').textContent = member.nim;
  $('#logout').addEventListener('click', () => {
    try { localStorage.removeItem(KEY); } catch (_) { /* abaikan */ }
    location.href = '/masuk';
  });

  // ---- util
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

  function notify(kind, title, body) {
    $('#msg').replaceChildren(el('div', { class: 'note ' + kind }, el('b', {}, title), body || ''));
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  function errorText(e) {
    if (e.status === 409 && Array.isArray(e.details) && e.details.length) {
      return e.details.map((d) => `${d.kode_barang || d.product_code}: diminta ${d.diminta ?? d.requested}, tersedia ${d.tersedia ?? d.available}`).join('; ');
    }
    if (Array.isArray(e.details)) return e.details.map((d) => d.message).filter(Boolean).join('; ');
    return '';
  }

  // ---- barang
  const cart = new Map(); // kode_barang -> {kode_barang, nama_barang, harga, stok, qty}
  let page = 1, totalPages = 1, query = '', timer = null, seq = 0;
  const productList = $('#products');

  function productRow(p) {
    const inCart = cart.get(p.kode_barang)?.qty || 0;
    const out = p.stok < 1 || inCart >= p.stok;
    return el('li', { class: 'row' },
      el('div', {},
        el('div', { class: 'name' }, p.nama_barang),
        el('div', { class: 'meta' }, `${p.kategori || ''} • stok ${p.stok}`)),
      el('div', { style: 'text-align:right' },
        el('div', { class: 'price' }, rp(p.harga)),
        el('button', { type: 'button', class: 'btn-add', onclick: () => addToCart(p), ...(out ? { disabled: '' } : {}) },
          p.stok < 1 ? 'Habis' : '+ Tambah')));
  }

  let shown = [];
  async function loadProducts(reset) {
    if (reset) { page = 1; shown = []; }
    const my = ++seq;
    try {
      const r = await apiClient.get('/products', { q: query, page, limit: PAGE_SIZE });
      if (my !== seq) return;
      shown = shown.concat(r.data || []);
      totalPages = r.meta?.total_pages || 1;
      renderProducts();
    } catch (e) {
      if (my !== seq) return;
      productList.replaceChildren(el('li', { class: 'empty' }, 'Gagal memuat barang: ' + e.message));
      $('#more').classList.add('hidden');
    }
  }

  function renderProducts() {
    productList.replaceChildren(...(shown.length
      ? shown.map(productRow)
      : [el('li', { class: 'empty' }, query ? 'Barang tidak ditemukan.' : 'Belum ada barang.')]));
    $('#more').classList.toggle('hidden', page >= totalPages);
  }

  $('#q').addEventListener('input', (ev) => {
    clearTimeout(timer);
    timer = setTimeout(() => { query = ev.target.value.trim(); loadProducts(true); }, 250);
  });
  $('#more').addEventListener('click', () => { page += 1; loadProducts(false); });

  // ---- keranjang
  function addToCart(p) {
    const cur = cart.get(p.kode_barang);
    if (cur) { if (cur.qty < p.stok) cur.qty += 1; }
    else cart.set(p.kode_barang, { kode_barang: p.kode_barang, nama_barang: p.nama_barang, harga: p.harga, stok: p.stok, qty: 1 });
    renderCart(); renderProducts();
  }

  function changeQty(code, delta) {
    const it = cart.get(code);
    if (!it) return;
    it.qty += delta;
    if (it.qty < 1) cart.delete(code);
    else if (it.qty > it.stok) it.qty = it.stok;
    renderCart(); renderProducts();
  }

  function cartTotal() {
    let t = 0;
    for (const it of cart.values()) t += it.harga * it.qty;
    return t;
  }

  function renderCart() {
    const items = [...cart.values()];
    $('#cart').replaceChildren(...(items.length ? items.map((it) =>
      el('li', { class: 'row' },
        el('div', {},
          el('div', { class: 'name' }, it.nama_barang),
          el('div', { class: 'qty' },
            el('button', { type: 'button', 'aria-label': 'Kurangi', onclick: () => changeQty(it.kode_barang, -1) }, '−'),
            el('span', {}, it.qty),
            el('button', { type: 'button', 'aria-label': 'Tambah', onclick: () => changeQty(it.kode_barang, 1) }, '+'))),
        el('div', { class: 'price' }, rp(it.harga * it.qty)))
    ) : [el('li', { class: 'empty' }, 'Keranjang masih kosong.')]));
    $('#total').textContent = rp(cartTotal());
    $('#checkout').disabled = items.length === 0;
  }

  // ---- bayar (simulasi) lalu buat pesanan
  const payMethod = () => document.querySelector('input[name="pay"]:checked').value;
  const modal = $('#modal');

  $('#checkout').addEventListener('click', () => {
    $('#modal-method').textContent = 'Metode: ' + (payMethod() === 'QRIS' ? 'QRIS' : 'Transfer');
    $('#modal-total').textContent = rp(cartTotal());
    modal.classList.remove('hidden');
  });
  $('#modal-cancel').addEventListener('click', () => modal.classList.add('hidden'));

  $('#modal-pay').addEventListener('click', async () => {
    const btn = $('#modal-pay');
    btn.disabled = true;
    try {
      const r = await apiClient.post('/orders', {
        nim: member.nim,
        channel: 'ONLINE',
        metode_pembayaran: payMethod(),
        items: [...cart.values()].map((it) => ({ kode_barang: it.kode_barang, qty: it.qty })),
      });
      modal.classList.add('hidden');
      cart.clear();
      renderCart();
      notify('ok', 'Pesanan berhasil', `Nomor pesanan ${r.data.order_no}. Total ${rp(r.data.total)}.`);
      loadProducts(true);
      loadOrders();
    } catch (e) {
      modal.classList.add('hidden');
      notify('err', e.message, errorText(e));
      loadProducts(true); // stok mungkin berubah
    } finally {
      btn.disabled = false;
    }
  });

  // ---- pesanan saya
  async function loadOrders() {
    const ul = $('#orders');
    try {
      const r = await apiClient.get('/orders', { nim: member.nim, limit: 5 });
      const data = r.data || [];
      ul.replaceChildren(...(data.length ? data.map((o) => {
        const d = new Date(o.created_at);
        const when = isNaN(d) ? '' : d.toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
        const n = (o.items || []).reduce((a, i) => a + i.qty, 0);
        return el('li', { class: 'row' },
          el('div', {},
            el('div', { class: 'name' }, o.order_no),
            el('div', { class: 'meta' }, `${when} • ${n} barang • ${o.metode_pembayaran}`)),
          el('div', { style: 'text-align:right' },
            el('div', { class: 'price' }, rp(o.total)),
            el('span', { class: 'badge' + (o.status === 'CANCELLED' ? ' cancel' : '') }, o.status === 'CANCELLED' ? 'Dibatalkan' : 'Selesai')));
      }) : [el('li', { class: 'empty' }, 'Belum ada pesanan.')]));
    } catch (e) {
      ul.replaceChildren(el('li', { class: 'empty' }, 'Gagal memuat pesanan: ' + e.message));
    }
  }

  renderCart();
  loadProducts(true);
  loadOrders();
})();
