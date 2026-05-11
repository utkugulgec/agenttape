import { useState } from 'react'
import SessionList from './SessionList'
import SessionDetail from './SessionDetail'

type View = { page: 'list' } | { page: 'detail'; id: string }

export default function App() {
  const [view, setView] = useState<View>({ page: 'list' })

  if (view.page === 'detail') {
    return (
      <SessionDetail
        sessionId={view.id}
        onBack={() => setView({ page: 'list' })}
      />
    )
  }

  return <SessionList onSelect={(id) => setView({ page: 'detail', id })} />
}
