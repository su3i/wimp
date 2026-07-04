import { createBrowserRouter, Navigate } from 'react-router-dom'
import { AppLayout } from '@/components/layout/AppLayout'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { Login } from '@/pages/Login'
import { Dashboard } from '@/pages/Dashboard'
import { Applications } from '@/pages/Applications'
import { ApplicationDetail } from '@/pages/ApplicationDetail'
import { Machines } from '@/pages/Machines'
import { MachineDetail } from '@/pages/MachineDetail'
import { Alerts } from '@/pages/Alerts'
import { Monitors } from '@/pages/Monitors'

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
          { path: '/machines', element: <Machines /> },
          { path: '/machines/:machineId', element: <MachineDetail /> },
          { path: '/monitors', element: <Monitors /> },
          { path: '/alerts', element: <Alerts /> },
        ],
      },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
])
