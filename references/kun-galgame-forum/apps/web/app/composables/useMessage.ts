import type { KunMessageType, KunMessagePosition } from '@kungal/ui-vue'
import { infoMessages } from '~/error/kunMessage'

// kungal-side wrapper around the layer's useKunMessage. The only
// kungal-specific bit is the `number → infoMessages[code]` lookup,
// kept here so call sites can stay `useMessage(233, 'error')` instead
// of resolving the i18n string at every site. Everything else
// (singleton state, deduping, MessageContainer auto-mount) lives in
// @kun/ui's useKunMessage and is reused as-is.
export const useMessage = (
  messageData: number | string,
  type: KunMessageType,
  duration = 3000,
  richText = false,
  position: KunMessagePosition = 'top-center'
) => {
  const resolved =
    typeof messageData === 'string'
      ? messageData
      : (infoMessages[messageData] ?? '')
  return useKunMessage(resolved, type, duration, richText, position)
}

export const useMessageState = useKunMessageState
