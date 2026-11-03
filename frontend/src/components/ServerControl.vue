<template>
  <div class="server-control row items-center no-wrap q-gutter-x-sm">
    <div class="status-pill row items-center no-wrap" :class="pillClass">
      <span class="status-dot" />
      <span class="q-ml-sm text-weight-medium ellipsis">{{ statusText }}</span>
    </div>

    <q-btn-dropdown
      split
      unelevated
      no-caps
      class="mode-btn"
      :icon="mode.icon"
      :label="modeLabel"
      dropdown-icon="expand_more"
      @click="router.push(modeTarget)"
    >
      <q-list style="min-width: 260px">
        <q-item-label header>When a job arrives</q-item-label>
        <q-item v-for="m in MODES" :key="m.value" clickable v-close-popup :active="m.value === s.mode" active-class="text-primary" @click="setMode(m.value)">
          <q-item-section avatar><q-icon :name="m.icon" /></q-item-section>
          <q-item-section>
            <q-item-label>{{ m.label }}</q-item-label>
            <q-item-label caption>{{ m.desc }}</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>
    </q-btn-dropdown>

    <q-btn
      v-if="!state.status.running"
      unelevated
      no-caps
      color="positive"
      icon="play_arrow"
      label="Start"
      :loading="state.busy"
      class="power-btn"
      @click="startServer"
    />
    <q-btn
      v-else
      unelevated
      no-caps
      color="negative"
      icon="stop"
      label="Stop"
      :loading="state.busy"
      class="power-btn"
      @click="stopServer"
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { MODES, state, startServer, stopServer, updateSettings, printerByID } from '../state'

const router = useRouter()
const s = computed(() => state.settings)
const mode = computed(() => MODES.find(m => m.value === s.value.mode) || MODES[0])

const modeLabel = computed(() => {
  if (s.value.mode === 'forward') {
    const p = printerByID.value[s.value.forwardPrinterID]
    return p ? `Forward → ${p.printerName}` : 'Forward → choose printer'
  }
  if (s.value.mode === 'relay') {
    const g = state.groups.find(g => g.groupID === s.value.relayGroupID)
    return g ? `Relay → ${g.name}` : 'Relay → choose group'
  }
  return 'Preview'
})

const modeTarget = computed(() => ({ forward: '/printers', relay: '/relay' }[s.value.mode] || '/'))

function setMode (m) {
  updateSettings({ mode: m }, 0)
  if (m === 'forward' && !s.value.forwardPrinterID) router.push('/printers')
  if (m === 'relay' && !s.value.relayGroupID) router.push('/relay')
}

const statusText = computed(() => {
  if (state.status.running) return `Listening on ${state.status.address}`
  if (state.status.error) return 'Not running'
  return `Stopped · port ${s.value.port}`
})

const pillClass = computed(() => state.status.running ? 'is-on' : state.status.error ? 'is-error' : 'is-off')
</script>
