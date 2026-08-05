// Disclosure (open/close) state helper. Previously provided by the @kun/ui
// Nuxt layer; the published @kungal/ui-* packages don't ship it, so the forum
// owns its own copy (it's a tiny generic state composable, not a UI concern).
import { computed, ref } from 'vue'

export interface UseDisclosureProps {
  isOpen?: boolean
  defaultOpen?: boolean
  onChange?: (isOpen: boolean) => void
  onOpen?: () => void
  onClose?: () => void
}

export const useKunDisclosure = (props: UseDisclosureProps = {}) => {
  const {
    isOpen: controlledIsOpen,
    defaultOpen = false,
    onChange,
    onOpen,
    onClose
  } = props

  const internalOpen = ref(defaultOpen)
  const isControlled = controlledIsOpen !== undefined

  const isOpen = computed<boolean>(() =>
    isControlled ? controlledIsOpen! : internalOpen.value
  )

  const setOpen = (value: boolean) => {
    if (!isControlled) {
      internalOpen.value = value
    }
    onChange?.(value)
    if (value) {
      onOpen?.()
    } else {
      onClose?.()
    }
  }

  return {
    isOpen,
    onOpen: () => {
      if (!isOpen.value) setOpen(true)
    },
    onClose: () => {
      if (isOpen.value) setOpen(false)
    },
    onOpenChange: () => setOpen(!isOpen.value)
  }
}
