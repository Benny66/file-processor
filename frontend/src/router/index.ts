import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

// 路由骨架：图片处理工作台 + 设置。
const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'image',
    component: () => import('../views/image/ImageWorkbench.vue'),
    meta: { title: '图片处理' },
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('../views/Settings.vue'),
    meta: { title: '设置' },
  },
// gen:route
]

const router = createRouter({
  // Wails 打包后是 file:// 协议，用 hash 历史避免路由刷新 404。
  history: createWebHashHistory(),
  routes,
})

export default router
