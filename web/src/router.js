import { createRouter, createWebHistory } from 'vue-router'

import ProjectsView from './views/ProjectsView.vue'
import TasksView from './views/TasksView.vue'
import ProjectView from './views/ProjectView.vue'
import TaskView from './views/TaskView.vue'
import AgentsView from './views/AgentsView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'projects', component: ProjectsView },
    { path: '/tasks', name: 'tasks', component: TasksView },
    { path: '/p/:slug', name: 'project', component: ProjectView, props: true },
    {
      path: '/t/:taskRef',
      name: 'task',
      component: TaskView,
      props: (route) => ({ taskRef: route.params.taskRef }),
    },
    { path: '/agents', name: 'agents', component: AgentsView },
    { path: '/:rest(.*)', redirect: '/' },
  ],
})
