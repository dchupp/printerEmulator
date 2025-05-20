const routes = [
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      { path: '', component: () => import('pages/IndexPage.vue') },
      { path: '/printers', component: () => import('pages/PrintersPage.vue') },
      { path: '/relay', component: () => import('pages/RelayPage.vue') },
      { path: '/zpl2net', component: () => import('pages/ZPL2NetPage.vue') } // Updated to new ZPL2NetPage component
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

