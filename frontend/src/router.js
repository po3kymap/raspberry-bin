import { createRouter, createWebHistory } from 'vue-router'
import { isAdmin, isLoggedIn } from './auth.js'
import AdminView from './views/AdminView.vue'
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
    { path: '/admin', name: 'admin', component: AdminView, meta: { admin: true } },
    { path: '/login', name: 'login', component: LoginView, meta: { guest: true } },
    { path: '/register', name: 'register', component: RegisterView, meta: { guest: true } },
  ],
})

router.beforeEach((to) => {
  if (to.meta.guest) {
    return isLoggedIn() ? { name: 'home' } : true
  }
  if (!to.meta.public && !isLoggedIn()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.admin && !isAdmin()) {
    return { name: 'home' }
  }
  return true
})

export default router
