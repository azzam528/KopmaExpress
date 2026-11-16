/**
 * KOPMA Express - Dashboard Page Script
 */

document.addEventListener('DOMContentLoaded', () => {
  // Period filter button selection
  const periodPill = document.querySelector('.bg-canvas-bg.rounded-xl.p-1.shadow-sm');
  if (periodPill) {
    const buttons = periodPill.querySelectorAll('button');
    buttons.forEach((btn) => {
      btn.addEventListener('click', () => {
        buttons.forEach((b) => {
          b.className = 'px-3 py-1.5 rounded-lg text-outline hover:text-on-surface font-label-lg text-label-lg transition-all';
        });
        btn.className = 'px-3 py-1.5 rounded-lg bg-surface-container-lowest text-primary font-title-md text-title-md shadow-sm transition-all';
        const period = btn.textContent.trim();
        console.log(`[Dashboard] Selected period: ${period}`);
      });
    });
  }

  // Report download button
  const downloadBtn = document.querySelector('button:has(span.material-symbols-outlined:contains("download"))');
  // Or query by text
  document.querySelectorAll('button').forEach((b) => {
    if (b.textContent.includes('Download report')) {
      b.addEventListener('click', () => {
        if (window.showToast) {
          window.showToast('info', 'Download Laporan', 'Menyiapkan berkas ekspor analitik...');
        }
      });
    }
  });

  // Check backend health
  if (window.apiClient) {
    window.apiClient.get('/health')
      .then((res) => {
        console.log('[Dashboard] Backend status:', res.data);
      })
      .catch((err) => {
        console.warn('[Dashboard] Health check notice:', err.message);
      });
  }
});
