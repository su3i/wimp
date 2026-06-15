import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { MobileBlock } from '@/components/common/MobileBlock'

export function AppLayout() {
  return (
    <>
      <MobileBlock />
      <div className="flex min-h-screen bg-canvas">
        <Sidebar />
        <main className="flex-1 min-w-0 p-6 overflow-auto">
          <Outlet />
        </main>
      </div>
    </>
  )
}
