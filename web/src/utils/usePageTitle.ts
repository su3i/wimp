import { useEffect } from 'react'

const BASE = 'Windows Infrastructure Management Platform'

export function usePageTitle(title?: string) {
  useEffect(() => {
    document.title = title ? `${title} | ${BASE}` : BASE
    return () => { document.title = BASE }
  }, [title])
}
