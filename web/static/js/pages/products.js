/**
 * KOPMA Express - Master Data Barang Page Script
 */

(function() {
  const openBtn = document.getElementById('open-modal-btn');
  const closeBtn = document.getElementById('close-modal-btn');
  const cancelBtn = document.getElementById('cancel-modal-btn');
  const modal = document.getElementById('item-modal');
  const form = document.getElementById('add-product-form');

  function toggleModal(show) {
    if (!modal) return;
    if (show) {
      modal.classList.remove('hidden');
    } else {
      modal.classList.add('hidden');
    }
  }

  if (openBtn) openBtn.addEventListener('click', () => toggleModal(true));
  if (closeBtn) closeBtn.addEventListener('click', () => toggleModal(false));
  if (cancelBtn) cancelBtn.addEventListener('click', () => toggleModal(false));
  
  // Close modal on click backdrop
  if (modal) {
    modal.addEventListener('click', (e) => {
      if (e.target === modal) toggleModal(false);
    });
  }

  if (form) {
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      toggleModal(false);
      if (window.showToast) {
        window.showToast('success', 'Barang Disimpan', 'Data barang berhasil diperbarui.');
      }
    });
  }

  // Check all checkboxes in table
  const checkAll = document.getElementById('check-all');
  const rowChecks = document.querySelectorAll('.row-checkbox');
  if (checkAll) {
    checkAll.addEventListener('change', () => {
      rowChecks.forEach(cb => cb.checked = checkAll.checked);
    });
  }

  // Quick instant search filter on client side
  const searchInput = document.getElementById('table-search-input');
  if (searchInput) {
    searchInput.addEventListener('input', function() {
      const query = this.value.toLowerCase();
      const rows = document.querySelectorAll('tbody tr');
      rows.forEach(row => {
        const text = row.innerText.toLowerCase();
        row.style.display = text.includes(query) ? '' : 'none';
      });
    });
  }
})();
