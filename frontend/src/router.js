import { createRouter, createWebHistory } from 'vue-router'
import AdminView from './views/AdminView.vue'
import HomeView from './views/HomeView.vue'
import StatsView from './views/StatsView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/stats', name: 'stats', component: StatsView },
    { path: '/admin', name: 'admin', component: AdminView },
  ],
})
