import { lazy } from 'react'
import { createBrowserRouter, Navigate, redirect } from 'react-router-dom'
import { AppLayout } from '@/components/layout/AppLayout'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { Login } from '@/pages/Login'

// Route-level code splitting - each page becomes its own chunk instead of one large
// bundle, loaded on first navigation and shown behind the Suspense fallback (LoadingScreen)
// in App.tsx. No artificial minimum delay here - the fallback should only ever be visible
// for as long as the chunk actually takes to fetch, which is usually instant. Each page's
// own skeleton/spinner (tied to its real useQuery state) is what represents data loading.
const Dashboard = lazy(() => import('@/pages/Dashboard').then(m => ({ default: m.Dashboard })))
const Applications = lazy(() => import('@/pages/Applications').then(m => ({ default: m.Applications })))
const ApplicationDetail = lazy(() => import('@/pages/ApplicationDetail').then(m => ({ default: m.ApplicationDetail })))
const Machines = lazy(() => import('@/pages/Machines').then(m => ({ default: m.Machines })))
const MachineDetail = lazy(() => import('@/pages/MachineDetail').then(m => ({ default: m.MachineDetail })))
const Activity = lazy(() => import('@/pages/Alerts').then(m => ({ default: m.Activity })))
const Incidents = lazy(() => import('@/pages/Incidents').then(m => ({ default: m.Incidents })))
const Settings = lazy(() => import('@/pages/Settings').then(m => ({ default: m.Settings })))

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <Login />,
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: '/', element: <Dashboard /> },
          { path: '/applications', element: <Applications /> },
          { path: '/applications/:appId', element: <ApplicationDetail /> },
          { path: '/hosts', element: <Machines /> },
          { path: '/hosts/:machineId', element: <MachineDetail /> },
          { path: '/activity', element: <Activity /> },
          { path: '/incidents', element: <Incidents /> },
          { path: '/settings', element: <Settings /> },
          // Legacy /machines URLs - kept as redirects so existing bookmarks and any link
          // shared before the rename still land on the right page, with the machine id
          // carried across so a deep link resolves to that same host, not just the list.
          // Done as loaders rather than <Navigate> elements so the redirect happens before
          // anything renders.
          { path: '/machines', loader: () => redirect('/hosts') },
          {
            path: '/machines/:machineId',
            loader: ({ params }) => redirect(`/hosts/${params.machineId ?? ''}`),
          },
        ],
      },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
])
