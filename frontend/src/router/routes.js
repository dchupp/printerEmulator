const routes = [
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      { path: '', component: () => import('pages/IndexPage.vue') },
      { path: '/printers', component: () => import('pages/PrintersPage.vue') },
      { path: '/relay', component: () => import('pages/ErrorNotFound.vue') }, // Placeholder for Printer Relay Menu
      { path: '/zpl2net', component: () => import('pages/ErrorNotFound.vue') } // Placeholder for ZPL to Network Printer
    ]
  },

  // Always leave this as last one,
  // but you can also remove it
  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/ErrorNotFound.vue')
  }
]

export default routes
