import 'vuetify/styles'
import '@/styles/theme.scss'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'

// Flat UI Colors - French palette (https://flatuicolors.com/palette/fr)
const theme = {
  light: {
    colors: {
      primary: '#4a69bd',
      secondary: '#3c6382',
      accent: '#82ccdd',
      success: '#78e08f',
      warning: '#fa983a',
      error: '#eb3b5a',
      info: '#60a3bc',
    },
  },
  dark: {
    colors: {
      primary: '#6a89cc',
      secondary: '#60a3bc',
      accent: '#82ccdd',
      success: '#78e08f',
      warning: '#fa983a',
      error: '#eb3b5a',
      info: '#60a3bc',
    },
  },
}

export default createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: 'light',
    themes: theme,
  },
})
