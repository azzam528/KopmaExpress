/**
 * KOPMA Express - Import Excel ETL Page Script
 */

(function initETLDashboard() {
  const dropzone = document.getElementById('dropzone');
  const fileInput = document.getElementById('file-input');
  const runBtn = document.getElementById('btn-run-etl');

  if (dropzone && fileInput) {
    ['dragenter', 'dragover'].forEach(eventName => {
      dropzone.addEventListener(eventName, (e) => {
        e.preventDefault();
        e.stopPropagation();
        dropzone.classList.add('bg-surface-container');
      });
    });

    ['dragleave', 'drop'].forEach(eventName => {
      dropzone.addEventListener(eventName, (e) => {
        e.preventDefault();
        e.stopPropagation();
        dropzone.classList.remove('bg-surface-container');
      });
    });

    dropzone.addEventListener('drop', (e) => {
      if (e.dataTransfer && e.dataTransfer.files.length > 0) {
        fileInput.files = e.dataTransfer.files;
        handleFileSelect(fileInput.files[0]);
      }
    });

    fileInput.addEventListener('change', () => {
      if (fileInput.files.length > 0) {
        handleFileSelect(fileInput.files[0]);
      }
    });

    function handleFileSelect(file) {
      const fileName = file.name;
      const fileSize = (file.size / (1024 * 1024)).toFixed(1) + ' MB';
      const badge = dropzone.querySelector('div.mt-space-md span.font-title-md');
      const sizeBadge = dropzone.querySelector('div.mt-space-md span.font-body-sm');
      if (badge) badge.textContent = fileName;
      if (sizeBadge) sizeBadge.textContent = fileSize;
    }
  }

  if (runBtn) {
    runBtn.addEventListener('click', () => {
      const originalText = runBtn.innerHTML;
      runBtn.innerHTML = '<span class="material-symbols-outlined animate-spin text-[20px]">refresh</span><span>Memproses Data...</span>';
      runBtn.disabled = true;
      setTimeout(() => {
        runBtn.innerHTML = '<span class="material-symbols-outlined text-[20px]">done_all</span><span>Sinkronisasi Selesai!</span>';
        if (window.showToast) {
          window.showToast('success', 'Import Berhasil', 'Data katalog dan riwayat penjualan selesai diproses.');
        }
        setTimeout(() => {
          runBtn.innerHTML = originalText;
          runBtn.disabled = false;
        }, 2500);
      }, 1800);
    });
  }
})();
