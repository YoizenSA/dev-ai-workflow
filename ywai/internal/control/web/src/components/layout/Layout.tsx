import { ReactNode } from 'react'
import Sidebar from './Sidebar'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  return (
    <div className="app-shell">
      <Sidebar />
      <main className="content">
        {children}
      </main>
    </div>
  )
}
