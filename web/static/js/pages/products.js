/**
 * KOPMA Express - Master Data Barang
 * Tersambung ke API /api/v1/products (PRD §6.1). Skema mengikuti collection `products`:
 * kode_barang, nama_barang, kategori, harga, stok.
 *
 * Bergantung pada: components/api.js (apiClient, formatCurrency, formatNumber) dan
 * main.js (showToast). Catatan keamanan: semua data dari server dirender lewat esc()
 * atau textContent, dan showToast TIDAK diberi data pengguna (main.js memakai innerHTML).
 */
(function () {
  'use strict';

  // Kategori mengikuti generator dummy PRD §5.4.
  const CATEGORIES = ['ATK', 'Makanan', 'Minuman', 'Seragam', 'Buku', 'Elektronik', 'Kebersihan', 'Lainnya'];

  const state = {
    q: '',
    category: '',
    sort: 'name',
    page: 1,
    limit: 20,
    total: 0,
    totalPages: 0,
    items: new Map(), // kode_barang -> produk, untuk tombol Edit
    editing: null,    // kode_barang yang sedang diedit (null = tambah baru)
    deleting: null,
  };

  const $ = (id) => document.getElementById(id);
  const toast = (type, title, msg) => { if (window.showToast) window.showToast(type, title, msg || ''); };

  function esc(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, (ch) => (
      { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]
    ));
  }

  // ------------------------------------------------------------------ tabel

  const COLSPAN = 6;

  function messageRow(html) {
    $('products-body').innerHTML =
      `<tr><td colspan="${COLSPAN}" class="py-12 px-4 text-center font-body-md text-body-md text-on-surface-variant">${html}</td></tr>`;
  }

  function stockCell(stok) {
    if (stok === 0) {
      return '<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-status-critical-bg text-status-critical-text font-label-sm text-label-sm font-semibold">' +
        '<span class="w-1.5 h-1.5 rounded-full bg-status-critical-text"></span>Habis</span>';
    }
    return `<span class="font-title-md text-title-md text-on-surface font-semibold">${esc(formatNumber(stok))}</span>`;
  }

  function rowHtml(p) {
    const code = esc(p.kode_barang);
    return '<tr class="hover:bg-surface-bright transition-colors">' +
      `<td class="py-3.5 px-4"><span class="font-title-md text-title-md text-primary font-bold">${code}</span></td>` +
      `<td class="py-3.5 px-4"><span class="font-title-md text-title-md text-on-surface">${esc(p.nama_barang)}</span></td>` +
      '<td class="py-3.5 px-4"><span class="px-2.5 py-1 rounded-lg bg-surface-container-high text-tertiary font-label-sm text-label-sm inline-block">' +
      `${esc(p.kategori)}</span></td>` +
      `<td class="py-3.5 px-4 text-right font-title-md text-title-md text-on-surface font-semibold">${esc(formatCurrency(p.harga))}</td>` +
      `<td class="py-3.5 px-4 text-center">${stockCell(p.stok)}</td>` +
      '<td class="py-3.5 px-4 text-center"><div class="inline-flex items-center gap-1">' +
      `<button type="button" data-action="edit" data-code="${code}" title="Edit barang" class="p-1.5 rounded-lg text-on-surface-variant hover:text-primary hover:bg-surface-container-low transition-colors"><span class="material-symbols-outlined text-[18px]">edit</span></button>` +
      `<button type="button" data-action="delete" data-code="${code}" title="Hapus barang" class="p-1.5 rounded-lg text-on-surface-variant hover:text-error hover:bg-surface-container-low transition-colors"><span class="material-symbols-outlined text-[18px]">delete</span></button>` +
      '</div></td></tr>';
  }

  function renderRows(items) {
    state.items = new Map(items.map((p) => [p.kode_barang, p]));
    if (items.length === 0) {
      const filtered = state.q || state.category;
      messageRow(filtered ? 'Tidak ada barang yang cocok dengan pencarian/filter.' : 'Belum ada barang. Klik "Tambah Barang Baru" atau import dari Excel.');
      return;
    }
    $('products-body').innerHTML = items.map(rowHtml).join('');
  }

  function renderPager() {
    const { page, limit, total, totalPages } = state;
    if (total === 0) {
      $('page-info').textContent = 'Tidak ada barang';
    } else {
      const from = (page - 1) * limit + 1;
      const to = Math.min(page * limit, total);
      $('page-info').textContent = `Menampilkan ${from}-${to} dari ${formatNumber(total)} barang`;
    }
    $('page-indicator').textContent = totalPages > 0 ? `Halaman ${page} dari ${totalPages}` : '';
    $('prev-btn').disabled = page <= 1;
    $('next-btn').disabled = page >= totalPages;
  }

  // Menolak respons lama bila pengguna mengetik cepat (request yang terlambat tiba).
  let loadSeq = 0;

  async function load() {
    const seq = ++loadSeq;
    messageRow('Memuat data…');
    try {
      const params = { page: state.page, limit: state.limit, sort: state.sort };
      if (state.q) params.q = state.q;
      if (state.category) params.category = state.category;

      const res = await apiClient.get('/products', params);
      if (seq !== loadSeq) return;

      const meta = res.meta || {};
      state.total = meta.total || 0;
      state.totalPages = meta.total_pages || 0;

      // Halaman melewati akhir data (mis. item terakhir di halaman ini baru dihapus): mundur.
      if (state.page > 1 && res.data.length === 0 && state.total > 0) {
        state.page = Math.max(1, state.totalPages);
        return load();
      }

      renderRows(res.data || []);
      renderPager();
    } catch (err) {
      if (seq !== loadSeq) return;
      state.total = 0;
      state.totalPages = 0;
      renderPager();
      messageRow('Gagal memuat data barang. ' +
        '<button type="button" data-action="retry" class="ml-1 text-primary font-semibold underline">Coba lagi</button>');
      toast('error', 'Gagal memuat data', err && err.status ? err.message : 'Tidak dapat terhubung ke server.');
    }
  }

  // -------------------------------------------------------------------- KPI

  function setText(id, text) { const n = $(id); if (n) n.textContent = text; }

  async function loadKpis() {
    // Total barang aktif: dari meta list tanpa filter (selalu tersedia).
    try {
      const r = await apiClient.get('/products', { limit: 1 });
      setText('kpi-total', formatNumber((r.meta && r.meta.total) || 0));
    } catch (e) {
      setText('kpi-total', '—');
    }

    // Valuasi & stok kritis: dari modul analitik (rekan tim). Bila belum aktif -> "—".
    try {
      const r = await apiClient.get('/analytics/summary');
      const d = r.data || {};
      setText('kpi-valuation', formatCurrency(d.total_valuasi_gudang));
      setText('kpi-critical', formatNumber(d.critical_count) + ' barang');
      setText('kpi-critical-note', 'Status CRITICAL dari forecast');
    } catch (e) {
      setText('kpi-valuation', '—');
      setText('kpi-critical', '—');
      setText('kpi-critical-note', 'Tersedia setelah modul analitik & forecast aktif');
    }
  }

  // ------------------------------------------------------------------ modal

  function show(id, visible) { $(id).classList.toggle('hidden', !visible); }

  function fillCategorySelect(select, withAll) {
    const opts = (withAll ? ['<option value="">Semua Kategori</option>'] : [])
      .concat(CATEGORIES.map((c) => `<option value="${esc(c)}">${esc(c)}</option>`));
    select.innerHTML = opts.join('');
  }

  function clearFormErrors() {
    document.querySelectorAll('[data-error-for]').forEach((n) => { n.textContent = ''; n.classList.add('hidden'); });
    const g = $('form-error');
    g.textContent = '';
    g.classList.add('hidden');
  }

  function setFieldError(field, message) {
    const n = document.querySelector(`[data-error-for="${field}"]`);
    if (n) { n.textContent = message; n.classList.remove('hidden'); return true; }
    return false;
  }

  function setGeneralError(message) {
    const g = $('form-error');
    g.textContent = message;
    g.classList.remove('hidden');
  }

  function openForm(product) {
    clearFormErrors();
    state.editing = product ? product.kode_barang : null;

    $('modal-title').textContent = product ? 'Edit Barang' : 'Tambah Barang Baru';
    $('submit-label').textContent = product ? 'Simpan Perubahan' : 'Simpan Barang';

    const kode = $('f-kode');
    kode.value = product ? product.kode_barang : '';
    kode.readOnly = !!product; // kode tidak bisa diubah (PUT mengambil kode dari URL)
    kode.classList.toggle('opacity-60', !!product);

    $('f-nama').value = product ? product.nama_barang : '';

    const cat = $('f-kategori');
    fillCategorySelect(cat, false);
    if (product) {
      // Kategori hasil import Excel bisa di luar daftar bawaan: tambahkan agar tidak hilang.
      if (!CATEGORIES.includes(product.kategori)) {
        cat.insertAdjacentHTML('afterbegin', `<option value="${esc(product.kategori)}">${esc(product.kategori)}</option>`);
      }
      cat.value = product.kategori;
    }

    $('f-harga').value = product ? product.harga : '';
    $('f-stok').value = product ? product.stok : '';

    show('item-modal', true);
    (product ? $('f-nama') : kode).focus();
  }

  function closeForm() { show('item-modal', false); }

  function setSaving(saving) {
    $('submit-btn').disabled = saving;
    $('submit-label').textContent = saving ? 'Menyimpan…' : (state.editing ? 'Simpan Perubahan' : 'Simpan Barang');
  }

  function showSaveError(err) {
    if (!err || !err.status) {
      setGeneralError('Tidak dapat terhubung ke server. Coba lagi.');
      return;
    }
    if (err.code === 'VALIDATION_ERROR' && Array.isArray(err.details) && err.details.length) {
      const unmatched = [];
      err.details.forEach((d) => { if (!setFieldError(d.field, d.message)) unmatched.push(d.message); });
      if (unmatched.length) setGeneralError(unmatched.join('. '));
      return;
    }
    if (err.code === 'DUPLICATE') {
      setFieldError('kode_barang', 'Kode barang sudah digunakan');
      return;
    }
    if (err.code === 'NOT_FOUND') {
      setGeneralError('Barang tidak ditemukan (mungkin sudah dihapus). Daftar akan dimuat ulang.');
      load();
      return;
    }
    setGeneralError(err.message || 'Gagal menyimpan barang.');
  }

  async function onSubmit(e) {
    e.preventDefault();
    clearFormErrors();

    const body = {
      nama_barang: $('f-nama').value.trim(),
      kategori: $('f-kategori').value,
      harga: Number($('f-harga').value),
      stok: Number($('f-stok').value),
    };

    setSaving(true);
    try {
      if (state.editing) {
        await apiClient.put(`/products/${encodeURIComponent(state.editing)}`, body);
        toast('success', 'Barang diperbarui', 'Perubahan tersimpan.');
      } else {
        body.kode_barang = $('f-kode').value.trim();
        await apiClient.post('/products', body);
        toast('success', 'Barang ditambahkan', 'Barang baru tersimpan.');
      }
      closeForm();
      load();
      loadKpis();
    } catch (err) {
      showSaveError(err);
    } finally {
      setSaving(false);
    }
  }

  // ------------------------------------------------------------------ hapus

  function askDelete(code) {
    const p = state.items.get(code);
    if (!p) return;
    state.deleting = code;
    $('delete-name').textContent = `${p.nama_barang} (${p.kode_barang})`; // textContent: aman
    show('delete-modal', true);
  }

  async function confirmDelete() {
    const code = state.deleting;
    if (!code) return;
    $('delete-confirm').disabled = true;
    try {
      await apiClient.delete(`/products/${encodeURIComponent(code)}`);
      toast('success', 'Barang dihapus', 'Barang dinonaktifkan dan tidak tampil lagi di daftar.');
      show('delete-modal', false);
      state.deleting = null;
      load();
      loadKpis();
    } catch (err) {
      show('delete-modal', false);
      toast('error', 'Gagal menghapus', err && err.status ? err.message : 'Tidak dapat terhubung ke server.');
      load();
    } finally {
      $('delete-confirm').disabled = false;
    }
  }

  // ------------------------------------------------------------------- init

  function debounce(fn, ms) {
    let t;
    return function () { clearTimeout(t); t = setTimeout(fn, ms); };
  }

  function init() {
    fillCategorySelect($('filter-category'), true);

    $('search-input').addEventListener('input', debounce(() => {
      state.q = $('search-input').value.trim();
      state.page = 1;
      load();
    }, 300));

    $('filter-category').addEventListener('change', () => {
      state.category = $('filter-category').value;
      state.page = 1;
      load();
    });

    $('filter-sort').addEventListener('change', () => {
      state.sort = $('filter-sort').value;
      state.page = 1;
      load();
    });

    $('page-size').addEventListener('change', () => {
      state.limit = Number($('page-size').value);
      state.page = 1;
      load();
    });

    $('reset-btn').addEventListener('click', () => {
      state.q = ''; state.category = ''; state.sort = 'name'; state.page = 1;
      $('search-input').value = '';
      $('filter-category').value = '';
      $('filter-sort').value = 'name';
      load();
    });

    $('prev-btn').addEventListener('click', () => { if (state.page > 1) { state.page -= 1; load(); } });
    $('next-btn').addEventListener('click', () => { if (state.page < state.totalPages) { state.page += 1; load(); } });

    // Delegasi klik untuk tombol di dalam baris tabel (baris dirender ulang tiap load).
    $('products-body').addEventListener('click', (e) => {
      const btn = e.target.closest('[data-action]');
      if (!btn) return;
      const action = btn.dataset.action;
      if (action === 'retry') return load();
      const code = btn.dataset.code;
      if (action === 'edit' && state.items.has(code)) openForm(state.items.get(code));
      if (action === 'delete') askDelete(code);
    });

    $('open-modal-btn').addEventListener('click', () => openForm(null));
    $('close-modal-btn').addEventListener('click', closeForm);
    $('cancel-modal-btn').addEventListener('click', closeForm);
    $('item-modal').addEventListener('click', (e) => { if (e.target === $('item-modal')) closeForm(); });
    $('product-form').addEventListener('submit', onSubmit);

    $('delete-cancel').addEventListener('click', () => show('delete-modal', false));
    $('delete-confirm').addEventListener('click', confirmDelete);
    $('delete-modal').addEventListener('click', (e) => { if (e.target === $('delete-modal')) show('delete-modal', false); });

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') { closeForm(); show('delete-modal', false); }
    });

    load();
    loadKpis();
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();