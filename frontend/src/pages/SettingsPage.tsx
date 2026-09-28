import { useEffect } from 'react'
import MainLayout from '@/components/layout/MainLayout'
import Settings from '@/components/Settings'

export default function SettingsPage() {
  // Set rather than render a <title>: React would add a second one after
  // index.html's, and the browser shows the first
  useEffect(() => {
    const previous = document.title
    document.title = 'Settings - Nanit Dashboard'
    return () => {
      document.title = previous
    }
  }, [])

  return (
    <MainLayout>
      <Settings />
    </MainLayout>
  );
}
