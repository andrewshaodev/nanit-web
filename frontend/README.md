# Nanit Dashboard Frontend

React single-page app for the Nanit Home Assistant Bridge, built with Vite and served by the Go backend.

## Features

- 🎯 **Real-time Dashboard** - Live sensor data with 5-second updates
- 📹 **Video Streaming** - HLS video player with live controls
- 📊 **Historical Data** - Temperature, humidity and day/night charts (Chart.js)
- 🎛️ **Device Controls** - Night light, standby mode controls
- 🔧 **Device Information** - Detailed device status and diagnostics
- 🔗 **Streaming Links** - RTMP and HLS URLs for external use
- 📱 **Responsive Design** - Works on desktop, tablet, and mobile
- 🔐 **Authentication** - 2FA setup flow

## Technology Stack

- **Build**: Vite, run with Bun
- **UI**: React 19 with React Router
- **Language**: TypeScript 7
- **Styling**: Tailwind CSS 4
- **Data Fetching**: SWR for real-time updates
- **Video**: video.js and HLS.js
- **Charts**: Chart.js
- **Linting**: oxlint

## Development Setup

### Prerequisites

- [Bun](https://bun.sh) 1.3+

### Local Development

```bash
bun install        # Install dependencies
bun run dev        # Dev server with hot reload
bun run build      # Production build into dist/
bun run typecheck  # tsc --noEmit
bun run lint       # oxlint
```

The dev server proxies `/api` to a running Go backend at `http://localhost:8080`
(the backend's default `NANIT_HTTP_PORT`). Set `NANIT_API_URL` to point it elsewhere:

```bash
NANIT_API_URL=http://n100:8080 bun run dev
```

## Integration with Go Backend

The root `Dockerfile` builds this app with Bun and copies `dist/` into the Go
image as `/app/web`. The Go server (`pkg/app/serve_react.go`) serves the
content-hashed files under `/assets/` with immutable caching, and answers every
other non-API path with `index.html` (revalidated on each load) so React Router
can handle it.

## Project Structure

```
frontend/
├── index.html              # Page shell; Vite injects the bundle
├── vite.config.ts          # Build config and dev API proxy
├── .oxlintrc.json          # Lint config
└── src/
    ├── main.tsx            # Entry point and routes
    ├── index.css           # Tailwind theme and shared styles
    ├── pages/              # Dashboard, Settings, Setup
    ├── components/         # React components
    │   ├── layout/         # Layout components
    │   ├── baby/           # Baby-specific components
    │   ├── charts/         # Chart.js charts
    │   ├── settings/       # Settings tabs
    │   └── ui/             # Reusable UI components
    ├── hooks/              # Custom React hooks
    ├── lib/                # Utilities and API client
    └── types/              # TypeScript type definitions
```

## Key Components

### Baby Card (`components/baby/BabyCard.tsx`)
Main component displaying baby information including:
- Live video stream
- Sensor data (temperature, humidity, motion, sound)
- Device controls
- Historical data charts
- Streaming URLs

### API Client (`lib/api.ts`)
TypeScript client for all backend API calls with proper error handling and type safety.

### Hooks (`hooks/`)
- `useStatus.ts` - Real-time status updates with SWR
- `useTemperatureUnit.ts` - Temperature unit conversion
- `useHistoricalData.ts` - Historical sensor data and analytics
- `useVideoPlayer.ts` - video.js player lifecycle

### Real-time Updates
Uses SWR with 5-second polling for live data updates. Automatically handles connection states and error recovery.

## API Integration

The frontend communicates with the Go backend through REST APIs:

- **Status**: `/api/status` - Real-time baby data
- **Controls**: `/api/control/*` - Device commands
- **History**: `/api/history/*` - Historical data
- **Streaming**: `/api/stream/*` - Video streaming
- **Auth**: `/api/auth/*` - Authentication

## Contributing

1. Follow TypeScript best practices
2. Use Tailwind CSS for styling
3. Ensure responsive design
4. Add proper error handling
5. Update types when API changes
