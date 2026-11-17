/**
 * KOPMA Express - Import Excel ETL Page Script
 * Endpoint: POST /api/v1/import/excel, GET /api/v1/import/jobs/:id
 */

(function initETLDashboard() {
  const dropzone = document.getElementById('dropzone');
  const fileInput = document.getElementById('file-input');
  const runBtn = document.getElementById('btn-run-etl');
  let selectedFile = null;

  // -------------------------------------------------------------
  // Drag & Drop and File Selection
  // -------------------------------------------------------------
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
        handleFileSelect(e.dataTransfer.files[0]);
      }
    });

    fileInput.addEventListener('change', () => {
      if (fileInput.files.length > 0) {
        handleFileSelect(fileInput.files[0]);
      }
    });

    const closeBtn = dropzone.querySelector('button[title="Hapus Berkas"]');
    if (closeBtn) {
      closeBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        clearFileSelect();
      });
    }
  }

  function handleFileSelect(file) {
    if (!file.name.toLowerCase().endsWith('.xlsx')) {
      alert('Hanya file spreadsheet .xlsx yang didukung.');
      return;
    }
    selectedFile = file;
    const badge = dropzone.querySelector('div.mt-space-md span.font-title-md');
    const sizeBadge = dropzone.querySelector('div.mt-space-md span.font-body-sm');
    if (badge) badge.textContent = file.name;
    if (sizeBadge) sizeBadge.textContent = (file.size / (1024 * 1024)).toFixed(2) + ' MB';
    if (window.showToast) {
      window.showToast('info', 'File Dipilih', file.name);
    }
  }

  function clearFileSelect() {
    selectedFile = null;
    if (fileInput) fileInput.value = '';
    const badge = dropzone.querySelector('div.mt-space-md span.font-title-md');
    const sizeBadge = dropzone.querySelector('div.mt-space-md span.font-body-sm');
    if (badge) badge.textContent = 'Pilih atau seret berkas .xlsx ke sini';
    if (sizeBadge) sizeBadge.textContent = '0 MB';
  }

  // -------------------------------------------------------------
  // Run ETL Pipeline
  // -------------------------------------------------------------
  if (runBtn) {
    runBtn.addEventListener('click', async () => {
      const fileToUpload = selectedFile || (fileInput && fileInput.files[0]);
      if (!fileToUpload) {
        alert('Silakan pilih berkas Excel (.xlsx) terlebih dahulu!');
        return;
      }

      // Ambil stock_mode yang dipilih
      const stockModeEl = document.querySelector('input[name="stock_mode"]:checked');
      const stockMode = stockModeEl ? stockModeEl.value : 'replace';

      const originalText = runBtn.innerHTML;
      runBtn.innerHTML = '<span class="material-symbols-outlined animate-spin text-[20px]">refresh</span><span>Mengunggah & Memproses...</span>';
      runBtn.disabled = true;

      const formData = new FormData();
      formData.append('file', fileToUpload);
      formData.append('stock_mode', stockMode);

      try {
        const resp = await fetch('/api/v1/import/excel', {
          method: 'POST',
          body: formData,
        });
        const res = await resp.json();

        if (!resp.ok || !res.success) {
          throw new Error(res.error?.message || 'Gagal memulai import');
        }

        const jobId = res.data.job_id;
        if (window.showToast) {
          window.showToast('info', 'Import Dimulai', `Pipeline ID: #${jobId}`);
        }
        pollJobStatus(jobId, originalText);
      } catch (err) {
        alert('Error Import: ' + err.message);
        if (window.showToast) {
          window.showToast('error', 'Gagal Memulai Import', err.message);
        }
        runBtn.innerHTML = originalText;
        runBtn.disabled = false;
      }
    });
  }

  // -------------------------------------------------------------
  // Real-Time Polling Job Status
  // -------------------------------------------------------------
  async function pollJobStatus(jobId, originalBtnHtml) {
    const interval = setInterval(async () => {
      try {
        const resp = await fetch('/api/v1/import/jobs/' + jobId);
        const res = await resp.json();
        if (!res.success) return;

        const job = res.data;
        updateUIProgress(job);

        if (job.status === 'DONE' || job.status === 'FAILED') {
          clearInterval(interval);
          runBtn.disabled = false;

          if (job.status === 'DONE') {
            runBtn.innerHTML = '<span class="material-symbols-outlined text-[20px]">done_all</span><span>Selesai!</span>';
            const summaryMsg = `Import Berhasil!\n` +
              `Total baris diproses: ${job.processed_rows || 0}\n` +
              `Barang baru (inserted): ${job.inserted || 0}\n` +
              `Barang diperbarui (updated): ${job.updated || 0}\n` +
              `Dilewati (skipped): ${job.skipped || 0}\n` +
              `Gagal (failed): ${job.failed || 0}`;

            if (window.showToast) {
              window.showToast('success', 'Import Berhasil', `${job.inserted || 0} barang baru masuk, ${job.updated || 0} diperbarui.`);
            }
            alert(summaryMsg);
          } else {
            runBtn.innerHTML = '<span class="material-symbols-outlined text-[20px]">error</span><span>Gagal</span>';
            const failReason = (job.errors && job.errors.length > 0)
              ? job.errors[0].reason
              : 'Gagal memproses berkas Excel.';

            if (window.showToast) {
              window.showToast('error', 'Import Gagal', failReason);
            }
            alert('Import Gagal:\n' + failReason);
          }

          setTimeout(() => {
            runBtn.innerHTML = originalBtnHtml;
          }, 3500);
        }
      } catch (e) {
        console.error('Polling error:', e);
      }
    }, 1000);
  }

  // -------------------------------------------------------------
  // Update UI Elements with Job Progress
  // -------------------------------------------------------------
  function updateUIProgress(job) {
    // Pipeline ID
    const pipelineIdEl = document.getElementById('pipeline-id-badge');
    if (pipelineIdEl) {
      pipelineIdEl.textContent = 'Pipeline ID: #' + (job.job_id || '-');
    }

    // Status Badge
    const statusBadge = document.getElementById('pipeline-status-badge');
    if (statusBadge) {
      if (job.status === 'DONE') {
        statusBadge.className = 'inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full bg-status-success-bg text-status-success-text font-label-sm text-label-sm font-semibold';
        statusBadge.innerHTML = '<span class="w-2 h-2 rounded-full bg-status-success-text"></span>Selesai';
      } else if (job.status === 'FAILED') {
        statusBadge.className = 'inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full bg-status-critical-bg text-status-critical-text font-label-sm text-label-sm font-semibold';
        statusBadge.innerHTML = '<span class="w-2 h-2 rounded-full bg-status-critical-text"></span>Gagal';
      } else {
        statusBadge.className = 'inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full bg-status-warning-bg text-status-warning-text font-label-sm text-label-sm font-semibold';
        statusBadge.innerHTML = '<span class="w-2 h-2 rounded-full bg-status-warning-text animate-ping"></span>Sedang Memproses Ingestion';
      }
    }

    // Processed metric
    const processedEl = document.getElementById('stat-processed');
    if (processedEl) {
      processedEl.textContent = (job.processed_rows || 0).toLocaleString('id-ID');
    }

    // Success (inserted + updated) metric
    const insertedEl = document.getElementById('stat-inserted');
    const successCount = (job.inserted || 0) + (job.updated || 0);
    if (insertedEl) {
      insertedEl.textContent = successCount.toLocaleString('id-ID');
    }

    const insertedPctEl = document.getElementById('stat-inserted-pct');
    if (insertedPctEl && job.processed_rows > 0) {
      const pct = ((successCount / job.processed_rows) * 100).toFixed(1);
      insertedPctEl.textContent = `(${pct}%)`;
    }

    // Failed metric
    const failedEl = document.getElementById('stat-failed');
    if (failedEl) {
      failedEl.textContent = (job.failed || 0).toLocaleString('id-ID');
    }

    const failedPctEl = document.getElementById('stat-failed-pct');
    if (failedPctEl && job.processed_rows > 0) {
      const pct = (((job.failed || 0) / job.processed_rows) * 100).toFixed(1);
      failedPctEl.textContent = `(${pct}%)`;
    }

    // Progress percentage
    let pct = 0;
    if (job.total_rows > 0) {
      pct = Math.min(100, Math.round(((job.processed_rows || 0) / job.total_rows) * 100));
    } else if (job.status === 'DONE') {
      pct = 100;
    } else if (job.status === 'FAILED') {
      pct = 100;
    } else {
      pct = 50;
    }

    const pctEl = document.getElementById('progress-pct-display');
    if (pctEl) pctEl.textContent = pct + '%';

    const barEl = document.getElementById('progress-bar-track');
    if (barEl) barEl.style.width = pct + '%';

    const phaseEl = document.getElementById('progress-phase-label');
    if (phaseEl) {
      if (job.status === 'DONE') {
        phaseEl.textContent = 'Proses Ingestion Database Selesai';
      } else if (job.status === 'FAILED') {
        phaseEl.textContent = 'Proses Ingestion Gagal';
      } else {
        phaseEl.textContent = 'Tahap Ingestion Database MongoDB';
      }
    }

    // Render error rows if any
    const errCount = job.errors ? job.errors.length : 0;
    const errCountBadge = document.getElementById('error-count-badge');
    if (errCountBadge) {
      errCountBadge.textContent = errCount + ' Ditemukan';
    }

    const errSummary = document.getElementById('error-summary-count');
    if (errSummary) {
      errSummary.textContent = errCount > 0 ? `Menampilkan ${errCount} entri error` : 'Tidak ada anomali';
    }

    const errTbody = document.getElementById('error-table-body');
    if (errTbody && job.errors && job.errors.length > 0) {
      errTbody.innerHTML = job.errors.map(err => `
        <tr class="hover:bg-surface-container-lowest transition-colors">
          <td class="py-3.5 px-space-md">
            <span class="inline-flex items-center px-2 py-0.5 rounded font-semibold bg-status-critical-bg text-status-critical-text">
              Baris ${err.row || '-'}
            </span>
          </td>
          <td class="py-3.5 px-space-md font-medium text-on-surface">${err.sheet || '-'}</td>
          <td class="py-3.5 px-space-md font-mono text-outline">-</td>
          <td class="py-3.5 px-space-md">
            <div class="flex items-center gap-2">
              <span class="material-symbols-outlined text-status-critical-text text-[16px]">warning</span>
              <span class="text-status-critical-text font-medium">${err.reason || '-'}</span>
            </div>
          </td>
          <td class="py-3.5 px-space-md text-right">
            <span class="text-xs text-outline">${err.sheet === 'SYSTEM' ? 'Error Fatal' : 'Dilewati'}</span>
          </td>
        </tr>
      `).join('');
    }
  }

  // -------------------------------------------------------------
  // Download Dummy Excel Handler
  // -------------------------------------------------------------
  const downloadDummyBtn = document.getElementById('btn-download-dummy');
  if (downloadDummyBtn) {
    downloadDummyBtn.addEventListener('click', (e) => {
      e.preventDefault();
      const items = document.getElementById('dummy-items')?.value || 300;
      const months = document.getElementById('dummy-months')?.value || 6;
      const dirty = document.getElementById('dummy-dirty')?.checked || true;
      window.location.href = `/api/v1/export/dummy-excel?items=${items}&months=${months}&dirty=${dirty}`;
    });
  }
})();
