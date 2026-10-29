import { isAdmin } from './auth.js'

// Where a logged-in user lands: admins get the admin app, everyone else the scan page
export function startPage() {
  return isAdmin() ? { name: 'admin-overview' } : { name: 'home' }
}

// Admins and users have separate apps, so each role may only open its own pages
export function isAllowed(to) {
  return Boolean(to.meta.admin) === isAdmin()
}
