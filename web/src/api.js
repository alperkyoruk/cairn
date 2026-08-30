// One place that talks to the server.
//
// The API returns {error:{kind,message}} on failure, and the messages are
// written to be shown to a person as they are -- so nothing here rewrites them.

export class ApiError extends Error {
  constructor(kind, message, status) {
    super(message)
    this.kind = kind
    this.status = status
  }
}

async function request(method, path, body) {
  const options = { method, headers: {} }
  if (body !== undefined) {
    options.headers['Content-Type'] = 'application/json'
    options.body = JSON.stringify(body)
  }

  const response = await fetch(`/api${path}`, options)
  if (response.status === 204) return null

  const text = await response.text()
  const payload = text ? JSON.parse(text) : null

  if (!response.ok) {
    const failure = payload?.error ?? {}
    throw new ApiError(failure.kind ?? 'internal', failure.message ?? response.statusText, response.status)
  }
  return payload
}

export const api = {
  session: () => request('GET', '/session'),
  setup: (username, password) => request('POST', '/setup', { username, password }),
  login: (username, password) => request('POST', '/login', { username, password }),
  logout: () => request('POST', '/logout'),

  board: () => request('GET', '/board'),

  projects: () => request('GET', '/projects'),
  project: (key) => request('GET', `/projects/${key}`),
  projectTasks: (key) => request('GET', `/projects/${key}/tasks`),
  createProject: (slug, name, description = '') =>
    request('POST', '/projects', { slug, name, description }),
  deleteProject: (key) => request('DELETE', `/projects/${key}`),

  task: (ref) => request('GET', `/tasks/${ref}`),
  createTask: (project_id, title, body = '', status = '') =>
    request('POST', '/tasks', { project_id, title, body, status }),
  updateTask: (ref, title, body) => request('PATCH', `/tasks/${ref}`, { title, body }),
  deleteTask: (ref) => request('DELETE', `/tasks/${ref}`),
  transition: (ref, to, state, worklog) =>
    request('POST', `/tasks/${ref}/transition`, { to, state: state ?? null, worklog: worklog ?? null }),
  writeState: (ref, state) => request('PUT', `/tasks/${ref}/state`, state),
  appendWorklog: (ref, worklog) => request('POST', `/tasks/${ref}/worklog`, worklog),

  agents: () => request('GET', '/agents'),
  createAgent: (name) => request('POST', '/agents', { name }),
  tokens: (agentId) => request('GET', `/agents/${agentId}/tokens`),
  issueToken: (agentId, name) => request('POST', `/agents/${agentId}/tokens`, { name }),
  revokeToken: (tokenId) => request('DELETE', `/tokens/${tokenId}`),
}
