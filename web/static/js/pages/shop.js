/**
 * KOPMA Express - /belanja (belanja online anggota).
 * Alur: cari barang -> keranjang -> pilih QRIS/Transfer -> konfirmasi (simulasi) -> POST /orders (channel ONLINE).
 */
(async function () {
  const PAGE_SIZE = 20;
  const $ = (s) => document.querySelector(s);
  const rp = (n) => typeof formatCurrency === 'function' ? formatCurrency(n) : 'Rp ' + Number(n || 0).toLocaleString('id-ID');

  // ---- Inisial Avatar Generator
  function getInitials(name) {
    if (!name) return 'ME';
    const clean = name.replace(/[^a-zA-Z0-9\s]/g, '').trim();
    const parts = clean.split(/\s+/).filter(Boolean);
    if (parts.length === 0) return 'ME';
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }

  // ---- Sesi autentikasi anggota (HttpOnly cookie via /api/v1/auth/me)
  let currentUser = null;
  try {
    const res = await fetch('/api/v1/auth/me', {
      headers: { 'Accept': 'application/json' }
    });
    if (!res.ok) {
      location.replace('/login');
      return;
    }
    const json = await res.json();
    if (!json || !json.success || !json.data) {
      location.replace('/login');
      return;
    }
    currentUser = json.data;
  } catch (err) {
    location.replace('/login');
    return;
  }

  const memberName = currentUser.name || (currentUser.member && (currentUser.member.nama || currentUser.member.name)) || currentUser.nim || 'Anggota';
  const memberNim = currentUser.nim || '';

  const nameEl = $('#who-name');
  if (nameEl) nameEl.textContent = memberName;

  const nimEl = $('#who-nim');
  if (nimEl) nimEl.textContent = memberNim ? `NIM: ${memberNim}` : '';

  const avatarEl = $('#who-avatar');
  if (avatarEl) {
    avatarEl.textContent = getInitials(memberName);
    avatarEl.setAttribute('aria-label', `Avatar ${memberName}`);
    avatarEl.setAttribute('title', memberName);
  }

  const logoutBtn = $('#logout');
  if (logoutBtn) {
    logoutBtn.addEventListener('click', async () => {
      try {
        await fetch('/api/v1/auth/logout', { method: 'POST' });
      } catch (_) { }
      location.href = '/login';
    });
  }

  // ---- Util DOM Builder
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
    const box = $('#msg');
    if (!box) return;
    const isOk = kind === 'ok';
    box.className = isOk
      ? 'mb-5 p-3.5 rounded-xl text-sm bg-teal-50 border border-teal-200 text-teal-900 flex items-center justify-between shadow-xs transition'
      : 'mb-5 p-3.5 rounded-xl text-sm bg-rose-50 border border-rose-200 text-rose-900 flex items-center justify-between shadow-xs transition';

    const textContent = el('div', { class: 'flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2' },
      el('b', { class: 'font-semibold' }, title),
      body ? el('span', { class: 'text-xs opacity-90' }, body) : ''
    );

    const closeBtn = el('button', {
      type: 'button',
      class: (isOk ? 'text-teal-700 hover:text-teal-950' : 'text-rose-700 hover:text-rose-950') + ' font-bold text-base leading-none px-1.5 py-0.5 ml-2 cursor-pointer',
      onclick: () => box.classList.add('hidden')
    }, '×');

    box.replaceChildren(textContent, closeBtn);
    box.classList.remove('hidden');
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  function errorText(e) {
    if (e.status === 409 && Array.isArray(e.details) && e.details.length) {
      return e.details.map((d) => `${d.kode_barang || d.product_code}: diminta ${d.diminta ?? d.requested}, tersedia ${d.tersedia ?? d.available}`).join('; ');
    }
    if (Array.isArray(e.details)) return e.details.map((d) => d.message).filter(Boolean).join('; ');
    return '';
  }

  // ---- Metode Pembayaran Interaktif
  const payMethod = () => {
    const checked = document.querySelector('input[name="pay"]:checked');
    return checked ? checked.value : 'QRIS';
  };

  document.querySelectorAll('input[name="pay"]').forEach((r) => {
    r.addEventListener('change', () => {
      const qrisSelected = payMethod() === 'QRIS';
      const qLabel = $('#label-pay-qris');
      const tLabel = $('#label-pay-transfer');
      if (qLabel && tLabel) {
        if (qrisSelected) {
          qLabel.className = 'flex items-center justify-center gap-2 p-2.5 border-2 border-teal-700 bg-teal-50/30 rounded-xl cursor-pointer text-xs font-semibold text-teal-900 transition';
          tLabel.className = 'flex items-center justify-center gap-2 p-2.5 border border-slate-200 hover:bg-slate-50 rounded-xl cursor-pointer text-xs font-medium text-slate-700 transition';
        } else {
          tLabel.className = 'flex items-center justify-center gap-2 p-2.5 border-2 border-teal-700 bg-teal-50/30 rounded-xl cursor-pointer text-xs font-semibold text-teal-900 transition';
          qLabel.className = 'flex items-center justify-center gap-2 p-2.5 border border-slate-200 hover:bg-slate-50 rounded-xl cursor-pointer text-xs font-medium text-slate-700 transition';
        }
      }
    });
  });

  // ---- Barang & Katalog
  const cart = new Map(); // kode_barang -> {kode_barang, nama_barang, harga, stok, qty}
  let page = 1, totalPages = 1, query = '', timer = null, seq = 0;
  const productList = $('#products');

  function productRow(p) {
    const inCart = cart.get(p.kode_barang)?.qty || 0;
    const stok = typeof p.stok === 'number' ? p.stok : 0;
    const isOutOfStock = stok < 1;
    const isCartFull = inCart >= stok;
    const disabled = isOutOfStock || isCartFull;
    const btnText = isOutOfStock ? 'Habis' : (isCartFull ? 'Penuh' : '+ Tambah');

    const btnClass = disabled
      ? 'px-3.5 py-1.5 bg-slate-100 text-slate-400 border border-slate-200 text-xs font-semibold rounded-lg cursor-not-allowed shrink-0'
      : 'px-3.5 py-1.5 bg-teal-50 hover:bg-teal-700 text-teal-800 hover:text-white border border-teal-200 hover:border-teal-700 text-xs font-semibold rounded-lg transition active:scale-95 shrink-0 cursor-pointer';

    return el('li', { class: 'py-3 flex flex-wrap sm:flex-nowrap items-center justify-between gap-3 group border-b border-slate-100 last:border-0' },
      el('div', { class: 'flex items-center gap-3' },
        el('div', { class: 'w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-teal-50 border border-teal-100/70 flex items-center justify-center text-teal-800 shrink-0 font-bold text-xs select-none' },
          (p.nama_barang || 'P').charAt(0).toUpperCase()
        ),
        el('div', {},
          el('div', { class: 'text-sm font-semibold text-slate-800 leading-snug' }, p.nama_barang),
          el('div', { class: 'text-xs text-slate-400 flex items-center gap-2 mt-0.5' },
            el('span', { class: 'font-mono text-slate-500' }, `KODE: ${p.kode_barang}`),
            p.kategori ? el('span', {}, '•') : '',
            p.kategori ? el('span', { class: 'text-slate-500' }, p.kategori) : '',
            el('span', {}, '•'),
            el('span', { class: isOutOfStock ? 'text-rose-600 font-medium' : 'text-emerald-700 font-medium' }, `Stok ${stok}`)
          )
        )
      ),
      el('div', { class: 'flex items-center justify-between sm:justify-end gap-3 w-full sm:w-auto pt-1 sm:pt-0' },
        el('div', { class: 'text-left sm:text-right' },
          el('div', { class: 'text-sm font-bold text-teal-800' }, rp(p.harga))
        ),
        el('button', {
          type: 'button',
          class: btnClass,
          onclick: () => !disabled && addToCart(p),
          ...(disabled ? { disabled: '' } : {})
        }, btnText)
      )
    );
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
      productList.replaceChildren(el('li', { class: 'py-6 text-center text-xs text-rose-500 bg-rose-50 rounded-xl' }, 'Gagal memuat barang: ' + e.message));
      $('#more').classList.add('hidden');
    }
  }

  function renderProducts() {
    productList.replaceChildren(...(shown.length
      ? shown.map(productRow)
      : [el('li', { class: 'py-8 text-center text-xs text-slate-400' }, query ? 'Barang tidak ditemukan.' : 'Belum ada barang tersedia.')]));
    $('#more').classList.toggle('hidden', page >= totalPages);
  }

  $('#q').addEventListener('input', (ev) => {
    clearTimeout(timer);
    timer = setTimeout(() => { query = ev.target.value.trim(); loadProducts(true); }, 250);
  });
  $('#more').addEventListener('click', () => { page += 1; loadProducts(false); });

  // ---- Keranjang
  function addToCart(p) {
    const cur = cart.get(p.kode_barang);
    if (cur) {
      if (cur.qty < p.stok) cur.qty += 1;
    } else {
      cart.set(p.kode_barang, {
        kode_barang: p.kode_barang,
        nama_barang: p.nama_barang,
        harga: p.harga,
        stok: p.stok,
        qty: 1
      });
    }
    renderCart();
    renderProducts();
  }

  function changeQty(code, delta) {
    const it = cart.get(code);
    if (!it) return;
    it.qty += delta;
    if (it.qty < 1) cart.delete(code);
    else if (it.qty > it.stok) it.qty = it.stok;
    renderCart();
    renderProducts();
  }

  function cartTotal() {
    let t = 0;
    for (const it of cart.values()) t += it.harga * it.qty;
    return t;
  }

  function cartRow(it) {
    return el('li', { class: 'py-2.5 flex items-center justify-between text-xs border-b border-slate-100 last:border-0' },
      el('div', { class: 'pr-2 flex-1' },
        el('div', { class: 'font-semibold text-slate-800 leading-tight' }, it.nama_barang),
        el('div', { class: 'text-[11px] text-slate-400 mt-0.5' }, `${it.qty}x @ ${rp(it.harga)}`)
      ),
      el('div', { class: 'flex items-center gap-2 shrink-0' },
        el('div', { class: 'flex items-center border border-slate-200 rounded-md overflow-hidden bg-slate-50 text-[11px]' },
          el('button', {
            type: 'button',
            class: 'px-1.5 py-0.5 text-slate-600 hover:bg-slate-200 transition font-bold cursor-pointer',
            'aria-label': 'Kurangi',
            onclick: () => changeQty(it.kode_barang, -1)
          }, '−'),
          el('span', { class: 'px-1.5 py-0.5 font-bold text-slate-700 min-w-[18px] text-center select-none' }, it.qty),
          el('button', {
            type: 'button',
            class: 'px-1.5 py-0.5 text-slate-600 hover:bg-slate-200 transition font-bold cursor-pointer',
            'aria-label': 'Tambah',
            onclick: () => changeQty(it.kode_barang, 1)
          }, '+')
        ),
        el('span', { class: 'font-semibold text-slate-800 text-right min-w-[68px]' }, rp(it.harga * it.qty)),
        el('button', {
          type: 'button',
          class: 'text-slate-400 hover:text-rose-600 font-bold px-1 transition text-sm cursor-pointer',
          title: 'Hapus barang',
          onclick: () => { cart.delete(it.kode_barang); renderCart(); renderProducts(); }
        }, '×')
      )
    );
  }

  function renderCart() {
    const items = [...cart.values()];
    const totalQty = items.reduce((sum, it) => sum + it.qty, 0);

    const cartBadge = $('#top-cart-badge');
    if (cartBadge) cartBadge.textContent = totalQty;

    const cartCountBadge = $('#cart-count-badge');
    if (cartCountBadge) cartCountBadge.textContent = `${totalQty} item`;

    $('#cart').replaceChildren(...(items.length
      ? items.map(cartRow)
      : [el('li', { class: 'py-6 text-center text-xs text-slate-400' }, 'Keranjang masih kosong.')]
    ));

    $('#total').textContent = rp(cartTotal());
    $('#checkout').disabled = items.length === 0;
  }

  // ---- Modal Konfirmasi & Pemesanan
  const modal = $('#modal');

  $('#checkout').addEventListener('click', () => {
    const method = payMethod();
    $('#modal-method').innerHTML = 'Metode: <b class="text-slate-800">' + (method === 'QRIS' ? 'QRIS Instan' : 'Transfer Bank') + '</b>';
    $('#modal-total').textContent = rp(cartTotal());
    modal.classList.remove('hidden');
  });

  $('#modal-cancel').addEventListener('click', () => modal.classList.add('hidden'));

  modal.addEventListener('click', (ev) => {
    if (ev.target === modal) modal.classList.add('hidden');
  });

  $('#modal-pay').addEventListener('click', async () => {
    const btn = $('#modal-pay');
    btn.disabled = true;
    btn.textContent = 'Memproses...';
    try {
      const r = await apiClient.post('/orders', {
        nim: memberNim,
        channel: 'ONLINE',
        metode_pembayaran: payMethod(),
        items: [...cart.values()].map((it) => ({ kode_barang: it.kode_barang, qty: it.qty })),
      });
      modal.classList.add('hidden');
      cart.clear();
      renderCart();
      notify('ok', 'Pesanan berhasil dibuat!', `Nomor pesanan #${r.data.order_no || r.data.id}. Total ${rp(r.data.total)}.`);
      loadProducts(true);
      loadOrders();
    } catch (e) {
      modal.classList.add('hidden');
      notify('err', 'Gagal membuat pesanan: ' + e.message, errorText(e));
      loadProducts(true); // perbarui kemungkinan stok berubah
    } finally {
      btn.disabled = false;
      btn.textContent = 'Bayar Sekarang';
    }
  });

  // ---- Pesanan Saya
  async function loadOrders() {
    const ul = $('#orders');
    try {
      const r = await apiClient.get('/orders', { nim: memberNim, limit: 10 });
      const data = r.data || [];

      const topOrdersBadge = $('#top-orders-badge');
      if (topOrdersBadge) topOrdersBadge.textContent = data.length;

      const ordersCountBadge = $('#orders-count-badge');
      if (ordersCountBadge) ordersCountBadge.textContent = `Riwayat (${data.length})`;

      ul.replaceChildren(...(data.length ? data.map((o) => {
        const d = new Date(o.created_at);
        const when = isNaN(d) ? '' : d.toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
        const itemsList = o.items || [];
        const itemsSummary = itemsList.map(i => `${i.qty}x ${i.nama_barang || i.product_name || i.kode_barang}`).join(', ') || 'Item pesanan';
        const isCancelled = o.status === 'CANCELLED';
        const statusBadgeClass = isCancelled
          ? 'text-[10px] font-bold px-2 py-0.5 rounded-full bg-rose-100 text-rose-800'
          : 'text-[10px] font-bold px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-800';
        const statusLabel = isCancelled ? 'Dibatalkan' : 'Selesai';

        return el('li', { class: 'p-3 bg-slate-50 border border-slate-200 rounded-xl space-y-2 hover:border-teal-300 transition' },
          el('div', { class: 'flex items-center justify-between' },
            el('span', { class: 'font-mono text-xs font-bold text-teal-900' }, `#${o.order_no || o.id}`),
            el('span', { class: statusBadgeClass }, statusLabel)
          ),
          el('div', { class: 'text-xs text-slate-700 line-clamp-1 font-medium', title: itemsSummary }, itemsSummary),
          el('div', { class: 'flex items-center justify-between pt-1 border-t border-slate-200/70 text-[11px]' },
            el('span', { class: 'text-slate-500' }, 'Total: ', el('b', { class: 'text-slate-800' }, rp(o.total))),
            el('span', { class: 'text-slate-500 font-medium' }, o.metode_pembayaran || 'QRIS')
          )
        );
      }) : [el('li', { class: 'p-6 text-center text-xs text-slate-400 bg-slate-50 rounded-xl border border-dashed border-slate-200' }, 'Belum ada pesanan.')]));
    } catch (e) {
      ul.replaceChildren(el('li', { class: 'p-4 text-center text-xs text-rose-500 bg-rose-50 rounded-xl border border-rose-200' }, 'Gagal memuat pesanan: ' + e.message));
    }
  }

  // Inisialisasi awal
  renderCart();
  loadProducts(true);
  loadOrders();
})();
