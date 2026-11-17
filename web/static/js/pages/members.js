/**
 * KOPMA Express - Master Data Anggota Page Script
 */

const memberData = [
  {
    name: "Adinda Rahma Salsabila",
    nim: "2105 1204 01",
    prodi: "SI Bisnis Digital '21",
    tier: "GOLD TIER",
    terpakai: "Rp 75.000",
    sisa: "Rp 175.000",
    tagihan: "Rp 75.000",
    status: "Aktif",
    statusKasbon: "Jatuh tempo 28 Maret"
  },
  {
    name: "Rian Pratama",
    nim: "2205 1204 15",
    prodi: "Teknik Informatika '22",
    tier: "SILVER TIER",
    terpakai: "Rp 240.000",
    sisa: "Rp 10.000",
    tagihan: "Rp 240.000",
    status: "Warning",
    statusKasbon: "Mendekati Limit Plafon!"
  },
  {
    name: "Siti Maulida",
    nim: "2005 1203 88",
    prodi: "Manajemen Bisnis '20",
    tier: "PLATINUM TIER",
    terpakai: "Rp 0",
    sisa: "Rp 250.000",
    tagihan: "Rp 0 (Lunas)",
    status: "Aktif",
    statusKasbon: "Tidak ada tagihan tertunggak"
  },
  {
    name: "Dimas Fajar Nugraha, M.Kom",
    nim: "1988 0412 2019",
    prodi: "Dosen Ilkom",
    tier: "FACULTY VIP",
    terpakai: "Rp 120.000",
    sisa: "Rp 380.000",
    tagihan: "Rp 120.000",
    status: "Dosen",
    statusKasbon: "Auto-debet payroll bulanan"
  },
  {
    name: "Nurul Izzati",
    nim: "2405 1201 02",
    prodi: "Akuntansi Keuangan '24",
    tier: "MEMBER BARU",
    terpakai: "Rp 0",
    sisa: "Rp 0",
    tagihan: "Rp 0",
    status: "Pending",
    statusKasbon: "Menunggu validasi KTM"
  },
  {
    name: "Bagus Wicaksono",
    nim: "2105 1204 44",
    prodi: "SI Bisnis Digital '21",
    tier: "GOLD TIER",
    terpakai: "Rp 35.000",
    sisa: "Rp 215.000",
    tagihan: "Rp 35.000",
    status: "Aktif",
    statusKasbon: "Jatuh tempo 3 April"
  }
];

window.selectMemberForPreview = function(index) {
  const member = memberData[index];
  if (!member) return;

  const nameEl = document.getElementById('previewCardName');
  const nimEl = document.getElementById('previewCardNIM');
  const prodiEl = document.getElementById('previewCardProdi');
  const sisaEl = document.getElementById('previewSisaPlafon');
  const tagihanEl = document.getElementById('previewTagihan');

  if (nameEl) nameEl.innerText = member.name;
  if (nimEl) nimEl.innerText = member.nim;
  if (prodiEl) prodiEl.innerText = member.prodi;
  if (sisaEl) sisaEl.innerText = member.sisa;
  if (tagihanEl) tagihanEl.innerText = member.tagihan;
  
  // Quick pop feedback effect
  const card = document.getElementById('virtualCard');
  if (card) {
    card.classList.add('scale-[1.02]');
    setTimeout(() => card.classList.remove('scale-[1.02]'), 150);
  }
};

window.toggleVirtualCardModal = function(show) {
  const modal = document.getElementById('cardModal');
  if (!modal) return;
  if (show) {
    modal.classList.remove('hidden');
  } else {
    modal.classList.add('hidden');
  }
};

document.addEventListener('DOMContentLoaded', () => {
  const searchInput = document.getElementById('memberSearchInput');
  if (searchInput) {
    searchInput.addEventListener('input', function(e) {
      const q = e.target.value.toLowerCase();
      const rows = document.querySelectorAll('tbody tr');
      rows.forEach(row => {
        const text = row.innerText.toLowerCase();
        row.style.display = text.includes(q) ? '' : 'none';
      });
    });
  }
});
