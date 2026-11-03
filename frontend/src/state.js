// Shared application state, kept in sync with the Go backend through
// bindings (calls) and events (pushes).
import { reactive, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import { Notify, Dark } from 'quasar'
import { EmulatorService as api } from '#bindings'
import * as ev from '../bindings/events'

export { api }

export const state = reactive({
  ready: false,
  version: '',
  settings: null,
  status: { running: false, address: '', error: '' },
  labels: [], // newest first
  jobs: [], // newest first
  stats: { jobs: 0, labels: 0, failures: 0, avgRenderMs: 0 },
  printers: [],
  groups: [],
  autoStart: false,
  busy: false
})

export const DPMM_OPTIONS = [
  { value: 6, label: '6 dpmm (152 dpi)' },
  { value: 8, label: '8 dpmm (203 dpi)' },
  { value: 12, label: '12 dpmm (300 dpi)' },
  { value: 24, label: '24 dpmm (600 dpi)' }
]

export const ROTATIONS = [0, 90, 180, 270]

export const MODES = [
  { value: 'preview', label: 'Preview', icon: 'visibility', desc: 'Show labels on screen' },
  { value: 'forward', label: 'Forward', icon: 'call_made', desc: 'Send each job to one printer' },
  { value: 'relay', label: 'Relay', icon: 'device_hub', desc: 'Send each job to a group of printers' }
]

export const printerByID = computed(() => Object.fromEntries(state.printers.map(p => [p.printerID, p])))

export function notifyError (e, prefix = '') {
  const msg = (e && (e.message || e.toString())) || 'Something went wrong'
  Notify.create({ type: 'negative', message: prefix ? `${prefix}: ${msg}` : msg })
}

export function notifyOK (message) {
  Notify.create({ type: 'positive', message, icon: 'check_circle' })
}

function applyTheme (theme) {
  Dark.set(theme === 'auto' ? 'auto' : theme === 'dark')
}

const MAX_JOBS = 200
let saveTimer = null
let pending = null

// updateSettings applies a change immediately in the UI and saves it after
// a short pause, so typing in a number field does not save every keystroke.
export function updateSettings (patch, delay = 400) {
  Object.assign(state.settings, patch)
  if ('theme' in patch) applyTheme(state.settings.theme)
  pending = { ...state.settings }
  clearTimeout(saveTimer)
  saveTimer = setTimeout(flushSettings, delay)
}

export async function flushSettings () {
  clearTimeout(saveTimer)
  if (!pending) return
  const next = pending
  pending = null
  try {
    state.settings = await api.UpdateSettings(next)
  } catch (e) {
    notifyError(e, 'Settings')
  }
}

export async function startServer () {
  state.busy = true
  try {
    await flushSettings()
    state.status = await api.Start()
  } catch (e) {
    notifyError(e, 'Could not start the printer')
  } finally {
    state.busy = false
  }
}

export async function stopServer () {
  state.busy = true
  try {
    state.status = await api.Stop()
  } finally {
    state.busy = false
  }
}

export async function refreshPrinters () {
  const [printers, groups] = await Promise.all([api.Printers(), api.RelayGroups()])
  state.printers = printers || []
  state.groups = groups || []
}

export async function refreshStats () {
  state.stats = await api.Stats()
}

let started = false

export async function init () {
  if (started) return
  started = true

  Events.On(ev.LabelsAdded, e => {
    const added = e.data || []
    // Newest first; the backend sends a job's labels in print order.
    state.labels.unshift(...added.slice().reverse())
    const limit = state.settings?.historyLimit || 200
    if (state.labels.length > limit) state.labels.length = limit
  })
  Events.On(ev.LabelsCleared, () => {
    state.labels = []
    state.jobs = []
  })
  Events.On(ev.JobDone, e => {
    const job = e.data
    state.jobs.unshift(job)
    if (state.jobs.length > MAX_JOBS) state.jobs.length = MAX_JOBS
    refreshStats()
    for (const d of job.deliveries || []) {
      if (!d.ok) Notify.create({ type: 'negative', icon: 'print_disabled', message: `${d.printerName}: ${d.error}` })
    }
    if (job.error) Notify.create({ type: 'warning', message: job.error })
  })
  Events.On(ev.ServerStatus, e => { state.status = e.data })
  Events.On(ev.SettingsChanged, e => {
    if (!pending) state.settings = e.data
  })

  const [version, settings, status, labels, jobs, stats, autoStart] = await Promise.all([
    api.Version(), api.Settings(), api.Status(), api.Labels(), api.Jobs(), api.Stats(), api.AutoStartEnabled()
  ])
  Object.assign(state, { version, settings, status, labels: labels || [], jobs: jobs || [], stats, autoStart })
  applyTheme(settings.theme)
  await refreshPrinters()
  state.ready = true
}

// ---- formatting helpers -------------------------------------------------

export function fmtTime (ms) {
  return new Date(ms).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function fmtDateTime (ms) {
  return new Date(ms).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function fmtBytes (n) {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

export function labelInches (l) {
  const dpi = l.dpmm * 25.4
  return `${(l.width / dpi).toFixed(2)} × ${(l.height / dpi).toFixed(2)} in`
}
