/**
 * KOPMA Express - Main Shared JavaScript
 * Handles global navigation, keyboard shortcuts, and UI helpers.
 */

document.addEventListener('DOMContentLoaded', () => {
  // 1. Highlight active navigation link
  const currentPath = window.location.pathname;
  const navLinks = document.querySelectorAll('aside nav a');

  navLinks.forEach((link) => {
    const href = link.getAttribute('href');
    const isExactMatch = href === currentPath;
    const isRootMatch = href === '/' && (currentPath === '' || currentPath === '/' || currentPath === '/dashboard');
    const isSectionMatch = href !== '/' && currentPath.startsWith(href);

    if (isExactMatch || isRootMatch || isSectionMatch) {
      link.setAttribute('aria-current', 'page');
      link.className = 'flex items-center gap-space-sm px-space-md py-space-sm transition-all duration-200 bg-canvas-bg text-primary font-title-md shadow-sm rounded-xl';
    } else {
      link.removeAttribute('aria-current');
      link.className = 'flex items-center gap-space-sm px-space-md py-space-sm rounded-xl text-on-primary/80 hover:bg-sidebar-hover hover:text-on-primary transition-all duration-200';
    }
  });

  // 2. Global search focus shortcut (Ctrl+K or '/')
  const searchInput = document.querySelector('header input[type="search"]');
  if (searchInput) {
    document.addEventListener('keydown', (e) => {
      if ((e.key === '/' || (e.ctrlKey && e.key.toLowerCase() === 'k')) && document.activeElement !== searchInput) {
        if (!['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement.tagName)) {
          e.preventDefault();
          searchInput.focus();
        }
      }
    });
  }
});

// 3. Global Toast Helper
window.showToast = function(type, title, message) {
  let container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    document.body.appendChild(container);
  }

  const toast = document.createElement('div');
  const bgColors = {
    success: 'bg-emerald-600 text-white',
    error: 'bg-red-600 text-white',
    warning: 'bg-amber-500 text-white',
    info: 'bg-slate-800 text-white',
  };
  const iconNames = {
    success: 'check_circle',
    error: 'error',
    warning: 'warning',
    info: 'info',
  };

  const color = bgColors[type] || bgColors.info;
  const icon = iconNames[type] || 'info';

  toast.className = `toast-item flex items-center gap-3 px-4 py-3 rounded-xl shadow-lg ${color} text-sm font-medium transform translate-y-8 opacity-0`;
  toast.innerHTML = `
    <span class="material-symbols-outlined text-[20px]">${icon}</span>
    <div class="flex flex-col">
      <span class="font-bold">${title}</span>
      ${message ? `<span class="text-xs opacity-90">${message}</span>` : ''}
    </div>
  `;

  container.appendChild(toast);

  // Animate In
  requestAnimationFrame(() => {
    toast.classList.remove('translate-y-8', 'opacity-0');
    toast.classList.add('translate-y-0', 'opacity-100');
  });

  // Animate Out
  setTimeout(() => {
    toast.classList.remove('translate-y-0', 'opacity-100');
    toast.classList.add('translate-y-8', 'opacity-0');
    setTimeout(() => toast.remove(), 300);
  }, 4000);
};

// 4. Session & Auth Profile Helper
async function syncUserProfile() {
  try {
    const res = await fetch('/api/v1/auth/me', {
      headers: { 'Accept': 'application/json' }
    });
    if (!res.ok) {
      if (res.status === 401 && !window.location.pathname.startsWith('/login')) {
        window.location.href = '/login';
      }
      return;
    }
    const json = await res.json();
    if (json && json.success && json.data) {
      const user = json.data;
      const displayName = user.name || user.nama || user.nim || 'Pengguna';
      const roleText = user.role === 'admin' ? 'Administrator' : 'Anggota';
      const roleSub = user.nim ? `${roleText} • ${user.nim}` : roleText;

      const userNameEls = document.querySelectorAll('#userProfileName, header .flex-col span.font-title-md');
      const userRoleEls = document.querySelectorAll('#userProfileRole, header .flex-col span.font-label-sm');

      userNameEls.forEach(el => { el.textContent = displayName; });
      userRoleEls.forEach(el => { el.textContent = roleSub; });
    }
  } catch (e) {
    // ignore
  }
}

window.logoutUser = async function() {
  try {
    await fetch('/api/v1/auth/logout', { method: 'POST' });
  } catch (e) {
    // ignore
  } finally {
    window.location.href = '/login';
  }
};

document.addEventListener('DOMContentLoaded', () => {
  syncUserProfile();

  document.querySelectorAll('#logoutBtn, [data-action="logout"]').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      window.logoutUser();
    });
  });
});
