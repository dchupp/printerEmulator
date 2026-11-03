<template>
  <q-page class="labels-page column no-wrap">
    <!-- Media bar: the settings a label needs, always in reach -->
    <div class="media-bar row items-center q-col-gutter-sm q-px-md q-py-sm">
      <div class="col-auto">
        <q-input
          dense outlined type="number" step="0.25" min="0.25" style="width: 104px"
          :model-value="s.labelWidth" label="Width (in)"
          @update:model-value="v => num('labelWidth', v)"
        >
          <template #prepend><q-icon name="straighten" size="18px" /></template>
        </q-input>
      </div>
      <div class="col-auto">
        <q-input
          dense outlined type="number" step="0.25" min="0.25" style="width: 104px"
          :model-value="s.labelHeight" label="Height (in)"
          @update:model-value="v => num('labelHeight', v)"
        >
          <template #prepend><q-icon name="height" size="18px" /></template>
        </q-input>
      </div>
      <div class="col-auto">
        <q-select
          dense outlined emit-value map-options style="width: 170px"
          :options="DPMM_OPTIONS" :model-value="s.dpmm" label="Density"
          @update:model-value="v => updateSettings({ dpmm: v }, 0)"
        />
      </div>
      <div class="col-auto">
        <q-btn-toggle
          dense unelevated no-caps toggle-color="primary" class="rot-toggle"
          :options="ROTATIONS.map(r => ({ value: r, label: r + '°' }))"
          :model-value="s.rotation"
          @update:model-value="v => updateSettings({ rotation: v }, 0)"
        />
        <q-tooltip>Rotate rendered labels</q-tooltip>
      </div>

      <q-space />

      <div class="col-auto row items-center no-wrap q-gutter-x-md stats text-caption">
        <div><b>{{ state.stats.jobs }}</b> jobs</div>
        <div><b>{{ state.stats.labels }}</b> labels</div>
        <div v-if="state.stats.avgRenderMs"><b>{{ state.stats.avgRenderMs.toFixed(1) }}</b> ms avg render</div>
      </div>
      <div class="col-auto row items-center no-wrap">
        <q-icon name="zoom_out" size="18px" class="text-grey-6" />
        <q-slider v-model="zoom" :min="0.15" :max="1" :step="0.05" style="width: 110px" class="q-mx-sm" color="primary" />
        <q-icon name="zoom_in" size="18px" class="text-grey-6" />
      </div>
      <div class="col-auto">
        <q-btn flat round dense :icon="showLog ? 'view_sidebar' : 'view_sidebar'" :color="showLog ? 'primary' : undefined" @click="showLog = !showLog">
          <q-tooltip>Activity log</q-tooltip>
        </q-btn>
        <q-btn flat round dense icon="delete_sweep" :disable="!state.labels.length" @click="clearAll">
          <q-tooltip>Clear all labels</q-tooltip>
        </q-btn>
      </div>
    </div>

    <q-banner v-if="s.mode !== 'preview' && !s.previewWhileForwarding" dense class="mode-banner">
      <template #avatar><q-icon name="info" color="primary" /></template>
      Jobs are being {{ s.mode === 'forward' ? 'forwarded' : 'relayed' }}, so labels are not shown here.
      <template #action>
        <q-btn flat dense no-caps color="primary" label="Show them too" @click="updateSettings({ previewWhileForwarding: true }, 0)" />
        <q-btn flat dense no-caps color="primary" label="Switch to preview" @click="updateSettings({ mode: 'preview' }, 0)" />
      </template>
    </q-banner>

    <div class="col row no-wrap" style="min-height: 0">
      <!-- Label wall -->
      <div class="col label-wall-scroll">
        <div v-if="!state.labels.length" class="empty-state column items-center justify-center text-center">
          <div class="empty-art"><q-icon name="receipt_long" size="56px" /></div>
          <div class="text-h6 q-mt-md">Waiting for labels</div>
          <div class="text-body2 text-grey-7 q-mt-xs" style="max-width: 440px">
            <template v-if="state.status.running">
              Print to <code>{{ state.status.address }}</code> from any app or a Zebra ZPL driver, or try the ZPL editor.
            </template>
            <template v-else>
              Start the printer, then send ZPL to <code>127.0.0.1:{{ s.port }}</code> from a Zebra ZPL driver or any app.
            </template>
          </div>
          <div class="row q-gutter-sm q-mt-lg">
            <q-btn v-if="!state.status.running" unelevated no-caps color="primary" icon="play_arrow" label="Start printer" :loading="state.busy" @click="startServer" />
            <q-btn outline no-caps color="primary" icon="code" label="Open ZPL editor" to="/editor" />
          </div>
        </div>

        <div v-else class="label-wall" :style="{ '--zoom': zoom }">
          <LabelCard
            v-for="(l, i) in state.labels"
            :key="l.id"
            :label="l"
            :zoom="zoom"
            :fresh="l.id > freshAfter"
            @open="openViewer(i)"
            @remove="remove(l)"
          />
        </div>
      </div>

      <!-- Activity log -->
      <transition name="slide">
        <div v-if="showLog" class="activity-panel column no-wrap">
          <div class="row items-center q-px-md q-py-sm">
            <div class="text-subtitle2">Activity</div>
            <q-space />
            <q-badge v-if="state.stats.failures" color="negative" :label="`${state.stats.failures} failed`" />
          </div>
          <q-separator />
          <q-scroll-area class="col">
            <div v-if="!state.jobs.length" class="text-caption text-grey-6 q-pa-md">No jobs yet.</div>
            <q-list dense separator>
              <q-item v-for="j in state.jobs" :key="j.id" class="q-py-sm">
                <q-item-section avatar style="min-width: 32px">
                  <q-icon :name="jobIcon(j)" :color="jobColor(j)" size="20px" />
                </q-item-section>
                <q-item-section>
                  <q-item-label class="text-weight-medium">
                    {{ j.labels ? `${j.labels} label${j.labels > 1 ? 's' : ''}` : modeVerb(j.mode) }}
                    <span class="text-grey-6 text-weight-regular"> · {{ fmtBytes(j.bytes) }}</span>
                  </q-item-label>
                  <q-item-label caption class="ellipsis">{{ j.source }}</q-item-label>
                  <q-item-label v-for="d in j.deliveries" :key="d.printerID" caption :class="d.ok ? 'text-positive' : 'text-negative'">
                    <q-icon :name="d.ok ? 'check' : 'close'" /> {{ d.printerName }}<span v-if="!d.ok"> — {{ d.error }}</span><span v-else class="text-grey-6"> · {{ d.millis }} ms</span>
                  </q-item-label>
                  <q-item-label v-if="j.error" caption class="text-warning">{{ j.error }}</q-item-label>
                </q-item-section>
                <q-item-section side top>
                  <q-item-label caption>{{ fmtTime(j.received) }}</q-item-label>
                  <q-item-label v-if="j.renderMs" caption>{{ j.renderMs.toFixed(1) }} ms</q-item-label>
                </q-item-section>
              </q-item>
            </q-list>
          </q-scroll-area>
        </div>
      </transition>
    </div>

    <LabelViewer v-model="viewer.open" v-model:index="viewer.index" :labels="state.labels" />
  </q-page>
