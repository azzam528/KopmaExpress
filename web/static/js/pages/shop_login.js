/**
 * KOPMA Express - /masuk (login sementara anggota, hanya NIM).
 * Hanya pengganti sampai login sungguhan dibuat. Tidak aman: siapa pun yang tahu NIM bisa masuk.
 */
(function () {
  const KEY = 'kopma_member';
  const $ = (s) => document.querySelector(s);

  function showError(text) {
    const box = document.createElement('div');
    box.className = 'note err';
    box.textContent = text;
    $('#msg').replaceChildren(box);
  }

  $('#form').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const nim = $('#nim').value.trim();
    if (!nim) return showError('NIM wajib diisi.');

    const btn = $('#go');
    btn.disabled = true;
    $('#msg').replaceChildren();
    try {
      const r = await apiClient.get('/members/' + encodeURIComponent(nim));
      const m = r.data;
      if (!m.is_active) return showError('Keanggotaan ini tidak aktif. Hubungi pengurus koperasi.');
      try { localStorage.setItem(KEY, JSON.stringify({ nim: m.nim, nama: m.nama, unit: m.unit || '' })); }
      catch (_) { return showError('Browser memblokir penyimpanan lokal, sehingga tidak bisa masuk.'); }
      location.href = '/belanja';
    } catch (e) {
      showError(e.status === 404 ? 'NIM tidak terdaftar sebagai anggota.' : (e.message || 'Gagal masuk, coba lagi.'));
    } finally {
      btn.disabled = false;
    }
  });
})();
