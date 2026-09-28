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
        // Chart.js picks the unit from the axis range: minutes for an hour,
        // hours for a day, days for a week or a month
        time: {
          displayFormats: {
            minute: 'h:mm a',
            hour: 'h a',
            day: 'EEE d MMM',
          },
          tooltipFormat: 'EEE d MMM, h:mm a',
        },
        grid,
        ticks: { ...ticks, maxRotation: 0, autoSkip: true, maxTicksLimit: 8 },
      },
    },
    grid,
    ticks,
  }
}

// timeUnit - the tick unit for a window this long. Chosen here rather than
// by Chart.js, which under a tick limit can land on odd steps such as every
// 21 hours across a week, labelled 11 PM, 8 PM, 5 PM...
function timeUnit(seconds: number): 'minute' | 'hour' | 'day' {
  if (seconds <= 2 * 60 * 60) return 'minute'
  if (seconds <= 2 * 24 * 60 * 60) return 'hour'
  return 'day'
}

// Built per render, so the colours follow the device's light/dark setting.
// range (unix seconds) pins the time axis to the selected window; left to
// itself Chart.js fits the axis to the data, so an hour of history in the
// 7-day view came out as an hour-wide axis.
export function temperatureHumidityOptions(scheme: ColorScheme, temperatureLabel: string, range: { start: number; end: number }) {
  const { grid, ticks, ...base } = baseChartOptions()

  return {
    ...base,
    scales: {
      ...base.scales,
      x: {
        ...base.scales.x,
        min: range.start * 1000,
        max: range.end * 1000,
        time: { ...base.scales.x.time, unit: timeUnit(range.end - range.start) },
      },
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
