import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

import './assets/styles/ziwu.css'
import './assets/styles/global.css'
import './assets/styles/naive-overrides.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.mount('#app')
