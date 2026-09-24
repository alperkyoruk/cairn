import { createRouter, createWebHistory } from 'vue-router'

import BoardView from './views/BoardView.vue'
import ProjectsView from './views/ProjectsView.vue'
import TasksView from './views/TasksView.vue'
import ProjectView from './views/ProjectView.vue'
import TaskView from './views/TaskView.vue'
import AgentsView from './views/AgentsView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    // The board is the root now. cairn-6 put the projects list here and gave
    // good reasons -- a cross-project list is only legible once you know the
    // projects, and a new install has projects before it has tasks. The first
    // survives: every card carries its project. The second is handled by the
    // board's own empty state, which sends you to make one.
    { path: '/', name: 'board', component: BoardView },
    { path: '/projects', name: 'projects', component: ProjectsView },
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
