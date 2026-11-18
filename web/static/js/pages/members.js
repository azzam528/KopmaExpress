/**
 * KOPMA Express - Master Anggota (/members)
 * CRUD anggota lewat /api/v1/members (PRD §6.2): cari, filter status, paging,
 * tambah/ubah, nonaktifkan/aktifkan, dan 5 pesanan terakhir per anggota.
 * Semua teks dari server dimasukkan lewat textContent (bukan innerHTML).
 */
(function () {
  const $ = (s) => document.querySelector(s);
  const byId = (id) => document.getElementById(id);
  const PAGE_SIZE = 10;
  const state = { q: '', status: '', page: 1, total: 0, totalPages: 1, editingNim: null };
  let listSeq = 0, detailSeq = 0, searchTimer = null, noticeTimer = null;

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

  function notify(kind, text) {
    const box = $('#notice');
    const cls = kind === 'ok'
      ? 'bg-status-success-bg text-status-success-text'
      : 'bg-status-critical-bg text-status-critical-text';
    box.className = 'mb-space-md p-space-md rounded-xl font-label-lg text-label-lg ' + cls;
    box.textContent = text;
    clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => box.classList.add('hidden'), 5000);
  }

  function fmtDate(iso, withTime) {
    const d = new Date(iso);
    if (!iso || isNaN(d) || d.getFullYear() < 2000) return '-'; // zero time dari Go = tahun 1
    const opt = withTime
      ? { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }
      : { day: 'numeric', month: 'short', year: 'numeric' };
    return d.toLocaleString('id-ID', opt);
  }

  function badge(active) {
    return el('span', {
      class: 'inline-block px-space-sm py-0.5 rounded-full font-label-sm text-label-sm ' +
        (active ? 'bg-status-success-bg text-status-success-text' : 'bg-surface-container text-outline'),
    }, active ? 'Aktif' : 'Nonaktif');
  }

  function iconBtn(icon, title, onclick) {
    return el('button', {
      type: 'button', title, 'aria-label': title, onclick,
      class: 'p-1.5 rounded-lg text-outline hover:bg-surface-container-low hover:text-primary transition-colors',
    }, el('span', { class: 'material-symbols-outlined text-[20px]' }, icon));
  }

  // ------------------------------------------------------------ ringkasan

  async function loadSummary() {
    try {
      const { data } = await apiClient.get('/members/summary');
      $('#sum-total').textContent = formatNumber(data.total);
      $('#sum-active').textContent = formatNumber(data.aktif);
      $('#sum-inactive').textContent = formatNumber(data.nonaktif);
      $('#sum-buyers').textContent = formatNumber(data.bertransaksi_30_hari) + ' bertransaksi dalam 30 hari terakhir';
    } catch (e) {
      // kartu dibiarkan "-"; kegagalan API sudah terlihat di tabel
    }
  }

  // --------------------------------------------------------------- tabel

  function stateRow(text, isError) {
    $('#rows').replaceChildren(el('tr', {},
      el('td', {
        colspan: '6',
        class: 'px-space-md py-space-xl text-center font-body-md text-body-md ' + (isError ? 'text-status-critical-text' : 'text-outline'),
      }, text)));
  }

  function renderRows(list) {
    if (!list.length) {
      stateRow(state.q || state.status ? 'Tidak ada anggota yang cocok dengan pencarian.' : 'Belum ada anggota.');
      return;
    }
    $('#rows').replaceChildren(...list.map((m) => el('tr', {
      class: 'border-t border-surface-container hover:bg-canvas-bg transition-colors' + (m.is_active ? '' : ' opacity-60'),
    },
      el('td', { class: 'px-space-md py-space-sm font-label-lg text-label-lg text-on-surface-variant whitespace-nowrap' }, m.nim),
      el('td', { class: 'px-space-md py-space-sm font-title-md text-title-md text-on-surface' }, m.nama),
      el('td', { class: 'px-space-md py-space-sm font-body-md text-body-md text-on-surface-variant' }, m.unit || '-'),
      el('td', { class: 'px-space-md py-space-sm font-body-md text-body-md text-on-surface-variant whitespace-nowrap' }, m.telepon || '-'),
      el('td', { class: 'px-space-md py-space-sm' }, badge(m.is_active)),
      el('td', { class: 'px-space-md py-space-sm' },
        el('div', { class: 'flex items-center justify-end gap-0.5' },
          iconBtn('receipt_long', 'Riwayat pesanan', () => openDetail(m)),
          iconBtn('edit', 'Ubah', () => openForm(m)),
          iconBtn(m.is_active ? 'person_off' : 'person_check', m.is_active ? 'Nonaktifkan' : 'Aktifkan kembali', () => toggleActive(m)),
          // hapus permanen hanya ditawarkan untuk anggota nonaktif (aturan yang sama ditegakkan server)
          m.is_active ? '' : iconBtn('delete', 'Hapus permanen', () => deletePermanent(m)))))));
  }

  function renderPager(count) {
    const from = state.total === 0 ? 0 : (state.page - 1) * PAGE_SIZE + 1;
    const to = from === 0 ? 0 : from + count - 1;
    $('#range').textContent = state.total === 0 ? '0 anggota' : `Menampilkan ${from}–${to} dari ${state.total}`;
    $('#page-info').textContent = `Halaman ${state.page} dari ${state.totalPages}`;
    $('#prev').disabled = state.page <= 1;
    $('#next').disabled = state.page >= state.totalPages;
  }

  async function loadList() {
    const seq = ++listSeq;
    stateRow('Memuat data anggota...');
    try {
      const params = { page: state.page, limit: PAGE_SIZE };
      if (state.q) params.q = state.q;
      if (state.status) params.status = state.status;
      const r = await apiClient.get('/members', params);
      if (seq !== listSeq) return; // respons usang, abaikan

      state.total = (r.meta && r.meta.total) || 0;
      state.totalPages = Math.max((r.meta && r.meta.total_pages) || 1, 1);
      if (state.page > state.totalPages) { // mis. baris terakhir halaman ini baru saja berpindah
        state.page = state.totalPages;
        return loadList();
      }
      const list = r.data || [];
      renderRows(list);
      renderPager(list.length);
    } catch (e) {
      if (seq !== listSeq) return;
      stateRow('Gagal memuat anggota: ' + e.message, true);
      $('#range').textContent = '';
      $('#page-info').textContent = '';
      $('#prev').disabled = true;
      $('#next').disabled = true;
    }
  }

  function refresh() { loadSummary(); return loadList(); }

  // ---------------------------------------------------------- modal form

  let nimCheckTimer = null, nimCheckSeq = 0;

  function clearNimError() {
    const p = byId('e-nim');
    if (p) {
      p.textContent = '';
      p.classList.add('hidden');
    }
  }

  async function checkNimDuplicate(nim) {
    const seq = ++nimCheckSeq;
    const trimmed = nim.trim();
    if (!trimmed) {
      clearNimError();
      return;
    }
    // Jika sedang ubah dan NIM sama dengan aslinya, jangan anggap duplikat ke diri sendiri
    if (state.editingNim && trimmed === state.editingNim) {
      clearNimError();
      return;
    }
    try {
      const res = await apiClient.get('/members/' + encodeURIComponent(trimmed));
      if (seq !== nimCheckSeq) return; // respons usang, abaikan
      if (res && res.data && res.data.nim) {
        const p = byId('e-nim');
        if (p) {
          p.textContent = `Anggota dengan NIM ${trimmed} sudah terdaftar`;
          p.classList.remove('hidden');
        }
      }
    } catch (e) {
      if (seq !== nimCheckSeq) return;
      // 404 artinya NIM belum terdaftar (tersedia untuk digunakan)
      if (e.status === 404 || e.code === 'NOT_FOUND') {
        clearNimError();
      }
    }
  }

  function clearErrors() {
    clearTimeout(nimCheckTimer);
    document.querySelectorAll('[id^="e-"]').forEach((p) => { p.textContent = ''; p.classList.add('hidden'); });
    const box = $('#form-error');
    if (box) {
      box.textContent = '';
      box.classList.add('hidden');
    }
  }

  function openForm(m) {
    clearTimeout(nimCheckTimer);
    nimCheckSeq++;
    state.editingNim = m ? m.nim : null;
    clearErrors();
    $('#form-title').textContent = m ? 'Ubah Anggota' : 'Tambah Anggota';
    $('#f-nim').value = m ? m.nim : '';
    $('#h-nim').classList.toggle('hidden', !m); // petunjuk hanya saat mengubah
    $('#f-nama').value = m ? m.nama : '';
    $('#f-unit').value = m ? (m.unit || '') : '';
    $('#f-telepon').value = m ? (m.telepon || '') : '';
    $('#form-modal').classList.remove('hidden');
    (m ? $('#f-nama') : $('#f-nim')).focus();
  }

  function closeForm() {
    clearTimeout(nimCheckTimer);
    nimCheckSeq++;
    clearErrors();
    state.editingNim = null;
    $('#member-form').reset();
    $('#form-modal').classList.add('hidden');
  }

  function showFormErrors(e) {
    let shown = false;
    if (Array.isArray(e.details)) {
      e.details.forEach((d) => {
        const p = byId('e-' + d.field);
        if (p) { p.textContent = d.message; p.classList.remove('hidden'); shown = true; }
      });
    }
    if (e.code === 'DUPLICATE') {
      const p = byId('e-nim');
      if (p) {
        p.textContent = e.message;
        p.classList.remove('hidden');
        shown = true;
      }
    }
    if (!shown) {
      const box = $('#form-error');
      if (box) {
        box.textContent = e.message || 'Terjadi kesalahan.';
        box.classList.remove('hidden');
      }
    }
  }

  async function submitForm(ev) {
    ev.preventDefault();
    clearTimeout(nimCheckTimer);
    clearErrors();
    const editing = state.editingNim;
    const nimInput = $('#f-nim').value.trim();
    const namaInput = $('#f-nama').value.trim();

    // Validasi lokal wajib diisi
    if (!nimInput) {
      const p = byId('e-nim');
      if (p) { p.textContent = 'NIM / NIP wajib diisi'; p.classList.remove('hidden'); }
      $('#f-nim').focus();
      return;
    }
    if (!namaInput) {
      const p = byId('e-nama');
      if (p) { p.textContent = 'Nama lengkap wajib diisi'; p.classList.remove('hidden'); }
      $('#f-nama').focus();
      return;
    }

    const body = {
      nama: namaInput,
      unit: $('#f-unit').value.trim(),
      telepon: $('#f-telepon').value.trim(),
    };
    const btn = $('#btn-save');
    btn.disabled = true;
    try {
      if (editing) {
        await apiClient.put('/members/' + encodeURIComponent(editing), { nim: nimInput, ...body });
        notify('ok', nimInput !== editing ? 'Data anggota diperbarui, NIM pada riwayat pesanan ikut diganti.' : 'Data anggota diperbarui.');
      } else {
        await apiClient.post('/members', { nim: nimInput, ...body });
        notify('ok', 'Anggota berhasil ditambahkan.');
      }
      if (!editing || nimInput !== editing) {
        // tampilkan anggota yang baru ditambah / diganti NIM-nya: cari berdasarkan NIM-nya
        state.q = nimInput; state.status = ''; state.page = 1;
        $('#q').value = nimInput; $('#status').value = '';
      }
      closeForm();
      refresh();
    } catch (e) {
      showFormErrors(e);
    } finally {
      btn.disabled = false;
    }
  }

  // ------------------------------------------------ nonaktifkan / aktifkan

  async function toggleActive(m) {
    const deactivate = m.is_active;
    const msg = deactivate
      ? `Nonaktifkan ${m.nama}? Anggota tidak bisa membuat pesanan baru, riwayat pesanan tetap tersimpan.`
      : `Aktifkan kembali ${m.nama}?`;
    if (!confirm(msg)) return;
    try {
      const path = '/members/' + encodeURIComponent(m.nim);
      if (deactivate) await apiClient.delete(path);
      else await apiClient.post(path + '/activate');
      notify('ok', deactivate ? 'Anggota dinonaktifkan.' : 'Anggota diaktifkan kembali.');
      refresh();
    } catch (e) {
      notify('err', e.message);
    }
  }

  async function deletePermanent(m) {
    const msg = `Hapus permanen ${m.nama}? Data anggota akan hilang dan tidak bisa dikembalikan. ` +
      'Hanya bisa untuk anggota yang belum punya pesanan.';
    if (!confirm(msg)) return;
    try {
      await apiClient.delete('/members/' + encodeURIComponent(m.nim) + '?permanent=true');
      notify('ok', 'Anggota dihapus permanen.');
      refresh();
    } catch (e) {
      notify('err', e.message); // mis. "Anggota punya 3 pesanan, nonaktifkan saja"
    }
  }

  // ------------------------------------------------------ modal detail

  async function openDetail(m) {
    const seq = ++detailSeq;
    $('#d-nama').textContent = m.nama;
    $('#d-nim').textContent = m.nim;
    $('#d-unit').textContent = m.unit || '-';
    $('#d-telepon').textContent = m.telepon || '-';
    $('#d-bergabung').textContent = fmtDate(m.joined_at, false);
    $('#d-status').replaceChildren(badge(m.is_active));
    const ul = $('#d-orders');
    ul.replaceChildren(el('li', { class: 'text-outline py-space-sm' }, 'Memuat...'));
    $('#detail-modal').classList.remove('hidden');

    try {
      const r = await apiClient.get('/orders', { nim: m.nim, limit: 5 });
      if (seq !== detailSeq) return;
      const orders = r.data || [];
      ul.replaceChildren(...(orders.length ? orders.map((o) => el('li', {
        class: 'py-space-sm flex items-center justify-between gap-3 border-b border-surface-container',
      },
        el('div', {},
          el('div', { class: 'font-title-md text-title-md text-on-surface' }, o.order_no),
          el('div', { class: 'font-label-sm text-label-sm text-outline' },
            `${fmtDate(o.created_at, true)} • ${o.metode_pembayaran}`)),
        el('div', { class: 'text-right' },
          el('div', { class: 'font-title-md text-title-md text-primary' }, formatCurrency(o.total)),
          el('span', {
            class: 'font-label-sm text-label-sm ' + (o.status === 'COMPLETED' ? 'text-status-success-text' : 'text-status-critical-text'),
          }, o.status)))
      ) : [el('li', { class: 'text-outline py-space-sm' }, 'Belum ada pesanan.')]));
    } catch (e) {
      if (seq !== detailSeq) return;
      ul.replaceChildren(el('li', { class: 'text-status-critical-text py-space-sm' }, 'Gagal memuat pesanan: ' + e.message));
    }
  }

  function closeDetail() { detailSeq++; $('#detail-modal').classList.add('hidden'); }

  // ---------------------------------------------------------------- event

  $('#f-nim').addEventListener('input', (e) => {
    clearNimError();
    clearTimeout(nimCheckTimer);
    const val = e.target.value.trim();
    if (val.length >= 3) {
      nimCheckTimer = setTimeout(() => {
        checkNimDuplicate(val);
      }, 400);
    }
  });

  ['f-nama', 'f-unit', 'f-telepon'].forEach((id) => {
    const inputEl = byId(id);
    if (inputEl) {
      inputEl.addEventListener('input', () => {
        const errEl = byId('e-' + id.replace('f-', ''));
        if (errEl) {
          errEl.textContent = '';
          errEl.classList.add('hidden');
        }
      });
    }
  });

  $('#btn-add').addEventListener('click', () => openForm(null));
  $('#member-form').addEventListener('submit', submitForm);
  $('#btn-cancel').addEventListener('click', closeForm);
  $('#btn-close-form').addEventListener('click', closeForm);
  $('#btn-close-detail').addEventListener('click', closeDetail);
  $('#form-modal').addEventListener('click', (e) => { if (e.target === e.currentTarget) closeForm(); });
  $('#detail-modal').addEventListener('click', (e) => { if (e.target === e.currentTarget) closeDetail(); });
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    closeForm();
    closeDetail();
  });

  $('#q').addEventListener('input', (e) => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => { state.q = e.target.value.trim(); state.page = 1; loadList(); }, 300);
  });
  $('#status').addEventListener('change', (e) => { state.status = e.target.value; state.page = 1; loadList(); });
  $('#prev').addEventListener('click', () => { if (state.page > 1) { state.page--; loadList(); } });
  $('#next').addEventListener('click', () => { if (state.page < state.totalPages) { state.page++; loadList(); } });

  refresh();
})();
