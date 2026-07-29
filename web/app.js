const API_BASE = '';

let state = {
  token: localStorage.getItem('yt_session_token') || '',
  currentUser: null,
  channels: [],
  selectedChannelId: null,
  videos: [],
  selectedType: 'all',
  page: 1,
  limit: 24,
  totalPages: 1,
  totalItems: 0,
  growthChart: null,
  hourlyChart: null
};

// Helper for Authenticated Fetch
async function fetchWithAuth(url, options = {}) {
  options.headers = options.headers || {};
  if (state.token) {
    options.headers['Authorization'] = `Bearer ${state.token}`;
  }
  const res = await fetch(url, options);
  if (res.status === 401 && !url.includes('/api/auth/login')) {
    // Session expired or invalid
    state.token = '';
    state.currentUser = null;
    localStorage.removeItem('yt_session_token');
    showLoginModal();
    throw new Error('Session expired. Please login again.');
  }
  return res;
}

// DOM Elements
const elTabNavOverview = document.getElementById('tab-nav-overview');
const elTabNavCompare = document.getElementById('tab-nav-compare');
const elOverviewSection = document.getElementById('section-overview');
const elCompareSection = document.getElementById('competitor-compare-section');

const elBtnSetPrimary = document.getElementById('btn-set-primary-channel');
const elBtnExportCSV = document.getElementById('btn-export-csv');
const elCompareMatrixContainer = document.getElementById('compare-matrix-container');

// Mobile Drawer
const elBtnHamburger = document.getElementById('btn-hamburger');
const elMobileDrawer = document.getElementById('mobile-drawer');
const elBtnLogoutMobile = document.getElementById('btn-logout-mobile');
const elBtnTriggerCollectMobile = document.getElementById('btn-trigger-collect-mobile');
const elBtnOpenAddModalMobile = document.getElementById('btn-open-add-modal-mobile');
const elMobileTabUsers = document.getElementById('mobile-tab-users');

const elTabNavUsers = document.getElementById('tab-nav-users');
const elAdminUsersSection = document.getElementById('admin-users-section');
const elUserInfoChip = document.getElementById('user-info-chip');
const elUserDisplayName = document.getElementById('user-display-name');
const elBtnLogout = document.getElementById('btn-logout');

const elModalLogin = document.getElementById('modal-login');
const elFormLogin = document.getElementById('form-login');
const elLoginUsername = document.getElementById('login-username');
const elLoginPassword = document.getElementById('login-password');
const elLoginErrorMsg = document.getElementById('login-error-msg');

const elModalAddUser = document.getElementById('modal-add-user');
const elFormAddUser = document.getElementById('form-add-user');
const elBtnOpenCreateUserModal = document.getElementById('btn-open-create-user-modal');
const elBtnCloseUserModal = document.getElementById('btn-close-user-modal');
const elBtnCancelUserModal = document.getElementById('btn-cancel-user-modal');
const elNewUserUsername = document.getElementById('new-user-username');
const elNewUserPassword = document.getElementById('new-user-password');
const elNewUserRole = document.getElementById('new-user-role');
const elUsersTableBody = document.getElementById('users-table-body');

const elStatsChannels = document.getElementById('stat-channels');
const elStatsVideos = document.getElementById('stat-videos');
const elStatsViews = document.getElementById('stat-views');
const elStatsSnapshots = document.getElementById('stat-snapshots');
const elActiveKeysText = document.getElementById('active-keys-text');

const elChannelList = document.getElementById('channel-list');
const elVideoContainer = document.getElementById('video-container');
const elBanner = document.getElementById('active-channel-banner');
const elBannerThumb = document.getElementById('banner-channel-thumb');
const elBannerTitle = document.getElementById('banner-channel-title');
const elBannerHandle = document.getElementById('banner-channel-handle');
const elBtnDeleteChannel = document.getElementById('btn-delete-channel');

const elBtnToggleHourlyChart = document.getElementById('btn-toggle-hourly-chart');
const elHourlyChartPanel = document.getElementById('hourly-chart-panel');
const elPeakHourBadge = document.getElementById('peak-hour-badge');

const elVideoSearch = document.getElementById('video-search-input');
const elVideoSort = document.getElementById('video-sort-select');

