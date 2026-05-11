import type { Session, Span } from './types'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}

export const api = {
  sessions: {
    list: (limit = 50, offset = 0) =>
      get<{ sessions: Session[] | null; limit: number; offset: number }>(
        `/sessions?limit=${limit}&offset=${offset}`,
      ),
    get: (id: string) => get<Session>(`/sessions/${id}`),
    spans: (id: string) => get<{ spans: Span[] | null }>(`/sessions/${id}/spans`),
  },
}
