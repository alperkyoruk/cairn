import { createRouter, createWebHistory } from 'vue-router'

import Board from './views/Board.vue'
import Project from './views/Project.vue'
import Task from './views/Task.vue'
import Agents from './views/Agents.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'board', component: Board },
    { path: '/p/:slug', name: 'project', component: Project, props: true },
    { path: '/t/:ref', name: 'task', component: Task, props: true },
    { path: '/agents', name: 'agents', component: Agents },
    { path: '/:rest(.*)', redirect: '/' },
  ],
})
