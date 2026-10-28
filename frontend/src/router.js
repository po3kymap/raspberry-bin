import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn } from './auth.js'
import AdminView from './views/AdminView.vue'
import HistoryView from './views/HistoryView.vue'
import HomeView from './views/HomeView.vue'
import LoginView from './views/LoginView.vue'
import RegisterView from './views/RegisterView.vue'
import StatsView from './views/StatsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/stats', name: 'stats', component: StatsView },
    { path: '/history', name: 'history', component: HistoryView },
    { path: '/admin', name: 'admin', component: AdminView, meta: { public: true } },
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
  return true
})

export default router
