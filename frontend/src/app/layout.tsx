import { Inter } from 'next/font/google'
import './globals.css'
import ClientTooltip from '@/components/ui/ClientTooltip'

const inter = Inter({ subsets: ['latin'] })

export const metadata = {
  title: 'Nanit Dashboard',
  description: 'Nanit Home Assistant Bridge Dashboard',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className={`${inter.className} bg-nanit-gray-50 min-h-screen`}>
        {children}
        <ClientTooltip />
      </body>
    </html>
  )
}