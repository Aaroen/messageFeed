import Alert from '@arco-design/web-vue/es/alert'
import '@arco-design/web-vue/es/alert/style/css.js'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import router from './router'
import './styles/app.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(Alert)

app.mount('#app')
