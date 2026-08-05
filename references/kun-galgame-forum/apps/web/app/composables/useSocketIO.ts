import { io, type Socket } from 'socket.io-client'

// let socket: Socket | null = null

// socket.io is currently disabled — this stub returns null. Typed as
// `Socket | null` so callers narrow correctly (a bare `return null` typed the
// result as `null`, making every `socket.on(...)` a `never`/null type error and
// a latent runtime crash if a handler were ever mounted).
export const useSocketIO = (): Socket | null => {
  // if (!socket) {
  //   socket = io()
  // }

  return null
}
