import { ENABLE_KUN_VISUAL_NOVEL_FORUM_WINTER_THEME } from '~/config/theme'

export const useKunSnowEffect = () => {
  const isSnowing = useState<boolean>('isSnowing', () => false)

  let intervalId: NodeJS.Timeout | null = null

  let container: HTMLElement | null = null

  const flakes = ['❄', '❅', '❆']

  const initContainer = () => {
    if (!import.meta.client) return

    container = document.getElementById('kun-snow-container')
    if (!container) {
      container = document.createElement('div')
      container.id = 'kun-snow-container'
      document.body.appendChild(container)
    }
  }

  const createSnowflake = () => {
    if (!container) return

    const flake = document.createElement('div')
    flake.classList.add('kun-snowflake')
    flake.textContent = flakes[Math.floor(Math.random() * flakes.length)]!

    flake.style.left = Math.random() * 100 + 'vw'

    const depth = Math.random()
    flake.style.fontSize = 8 + depth * 20 + 'px'
    flake.style.opacity = (0.4 + depth * 0.6).toString()
    flake.style.animationDuration = 4 + (1 - depth) * 6 + 's'

    flake.addEventListener('animationend', () => {
      flake.remove()
    })

    container.appendChild(flake)
  }

  const startSnow = () => {
    if (!import.meta.client || isSnowing.value) return

    if (!ENABLE_KUN_VISUAL_NOVEL_FORUM_WINTER_THEME) return

    initContainer()
    isSnowing.value = true

    intervalId = setInterval(createSnowflake, 120)
  }

  const stopSnow = () => {
    if (!import.meta.client) return

    isSnowing.value = false
    if (intervalId) {
      clearInterval(intervalId)
      intervalId = null
    }
  }

  const toggleSnow = () => {
    if (isSnowing.value) {
      stopSnow()
    } else {
      startSnow()
    }
  }

  onUnmounted(() => {
    if (intervalId) clearInterval(intervalId)
  })

  return {
    isSnowing,
    startSnow,
    stopSnow,
    toggleSnow
  }
}
