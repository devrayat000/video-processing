import { Link, Outlet, useLocation } from 'react-router'
import { Button } from '@/components/ui/button'

export default function Layout() {
  const location = useLocation()

  const navItems = [
    { path: '/', label: 'Dashboard' },
    { path: '/upload', label: 'Upload' },
    { path: '/videos', label: 'Videos' },
  ]

  return (
    <div className="min-h-screen">
      <nav className="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container mx-auto px-4 py-3 flex items-center gap-6">
          <Link to="/" className="font-semibold text-lg">
            Video Processing
          </Link>
          <div className="flex gap-2">
            {navItems.map((item) => (
              <Button
                key={item.path}
                asChild
                variant={location.pathname === item.path ? 'default' : 'ghost'}
                size="sm"
              >
                <Link to={item.path}>{item.label}</Link>
              </Button>
            ))}
          </div>
        </div>
      </nav>
      <div className="container mx-auto">
        <Outlet />
      </div>
    </div>
  )
}