</template>

<script setup>
import { computed, reactive, ref, watch, onActivated } from 'vue'
import { useQuasar } from 'quasar'
import LabelCard from '../components/LabelCard.vue'
import LabelViewer from '../components/LabelViewer.vue'
import { api, state, updateSettings, startServer, DPMM_OPTIONS, ROTATIONS, fmtTime, fmtBytes } from '../state'

defineOptions({ name: 'LabelsPage' })

const $q = useQuasar()
const s = computed(() => state.settings)

function readPref (k, d) { try { const v = localStorage.getItem(k); return v === null ? d : JSON.parse(v) } catch { return d } }
function writePref (k, v) { try { localStorage.setItem(k, JSON.stringify(v)) } catch {} }
const zoom = ref(readPref('labels.zoom', 0.45))
const showLog = ref(readPref('labels.showLog', true))
watch(zoom, v => writePref('labels.zoom', v))
watch(showLog, v => writePref('labels.showLog', v))

// Highlight labels that arrived since the page was last shown.
const freshAfter = ref(state.labels[0]?.id ?? 0)
onActivated(() => { setTimeout(() => { freshAfter.value = state.labels[0]?.id ?? 0 }, 2500) })
watch(() => state.labels[0]?.id, () => {
  clearTimeout(freshTimer)
  freshTimer = setTimeout(() => { freshAfter.value = state.labels[0]?.id ?? 0 }, 2500)
})
let freshTimer = null

function num (key, v) {
  const n = parseFloat(v)
  if (n > 0) updateSettings({ [key]: n })
}

const viewer = reactive({ open: false, index: 0 })
function openViewer (i) { viewer.index = i; viewer.open = true }

async function remove (l) {
  await api.RemoveLabel(l.id)
  const i = state.labels.findIndex(x => x.id === l.id)
  if (i >= 0) state.labels.splice(i, 1)
}

function clearAll () {
  $q.dialog({
    title: 'Clear all labels?',
    message: 'Removes every label and the activity log from the emulator. Files already saved to disk are kept.',
    cancel: { flat: true, noCaps: true },
    ok: { label: 'Clear', color: 'negative', unelevated: true, noCaps: true },
    persistent: false
  }).onOk(() => api.ClearLabels())
}

function modeVerb (m) { return { forward: 'Forwarded', relay: 'Relayed' }[m] || 'No labels' }
function jobFailed (j) { return j.error || (j.deliveries || []).some(d => !d.ok) }
function jobIcon (j) { return jobFailed(j) ? 'error_outline' : j.mode === 'preview' ? 'receipt_long' : j.mode === 'forward' ? 'call_made' : 'device_hub' }
function jobColor (j) { return jobFailed(j) ? 'negative' : 'primary' }
</script>
