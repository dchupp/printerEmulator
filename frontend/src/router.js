import { createRouter, createWebHashHistory } from 'vue-router'
import LabelsPage from './pages/LabelsPage.vue'

export default createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'labels', component: LabelsPage, meta: { title: 'Labels', icon: 'receipt_long' } },
    { path: '/editor', name: 'editor', component: () => import('./pages/EditorPage.vue'), meta: { title: 'ZPL Editor', icon: 'code' } },
    { path: '/printers', name: 'printers', component: () => import('./pages/PrintersPage.vue'), meta: { title: 'Network Printers', icon: 'print' } },
    { path: '/relay', name: 'relay', component: () => import('./pages/RelayPage.vue'), meta: { title: 'Relay Groups', icon: 'device_hub' } },
    { path: '/settings', name: 'settings', component: () => import('./pages/SettingsPage.vue'), meta: { title: 'Settings', icon: 'tune' } },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
})