const elPaginationBar = document.getElementById('pagination-bar');
const elPageSizeSelect = document.getElementById('page-size-select');
const elBtnPrevPage = document.getElementById('btn-prev-page');
const elBtnNextPage = document.getElementById('btn-next-page');
const elPageInfoText = document.getElementById('page-info-text');

const elModalAddChannel = document.getElementById('modal-add-channel');
const elBtnOpenAddModal = document.getElementById('btn-open-add-modal');
const elBtnCloseModal = document.getElementById('btn-close-modal');
const elBtnCancelModal = document.getElementById('btn-cancel-modal');
const elFormAddChannel = document.getElementById('form-add-channel');
const elInputChannelId = document.getElementById('input-channel-id');

const elBtnTriggerCollect = document.getElementById('btn-trigger-collect');

const elModalChart = document.getElementById('modal-video-chart');
const elBtnCloseChartModal = document.getElementById('btn-close-chart-modal');
const elChartVideoTitle = document.getElementById('chart-video-title');
const elChartVideoPub = document.getElementById('chart-video-pub');
const elChartStatViews = document.getElementById('chart-stat-views');
const elChartStatLikes = document.getElementById('chart-stat-likes');
const elChartStatComments = document.getElementById('chart-stat-comments');
const elChartStatDuration = document.getElementById('chart-stat-duration');

// Helper: Switch visible tab section
function switchTab(tab) {
  // Desktop nav tabs
  elTabNavOverview.classList.toggle('active', tab === 'overview');
  elTabNavCompare.classList.toggle('active', tab === 'compare');
  elTabNavUsers.classList.toggle('active', tab === 'users');

  // Sections
  elOverviewSection.classList.toggle('hidden', tab !== 'overview');
  elCompareSection.classList.toggle('hidden', tab !== 'compare');
  elAdminUsersSection.classList.toggle('hidden', tab !== 'users');

  // Mobile nav tabs
  document.querySelectorAll('.mobile-nav-btn').forEach(btn => {
    btn.classList.toggle('active', btn.getAttribute('data-tab') === tab);
  });

  // Close mobile drawer
  elMobileDrawer.classList.add('hidden');

  if (tab === 'compare') fetchCompetitorComparison();
  if (tab === 'users') fetchAdminUsersList();
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  checkAuthStatus();

  // Hamburger Toggle
  elBtnHamburger.addEventListener('click', () => {
    elMobileDrawer.classList.toggle('hidden');
  });

  // Mobile Drawer: nav tab buttons
  document.querySelectorAll('.mobile-nav-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const tab = btn.getAttribute('data-tab');
      switchTab(tab);
    });
  });

  // Desktop Nav Tabs
  elTabNavOverview.addEventListener('click', () => switchTab('overview'));
  elTabNavCompare.addEventListener('click', () => switchTab('compare'));
  elTabNavUsers.addEventListener('click', () => switchTab('users'));

  // Category Tab Click Handlers
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      e.target.classList.add('active');
      state.selectedType = e.target.getAttribute('data-type') || 'all';
      state.page = 1;
      fetchVideosForSelectedChannel();
    });
  });

  // Pagination Handlers
  elPageSizeSelect.addEventListener('change', (e) => {
    state.limit = parseInt(e.target.value, 10) || 24;
    state.page = 1;
    fetchVideosForSelectedChannel();
  });

  elBtnPrevPage.addEventListener('click', () => {
    if (state.page > 1) { state.page--; fetchVideosForSelectedChannel(); }
  });

  elBtnNextPage.addEventListener('click', () => {
    if (state.page < state.totalPages) { state.page++; fetchVideosForSelectedChannel(); }
  });

  // Toggle Hourly Chart Panel
  elBtnToggleHourlyChart.addEventListener('click', () => {
    const isHidden = elHourlyChartPanel.classList.toggle('hidden');
    if (!isHidden && state.selectedChannelId) fetchHourlyStatsForSelectedChannel();
  });

  // Set Primary Channel
  elBtnSetPrimary.addEventListener('click', handleSetPrimaryChannel);

  // Export CSV
  elBtnExportCSV.addEventListener('click', () => {
    if (state.selectedChannelId) window.open(`${API_BASE}/api/export/channel/${state.selectedChannelId}`, '_blank');
  });

  // Add Channel Modal
  elBtnOpenAddModal.addEventListener('click', () => elModalAddChannel.classList.remove('hidden'));
  if (elBtnOpenAddModalMobile) elBtnOpenAddModalMobile.addEventListener('click', () => {
    elMobileDrawer.classList.add('hidden');
    elModalAddChannel.classList.remove('hidden');
  });
  elBtnCloseModal.addEventListener('click', () => elModalAddChannel.classList.add('hidden'));
  elBtnCancelModal.addEventListener('click', () => elModalAddChannel.classList.add('hidden'));
  elFormAddChannel.addEventListener('submit', handleAddChannel);

  // Trigger Collect
  elBtnTriggerCollect.addEventListener('click', handleTriggerCollect);
  if (elBtnTriggerCollectMobile) elBtnTriggerCollectMobile.addEventListener('click', () => {
    elMobileDrawer.classList.add('hidden');
    handleTriggerCollect();
  });

  elBtnDeleteChannel.addEventListener('click', handleDeleteChannel);
  elBtnCloseChartModal.addEventListener('click', () => elModalChart.classList.add('hidden'));

  // Login Form
  elFormLogin.addEventListener('submit', handleLoginSubmit);
  elBtnLogout.addEventListener('click', handleLogout);
  if (elBtnLogoutMobile) elBtnLogoutMobile.addEventListener('click', () => {
    elMobileDrawer.classList.add('hidden');
    handleLogout();
  });

  // Admin Create User Modal
  elBtnOpenCreateUserModal.addEventListener('click', () => elModalAddUser.classList.remove('hidden'));
  elBtnCloseUserModal.addEventListener('click', () => elModalAddUser.classList.add('hidden'));
  elBtnCancelUserModal.addEventListener('click', () => elModalAddUser.classList.add('hidden'));
  elFormAddUser.addEventListener('submit', handleCreateUserSubmit);

  elVideoSearch.addEventListener('input', debounce(() => {
    state.page = 1;
    fetchVideosForSelectedChannel();
  }, 300));

  elVideoSort.addEventListener('change', () => {
    state.page = 1;
    fetchVideosForSelectedChannel();
  });

  // Close drawer when clicking outside
  document.addEventListener('click', (e) => {
    if (!elMobileDrawer.classList.contains('hidden') &&
        !elMobileDrawer.contains(e.target) &&
        !elBtnHamburger.contains(e.target)) {
      elMobileDrawer.classList.add('hidden');
    }
  });
});

