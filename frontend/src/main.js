import { createApp } from 'vue'
import { Quasar, Dialog, Notify, Dark } from 'quasar'
import '@quasar/extras/material-icons/material-icons.css'
import '@quasar/extras/material-icons-outlined/material-icons-outlined.css'
import '@quasar/extras/roboto-font/roboto-font.css'
import 'quasar/src/css/index.sass'
import './css/app.scss'

import App from './App.vue'
import router from './router'

createApp(App)
  .use(Quasar, {
    plugins: { Dialog, Notify, Dark },
    config: {
      dark: 'auto',
      notify: { position: 'bottom-right', timeout: 2500 }
    }
  })
  .use(router)
  .mount('#app')
