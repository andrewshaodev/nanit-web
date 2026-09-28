// Global tooltip configuration for react-tooltip
export const tooltipConfig = {
  // Default styling that matches the app design
  place: 'top' as const,
  // Catppuccin CSS variables, so tooltips follow the Latte/Mocha switch
  style: {
    backgroundColor: 'var(--color-ctp-crust)',
    color: 'var(--color-ctp-text)',
    borderRadius: '8px',
    padding: '8px 12px',
    fontSize: '14px',
    boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
    zIndex: 50,
  },
  // Border as separate prop
  border: '1px solid var(--color-ctp-surface2)',
  // Animation settings
  delayShow: 500,
  delayHide: 0,
  // Responsive positioning
  offset: 8,
}

// Specialized configs for different tooltip types
export const sensorTooltipConfig = {
  ...tooltipConfig,
  place: 'bottom' as const,
}

export const timelineTooltipConfig = {
  ...tooltipConfig,
  place: 'top' as const,
  style: {
    ...tooltipConfig.style,
    minWidth: '180px',
    textAlign: 'left' as const,
  },
}

export const errorTooltipConfig = {
  ...tooltipConfig,
  place: 'bottom' as const,
  style: {
    ...tooltipConfig.style,
    backgroundColor: 'var(--color-ctp-red)', // Red background for errors
    color: 'var(--color-ctp-base)',
    maxWidth: '300px',
  },
}