// Check Auth Status on Start
async function checkAuthStatus() {
  const storedToken = localStorage.getItem('yt_session_token');
  if (!storedToken) {
    showLoginModal();
    return;
  }
  state.token = storedToken;

  try {
    const res = await fetchWithAuth(`${API_BASE}/api/auth/me`);
    if (!res.ok) throw new Error('Unauthorized');
    const user = await res.json();

    state.currentUser = user;

    // Update user chip in navbar
    elUserDisplayName.textContent = `${user.username}`;
    elUserInfoChip.classList.remove('hidden');
    elModalLogin.classList.add('hidden');

    // Show/hide admin tab
    const isAdminOrSuper = user.role === 'admin' || user.role === 'superadmin';
    elTabNavUsers.classList.toggle('hidden', !isAdminOrSuper);
    if (elMobileTabUsers) elMobileTabUsers.classList.toggle('hidden', !isAdminOrSuper);

    // Show logout in mobile drawer
    if (elBtnLogoutMobile) elBtnLogoutMobile.classList.remove('hidden');

    fetchDashboardSummary();
    fetchChannels();
  } catch (err) {
    console.warn('Auth check failed:', err);
    showLoginModal();
  }
}

function showLoginModal() {
  elModalLogin.classList.remove('hidden');
  elUserInfoChip.classList.add('hidden');
  elTabNavUsers.classList.add('hidden');
  if (elMobileTabUsers) elMobileTabUsers.classList.add('hidden');
  if (elBtnLogoutMobile) elBtnLogoutMobile.classList.add('hidden');
}

