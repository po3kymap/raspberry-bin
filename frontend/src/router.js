import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn } from './auth.js'
import { isAllowed, startPage } from './roles.js'
import AdminActiveView from './views/admin/AdminActiveView.vue'
import AdminActivityView from './views/admin/AdminActivityView.vue'
import AdminBinsView from './views/admin/AdminBinsView.vue'
import AdminOverviewView from './views/admin/AdminOverviewView.vue'
import AdminRejectedView from './views/admin/AdminRejectedView.vue'
import AdminSettingsView from './views/admin/AdminSettingsView.vue'
import AdminSortingView from './views/admin/AdminSortingView.vue'
import AdminUsersView from './views/admin/AdminUsersView.vue'
import HistoryView from './views/HistoryView.vue'
import HomeView from './views/HomeView.vue'
import LoginView from './views/LoginView.vue'
import RegisterView from './views/RegisterView.vue'
import SettingsView from './views/SettingsView.vue'
import StatsView from './views/StatsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/stats', name: 'stats', component: StatsView },
    { path: '/history', name: 'history', component: HistoryView },
    { path: '/settings', name: 'settings', component: SettingsView },
    {
      path: '/admin',
      // Child routes inherit this meta, so every admin page is checked by the guard
      meta: { admin: true },
      redirect: { name: 'admin-overview' },
      children: [
        { path: 'overview', name: 'admin-overview', component: AdminOverviewView },
        { path: 'overview/sorting', name: 'admin-sorting', component: AdminSortingView },
        { path: 'overview/rejected', name: 'admin-rejected', component: AdminRejectedView },
        { path: 'overview/active', name: 'admin-active', component: AdminActiveView },
        { path: 'overview/activity', name: 'admin-activity', component: AdminActivityView },
        { path: 'bins', name: 'admin-bins', component: AdminBinsView },
        { path: 'users', name: 'admin-users', component: AdminUsersView },
        { path: 'settings', name: 'admin-settings', component: AdminSettingsView },
      ],
    },
    { path: '/login', name: 'login', component: LoginView, meta: { guest: true } },
    { path: '/register', name: 'register', component: RegisterView, meta: { guest: true } },
  ],
})

router.beforeEach((to) => {
  if (to.meta.guest) {
    return isLoggedIn() ? startPage() : true
  }
  if (!isLoggedIn()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (!isAllowed(to)) {
    return startPage()
  }
  return true
})

export default router
