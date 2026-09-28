import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  TimeScale,
  BarElement,
  Filler,
} from 'chart.js'
import 'chartjs-adapter-date-fns'
import type { ColorScheme } from '@/hooks/useColorScheme'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  TimeScale,
  BarElement,
  Filler
)

// ctp - a Catppuccin colour as the page currently has it, Latte or Mocha
// depending on the device. Chart.js draws on a canvas and needs real values,
// so they are read from the CSS variables the theme defines.
export function ctp(name: string, alpha = 1): string {
  const hex = getComputedStyle(document.documentElement).getPropertyValue(`--catppuccin-color-${name}`).trim()
  if (!/^#[0-9a-f]{6}$/i.test(hex)) return 'gray'
  if (alpha === 1) return hex
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16))
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

// ctpLine - an accent for chart lines. Latte's pastel accents are too faint
// on its light background, so light mode takes a darker shade (3:1 or better).
export function ctpLine(name: string, scheme: ColorScheme, alpha = 1): string {
  return ctp(scheme === 'light' ? `${name}-700` : name, alpha)
}

function baseChartOptions() {
  const grid = { color: ctp('surface1') }
  const ticks = { color: ctp('subtext0') }

  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      mode: 'index' as const,
      intersect: false,
    },
    plugins: {
      legend: {
        position: 'top' as const,
        labels: { color: ctp('subtext1') },
      },
      tooltip: {
        backgroundColor: ctp('crust', 0.95),
        titleColor: ctp('text'),
        bodyColor: ctp('subtext1'),
        borderColor: ctp('surface2'),
        borderWidth: 1,
      },
    },
    scales: {
      x: {
        type: 'time' as const,
        time: {
          displayFormats: {
            hour: 'HH:mm',
            day: 'MMM dd',
          },
        },
        grid,
        ticks,
      },
    },
    grid,
    ticks,
  }
}

// Built per render, so the colours follow the device's light/dark setting
export function temperatureHumidityOptions(scheme: ColorScheme, temperatureLabel: string) {
  const { grid, ticks, ...base } = baseChartOptions()

  return {
    ...base,
    scales: {
      ...base.scales,
      y: {
        type: 'linear' as const,
        display: true,
        position: 'left' as const,
        title: {
          display: true,
          text: temperatureLabel,
          color: ctpLine('peach', scheme),
        },
        grid,
        ticks,
      },
      y1: {
        type: 'linear' as const,
        display: true,
        position: 'right' as const,
        title: {
          display: true,
          text: 'Humidity (%)',
          color: ctpLine('sky', scheme),
        },
        grid: {
          drawOnChartArea: false,
        },
        ticks,
      },
    },
  }
}
