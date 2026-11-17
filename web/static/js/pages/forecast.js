/**
 * KOPMA Express - Forecast & Restock Page Script
 */

document.addEventListener('DOMContentLoaded', () => {
  // Recalculate animation feedback
  const btnRecalculate = document.getElementById('btnRecalculate');
  const spinIcon = document.getElementById('spinIcon');
  if (btnRecalculate && spinIcon) {
    btnRecalculate.addEventListener('click', function() {
      spinIcon.classList.add('rotate-[720deg]');
      btnRecalculate.classList.add('opacity-90');
      
      if (window.apiClient) {
        window.apiClient.post('/forecast/recalculate', {})
          .then(() => {
            if (window.showToast) {
              window.showToast('success', 'Kalkulasi Selesai', 'Model WMA & Regresi OLS berhasil diperbarui.');
            }
          })
          .catch((err) => {
            console.warn('[Forecast] Recalculate notice:', err.message);
          });
      }

      setTimeout(() => {
        spinIcon.classList.remove('rotate-[720deg]');
        btnRecalculate.classList.remove('opacity-90');
      }, 1000);
    });
  }

  // Search input filter
  const searchInput = document.getElementById('tableSearch');
  if (searchInput) {
    searchInput.addEventListener('input', function(e) {
      const term = e.target.value.toLowerCase();
      const rows = document.querySelectorAll('#forecastTable tbody tr');
      rows.forEach(row => {
        const text = row.innerText.toLowerCase();
        row.style.display = text.includes(term) ? '' : 'none';
      });
    });
  }
});

// Status filtering in table attached to window
window.filterStatus = function(status) {
  const rows = document.querySelectorAll('#forecastTable tbody tr');
  rows.forEach(row => {
    const rowStatus = row.getAttribute('data-status');
    if (status === 'ALL') {
      row.style.display = '';
    } else if (status === 'CRITICAL' && rowStatus === 'CRITICAL') {
      row.style.display = '';
    } else if (status === 'OVERSTOCK' && rowStatus === 'OVERSTOCK') {
      row.style.display = '';
    } else if (status === 'SAFE' && rowStatus === 'SAFE') {
      row.style.display = '';
    } else {
      row.style.display = 'none';
    }
  });
};
