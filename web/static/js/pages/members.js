/**
 * KOPMA Express - Master Data Anggota Page Script
 */

const memberData = [
  {
    name: "Adinda Rahma Salsabila",
    initial: "AR",
    nim: "2105120401",
    prodi: "SI Bisnis Digital",
    phone: "adinda.rahma@student.ac.id",
    terpakai: "Rp 75.000",
    sisa: "Rp 175.000",
    tagihan: "Rp 75.000",
    statusKasbon: "Jatuh tempo 28 Maret"
  },
  {
    name: "Rian Pratama",
    initial: "RP",
    nim: "2205120415",
    prodi: "Teknik Informatika",
    phone: "rian.pratama@student.ac.id",
    terpakai: "Rp 240.000",
    sisa: "Rp 10.000",
    tagihan: "Rp 240.000",
    statusKasbon: "Mendekati Limit Plafon!"
  },
  {
    name: "Siti Maulida",
    initial: "SM",
    nim: "2005120388",
    prodi: "Manajemen Bisnis",
    phone: "s.maulida@student.ac.id",
    terpakai: "Rp 0",
    sisa: "Rp 250.000",
    tagihan: "Rp 0 (Lunas)",
    statusKasbon: "Tidak ada tagihan tertunggak"
  },
  {
    name: "Dimas Fajar Nugraha, M.Kom",
    initial: "DF",
    nim: "198804122019",
    prodi: "Dosen Ilkom",
    phone: "dimas.fajar@univ.ac.id",
    terpakai: "Rp 120.000",
    sisa: "Rp 380.000",
    tagihan: "Rp 120.000",
    statusKasbon: "Auto-debet payroll bulanan"
  },
  {
    name: "Nurul Izzati",
    initial: "NI",
    nim: "2405120102",
    prodi: "Akuntansi Keuangan",
    phone: "nurul.izzati@student.ac.id",
    terpakai: "Rp 0",
    sisa: "Rp 0",
    tagihan: "Rp 0",
    statusKasbon: "Plafon belum aktif"
  },
  {
    name: "Bagus Wicaksono",
    initial: "BW",
    nim: "2105120444",
    prodi: "SI Bisnis Digital",
    phone: "bagus.w@student.ac.id",
    terpakai: "Rp 35.000",
    sisa: "Rp 215.000",
    tagihan: "Rp 35.000",
    statusKasbon: "Jatuh tempo 3 April"
  }
];

window.selectMemberForPreview = function(index) {
  const member = memberData[index];
  if (!member) return;

  const nameEl = document.getElementById('previewCardName');
  const nimEl = document.getElementById('previewCardNIM');
  const prodiEl = document.getElementById('previewCardProdi');
  const phoneEl = document.getElementById('previewCardPhone');
  const avatarEl = document.getElementById('previewCardAvatar');
  const sisaEl = document.getElementById('previewSisaPlafon');
  const tagihanEl = document.getElementById('previewTagihan');
  const statusKasbonEl = document.getElementById('previewStatusKasbon');

  if (nameEl) nameEl.innerText = member.name;
  if (nimEl) nimEl.innerText = "NIM: " + member.nim;
  if (prodiEl) prodiEl.innerText = member.prodi;
  if (phoneEl) phoneEl.innerText = member.phone;
  if (avatarEl) avatarEl.innerText = member.initial;
  if (sisaEl) sisaEl.innerText = member.sisa;
  if (tagihanEl) tagihanEl.innerText = member.tagihan;
  if (statusKasbonEl) statusKasbonEl.innerText = member.statusKasbon;

  // Gentle feedback effect
  const card = document.getElementById('selectedMemberCard');
  if (card) {
    card.classList.add('scale-[1.01]');
    setTimeout(() => card.classList.remove('scale-[1.01]'), 150);
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
