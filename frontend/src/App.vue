<template>
  <q-layout view="hHh LpR fFf">
    <q-header class="app-header">
      <q-toolbar class="q-px-md" style="min-height: 60px">
        <q-btn flat dense round icon="menu" aria-label="Menu" @click="mini = !mini" />
        <div class="row items-center no-wrap q-ml-sm">
          <q-avatar size="34px" class="brand-mark"><q-icon name="print" size="20px" /></q-avatar>
          <div class="q-ml-sm column">
            <span class="text-subtitle1 text-weight-bold leading-tight">Printer Emulator</span>
            <span class="text-caption header-sub leading-tight">ZPL · rendered locally</span>
          </div>
        </div>

        <q-space />

        <ServerControl v-if="state.ready" />

        <q-space />

        <q-btn flat dense no-caps icon="code" label="GitHub" class="q-mr-xs gt-sm" @click="open('https://github.com/dchupp/printerEmulator')" />
        <q-btn flat dense no-caps class="q-mr-sm gt-sm" @click="open('https://data-magik.com')">
          <img src="/datamagik.svg" alt="" class="dm-logo q-mr-sm" />DataMagik
        </q-btn>
        <q-btn flat dense round :icon="themeIcon" @click="cycleTheme">
          <q-tooltip>Theme: {{ state.settings?.theme || 'auto' }}</q-tooltip>
        </q-btn>
        <q-chip dense square class="version-chip q-ml-sm" icon="electric_bolt">v{{ state.version }}</q-chip>
      </q-toolbar>
    </q-header>

    <q-drawer :model-value="true" side="left" :mini="mini" :width="220" :mini-width="64" bordered persistent class="app-drawer">
      <q-list padding class="nav-list">
        <q-item
          v-for="r in navRoutes"
          :key="r.name"
          clickable
          v-ripple
          :to="r.path"
          exact
          active-class="nav-active"
          class="nav-item"
        >
          <q-item-section avatar>
            <q-icon :name="r.meta.icon" />
            <q-badge v-if="r.name === 'labels' && unseen" floating color="accent" rounded>{{ unseen > 99 ? '99+' : unseen }}</q-badge>
          </q-item-section>
          <q-item-section>{{ r.meta.title }}</q-item-section>
          <q-tooltip v-if="mini" anchor="center right" self="center left" :offset="[8, 0]">{{ r.meta.title }}</q-tooltip>
        </q-item>
      </q-list>
    </q-drawer>

    <q-page-container>
      <div v-if="!state.ready" class="fullscreen flex flex-center">
        <q-spinner-dots color="primary" size="48px" />
      </div>
      <router-view v-else v-slot="{ Component }">
        <keep-alive include="LabelsPage,EditorPage">
          <component :is="Component" />
        </keep-alive>
      </router-view>
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Browser } from '@wailsio/runtime'
import ServerControl from './components/ServerControl.vue'
import { init, state, updateSettings, notifyError } from './state'

const mini = ref(true)
const router = useRouter()
const route = useRoute()
const navRoutes = router.getRoutes().filter(r => r.meta?.title).sort((a, b) => order(a) - order(b))
function order (r) { return ['labels', 'editor', 'printers', 'relay', 'settings'].indexOf(r.name) }

// Labels that arrived while another page was open.
const seen = ref(0)
const unseen = computed(() => route.name === 'labels' ? 0 : Math.max(0, totalLabels.value - seen.value))
const totalLabels = computed(() => state.stats.labels)
watch(() => route.name, name => { if (name === 'labels') seen.value = totalLabels.value })
watch(totalLabels, n => { if (route.name === 'labels') seen.value = n })

const themeIcon = computed(() => ({ light: 'light_mode', dark: 'dark_mode' }[state.settings?.theme] || 'brightness_auto'))
function cycleTheme () {
  const next = { auto: 'light', light: 'dark', dark: 'auto' }[state.settings?.theme || 'auto']
  updateSettings({ theme: next }, 0)
}

function open (url) { Browser.OpenURL(url) }

onMounted(async () => {
  try {
    await init()
    seen.value = state.stats.labels
  } catch (e) {
    notifyError(e, 'Could not load the emulator')
  }
})
</script>
