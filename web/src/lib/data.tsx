// The person's homes, things, plants and pros, loaded once for every picker; any
// change in the app ("phm:changed") reloads them.
import React, { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api, V1 } from './api'
import type { Home, Plant, Pro, Thing } from './types'

interface Data { homes: Home[] | null; things: Thing[]; plants: Plant[]; pros: Pro[]; reload: () => void }
const Ctx = createContext<Data>({ homes: null, things: [], plants: [], pros: [], reload: () => {} })
export const useData = () => useContext(Ctx)

/** Something was added or changed: lists and pages reload. */
export const announceChange = () => window.dispatchEvent(new CustomEvent('phm:changed'))
export function useOnChange(cb: () => void) {
  const ref = React.useRef(cb)
  ref.current = cb
  useEffect(() => {
    const h = () => ref.current()
    window.addEventListener('phm:changed', h)
    return () => window.removeEventListener('phm:changed', h)
  }, [])
}

export function DataProvider({ children }: { children: React.ReactNode }) {
  const [homes, setHomes] = useState<Home[] | null>(null)
  const [things, setThings] = useState<Thing[]>([])
  const [plants, setPlants] = useState<Plant[]>([])
  const [pros, setPros] = useState<Pro[]>([])
  const reload = useCallback(() => {
    api.get<Home[]>(`${V1}/homes`).then(setHomes).catch(() => setHomes([]))
    api.get<Thing[]>(`${V1}/things`).then(setThings).catch(() => {})
    api.get<Plant[]>(`${V1}/plants`).then(setPlants).catch(() => {})
    api.get<Pro[]>(`${V1}/pros`).then(setPros).catch(() => {})
  }, [])
  useEffect(() => { reload() }, [reload])
  useOnChange(reload)
  return <Ctx.Provider value={{ homes, things, plants, pros, reload }}>{children}</Ctx.Provider>
}