// Login Submit Handler
async function handleLoginSubmit(e) {
  e.preventDefault();
  const username = elLoginUsername.value.trim();
  const password = elLoginPassword.value;
  if (!username || !password) return;

  elLoginErrorMsg.classList.add('hidden');

  try {
    const res = await fetch(`${API_BASE}/api/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });

    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Tên đăng nhập hoặc mật khẩu không chính xác');
    }

    const data = await res.json();
    state.token = data.token;
    state.currentUser = data.user;
    localStorage.setItem('yt_session_token', data.token);

    elLoginUsername.value = '';
    elLoginPassword.value = '';
    elModalLogin.classList.add('hidden');

    await checkAuthStatus();
  } catch (err) {
    elLoginErrorMsg.textContent = err.message;
    elLoginErrorMsg.classList.remove('hidden');
  }
}

// Logout Handler
async function handleLogout() {
  try {
    await fetchWithAuth(`${API_BASE}/api/auth/logout`, { method: 'POST' });
  } catch (err) {}

  state.token = '';
  state.currentUser = null;
  localStorage.removeItem('yt_session_token');
  showLoginModal();
}

// Admin Users Management
async function fetchAdminUsersList() {
  try {
    const res = await fetchWithAuth(`${API_BASE}/api/users`);
    if (!res.ok) throw new Error('Failed to fetch users');
    const users = await res.json();
    renderUsersTable(users || []);
  } catch (err) {
    elUsersTableBody.innerHTML = `<tr><td colspan="5" class="text-center">Lỗi: ${err.message}</td></tr>`;
  }
}

function renderUsersTable(users) {
  if (!users || users.length === 0) {
    elUsersTableBody.innerHTML = `<tr><td colspan="5" class="text-center">Chưa có người dùng nào.</td></tr>`;
    return;
  }

  elUsersTableBody.innerHTML = users.map(u => `
    <tr>
      <td>${u.id}</td>
      <td><strong>${escapeHtml(u.username)}</strong></td>
      <td>${u.role === 'admin' ? '<span class="badge-gold">ADMIN</span>' : '<span class="badge-comp">USER</span>'}</td>
      <td>${new Date(u.created_at).toLocaleString('vi-VN')}</td>
      <td>
        ${u.id !== state.currentUser.id ? `<button onclick="deleteUser(${u.id})" class="btn-danger-sm">Xóa</button>` : '<span class="sub-text">(Đang online)</span>'}
      </td>
    </tr>
  `).join('');
}

async function handleCreateUserSubmit(e) {
  e.preventDefault();
  const username = elNewUserUsername.value.trim();
  const password = elNewUserPassword.value.trim();
  const role = elNewUserRole.value;

  if (!username || !password) return;

  try {
    const res = await fetchWithAuth(`${API_BASE}/api/users`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password, role })
    });

    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Failed creating user');
    }

    elModalAddUser.classList.add('hidden');
    elNewUserUsername.value = '';
    elNewUserPassword.value = '';
    alert('Đã tạo tài khoản người dùng thành công!');
    fetchAdminUsersList();
  } catch (err) {
    alert('Lỗi: ' + err.message);
  }
}

async function deleteUser(id) {
  if (!confirm('Bạn có chắc chắn muốn xóa tài khoản người dùng này?')) return;

  try {
    const res = await fetchWithAuth(`${API_BASE}/api/users/${id}`, { method: 'DELETE' });
    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Failed deleting user');
    }
    fetchAdminUsersList();
  } catch (err) {
    alert('Lỗi khi xóa tài khoản: ' + err.message);
  }
}

// Fetch Dashboard Summary
async function fetchDashboardSummary() {
  try {
    const [resStats, resHealth] = await Promise.all([
      fetchWithAuth(`${API_BASE}/api/stats/summary`),
      fetchWithAuth(`${API_BASE}/api/health`)
    ]);

    if (resStats.ok) {
      const stats = await resStats.json();
      elStatsChannels.textContent = formatNumber(stats.total_channels || 0);
      elStatsVideos.textContent = formatNumber(stats.total_videos || 0);
      elStatsViews.textContent = formatNumber(stats.total_views || 0);
      elStatsSnapshots.textContent = formatNumber(stats.total_snapshots || 0);
    }

    if (resHealth.ok) {
      const health = await resHealth.json();
      elActiveKeysText.textContent = `${health.active_api_keys || 0} API Keys Active`;
    }
  } catch (err) {
    console.error('Failed fetching summary stats:', err);
  }
}

// Fetch Channels
async function fetchChannels() {
  try {
    const res = await fetch(`${API_BASE}/api/channels`);
    if (!res.ok) throw new Error('Failed loading channels');
    const channels = await res.json();
    state.channels = channels || [];

    renderChannelList();

    if (state.channels.length > 0 && !state.selectedChannelId) {
      selectChannel(state.channels[0].channel_id);
    }
  } catch (err) {
    elChannelList.innerHTML = `<div class="empty-state">Chưa có kênh nào. Hãy bấm "Thêm Kênh YouTube"!</div>`;
  }
}

// Select Channel
function selectChannel(channelId) {
  state.selectedChannelId = channelId;
  renderChannelList();

  const channel = state.channels.find(c => c.channel_id === channelId);
  if (channel) {
    elBanner.classList.remove('hidden');
    elBannerThumb.src = channel.thumbnail_url;
    elBannerTitle.textContent = channel.title;
    elBannerHandle.textContent = channel.handle || channel.channel_id;
  } else {
    elBanner.classList.add('hidden');
  }

  if (!elHourlyChartPanel.classList.contains('hidden')) {
    fetchHourlyStatsForSelectedChannel();
  }

  fetchVideosForSelectedChannel();
}
// Render Channel Sidebar
function renderChannelList() {
  if (!state.channels || state.channels.length === 0) {
    elChannelList.innerHTML = `<div class="empty-state">Chưa theo dõi kênh nào.</div>`;
    return;
  }

  elChannelList.innerHTML = state.channels.map(ch => `
    <div class="channel-item ${ch.channel_id === state.selectedChannelId ? 'active' : ''}" onclick="selectChannel('${ch.channel_id}')">
      <img src="${ch.thumbnail_url || 'https://via.placeholder.com/42'}" alt="" class="channel-avatar">
      <div class="channel-meta">
        <div class="channel-title">
          ${escapeHtml(ch.title)}
        </div>
        <div class="channel-handle">
          ${ch.is_my_channel ? '<span class="badge-gold">👑 Kênh Của Tôi</span>' : '<span class="badge-comp">⚔️ Đối Thủ</span>'}
          ${escapeHtml(ch.handle || '')}
        </div>
      </div>
    </div>
  `).join('');
}

// Fetch & Render Hourly Peak Viewing Stats
async function fetchHourlyStatsForSelectedChannel() {
  if (!state.selectedChannelId) return;

  try {
    const res = await fetch(`${API_BASE}/api/channels/${state.selectedChannelId}/hourly-stats`);
    if (!res.ok) return;
    const stats = await res.json();
    renderHourlyPeakChart(stats || []);
  } catch (err) {
    console.error('Failed fetching hourly peak stats:', err);
  }
}

function renderHourlyPeakChart(stats) {
  const ctx = document.getElementById('hourlyChart').getContext('2d');

  if (state.hourlyChart) {
    state.hourlyChart.destroy();
  }

  // Find Peak Hour
  let peakStat = null;
  let maxViews = -1;
  stats.forEach(s => {
    if (s.total_view_delta > maxViews) {
      maxViews = s.total_view_delta;
      peakStat = s;
    }
  });

  if (peakStat && maxViews > 0) {
    elPeakHourBadge.textContent = `🏆 Khung Giờ Vàng: ${peakStat.hour_formatted} (+${formatNumber(peakStat.total_view_delta)} views)`;
  } else {
    elPeakHourBadge.textContent = `🏆 Khung Giờ Vàng: Cần thêm dữ liệu snapshot`;
  }

  const labels = stats.map(s => `${s.hour.toString().padStart(2, '0')}:00`);
  const data = stats.map(s => s.total_view_delta);

  // Background Colors: Highlight Peak Hours in bright amber/gold
  const backgroundColors = data.map(val => (val === maxViews && maxViews > 0) ? '#f59e0b' : 'rgba(99, 102, 241, 0.4)');
  const borderColors = data.map(val => (val === maxViews && maxViews > 0) ? '#fbbf24' : '#6366f1');

  state.hourlyChart = new Chart(ctx, {
    type: 'bar',
    data: {
      labels: labels,
      datasets: [{
        label: 'Lượt Views Tăng Trưởng theo Khung Giờ',
        data: data,
        backgroundColor: backgroundColors,
        borderColor: borderColors,
        borderWidth: 1,
        borderRadius: 6
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        x: { ticks: { color: '#94a3b8' }, grid: { display: false } },
        y: { ticks: { color: '#94a3b8' }, grid: { color: 'rgba(255,255,255,0.05)' } }
      },
      plugins: {
        legend: { labels: { color: '#f8fafc', font: { family: 'Plus Jakarta Sans' } } }
      }
    }
  });
}

// Fetch Videos with Pagination & Category Filter
async function fetchVideosForSelectedChannel() {
  if (!state.selectedChannelId) return;

  const query = elVideoSearch.value.trim();
  const sort = elVideoSort.value;
  const type = state.selectedType;
  const page = state.page;
  const limit = state.limit;

  try {
    const res = await fetch(`${API_BASE}/api/channels/${state.selectedChannelId}/videos?q=${encodeURIComponent(query)}&sort=${encodeURIComponent(sort)}&type=${encodeURIComponent(type)}&page=${page}&limit=${limit}`);
    if (!res.ok) throw new Error('Failed to fetch videos');
    const data = await res.json();
    
    state.videos = data.items || [];
    state.totalItems = data.total || 0;
    state.page = data.page || 1;
    state.limit = data.limit || 24;
    state.totalPages = data.total_pages || 1;

    renderVideoGrid();
    renderPaginationBar();
  } catch (err) {
    elVideoContainer.innerHTML = `<div class="empty-state">Lỗi khi tải danh sách video: ${err.message}</div>`;
    elPaginationBar.classList.add('hidden');
  }
}

// Render Pagination Controls
function renderPaginationBar() {
  if (state.totalItems === 0) {
    elPaginationBar.classList.add('hidden');
    return;
  }

  elPaginationBar.classList.remove('hidden');
  elPageInfoText.textContent = `Trang ${state.page} / ${state.totalPages} (Tổng ${formatNumber(state.totalItems)} video)`;

  elBtnPrevPage.disabled = state.page <= 1;
  elBtnNextPage.disabled = state.page >= state.totalPages;
}

// Render Video Grid
function renderVideoGrid() {
  if (!state.videos || state.videos.length === 0) {
    elVideoContainer.innerHTML = `<div class="empty-state">Không tìm thấy video nào.</div>`;
    return;
  }

  elVideoContainer.innerHTML = state.videos.map(v => `
    <div class="video-card" onclick="openVideoChart('${v.video_id}')">
      <div class="video-thumb-wrapper">
        <img src="${v.thumbnail_url}" alt="${escapeHtml(v.title)}" loading="lazy">
        <span class="duration-badge">${formatDuration(v.duration_seconds)}</span>
      </div>
      <div class="video-card-body">
        <h4 class="video-card-title">${escapeHtml(v.title)}</h4>
        <div class="video-card-stats">
          <span class="stat-item">👁️ ${formatNumber(v.latest_views)}</span>
          <span class="stat-item">👍 ${formatNumber(v.latest_likes)}</span>
          <span class="stat-item">💬 ${formatNumber(v.latest_comments)}</span>
        </div>
      </div>
    </div>
  `).join('');
}

// Open Video Chart Modal & Render Growth Chart
async function openVideoChart(videoId) {
  const video = state.videos.find(v => v.video_id === videoId);
  if (!video) return;

  elChartVideoTitle.textContent = video.title;
  elChartVideoPub.textContent = `Đăng ngày: ${new Date(video.published_at).toLocaleDateString('vi-VN')}`;
  elChartStatViews.textContent = formatNumber(video.latest_views);
  elChartStatLikes.textContent = formatNumber(video.latest_likes);
  elChartStatComments.textContent = formatNumber(video.latest_comments);
  elChartStatDuration.textContent = formatDuration(video.duration_seconds);

  elModalChart.classList.remove('hidden');

  try {
    const res = await fetch(`${API_BASE}/api/videos/${videoId}/history?limit=100`);
    if (!res.ok) return;
    const history = await res.json();
    renderGrowthChart(history || []);
  } catch (err) {
    console.error('Failed fetching video history:', err);
  }
}

// Render Chart.js
function renderGrowthChart(history) {
  const ctx = document.getElementById('growthChart').getContext('2d');

  if (state.growthChart) {
    state.growthChart.destroy();
  }

  const labels = history.map(h => new Date(h.collected_at).toLocaleString('vi-VN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }));
  const viewsData = history.map(h => h.view_count);
  const likesData = history.map(h => h.like_count);

  state.growthChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels: labels,
      datasets: [
        {
          label: 'Lượt Views',
          data: viewsData,
          borderColor: '#3b82f6',
          backgroundColor: 'rgba(59, 130, 246, 0.1)',
          fill: true,
          tension: 0.3,
          yAxisID: 'y'
        },
        {
          label: 'Lượt Likes',
          data: likesData,
          borderColor: '#ff3b30',
          backgroundColor: 'transparent',
          tension: 0.3,
          yAxisID: 'y1'
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      scales: {
        x: { ticks: { color: '#94a3b8' }, grid: { color: 'rgba(255,255,255,0.05)' } },
        y: {
          type: 'linear',
          display: true,
          position: 'left',
          ticks: { color: '#3b82f6' },
          grid: { color: 'rgba(255,255,255,0.05)' }
        },
        y1: {
          type: 'linear',
          display: true,
          position: 'right',
          ticks: { color: '#ff3b30' },
          grid: { drawOnChartArea: false }
        }
      },
      plugins: {
        legend: { labels: { color: '#f8fafc', font: { family: 'Plus Jakarta Sans' } } }
      }
    }
  });
}

// Add Channel Form Submit
async function handleAddChannel(e) {
  e.preventDefault();
  const inputVal = elInputChannelId.value.trim();
  if (!inputVal) return;

  const btnSubmit = elFormAddChannel.querySelector('button[type="submit"]');
  btnSubmit.disabled = true;
  btnSubmit.textContent = 'Đang Quét & Thêm...';

  try {
    const res = await fetch(`${API_BASE}/api/channels`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ handle_or_id: inputVal })
    });

    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Failed adding channel');
    }

    elModalAddChannel.classList.add('hidden');
    elInputChannelId.value = '';

    await fetchChannels();
    await fetchDashboardSummary();
  } catch (err) {
    alert('Lỗi: ' + err.message);
  } finally {
    btnSubmit.disabled = false;
    btnSubmit.textContent = 'Thêm & Quét Ngay';
  }
}

// Trigger Collector
async function handleTriggerCollect() {
  elBtnTriggerCollect.disabled = true;
  elBtnTriggerCollect.innerHTML = 'Đang Ghi Snapshot...';

  try {
    const res = await fetch(`${API_BASE}/api/collector/trigger`, { method: 'POST' });
    if (res.ok) {
      alert('Đã kích hoạt thu thập dữ liệu ngầm!');
      setTimeout(() => {
        fetchDashboardSummary();
        fetchVideosForSelectedChannel();
      }, 3000);
    }
  } catch (err) {
    alert('Lỗi: ' + err.message);
  } finally {
    elBtnTriggerCollect.disabled = false;
    elBtnTriggerCollect.innerHTML = `
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/></svg>
      Ghi Snapshot Ngay
    `;
  }
}

// Delete Channel
async function handleDeleteChannel() {
  if (!state.selectedChannelId) return;
  if (!confirm('Bạn có chắc chắn muốn xóa kênh này và toàn bộ dữ liệu lịch sử snapshot liên quan?')) return;

  try {
    const res = await fetch(`${API_BASE}/api/channels/${state.selectedChannelId}`, { method: 'DELETE' });
    if (res.ok) {
      state.selectedChannelId = null;
      await fetchChannels();
      await fetchDashboardSummary();
    }
  } catch (err) {
    alert('Lỗi khi xóa kênh: ' + err.message);
  }
}

// Helper Utilities
function formatNumber(num) {
  if (num === null || num === undefined) return '0';
  return new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 1 }).format(num);
}

function formatDuration(seconds) {
  if (!seconds) return '00:00';
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/[&<>"']/g, m => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;'
  })[m]);
}

// Handle Set Primary Channel
async function handleSetPrimaryChannel() {
  if (!state.selectedChannelId) return;

  try {
    const res = await fetch(`${API_BASE}/api/channels/${state.selectedChannelId}/set-primary`, { method: 'PUT' });
    if (!res.ok) throw new Error('Failed to set primary channel');
    
    alert('Đã đặt kênh thành công thành Kênh Của Tôi (👑)!');
    await fetchChannels();
  } catch (err) {
    alert('Lỗi: ' + err.message);
  }
}

// Fetch & Render Competitor Comparison Matrix
async function fetchCompetitorComparison() {
  const myChannel = state.channels.find(c => c.is_my_channel) || state.channels[0];
  const competitors = state.channels.filter(c => c.channel_id !== (myChannel ? myChannel.channel_id : ''));

  if (!myChannel && competitors.length === 0) {
    elCompareMatrixContainer.innerHTML = `<div class="empty-state">Vui lòng thêm kênh để so sánh.</div>`;
    return;
  }

  const myChanID = myChannel ? myChannel.channel_id : '';
  const compIDs = competitors.map(c => c.channel_id).join(',');

  try {
    const res = await fetch(`${API_BASE}/api/analytics/compare?my_channel=${myChanID}&competitors=${compIDs}`);
    if (!res.ok) throw new Error('Failed to fetch comparison stats');
    const data = await res.json();
    renderCompetitorMatrix(data);
  } catch (err) {
    elCompareMatrixContainer.innerHTML = `<div class="empty-state">Lỗi khi tải dữ liệu so sánh đối thủ: ${err.message}</div>`;
  }
}

function renderCompetitorMatrix(data) {
  let cardsHTML = '';

  if (data.my_channel) {
    cardsHTML += createCompetitorCard(data.my_channel, true);
  }

  if (data.competitors && data.competitors.length > 0) {
    cardsHTML += data.competitors.map(c => createCompetitorCard(c, false)).join('');
  }

  if (!cardsHTML) {
    elCompareMatrixContainer.innerHTML = `<div class="empty-state">Chưa có đủ kênh để hiển thị bảng so sánh.</div>`;
    return;
  }

  elCompareMatrixContainer.innerHTML = cardsHTML;
}

function createCompetitorCard(metric, isMyChannel) {
  return `
    <div class="compare-card ${isMyChannel ? 'my-card' : ''}">
      <div class="compare-card-header">
        <img src="${metric.thumbnail_url || 'https://via.placeholder.com/50'}" alt="" class="compare-card-avatar">
        <div>
          <h4>${escapeHtml(metric.title)}</h4>
          <div>${isMyChannel ? '<span class="badge-gold">👑 Kênh Của Tôi</span>' : '<span class="badge-comp">⚔️ Đối Thủ</span>'}</div>
        </div>
      </div>
      <div class="compare-card-body">
        <div class="compare-metric-row">
          <span class="compare-metric-label">Người Đăng Ký (Subscribers)</span>
          <span class="compare-metric-value">${formatNumber(metric.subscriber_count)}</span>
        </div>
        <div class="compare-metric-row">
          <span class="compare-metric-label">Tổng Lượt Views Kênh</span>
          <span class="compare-metric-value">${formatNumber(metric.total_view_count)}</span>
        </div>
        <div class="compare-metric-row">
          <span class="compare-metric-label">Tổng Số Video</span>
          <span class="compare-metric-value">${formatNumber(metric.video_count)}</span>
        </div>
        <div class="compare-metric-row">
          <span class="compare-metric-label">Trung Bình Views / Video</span>
          <span class="compare-metric-value">${formatNumber(metric.avg_views_per_video)}</span>
        </div>
        <div class="compare-metric-row">
          <span class="compare-metric-label">Tỷ Lệ Tương Tác (Engagement)</span>
          <span class="compare-metric-value">${metric.engagement_rate.toFixed(2)}%</span>
        </div>
        <div class="compare-metric-row">
          <span class="compare-metric-label">Tần Suất Đăng Bài (30 Ngày)</span>
          <span class="compare-metric-value">${metric.posting_frequency.toFixed(1)} video/tuần</span>
        </div>
      </div>
    </div>
  `;
}
