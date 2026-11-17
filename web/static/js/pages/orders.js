/**
 * KOPMA Express - Orders / Kasir Mock Page Script
 */

(function() {
  const totalTagihan = 150100;
  const amountInput = document.getElementById('amount-paid');
  const kembalianDisplay = document.getElementById('kembalian-display');
  const btnPas = document.getElementById('pas-btn');
  const cashChips = document.querySelectorAll('.cash-chip');
  const btnProcess = document.getElementById('btn-process-order');
  const toast = document.getElementById('atomic-toast');

  function calculateChange() {
    if (!amountInput || !kembalianDisplay) return;
    const rawVal = amountInput.value.replace(/[^0-9]/g, '');
    const paid = parseInt(rawVal || '0', 10);
    const diff = paid - totalTagihan;
    if (diff >= 0) {
      kembalianDisplay.textContent = 'Rp ' + diff.toLocaleString('id-ID');
      kembalianDisplay.className = 'text-headline-md font-headline-md text-on-secondary-container';
    } else {
      kembalianDisplay.textContent = 'Kurang Rp ' + Math.abs(diff).toLocaleString('id-ID');
      kembalianDisplay.className = 'text-headline-md font-headline-md text-status-critical-text';
    }
  }

  if (amountInput) {
    amountInput.addEventListener('input', function() {
      calculateChange();
    });
  }

  if (btnPas) {
    btnPas.addEventListener('click', function() {
      if (!amountInput) return;
      amountInput.value = '150.100';
      calculateChange();
    });
  }

  cashChips.forEach(chip => {
    chip.addEventListener('click', function() {
      const val = this.textContent.replace('Rp ', '').trim();
      if (amountInput) {
        amountInput.value = val;
        calculateChange();
      }
    });
  });

  if (btnProcess && toast) {
    btnProcess.addEventListener('click', function() {
      const originalContent = btnProcess.innerHTML;
      btnProcess.innerHTML = '<span class="material-symbols-outlined animate-spin text-[24px]">progress_activity</span><span>Memvalidasi Mutex Database...</span>';
      btnProcess.disabled = true;

      setTimeout(() => {
        btnProcess.innerHTML = originalContent;
        btnProcess.disabled = false;
        
        toast.classList.remove('translate-y-32', 'opacity-0');
        toast.classList.add('translate-y-0', 'opacity-100');

        setTimeout(() => {
          toast.classList.remove('translate-y-0', 'opacity-100');
          toast.classList.add('translate-y-32', 'opacity-0');
        }, 3500);
      }, 650);
    });
  }
})();
